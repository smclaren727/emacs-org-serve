// vulpea-serve is a single-binary HTTP service that exposes a personal Org-mode
// notes vault (indexed by Emacs/vulpea into vulpea.db) to iPhone PWAs.
//
// It opens vulpea.db READ-ONLY and serves JSON + an embedded PWA. Writes (later
// milestones) go back through Emacs via emacsclient, never to the DB directly.
package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
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

func dbPath() string {
	return env("VULPEA_DB", os.ExpandEnv("$HOME/.emacs.d/var/vulpea/vulpea.db"))
}

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

func handleContacts(w http.ResponseWriter, r *http.Request) {
	cs, err := contacts()
	if err != nil {
		log.Printf("contacts query: %v", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(cs); err != nil {
		log.Printf("contacts encode: %v", err)
	}
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
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := env("LISTEN", "127.0.0.1:8765")
	log.Printf("vulpea-serve on http://%s  (db=%s, read-only)", addr, dbPath())
	log.Fatal(http.ListenAndServe(addr, mux))
}
