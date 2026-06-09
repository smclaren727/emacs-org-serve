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
  text): note bodies, Saves (`References/*.org`), Bookmarks. Read via the file
  path from the DB, or directly for files vulpea doesn't index.

## Apps

| App | Shows | Source | Data (2026-06-09) | Status |
|---|---|---|---|---|
| **Contacts** | 169 people; tap to call/email | DB props (`EMAIL*`/`PHONE*`/`COMPANY`) | rich | ✅ built |
| **Notes** | ~285 notes/headlines + body (References **excluded** — they live in Saves; `/api/notes` filters `path NOT LIKE '%/References/%'`) | DB + file slice | full | ✅ built |
| **Bookmarks** | 65 links grouped by category | file parse (`bookmarks.org`) | 65 | ✅ built |
| **Saves** | triaged, LLM-enriched saves (type/title/summary/tags) + X-post media thumbnails, newest-first, search + source filter, inline reader | file parse (`References/*.org`; also vulpea-indexed) | 844 | ✅ built |
| **Tasks** | open TODOs grouped by area/project, with parent-heading context + due dates | DB (`todo`) | 95 (≈no dates) | ✅ built |
| **Reading List** | folded into **Saves** — every capture (incl. web clips) is auto-filed into `References/` by the triage | — | — | ↪ folded into Saves |
| **Today / Agenda** | scheduled / due today | DB (`scheduled`/`deadline`) | ~none (1 / 4) | ⛔ deferred — needs a scheduling habit |
| **Journal** | daily entries, newest first, bodies inline | DB (tag `journal`) + file body | 3 | ✅ built |

Notes on Tasks & Reading List:
- **Tasks** ships as a *grouped task list*, not a calendar — there's almost no
  scheduled/deadline data yet, so an agenda-by-date would be empty. It groups by
  file (`file_title`) in PARA order, surfaces the parent heading as context, and
  shows the few real due dates as badges.
- **Reading List** is folded into **Saves**: the harness triage (see
  `capture-restructure.md`) now enriches every capture into a `References/*.org`
  note that *is* vulpea-indexed. emacs-org-serve only reads those notes
  (`referenceFromFile`); it does not run the triage. Bodies still live in the
  files, so Saves reads them directly.

## Read API

- `GET /api/contacts`
- `GET /api/notes` · `GET /api/note?id=<id>`
- `GET /api/bookmarks`
- `GET /api/saves` · `GET /api/save?id=` · `GET /api/save-media?file=` *(serves X-post thumbnails)*
- `GET /api/journal`
- `GET /api/tasks`

UI: one PWA, tabbed (Contacts / Notes / Bookmarks / Saves / Journal / Tasks), shared `web/app.css`
plus `web/org.js` (the read-only Org→HTML renderer shared by Notes, Journal, and Saves).

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
