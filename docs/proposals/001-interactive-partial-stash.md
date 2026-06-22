# Proposal 001 — Interactive Partial Stash (Visual Hunk & Line Picker)

> *"Don't bury the whole chest. Pick the gems worth keeping."*

| Field | Value |
|---|---|
| **Status** | Proposed — awaiting approval |
| **Author** | Claude (feature sweep + research, 2026-06-21) |
| **Target version** | `v0.2.0` — "Surgeon" |
| **Branch** | `claude/git-tool-feature-proposal-uiitdk` |
| **PRD link** | Extends FR-02.4 (New Stash → Patch mode), §10 Screen 6, §11 Keymap |
| **Depends on** | Existing `diffview`, `worddiff`, `filetree`, `newstash` components; `git` ops |
| **Git requirement** | ≥ 2.35 (`git stash push --staged`), uses `git apply --cached` (universal) |

---

## 1. TL;DR

nidhi sells itself as **"git stash mastery"**, yet its one truly surgical operation —
stashing *part* of your changes — is the weakest screen in the app. The "Patch mode"
toggle in the New Stash screen does **not** give you a nidhi experience: it calls
`tea.Exec` and drops you into git's raw `git stash push -p` prompt
(`Stage this hunk [y,n,q,a,d,s,e,?]`), which is exactly the painful CLI UX nidhi
exists to replace.

This proposal adds a **native, fully-visual, interactive hunk & line picker** — a new
`PARTIAL` screen where you scroll a live Agni-themed diff, toggle individual **hunks**
(and drill into individual **lines**) with checkboxes, watch a live `+X / −Y across N files`
tally, then create a stash containing *exactly* the selected changes. The rest of your
working tree is left untouched.

It is the single highest-leverage feature left to build: it is **visual** (a checkbox
diff is the centerpiece), it is a **daily-driver** workflow (selective stashing is one of
the most common reasons people reach for `-p`), and it is **uniquely better than the CLI**
(line-level selection via checkboxes vs. git's hostile `e`/manual-patch-editing flow).

---

## 2. Why this feature (research-backed)

### 2.1 The gap, in nidhi's own code

- `internal/ui/screens/newstash.go` exposes a **"Patch mode"** checkbox whose only effect
  is to set the `--patch` flag, which is then run via `tea.Exec` — handing control to
  git's terminal prompt. nidhi renders nothing. There is **no nidhi hunk selector**.
- nidhi already owns every primitive needed to do this *better*:
  - `internal/ui/components/diffview.go` — unified diff parser + scrollable viewport.
  - `internal/ui/components/worddiff.go` — Myers token-level intra-line emphasis.
  - `internal/ui/components/filetree.go` — per-file grouping (staged/working/untracked).
  - The Agni theme has dedicated `DiffAdded*/DiffRemoved*` tokens already.
- The DETAIL screen proves the split-pane diff UX works; PARTIAL is the *interactive*
  sibling of that read-only view.

### 2.2 What power users actually want (2026)

Modern stash workflows are about **precision**, not bulk save/restore:

- Selective stashing (`git stash push -p`, pathspec stashing) is the canonical
  "stash the unrelated change I just noticed" move.
- `git stash push --keep-index` to test only the staged subset before pushing.
- Staging + stashing combined for small, reviewable, revertible increments.

Git's own tooling for this is the `-p` interactive loop, widely considered one of git's
worst UXs (single-letter prompts, no overview, line-level only via the dreaded `e`
manual patch edit). Tools like lazygit/magit win developer love precisely because they
turn this into a visual checkbox experience. nidhi, being *stash-first*, should have the
best partial-stash UX of any tool — not delegate to the thing it set out to replace.

### 2.3 Fit against the selection criteria

| Criterion | How this feature scores |
|---|---|
| Most interesting / highest leverage | Closes the flagship gap in a "stash mastery" tool |
| Has a visual UI component | ✅ A checkbox/tri-state diff is the entire screen |
| Daily-driver value | ✅ Selective stashing is an everyday move |
| Uniquely better than CLI | ✅ Line-level checkbox selection, live tally, overview |
| Reuses existing architecture | ✅ diffview + worddiff + filetree + newstash message field |
| Leverages latest git | ✅ `stash push --staged` (2.35+) + `apply --cached` plumbing |

---

## 3. User experience

### 3.1 Entry points

1. **Direct:** `P` (Shift-p) from LIST/PREVIEW → opens PARTIAL on current working-tree changes.
   (`p` is "pop"; `P` reads naturally as "Patch/Pick".)
2. **Via New Stash:** the existing "Patch mode" checkbox, on `Enter`, routes into the
   native PARTIAL picker instead of shelling out to `git stash push -p`.
3. Empty state: if there are no changes vs `HEAD`, show "Nothing to stash" and return.

### 3.2 The PARTIAL screen (mockup)

```
┌─ Status Bar ──────────────────────────────────────────────────────┐
│ ◆ nidhi  ⎇ main  3 changed files                       git 2.53   │
├─ Partial Stash ───────────────────────────────────────────────────┤
│ ▣ src/auth/token.go            +12 −4    (2/2 hunks)              │
│   ▣ @@ -42,7 +42,12 @@ func RefreshToken(...)                     │
│      ▣ +   newToken, err := provider.Refresh(token)              │
│      ▣ +   if err != nil {                                       │
│      ▢ +     log.Debug("refresh path")        ← line unselected  │
│      ▣ −   return nil, ErrExpired                                │
│   ▢ @@ -88,3 +93,6 @@ func validate(...)        ← hunk unselected │
│ ▢ pkg/db/pool.go               +0  −0    (0/3 hunks)             │
│ ▣ cmd/serve.go                 +5  −0    (1/1 hunks)  partial    │
├───────────────────────────────────────────────────────────────────┤
│  Selected: +17 −4 · 3 hunks · 2 files                             │
│ Space toggle  v line-mode  a file  A all  Enter stash  Esc  PICK  │
└───────────────────────────────────────────────────────────────────┘
```

- **Checkbox gutter** with tri-state markers: `▣` selected, `▢` empty, `▨`/`partial`
  for files/hunks that are partially selected.
- Selected hunks/lines render in full Agni `DiffAdded/DiffRemoved` colors (with
  existing word-level emphasis); unselected ones render **dimmed** — so the screen
  *visually previews* exactly what the stash will contain.
- **Live tally** in the footer recomputes on every toggle.
- Two granularities: **hunk-level** (default, fast) and **line-level** (`v` toggles into
  line mode for the focused hunk — the differentiator git makes painful).

### 3.3 Keymap (PARTIAL mode)

| Key | Action |
|---|---|
| `j`/`k`, `↑`/`↓` | Move focus (file → hunk → line depending on granularity) |
| `g`/`G` | First / last |
| `Ctrl+D`/`Ctrl+U` | Page scroll |
| `Space` | Toggle selection of focused file/hunk/line |
| `v` | Enter/exit line-selection mode for the focused hunk |
| `a` | Toggle all hunks in the focused file |
| `A` | Toggle all changes (select none ↔ select all) |
| `Tab` | Collapse/expand focused file |
| `Enter` | Confirm → prompt for message → create stash |
| `Esc` | Cancel, discard selection, return to previous mode |

`?` help overlay gains a "Partial Stash" category.

---

## 4. Implementation design

### 4.1 The deterministic stash mechanism (no interactive git)

The hard part is creating a stash of *exactly* the selected hunks/lines while leaving the
rest of the working tree intact — **without** driving git's interactive prompt. We do this
with standard plumbing and a transactional index dance (journaled, like rename/reorder):

1. **Source the diff:** `git diff HEAD --no-color` (worktree+index vs HEAD), parse into
   `files → hunks → lines`. (Scope is configurable later; v1 targets "all changes vs HEAD".)
2. **Build the selected patch:** from the user's selection, synthesize a valid unified
   diff — including line-level subsets — by recomputing each `@@ -a,b +c,d @@` header and
   emitting only the chosen `+`/`−` lines plus surrounding context. This "patch surgery"
   is the core algorithm and is fully unit-testable in isolation.
3. **Validate:** `git apply --cached --check selected.patch`. If it doesn't apply cleanly
   (e.g. overlapping pre-staged changes), abort safely with a clear error toast — never
   leave the repo half-modified.
4. **Transactional apply (journaled):**
   1. Snapshot the existing index: `restore.patch = git diff --cached --binary`.
   2. `git reset -q` — unstage everything (index → HEAD, **worktree untouched**).
   3. `git apply --cached selected.patch` — stage exactly the selection.
   4. `git stash push --staged -m "<msg>"` — the stash now contains *only* the selection;
      git removes it from index **and** worktree.
   5. `git apply --cached restore.patch` — restore the user's original staged state.
5. **Crash safety:** write a journal (XDG state dir, same pattern as
   `reorder/journal.go`) before step 4 and clear it on success; recover on next launch.

> Edge cases (overlapping staged+selected, binary files, mode changes, CRLF) are handled
> by the `--check` gate in step 3 and `--binary` snapshots; where a clean result can't be
> guaranteed, we refuse and explain rather than risk the working tree. nidhi's prime
> directive is "no data loss, ever."

### 4.2 New/changed code (estimate)

| File | Change |
|---|---|
| `internal/git/patch.go` *(new)* | Diff model (`File/Hunk/Line`), parser, **patch-surgery** builder, selection state. Pure, no git — heavily unit-tested. |
| `internal/git/partialstash.go` *(new)* | The transactional `CreatePartialStash(selection, msg)` orchestration + journal. |
| `internal/ui/screens/partial.go` *(new)* | `PARTIAL` screen: checkbox diff renderer (reuses diffview/worddiff), focus + granularity state, keymap. |
| `internal/ui/screens/newstash.go` | Route "Patch mode" → PARTIAL instead of `tea.Exec`. |
| `internal/core/app.go`, `mode.go` | Register `ModePartial`, route `P` key, wire screen. |
| `internal/ui/components/footer.go` | PARTIAL footer hints + `PICK` badge (purple, per mockup badge convention). |
| `internal/ui/screens/help.go` | Add "Partial Stash" keybind category. |
| `README.md`, `docs/man/nidhi.1`, `docs/PROGRESS.md` | Document the feature & keys. |

### 4.3 Out of scope for v1 (future follow-ons)

- **Selective *apply*** (apply only chosen hunks *from an existing stash*) — the natural
  inverse, reusing the same picker + patch engine. Strong candidate for Proposal 002.
- Per-scope picking (untracked-file inclusion toggles inside the picker).
- Mouse click-to-toggle on checkboxes (the existing mouse layer can extend later).

---

## 5. Testing strategy — Red/Green TDD + shux automation

### 5.1 Unit (TDD, red first)

- **Patch surgery is the crown jewel of the test suite.** Table-driven tests written
  *before* implementation: single-hunk, multi-hunk, line-subset, add-only, delete-only,
  context-only hunks, header recomputation, no-trailing-newline, CRLF, adjacent hunks.
- `CreatePartialStash` integration tests against **real temp git repos** (no mocks, per
  CLAUDE.md): selecting a subset produces a stash whose diff equals the selection AND the
  remaining working tree equals the unselected changes; pre-staged state survives; the
  `--check` abort path leaves the repo pristine; journal recovery works after a simulated
  crash.

### 5.2 Full-interactive TUI automation (shux)

Driven by a dedicated **TUI-testing subagent** (see §6) using the now-installed `shux`:
spawn nidhi in a real PTY pane, send keystrokes (`P`, `j/k`, `Space`, `v`, `Enter`),
capture **PNG snapshots** and text scrapes of the pane, and assert both *behavior* (the
created stash matches the on-screen selection) and *visuals* (checkbox glyphs, dim vs.
full-color rows, live tally). Content-change assertions over keyword presence, polling
over sleeps — consistent with the existing iTerm2 test philosophy in CLAUDE.md.

### 5.3 Hard Definition of Done (gating the PR)

The PR may **only** proceed once the TUI-testing subagent returns a **GREEN** verdict
against every gate below. Any single RED blocks the PR.

**Functional gates**
1. `P` from LIST opens PARTIAL; `Esc` returns with zero repo mutation.
2. `Space` toggles hunk selection; footer tally updates to the correct `+X/−Y`.
3. `v` enters line mode; toggling a single `+` line is reflected in the created stash.
4. `Enter` → message → creates a stash whose `git stash show -p` diff is **exactly** the
   selection; unselected changes remain in the working tree.
5. Pre-existing staged changes are intact after the operation.
6. The `--check` failure path shows an error toast and leaves the repo **unchanged**.
7. New Stash "Patch mode" routes into PARTIAL (no raw git prompt appears).

**Visual gates (PNG snapshots)**
8. Tri-state checkboxes render (`▣`/`▢`/partial) and selected rows are full-color while
   unselected rows are dimmed.
9. `PICK` mode badge + correct footer hints are visible.
10. Layout is intact at 80×24 and 120×40 (no overflow/clipping).

**Quality gates**
11. `make check` (lint + test) passes; coverage on new `git` code ≥ 80%.
12. No regression: existing 933+ tests still pass.

---

## 6. Execution plan (after approval)

1. **TDD red:** land failing unit tests for patch surgery + `CreatePartialStash`.
2. **TDD green:** implement `internal/git/patch.go` and `partialstash.go` to pass them.
3. **UI:** build `PARTIAL` screen, wire mode/keys/footer/help, route New Stash patch mode.
4. **shux automation:** write the interactive test script; iterate to green.
5. **Independent judging:** a **specialized TUI-testing subagent** runs §5.3 with shux and
   returns a strict GREEN/RED verdict per gate. It gets the DoD above as its contract and
   judges independently — **only on its GREEN do we open the PR.**
6. Update README / man / PROGRESS; commit; (PR only when you explicitly ask).

---

## 7. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Patch surgery bugs corrupt the working tree | `git apply --check` gate + journal + refuse-on-doubt; never touch `.git/` directly |
| Overlapping pre-staged + selected hunks | Snapshot/restore index with `--binary`; abort cleanly if apply fails |
| Binary files / renames / mode changes | Detect and either include whole-file or exclude with a visible note in v1 |
| Scope creep (selective apply, mouse, scopes) | Explicitly deferred to follow-on proposals |
| shux flakiness in CI | Local/agent-driven gate for now (matches current iTerm2 approach); polling not sleeps |

---

## 8. Sources

- git-stash documentation — <https://git-scm.com/docs/git-stash>
- `git stash --staged` (Git 2.35) — partial/staged stashing semantics
- Git 2.51 stash export/import & chained stash model — <https://www.ewere.tech/blog/git-2-51-released/>
- 2026 stash workflow playbooks (selective stashing, keep-index pre-push validation)
- nidhi codebase inventory (this repo): `newstash.go`, `diffview.go`, `worddiff.go`,
  `filetree.go`, `operations.go`, `reorder/journal.go`
