# vulpea-serve — Build Plan

Status: active working doc. Created 2026-06-05.

Self-contained plan for building `vulpea-serve`. Pairs with `AGENTS.md` (the
must-know facts and rules). Read `AGENTS.md` first.

## Goal

Expose the Org notes vault to iPhone PWAs. One static Go binary on a NixOS node
reads `vulpea.db` (read-only) and serves JSON + an embedded PWA; writes go back
through Emacs. Reached from the phone over Tailscale.

## The larger arc (where this fits)

- **Phase 1 — DONE (in the Emacs config repo).** Migrated the notes engine to
  vulpea (own SQLite DB), added the `[[` link capf, contact-email helper, a
  live sidebar (`vulpea-ui`), and daily journaling (`vulpea-journal`).
- **Phase 2 — THIS REPO.** The serve layer: Go service reading `vulpea.db`,
  write-bridge through Emacs, Tailscale access, NixOS deploy.
- **Phase 3.** More PWAs: contacts (first), then bookmarks, capture, today,
  and the flagship **reading queue** (once `vulpea-meta` is in use).

## Architecture

See the diagram in `AGENTS.md`. Key properties:

- **Reads** are direct SQLite, **Emacs-independent** — the PWA keeps working
  read-only even if Emacs is down.
- **Writes** are forwarded to Emacs (`emacsclient --eval` → whitelisted
  command) so Org structure and metadata stay correct; the DB re-syncs after.
- The vault reaches the node via **Syncthing** (bidirectional, **active** since
  2026-06-06: Mac ↔ `loxley:/srv/loxley/All-The-Things` over Tailscale). The
  node's Emacs daemon indexes it into a local `vulpea.db`.
- For development, run against the Mac dev DB:
  `~/.emacs.d/var/vulpea/vulpea.db` (set `VULPEA_DB`).

## The emacsql encoding (read `AGENTS.md` for the full story)

`vulpea.db` stores values in Lisp printed form. Text is double-quoted. Query
the **normalized tables** with **encoded params** and **decode** results:

```go
func enc(s string) string { return strconv.Quote(s) }              // "contact" -> "\"contact\""
func dec(s string) string { if u, err := strconv.Unquote(s); err == nil { return u }; return strings.Trim(s, `"`) }
```

Do **not** parse the `notes` JSON blob columns (double-encoded). Join the
normalized tables instead.

## Schema (verified)

| Table | Columns |
|---|---|
| `notes` | `id, path, level, pos, title, properties, tags, aliases, meta, links, todo, priority, scheduled, deadline, closed, outline_path, attach_dir, file_title, created_at, modified_at` |
| `tags` | `note_id, tag` |
| `meta` | `note_id, key, value` |
| `properties` | `note_id, key, value` |
| `links` | `source, dest, type, pos, description` |
| `files` | `path, hash, mtime, size` |
| `schema_registry` | `name, version, created_at` |

Example (the contacts query, accounting for encoding):

```sql
-- everyone tagged contact, with a work email
SELECT n.id, n.title, p.value
FROM notes n
JOIN tags t       ON t.note_id = n.id
JOIN properties p ON p.note_id = n.id
WHERE t.tag = '"contact"' AND p.key = '"EMAIL_WORK"';
```

## Milestones

### M1 — Contacts slice, end-to-end (the proof-of-loop)

Uses data that exists today (167 contact notes; the `meta` table is empty).

- [ ] `go.mod` + `main.go`: open `vulpea.db` read-only; `enc`/`dec` shim.
- [ ] `GET /api/contacts` → JSON: for each note tagged `contact`, return name
      (`notes.title`) + emails/phones (from `properties`, keys `EMAIL*` /
      `PHONE*`). Decode all values.
- [ ] `web/`: a one-page PWA that fetches `/api/contacts`, renders searchable
      cards (tap to call/email), with `manifest.json` (`display: standalone`)
      and a minimal `sw.js` for offline cache.
- [ ] Run locally (`VULPEA_DB=…/vulpea.db go run .`), open in a browser,
      verify against `sqlite3` counts.

### M2 — Write-bridge

- [ ] `emacsCall(fn, args…)` → `emacsclient -s <socket> --eval "(fn "a" …)"`,
      args quoted. Whitelist a fixed set of commands.
- [ ] First write path: a "quick capture" endpoint → an Emacs capture command
      (define the elisp command on the Emacs side; keep the Go side dumb).
- [ ] Verify the round-trip: POST → Org file changes → DB re-indexes → the new
      data appears in a read query.

### M3 — Package & deploy on the node

- [ ] `GOOS=linux` cross-compile; `buildGoModule` derivation.
- [ ] `systemd` service (sketch below): runs the binary, `After=` the Emacs
      daemon, `VULPEA_DB` + `EMACS_SOCKET` env, bound to localhost/tailnet.
- [ ] Tailscale: reach it from the phone; Add-to-Home-Screen.
- [ ] Confirm Syncthing round-trip (write on phone → Org file on node → synced
      back to the Mac).

### M4+ — More slices

Bookmarks, today/journal, and the **reading queue** (needs `vulpea-meta`
adoption: reading items tagged with `status :: queued`, queried from the
`meta` table).

## Skeleton (starting point — flesh out in M1)

```go
// go.mod
module vulpea-serve

go 1.23

require modernc.org/sqlite v1.x   // pure-Go SQLite, no cgo
```

```go
// main.go (skeleton — error handling abbreviated)
package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed web
var webFS embed.FS

var db *sql.DB

func env(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }
func dbPath() string           { return env("VULPEA_DB", os.ExpandEnv("$HOME/.emacs.d/var/vulpea/vulpea.db")) }

// emacsql stores Lisp printed form; strings are double-quoted.
func enc(s string) string { return strconv.Quote(s) }
func dec(s string) string { if u, err := strconv.Unquote(s); err == nil { return u }; return strings.Trim(s, `"`) }

type Contact struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Emails map[string]string `json:"emails"`
	Phones map[string]string `json:"phones"`
}

func contacts() ([]Contact, error) {
	// JOIN notes/tags/properties WHERE t.tag = enc("contact");
	// group rows by note id, bucket EMAIL*/PHONE* keys, dec() every value.
	// (implement in M1)
	return nil, nil
}

// Writes go ONLY through Emacs, and ONLY via whitelisted commands.
func emacsCall(fn string, args ...string) error {
	q := make([]string, len(args))
	for i, a := range args { q[i] = strconv.Quote(a) }
	form := "(" + fn + " " + strings.Join(q, " ") + ")"
	return exec.Command("emacsclient", "-s", env("EMACS_SOCKET", "server"), "--eval", form).Run()
}

func jsonAPI[T any](fn func() ([]T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := fn()
		if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
}

func main() {
	var err error
	db, err = sql.Open("sqlite", "file:"+dbPath()+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil { log.Fatal(err) }
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/contacts", jsonAPI(contacts))
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := env("LISTEN", "127.0.0.1:8765")
	log.Printf("vulpea-serve on %s (db=%s)", addr, dbPath())
	log.Fatal(http.ListenAndServe(addr, mux))
}
```

```html
<!-- web/index.html (skeleton) -->
<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="manifest" href="/manifest.json"><title>Contacts</title></head>
<body><input id="q" placeholder="Search…"><ul id="list"></ul>
<script>
fetch('/api/contacts').then(r=>r.json()).then(cs=>{/* render + filter */});
if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js');
</script></body></html>
```

```json
// web/manifest.json
{ "name": "Notes", "short_name": "Notes", "start_url": "/",
  "display": "standalone", "background_color": "#1f2430", "theme_color": "#1f2430" }
```

## NixOS deploy (sketch for M3)

> Deploy target: the **`nix-loxley-node`** flake (`/Users/seanmclaren/Developer/nix-loxley-node`),
> synced to the node at `/srv/loxley/nixos` and applied with
> `nixos-rebuild switch --flake path:/srv/loxley/nixos#loxley` (snapper-snapshot
> `root` + `srv-loxley` first). Run as the **`loxley`** user (home `/srv/loxley`);
> model on the existing `read-later-ingress.nix`. The sketch below predates node
> specifics — use `after = [ "loxley-emacs.service" ]`, drop `DynamicUser`, and
> point `VULPEA_DB` under `/srv/loxley`.

```nix
# package
vulpea-serve = pkgs.buildGoModule {
  pname = "vulpea-serve"; version = "0.1.0";
  src = ./.; vendorHash = null; # set after `go mod tidy`
};

# service
systemd.services.vulpea-serve = {
  description = "vulpea-serve PWA backend";
  after = [ "emacs.service" ]; wantedBy = [ "multi-user.target" ];
  environment = {
    VULPEA_DB = "/path/on/node/vulpea.db";
    EMACS_SOCKET = "server";
    LISTEN = "127.0.0.1:8765";   # Tailscale fronts it; do not expose publicly
  };
  serviceConfig = { ExecStart = "${vulpea-serve}/bin/vulpea-serve"; Restart = "on-failure"; DynamicUser = true; };
};
```

## Dependencies / open items

- **Syncthing** to the node is **active** (Mac ↔ `loxley:/srv/loxley/All-The-Things`,
  bidirectional over Tailscale). Devices/folders are managed at runtime via the
  Syncthing REST API — the `nix-loxley-node` config sets `overrideDevices/Folders
  = false` by design, so the pairing persists across rebuilds.
- The node's Emacs must run **vulpea** (installed via Nix, since
  `use-package-always-ensure` is off on that host) and index the synced vault.
- Decide the node's `vulpea.db` path + the Emacs daemon **socket name** for the
  bridge.
- **Reading-queue slice** depends on adopting `vulpea-meta` in the capture
  workflow (the `meta` table is empty today).
- Define the **whitelisted Emacs write commands** (capture, set-meta) on the
  Emacs side before wiring M2 writes.
