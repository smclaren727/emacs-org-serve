# AGENTS.md — vulpea-serve

Context for coding agents. Read this first; then `docs/build-plan.md` for the
detailed plan and skeleton. This file is the source of truth for the
must-know facts and rules.

## What this is

`vulpea-serve` is a small, single-binary **Go** HTTP service that exposes a
personal Org-mode notes vault to **iPhone PWAs** (Add-to-Home-Screen mini
apps). It is **Phase 2** of a larger project; Phase 1 (the Emacs side) is
already done.

- The notes live in `~/All-The-Things/` as Org files (PARA layout). They are
  the **source of truth**.
- An Emacs package called **vulpea** (v2, its own SQLite DB) indexes those
  files into **`vulpea.db`** (SQLite). That database is this service's input.
- `vulpea-serve` **reads `vulpea.db` directly** (read-only) and serves JSON +
  an embedded PWA. **Writes go back through Emacs** (see Safety rules).

## Architecture

```
 iPhone (Safari → Add to Home Screen = PWA)
      │  HTTPS over Tailscale (or SSH tunnel); service binds localhost/tailnet only
      ▼
 ┌──────────────────────── nix-node (NixOS, always-on) ────────────────────────┐
 │  vulpea-serve (this repo)                                                    │
 │     ├── READ  ──►  vulpea.db (SQLite, read-only)      ← fast, Emacs-independent│
 │     └── WRITE ──►  emacsclient --eval "(whitelisted-cmd …)"                   │
 │                         │                                                     │
 │                         ▼                                                     │
 │                    emacs --daemon (vulpea) ──► *.org files (source of truth)  │
 └──────────────────────────────────────────────────────────────────────────────┘
        ▲ Syncthing (bidirectional) syncs ~/All-The-Things between Mac and node
```

- The node's Emacs daemon indexes the Syncthing-synced vault → a **local**
  `vulpea.db` that `vulpea-serve` reads.
- Writes from a PWA → `vulpea-serve` → `emacsclient` → node Emacs edits the Org
  file → vulpea re-indexes → Syncthing propagates the change back to the Mac.
- **Dev mode (before the node exists):** run against the Mac's dev DB at
  `~/.emacs.d/var/vulpea/vulpea.db` and a local Emacs daemon for the bridge.

## ⚠️ Critical: `vulpea.db` is emacsql-encoded

`vulpea.db` is written by **emacsql**, which stores every value in its Lisp
**printed (prin1) form**. This is the single biggest gotcha — verified against
the live DB:

- Text is **double-quote-wrapped**: `tags.tag` is `"contact"` (6 chars incl.
  quotes), `notes.title` is `"Buy Changes - Denim"`, `properties.value` is
  `"a@b.com"`.
- So `WHERE tag = 'contact'` returns **0 rows**; `WHERE tag = '"contact"'`
  returns 167. You must **encode query params** (wrap in quotes) and **decode
  results** (unquote).
- The JSON blob columns on `notes` (`tags`, `meta`, `links`, `properties`,
  `aliases`) are **double-encoded** (prin1 of a JSON string; empty shows as
  `"null"`). **Do not parse the blob columns.** Use the **normalized tables**
  (`tags`, `meta`, `properties`, `links`) instead.
- In Go: `enc(s) = strconv.Quote(s)` for params; `dec(s) = strconv.Unquote(s)`
  (fallback: trim `"`) for results. (Edge cases: non-ASCII / control chars —
  elisp print syntax ≈ Go/JSON string syntax for ASCII; handle exotic cases if
  they appear.)

Always verify a query against the real DB with `sqlite3` before trusting it.

## Schema (verified, vulpea schema v3)

Backend: `emacsql-sqlite-builtin`. Identifiers use **underscores**.

| Table | Columns |
|---|---|
| `notes` | `id`, `path`, `level`, `pos`, `title`, `properties`, `tags`, `aliases`, `meta`, `links`, `todo`, `priority`, `scheduled`, `deadline`, `closed`, `outline_path`, `attach_dir`, `file_title`, `created_at`, `modified_at` |
| `tags` | `note_id`, `tag` |
| `meta` | `note_id`, `key`, `value` (EAV — vulpea-meta key/values) |
| `properties` | `note_id`, `key`, `value` (Org PROPERTY drawer entries) |
| `links` | `source`, `dest`, `type`, `pos`, `description` |
| `files` | `path`, `hash`, `mtime`, `size` (change-detection ledger) |
| `schema_registry` | `name`, `version`, `created_at` |

Query the normalized tables and join to `notes` for `id`/`title`/`path`. All
text values are emacsql-encoded — `enc`/`dec` accordingly.

## Data reality (as of 2026-06-05, dev DB)

- **280 notes** from 253 files.
- **167 notes tagged `contact`** with `EMAIL`/`EMAIL_WORK`/`EMAIL_HOME`/
  `EMAIL_OTHER`, `PHONE`/`PHONE_CELL`/…, `COMPANY`, etc. in `properties`.
- The **`meta` table is empty** — vulpea-meta has not been adopted yet.

**First slice = contacts** (real data, demonstrable on day one). The
reading-queue slice (the eventual flagship) depends on the user adopting
`vulpea-meta` (`status :: queued` etc.) first, so it is deferred.

## Safety rules (do not violate)

1. **`vulpea.db` is a derived read-replica.** Org files are the source of
   truth. Never write to the DB. Open it **read-only** (`?mode=ro`).
2. **Writes go only through Emacs**, via `emacsclient --eval` calling
   **named, whitelisted commands** (e.g. `vulpea-create`, `vulpea-meta-set`,
   `org-capture` wrappers). **Never** pass raw user input as elisp, and never
   expose a general eval endpoint — that is remote code execution.
3. **Bind to localhost / the tailnet only.** Reach it from the phone over
   Tailscale or an SSH tunnel. Do not expose it publicly.
4. Coexist with Emacs's writer: open with `_pragma=busy_timeout(5000)` and
   retry on `SQLITE_BUSY`.

## Build conventions

- **Go**, standard library first (`net/http`, `database/sql`,
  `encoding/json`), plus **`modernc.org/sqlite`** (pure-Go, no cgo).
- **One static binary.** Embed the entire PWA with `go:embed` so the binary
  ships the app. Cross-compile for the node (`GOOS=linux`).
- Deploy on NixOS via `buildGoModule` + a `systemd` service (see build-plan).
- Keep the service **thin**: query → JSON, serve static, forward writes. The
  hard/dynamic logic (correct Org writing, metadata semantics) lives in Emacs.

## How to verify against reality

```sh
DB=~/.emacs.d/var/vulpea/vulpea.db
sqlite3 "$DB" '.tables'
sqlite3 "$DB" "select count(*) from tags where tag='\"contact\"';"   # -> 167
sqlite3 "$DB" "select value from properties where key='\"EMAIL\"' limit 3;"
```

Prefer checking the DB (and, for behavior questions, the live Emacs via
`emacsclient --eval`) over assuming.
