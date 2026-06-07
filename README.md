# emacs-org-serve

A small, single-binary **Go** service that exposes a personal Org-mode notes
vault to **iPhone PWAs** (Add-to-Home-Screen mini apps), by reading the
**SQLite database** that the Emacs package [vulpea](https://github.com/d12frosted/vulpea)
builds from the vault.

> **Status:** built and running. The Contacts, Notes, and Bookmarks read apps
> work, served from `vulpea.db` and deployed on a NixOS node behind Tailscale.
> Start with `AGENTS.md`; `docs/build-plan.md` has the plan it grew from.

## The idea in one paragraph

Org files in `~/All-The-Things/` are the source of truth. Emacs (vulpea) indexes
them into `vulpea.db`. `emacs-org-serve` reads that DB **read-only** and serves JSON
+ an embedded PWA; **writes go back through Emacs** via `emacsclient` calling
whitelisted commands. The phone reaches it over **Tailscale**. One static binary,
deployed on a NixOS node with systemd. See `AGENTS.md` for the architecture
diagram and the rules.

## Why Go

Single static binary → trivial NixOS/systemd deploy; pure-Go SQLite
(`modernc.org/sqlite`, no cgo); `go:embed` ships the whole PWA inside the
binary; boring long-run reliability and a tiny footprint for an always-on
home service. The hard, dynamic work lives in Emacs, so the service stays a
thin, stable read-layer — Go's sweet spot.

## Quickstart

```sh
# Dev: run against the Mac's vulpea database
export VULPEA_DB="$HOME/.emacs.d/var/vulpea/vulpea.db"
go run .                      # serves http://127.0.0.1:8765
# open the PWA in a browser; the first slice is a contacts lookup
```

## Layout

```
emacs-org-serve/
├── AGENTS.md            # agent orientation — read first
├── CLAUDE.md            # @AGENTS.md pointer
├── README.md
├── docs/build-plan.md   # detailed plan, schema, skeleton, milestones
├── go.mod
├── main.go              # server, encoding shim, read queries, write-bridge
└── web/                 # go:embed'd PWA (index.html, manifest.json, sw.js)
```

## Must-know before writing SQL

`vulpea.db` is **emacsql-encoded**: text values are stored double-quoted
(`"contact"`, not `contact`), and the JSON blob columns are double-encoded.
Query the **normalized tables** with **quoted literals**, and decode results.
Details + the `enc`/`dec` shim in `AGENTS.md`.
