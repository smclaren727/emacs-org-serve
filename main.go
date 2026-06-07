// emacs-org-serve is a single-binary HTTP service that exposes a personal Org-mode
// notes vault (indexed by Emacs/vulpea into vulpea.db) to iPhone PWAs.
//
// It opens vulpea.db READ-ONLY and serves JSON + an embedded PWA. Note bodies
// are read live from the synced .org files (the DB indexes metadata only, not
// content). Writes (later milestones) go back through Emacs via emacsclient,
// never to the DB or files directly.
package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed web
var webFS embed.FS

var db *sql.DB

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func dbPath() string   { return env("VULPEA_DB", os.ExpandEnv("$HOME/.emacs.d/var/vulpea/vulpea.db")) }
func vaultDir() string { return env("VAULT_DIR", os.ExpandEnv("$HOME/All-The-Things")) }

// vulpea.db is written by emacsql, which stores every value in its Lisp printed
// (prin1) form — strings come out double-quote-wrapped. enc wraps a query param
// the same way; dec unwraps a stored value.
func enc(s string) string { return strconv.Quote(s) }

func dec(s string) string {
	if u, err := strconv.Unquote(s); err == nil {
		return u
	}
	return strings.Trim(s, `"`)
}

func decN(ns sql.NullString) string {
	if ns.Valid {
		return dec(ns.String)
	}
	return ""
}

func nsRaw(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// ---------------------------------------------------------------------------
// Contacts
// ---------------------------------------------------------------------------

// Field is one labeled contact method, e.g. {Label:"Work", Value:"a@b.com"}.
type Field struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Contact is one note tagged `contact`, with its emails/phones bucketed.
type Contact struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Company string  `json:"company,omitempty"`
	Status  string  `json:"status,omitempty"`
	Emails  []Field `json:"emails"`
	Phones  []Field `json:"phones"`
}

// titleCase turns "PHONE_CELL"'s suffix "CELL" into "Cell", "HOME OFFICE" into
// "Home Office", etc.
func titleCase(s string) string {
	parts := strings.Fields(strings.ToLower(s))
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// label derives a display label from a property key. "EMAIL_WORK" -> "Work";
// the bare key "EMAIL" -> base ("Email").
func label(key, prefix, base string) string {
	rest := strings.TrimPrefix(key, prefix+"_")
	if rest == key || rest == "" {
		return base
	}
	return titleCase(strings.ReplaceAll(rest, "_", " "))
}

func contacts() ([]Contact, error) {
	// One pass: every contact note (LEFT JOIN keeps contacts that have no
	// email/phone), with its EMAIL*/PHONE*/COMPANY/STATUS properties. Group in Go.
	const q = `
SELECT n.id, n.title, p.key, p.value
FROM notes n
JOIN tags t ON t.note_id = n.id
LEFT JOIN properties p ON p.note_id = n.id
  AND (p.key LIKE '"EMAIL%' OR p.key LIKE '"PHONE%'
       OR p.key = '"COMPANY"' OR p.key = '"STATUS"')
WHERE t.tag = ?
ORDER BY n.title`

	rows, err := db.Query(q, enc("contact"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := map[string]*Contact{}
	var order []string
	for rows.Next() {
		var id, title string
		var key, val sql.NullString
		if err := rows.Scan(&id, &title, &key, &val); err != nil {
			return nil, err
		}
		id = dec(id)
		c := byID[id]
		if c == nil {
			c = &Contact{ID: id, Name: dec(title), Emails: []Field{}, Phones: []Field{}}
			byID[id] = c
			order = append(order, id)
		}
		if !key.Valid || !val.Valid {
			continue // contact with no matching property (LEFT JOIN NULL row)
		}
		k, v := dec(key.String), dec(val.String)
		switch {
		case k == "COMPANY":
			c.Company = v
		case k == "STATUS":
			c.Status = v
		case strings.HasPrefix(k, "EMAIL"):
			c.Emails = append(c.Emails, Field{label(k, "EMAIL", "Email"), v})
		case strings.HasPrefix(k, "PHONE"):
			c.Phones = append(c.Phones, Field{label(k, "PHONE", "Phone"), v})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Contact, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Notes (read-only browser over the whole vault)
// ---------------------------------------------------------------------------

// NoteMeta is the index entry for a whole-file note (level 0) or headline (>0).
type NoteMeta struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Level     int      `json:"level"`
	File      string   `json:"file"`
	Outline   []string `json:"outline"`
	Tags      []string `json:"tags"`
	Todo      string   `json:"todo,omitempty"`
	Scheduled string   `json:"scheduled,omitempty"`
	Deadline  string   `json:"deadline,omitempty"`
}

// NoteDetail adds the body text, read live from the .org file.
type NoteDetail struct {
	NoteMeta
	Body string `json:"body"`
}

var quotedRe = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

// parseOutline turns an emacsql outline_path list like ("Marketing" "Finance")
// into ["Marketing","Finance"]. Empty / "nil" -> nil.
func parseOutline(raw string) []string {
	if raw == "" || raw == "nil" {
		return nil
	}
	var out []string
	for _, m := range quotedRe.FindAllStringSubmatch(raw, -1) {
		if s, err := strconv.Unquote(`"` + m[1] + `"`); err == nil {
			out = append(out, s)
		} else {
			out = append(out, m[1])
		}
	}
	return out
}

// relPath renders an absolute vault path relative to the vault root for display.
func relPath(p string) string {
	if r, err := filepath.Rel(vaultDir(), p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}

// withinVault guards the body reader against paths outside the vault (the path
// comes from the DB, but defense-in-depth is cheap).
func withinVault(p string) bool {
	v, err := filepath.Abs(vaultDir())
	if err != nil {
		return false
	}
	ap, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	return ap == v || strings.HasPrefix(ap, v+string(os.PathSeparator))
}

// readBody returns the note's text from its .org file. Whole-file notes (level
// 0) return the entire file; a headline returns its subtree (from its line down
// to the next headline of equal-or-shallower depth).
func readBody(path string, level, pos int) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	runes := []rune(string(b)) // pos is a character offset, not a byte offset
	start := pos - 1
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	seg := string(runes[start:])
	if level <= 0 {
		return seg, nil
	}
	stop := regexp.MustCompile(fmt.Sprintf(`^\*{1,%d} `, level))
	lines := strings.Split(seg, "\n")
	end := len(lines)
	for i := 1; i < len(lines); i++ {
		if stop.MatchString(lines[i]) {
			end = i
			break
		}
	}
	return strings.Join(lines[:end], "\n"), nil
}

func notesList() ([]NoteMeta, error) {
	rows, err := db.Query(`
SELECT id, title, level, outline_path, todo, scheduled, deadline, path
FROM notes ORDER BY path, pos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []NoteMeta
	idx := map[string]int{} // raw (encoded) id -> index in list
	for rows.Next() {
		var rawID, title, path string
		var level int
		var outline, todo, sched, dead sql.NullString
		if err := rows.Scan(&rawID, &title, &level, &outline, &todo, &sched, &dead, &path); err != nil {
			return nil, err
		}
		idx[rawID] = len(list)
		list = append(list, NoteMeta{
			ID: dec(rawID), Title: dec(title), Level: level,
			File: relPath(dec(path)), Outline: parseOutline(nsRaw(outline)),
			Tags: []string{}, Todo: decN(todo), Scheduled: decN(sched), Deadline: decN(dead),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tagRows, err := db.Query(`SELECT note_id, tag FROM tags`)
	if err != nil {
		return nil, err
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var rawID, tag string
		if err := tagRows.Scan(&rawID, &tag); err != nil {
			return nil, err
		}
		if i, ok := idx[rawID]; ok {
			list[i].Tags = append(list[i].Tags, dec(tag))
		}
	}
	return list, tagRows.Err()
}

func noteByID(id string) (*NoteDetail, error) {
	var title, path string
	var level, pos int
	var outline, todo, sched, dead sql.NullString
	err := db.QueryRow(`
SELECT title, level, pos, path, outline_path, todo, scheduled, deadline
FROM notes WHERE id = ?`, enc(id)).Scan(&title, &level, &pos, &path, &outline, &todo, &sched, &dead)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p := dec(path)
	if !withinVault(p) {
		return nil, fmt.Errorf("note path outside vault: %s", p)
	}
	body, err := readBody(p, level, pos)
	if err != nil {
		return nil, err
	}
	nd := &NoteDetail{
		NoteMeta: NoteMeta{
			ID: id, Title: dec(title), Level: level, File: relPath(p),
			Outline: parseOutline(nsRaw(outline)), Tags: []string{},
			Todo: decN(todo), Scheduled: decN(sched), Deadline: decN(dead),
		},
		Body: body,
	}
	tagRows, err := db.Query(`SELECT tag FROM tags WHERE note_id = ?`, enc(id))
	if err != nil {
		return nil, err
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var tag string
		if err := tagRows.Scan(&tag); err != nil {
			return nil, err
		}
		nd.Tags = append(nd.Tags, dec(tag))
	}
	return nd, tagRows.Err()
}

// ---------------------------------------------------------------------------
// Bookmarks (parsed from bookmarks.org — not a vulpea note, so read from file)
// ---------------------------------------------------------------------------

// Bookmark is one [[url][title]] headline under a category path.
type Bookmark struct {
	URL      string   `json:"url"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Created  string   `json:"created,omitempty"`
}

func bookmarksFile() string {
	return env("BOOKMARKS_FILE", filepath.Join(vaultDir(), "50-Resources", "bookmarks.org"))
}

var bmHead = regexp.MustCompile(`^(\*+)\s+(.*)$`)
var bmLink = regexp.MustCompile(`\[\[([^\]]+)\](?:\[([^\]]*)\])?\]`)

func bookmarks() ([]Bookmark, error) {
	f := bookmarksFile()
	if !withinVault(f) {
		return nil, fmt.Errorf("bookmarks file outside vault: %s", f)
	}
	data, err := os.ReadFile(f)
	if err != nil {
		if os.IsNotExist(err) {
			return []Bookmark{}, nil
		}
		return nil, err
	}
	lines := strings.Split(string(data), "\n")

	type cat struct {
		level int
		name  string
	}
	var stack []cat
	out := []Bookmark{}
	for i, line := range lines {
		m := bmHead.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		level, title := len(m[1]), strings.TrimSpace(m[2])
		for len(stack) > 0 && stack[len(stack)-1].level >= level {
			stack = stack[:len(stack)-1] // pop siblings/deeper
		}
		lk := bmLink.FindStringSubmatch(title)
		if lk == nil || !strings.HasPrefix(title, "[[") { // plain heading = category
			stack = append(stack, cat{level, title})
			continue
		}
		desc := strings.TrimSpace(lk[2])
		if desc == "" {
			desc = strings.TrimSpace(lk[1])
		}
		names := make([]string, len(stack))
		for j, c := range stack {
			names[j] = c.name
		}
		bm := Bookmark{URL: strings.TrimSpace(lk[1]), Title: desc, Category: strings.Join(names, " › "), Tags: []string{}}
		for j := i + 1; j < len(lines); j++ { // adjacent PROPERTIES drawer
			ls := strings.TrimSpace(lines[j])
			switch {
			case ls == ":PROPERTIES:" || ls == "":
				continue
			case ls == ":END:":
				j = len(lines)
			case strings.HasPrefix(ls, ":CREATED:"):
				bm.Created = strings.Trim(strings.TrimSpace(strings.TrimPrefix(ls, ":CREATED:")), "[]")
			case strings.HasPrefix(ls, ":TAGS:"):
				for _, t := range strings.Split(strings.TrimPrefix(ls, ":TAGS:"), ",") {
					if t = strings.TrimSpace(t); t != "" {
						bm.Tags = append(bm.Tags, t)
					}
				}
			default:
				j = len(lines) // left the drawer
			}
			if j >= len(lines) {
				break
			}
		}
		out = append(out, bm)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode: %v", err)
	}
}

func handleContacts(w http.ResponseWriter, r *http.Request) {
	cs, err := contacts()
	if err != nil {
		log.Printf("contacts query: %v", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, cs)
}

func handleNotes(w http.ResponseWriter, r *http.Request) {
	ns, err := notesList()
	if err != nil {
		log.Printf("notes query: %v", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, ns)
}

func handleNote(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	nd, err := noteByID(id)
	if err != nil {
		log.Printf("note %s: %v", id, err)
		http.Error(w, "read failed", http.StatusInternalServerError)
		return
	}
	if nd == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, nd)
}

func handleBookmarks(w http.ResponseWriter, r *http.Request) {
	bs, err := bookmarks()
	if err != nil {
		log.Printf("bookmarks: %v", err)
		http.Error(w, "read failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, bs)
}

func main() {
	var err error
	dsn := "file:" + dbPath() + "?mode=ro&_pragma=busy_timeout(5000)"
	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("open db %s: %v", dbPath(), err)
	}

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/contacts", handleContacts)
	mux.HandleFunc("/api/notes", handleNotes)
	mux.HandleFunc("/api/note", handleNote)
	mux.HandleFunc("/api/bookmarks", handleBookmarks)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := env("LISTEN", "127.0.0.1:8765")
	log.Printf("emacs-org-serve on http://%s  (db=%s, vault=%s, read-only)", addr, dbPath(), vaultDir())
	log.Fatal(http.ListenAndServe(addr, mux))
}
