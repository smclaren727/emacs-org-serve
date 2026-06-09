# Capture Restructure + Auto-File (Tier 2) — migration plan

Status: active plan. Created 2026-06-08. The detailed plan for the capture →
References → Knowledge restructure and the auto-file (triage) loop. This is the
"Tier 2" work referenced in `pwa-apps.md`; it spans four repos (see Touch map).

## Goal

Turn the fragmented, tool-named capture pipelines into one clean flow:

```
capture (bookmarklet / read-later ingress / Field Theory)
   → raw ingest (plumbing, by source)
   → triage (fetch + LLM categorize/summarize/link, at capture time)
   → one flat, enriched .org store you consult/query/link
```

Driving decisions (settled in design discussion 2026-06-08):
- **Organize by role (folders) + everything-else by metadata.** No per-medium
  content folders (no `X-Posts/` / `Videos-And-Transcripts/` folders); medium is
  a `:TYPE:` property. This matches Hermes Link Curator (flat store + `Type`
  field + metadata-computed views), read-later apps, Zettelkasten, and PARA's own
  "organize by actionability, not kind."
- **The graph is link-based, folder-agnostic** (org-roam/vulpea) — placement is a
  semantic-role choice, not a graph-capability one.
- **Three roles, three homes:** References (consult/query/link), Resources
  (active support), Knowledge (your synthesis).
- "X-Bookmarks" retires as a name everywhere — an X bookmark just flags a
  Post/Thread to save; the content is an X-Post (a `:TYPE:`, not a folder).
- `bookmarks.org` is a utility **launcher** (sites you revisit to act/find) — it
  stays in Resources with its own Bookmarks tab and never enters this pipeline.

## Target structure

```
00-Capture/
  inbox.org, drafts.org, journal.org, loose triage notes        (unchanged)
  Ingest/                       raw capture plumbing (never browsed by hand)
    Saved-Links/                unified web-clip queue+snapshots
                                  (bookmarklet #3 + read-later ingress)
    X-Posts/                    Field Theory raw sync: sqlite db, jsonl, media
References/                   NEW top-level: flat, enriched .org saved content
    <kebab-slug>.org              one note per saved item (schema below)
40-Knowledge/                    your synthesized notes (knowledge.org + future)
50-Resources/                    active support: Contacts/, bookmarks.org,
                                  feeds.org, email-notes.org
journal/                         (unchanged)
10-Projects/ 20-Areas/ 30-Interests/ 60-Archive/ 90-Harness/      (unchanged)
```

`References` slots between Knowledge (40) and Resources (50), grouping the
"information you keep" cluster. Number is adjustable (open item).

## Canonical References note schema (draft — finalize before Phase 3)

One `.org` file per saved item, vulpea-indexed, link-ready:

```org
:PROPERTIES:
:ID:            <uuid>
:TYPE:          article | video | image | product | x-post | thread | other
:SOURCE:        save-link | field-theory | read-later
:URL:           https://...
:CANONICAL_URL: https://...
:STATUS:        triaged          ; queued | triaged | review | failed
:CAPTURED:      [2026-06-08 ...]
:TRIAGED:       [2026-06-08 ...]
:END:
#+title: <title>
#+filetags: :topic_a:topic_b:

<one-paragraph summary>

<key points / excerpt / transcript>

* Links
- [[id:...]]  entity / topic notes the triage step resolved
```

Notes:
- Reference content is mostly *not* CGME/SBPF (those tag families are for
  actionable items, per `digital-life-os.org`). Triage adds type/topic/summary/
  links; it does **not** force `core/growth/...` onto a saved tweet.
- X-post specifics (author, likes, reposts, thread) are extra optional properties
  on the same note type — not a separate folder/type system.

## Triage + autonomy model (the three decisions)

1. **One front door.** Every source (Field Theory, the unified Saved-Links queue,
   the read-later ingress) feeds ONE triage processor that writes the canonical
   `References` note. Tool dumps are pure feeders.
2. **Auto-file by default, escalate the exceptions.** Placement is almost always
   `References` (low-stakes, high-confidence), so auto-file the bulk. Route to
   review (harness shadow→approve→Telegram) only when an item *smells actionable*
   (could be a Project/Area/Interest) or model confidence is low.
3. **Conservative refile.** Default destination is `References`. Do NOT
   auto-scatter into Projects/Areas/Interests; surface "could be a project/
   interest" as a human-promotion *suggestion* (matches "Capture is intake, not a
   final home" and "Interests can become projects later").

Provider: node-local **Codex CLI primary, Claude CLI fallback** (already the
harness's live route). Triage generalizes the existing
`evaluate-external-capability` workflow.

## Phased plan (non-breaking if ordered as below)

### Phase 1 — Plan + schema  ← this doc
- Write this plan. Finalize the open items (note schema, `45-` number, confidence
  threshold) before Phase 3. No live changes; `all-the-things.org` is updated as
  the live tree changes (the map must mirror reality, not a future layout).

### Phase 2 — Relocate plumbing (live migration; do in two halves)
Move raw ingest into `00-Capture/Ingest/`, unify the two web-clip queues, repoint
every reader/writer, keep everything working. Snapshot first.

- **2a — Field Theory / X-Posts.** ✓ DONE 2026-06-08 (node `82befb5`): `X-Bookmarks`
  → `00-Capture/Ingest/X-Posts`, service → `loxley-x-posts-sync`, serve `X_BOOKMARKS_DIR`
  repointed; `/api/saves` unchanged at 840. Original steps: field-theory.nix: `FT_DATA_DIR` →
  `Ingest/X-Posts`; rename service `loxley-x-bookmarks-sync` → `loxley-x-posts-sync`;
  keep `ft md` for now (Saves still reads markdown until Phase 5); tmpfiles; move
  existing `bookmarks/` data. Repoint emacs-org-serve `X_BOOKMARKS_DIR`. Redeploy
  node + serve. Verify daily sync + Saves X-posts.
- **2b — Saved-Links.** ✓ DONE 2026-06-08 (node `f717f71`): `Save-Link` →
  `00-Capture/Ingest/Saved-Links`, read-later ingress unified to the same root, serve
  `SAVE_LINK_DIR` repointed, Mac Emacs daemon hot-updated, empty `Read-Later/` retired;
  `/api/saves` unchanged at 840. NOTE: `my-emacs` `my-save-link.el` one-liner left
  **uncommitted** for the user to review/commit. Original steps: read-later-ingress.nix `readLaterRoot` →
  `Ingest/Saved-Links`; `MY_SAVE_LINK_ROOT` (node Emacs + Mac) → `Ingest/Saved-Links`;
  repoint bookmarklet #3 handler target; move existing `Save-Link/items` +
  `Read-Later/*`. Repoint emacs-org-serve `SAVE_LINK_DIR`. Redeploy. Verify
  ingress + Emacs capture + Saves clips.
- Update `all-the-things.org` (Ingest exists; create empty `References/`).

Bookmarklets #1 (bookmarks.org) and #2 (feeds.org) are untouched.

### Phase 3 — Triage workflow (Emacs-Harness)
`triage-capture`: poll `Ingest/` → enrich via Codex → write a `References/` note
(schema above) → **auto-file everything, no review** (locked). Lives in
`All-The-Things/90-Harness/{Skills,Workflows}/`.

**Scoping (2026-06-08):** harness already has the workflow/skill Org format with
poll-autonomy metadata (`#+AUTONOMY_*`), the codex-cli provider, `fetch-url`, and
`eh-import-staging-render-reference-note`; `evaluate-external-capability` is the
near-template. **X-Posts need NO fetch** — full content is in the FT
`bookmarks.jsonl` (text/author/engagement/media/links/tags); Saved-Links need a
fetch. **Write-mechanism fork (needs user):** action surface
(`append-note`/`append-draft`/`create-task`/…) has no clean "create one new
standalone `.org` per save in `References/`", and the reference-note renderer
STAGES for review — so auto-write needs either **(A)** a new guarded `create-note`
action (Emacs-Harness core elisp + policy.json) or **(B)** a dedicated
`references-create-note` elisp writer exposed as a harness skill (lighter; rec).
Constraints: `fetch-url` policy-capped at 8000 output chars; node Emacs daemon NOT
reachable via plain `emacsclient` (harness drives it through its own bridge) → I
can't unit-test the write path; the harness's own execution is the test loop.

**Update (2026-06-08) — correction + decision + progress.** The daemon IS reachable:
`emacsclient -s /srv/loxley/.emacs.d/var/server/loxley` (custom socket path) — so I can
test live (earlier "can't test" was wrong). **Lean-triage-first chosen** (approach 2):
build a harness-compatible `Emacs-Harness/Lisp/references-triage.el` on the existing
`loxley` daemon (vulpea-create + node `codex` + a timer), then re-provision the full
Emacs-Harness as the next phase and migrate the triage in. The harness `fetch-url` cap is
moot for the lean path (we use `codex`/our own fetch). ✅ **Writer done + verified** —
`references-create-note` writes a schema-correct note to `References/` AND vulpea indexes
it; the file-level `:PROPERTIES:`/`:ID:` drawer MUST precede `#+title` (else vulpea ignores
the node — the same reason Save-Link `.org` items aren't in `vulpea.db`). NEXT: codex
enrichment + per-source runner (X-Posts `bookmarks.jsonl`; Saved-Links items/queue) +
processed-state tracking + a systemd timer + backfill of the ~840 existing captures.

✅ **Enrichment + X-Posts runner DONE + verified (2026-06-08).** Enrichment via **local
ollama `llama3.2:3b`** — chosen over codex for the bulk backfill (free/local/clean, no
agent-sandbox friction; `codex --dangerously-bypass...` is also classifier-blocked). Codex
stays a fallback for hard cases. `references-triage-run-x-posts` reads
`X-Posts/bookmarks/bookmarks.jsonl` → ollama (type/title/summary/tags) → indexed
`References/` note, ~9s/item warm, idempotent via `References/.triage-state.json`. Test
filed 2 real notes correctly. NEXT: Saved-Links runner (URL → fetch readable text →
enrich), then a systemd timer + `loxley.nix` wiring (load the module, run the timer, map
`References/` in all-the-things.org), then the throttled backfill.

✅ **Saved-Links runner + driver DONE + verified (2026-06-08).** `references-triage-run-saved-links`
parses `items/*.org` → title (+ inline readable snapshot when present) → enrich → indexed note
(classified `article`). `references-triage-run` drives both sources. 4 real notes filed so far
(2 x-posts + 2 save-links). Module `Emacs-Harness/Lisp/references-triage.el` is complete (writer +
ollama enrichment + state + both runners + driver). NEXT: a systemd timer + `loxley.nix` wiring
(add an `emacs-harness` flake input → load the module into the `loxley` daemon → timer runs
`references-triage-run` → ensure `References/`), update `all-the-things.org`, then the throttled
backfill of the ~835 remaining captures. v2: readability content-fetch for metadata-only web links.

✅ **TRIAGE WIRED + LIVE + AUTONOMOUS (2026-06-08, node `9f54ae7`).** Added an `emacs-harness`
flake input via **`git+ssh://`** (Emacs-Harness is PRIVATE — `github:` 404s; the node has SSH
access). `loxley.nix`: a `loxley-triage-run` wrapper that `emacsclient --eval`s
`(load references-triage.el)(references-triage-run :max 15)` against the `loxley` daemon, a
`references-triage.service` (oneshot) + `.timer` (every 30m), and a `References/` tmpfile. Live
service run: `Result=success`, 30 notes filed in 4.5 min; timer registered. 34 References notes
total, vulpea-indexed. Backfill of the remaining ~800 kicked off as a node background loop
(`/srv/loxley/.local/state/triage-backfill.log`). NEXT: confirm backfill completes, update
`all-the-things.org` (References + Ingest), then Phase 5 (repoint Saves reader to `References/`).

✅ **ALL PHASES ESSENTIALLY DONE (2026-06-09).** Backfill complete: **840/840** captures
auto-filed into `References/` as codex (gpt-5.5) enriched, vulpea-indexed notes (zero dup URLs;
types: 811 x-post / 15 article / 7 tool / 5 product / 1 video / 1 other). Steady-state timer
re-enabled on the fixed module (Emacs-Harness `2e0af6e`: codex `timeout 75` + per-item state
save). `all-the-things.org` map updated. **Phase 5** (Saves tab → `References/` via a new
`referenceFromFile` reader; `saves()`/`handleSave` repointed; Save struct + `saves.html`
unchanged) coded + gofmt'd + verified locally over all 840 (`/api/saves` returns 840 enriched,
`/api/save` returns the org body); committed at emacs-org-serve **`a462d15`**.
⏳ **DEPLOY PENDING:** pushing emacs-org-serve `main` is blocked by the autonomous-push
guardrail — needs an explicit user go-ahead, then node flake-bump emacs-org-serve → rebuild →
switch. Remaining doc cleanup (post-deploy): `SAVES.md`, `pwa-apps.md`.

### Phase 4 — Backfill
Run triage over the existing ~811 X-posts + ~29 clips → `References` notes.

### Phase 5 — Reader cutover
emacs-org-serve Saves reads `References` via `vulpea.db` (now indexed `.org`);
drop FT `ft md`; retire the markdown/items file-parse paths. Update `SAVES.md` +
`pwa-apps.md`. Then `00-Capture/Ingest/` holds only true plumbing.

## Cross-repo touch map

| Repo | Phase(s) | Changes |
|---|---|---|
| **All-The-Things** | 2,3,4 | move dirs; create `References/`; `Ingest/`; update `all-the-things.org`, `SAVES.md`; triage Skill/Workflow `.org` |
| **nix-loxley-node** | 2 | `field-theory.nix` (paths + service rename), `read-later-ingress.nix` (root), tmpfiles, readme; redeploy |
| **~/.emacs.d** | 2 | `my-save-link.el` (`MY_SAVE_LINK_ROOT`), save-link CLI scripts, help doc; bookmarklet #3 target |
| **emacs-org-serve** | 2,5 | `main.go` saves source paths (env now; `vulpea.db` in P5), `SAVES.md`, `pwa-apps.md`; redeploy |
| **Emacs-Harness** | 3 | `triage-capture` parser/runner support if needed |

## Safety / rollback
- snapper snapshots (root + srv-loxley) before every node switch.
- `nixos-rebuild build` + `nix store diff-closures` before `switch` (expect only
  the intended units to change).
- Phase 2 keeps the Saves tab working throughout (readers repointed; `ft md`
  retained until Phase 5), so no user-facing gap.
- Vault moves are reversible; the live tree and `all-the-things.org` are updated
  together so the map never lies.

## Decisions locked (2026-06-08)
- **Name/number:** top-level `References/` (unnumbered). User will de-number ALL PARA folders later — do NOT add a `NN-` prefix.
- **Schema:** confirmed as drafted above.
- **Autonomy (v1):** auto-file EVERYTHING into `References/` — no confidence gate, no review/Telegram escalation. (Supersedes "escalate exceptions" in the triage model above.) Items belonging to a task/project/area get a SEPARATE ingestion path, decided later — not this triage.
- **FT markdown:** retained during transition (Phase 2a); retired in Phase 5.
- **Provider:** Codex-CLI primary, Claude fallback.
- **my-emacs:** `my-save-link.el` change committed + pushed (user authorized).
