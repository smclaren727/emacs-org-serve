# emacs-org-serve — PWA app catalog

Status: active working doc. Created 2026-06-06. Pairs with `build-plan.md`.

The mini-apps `emacs-org-serve` exposes to the iPhone, grounded in the **live**
vault/DB (numbers as of 2026-06-06). Reads are the current focus; writes are
deferred and discussed at the end.

## Read data sources

Apps read from two places — keep this distinction in mind when adding one:

- **`vulpea.db`** (the emacsql index): metadata only (title, tags, properties,
  todo, links, outline). → Contacts, Notes, Tasks.
- **Synced `.org` / `.jsonl` files** under `~/All-The-Things` (the DB has no body
  text): note bodies, Reading List, Bookmarks. Read via the file path from the
  DB, or directly for files vulpea doesn't index.

## Apps

| App | Shows | Source | Data (2026-06-06) | Status |
|---|---|---|---|---|
| **Contacts** | 169 people; tap to call/email | DB props (`EMAIL*`/`PHONE*`/`COMPANY`) | rich | ✅ built |
| **Notes** | all 285 notes/headlines + body | DB + file slice | full | ✅ built |
| **Bookmarks** | 65 links grouped by category | file parse (`bookmarks.org`) | 65 | ✅ built |
| **Tasks** | open TODOs by project/area | DB (`todo`) | 95 (≈no dates) | ▢ planned |
| **Reading List** | saved articles (title/URL/date) → read or open | files (`Read-Later/items/` + `logs/index.jsonl`) | 29 | ▢ planned (flagship) |
| **Today / Agenda** | scheduled / due today | DB (`scheduled`/`deadline`) | ~none (1 / 4) | ⛔ deferred — needs a scheduling habit |
| **Journal** | daily entries | DB / files | 3 | ➖ fold into Today/Notes |

Notes on the deferred ones:
- **Tasks** is a *grouped task list*, not a calendar — there's almost no
  scheduled/deadline data yet, so an agenda-by-date would be empty.
- **Reading List** items are real and well-structured (`:URL:`, `:title:`,
  date, plus a JSONL index) but **not indexed in `vulpea.db`** — it needs the
  file-based read path, or `vulpea-meta` adoption later.

## Read API

- `GET /api/contacts`
- `GET /api/notes` · `GET /api/note?id=<id>`
- `GET /api/bookmarks`
- *(planned)* `GET /api/tasks` · `GET /api/reading`

UI: one PWA, tabbed (Contacts / Notes / Bookmarks …), shared `web/app.css`.

## Writes (deferred — decide per app)

Not built yet. The open question is **feature-level vs service-level**, and the
data suggests it's *mixed*:

- **Notes / Tasks / Contacts → Emacs bridge** (feature-level): Go calls
  `emacsclient -s server --eval "(<whitelisted-cmd> …)"` with quoted args; the
  elisp does the real Org edit; vulpea re-indexes; Syncthing propagates. Daemon
  is up (socket `server`, `org-directory ~/All-The-Things`, `vulpea` +
  `org-capture` loaded).
- **Reading List → already has a service-level writer**: a read-later ingress
  service on the node (an authenticated HTTP endpoint that drops items into the
  vault).
- **Bookmarks → Emacs** (append to `bookmarks.org`) or a small writer.

Safety rule (unchanged): never write the DB or `.org` files directly from Go;
writes go through Emacs or an existing ingress service.
