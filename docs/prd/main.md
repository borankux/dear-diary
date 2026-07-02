# Dear Diary PRD

> Last updated: 2026-06-30
> Product status: v0.6.1 AI Inbox semantics implemented
> Repository: `github.com/borankux/dear-diary`
> Primary user: Frank

## 1. Executive Summary

Dear Diary is a Vim-first, local-first macOS terminal journal for fast daily capture and later reflection. Its core promise is simple: typing `diary` from anywhere opens today's Markdown journal in the user's editor, while `diary browse` and `diary search` make the archive usable without leaving the terminal.

The product has evolved from a lightweight journal CLI into a local personal operating surface:

1. **Capture**: write dated Markdown entries quickly in Vim or another configured editor.
2. **Recall**: browse a TUI calendar, search historical entries, view writing streaks, and see "on this day" memories.
3. **Extract**: process recent diary entries with an LLM to derive Todos and Memories.
4. **Inbox**: inspect extracted candidates as an AI Inbox, then promote only the items worth keeping.

The core journal experience is already usable. v0.6.1 turns the AI extraction layer into an Inbox-based closure loop: AI output enters a pending candidate layer, the default view shows a summary instead of forcing item-by-item review, and explicit triage promotes or dismisses candidates.

`write diary -> extract candidates -> inbox summary -> triage -> promote / dismiss -> complete / archive -> avoid duplicate generation`

## 2. Product Goals

### 2.1 Primary Goal

Make Dear Diary a daily-use personal journal and reflection system that remains fast, trustworthy, inspectable, and easy to recover from.

### 2.2 Current Phase Goal

Keep v0.6.1 small and run it for daily use: no new asset types until the existing AI Inbox -> Todo / Memory loop feels reliable.

### 2.3 Success Criteria

- `diary` opens today's entry quickly and never blocks on AI, network, database, or dashboard work.
- Journal source files stay plain Markdown and remain readable outside the app.
- Generated artifacts never pollute the canonical journal archive.
- AI extraction is provider-neutral, auditable, and reversible.
- Todos and Memories can be promoted from candidates, completed, archived, and deduplicated.
- Re-running commands does not create duplicate records or misleading dashboard output.
- The user can understand the current system state from the CLI, dashboard, and project docs.

## 3. Product Principles

1. **Vim-first**: the editor is the writing surface; Dear Diary routes dates and provides structure.
2. **Local-first**: source journals are local Markdown files under the user's control.
3. **Plain files as source of truth**: diary dates come from filenames, not parsed Markdown content.
4. **No magic overwrite**: the app should append or generate side outputs, never surprise-edit personal diary content.
5. **Fast capture, slower processing**: writing must remain instant; AI processing can be explicit and asynchronous later.
6. **Provider-neutral AI**: DeepSeek is the current available backend, not a permanent product identity.
7. **Closed loops over feature sprawl**: every extracted item must have a lifecycle, not just an append-only pile.

## 4. Target Users

### 4.1 Primary User

Frank: a terminal-heavy, Vim-comfortable user who wants a fast personal log, engineering diary, and self-reflection archive. He values proof, direct file ownership, and durable artifacts over decorative UX.

### 4.2 Secondary Users

- Developers who want a local Markdown journal.
- Vim or terminal users who dislike web-first journal products.
- Local-first users who want search and lightweight automation without a hosted account.

## 5. Core User Problems

1. **Daily writing friction**: opening the right dated file manually is annoying enough to break the habit.
2. **Historical recall friction**: plain Markdown archives become hard to browse and search over time.
3. **Fragmented work memory**: diary entries contain tasks, insights, questions, and decisions, but they remain buried in prose.
4. **AI noise risk**: raw extraction can create duplicate, stale, or low-quality Todos and Memories if there is no review loop.
5. **Privacy ambiguity**: once LLM processing exists, the product must be explicit about what is local and what is sent to a provider.

## 6. Current Product Scope

### 6.1 Core Journal

Dear Diary stores entries at:

```text
~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md
```

Default file format:

```markdown
# YYYY-MM-DD 周X

## HH:MM

Entry text...
```

If the user opens today's existing entry through the default `diary` command, the app appends a new `## HH:MM` section unless the last non-empty line is already a timestamp from the last five minutes.

### 6.2 Commands

| Command | Current Behavior | Status |
|---|---|---|
| `diary` | Print daily highlight if available, then open today's journal | Active |
| `diary today` | Open today's journal explicitly | Active |
| `diary yesterday` / `diary y` | Open yesterday without appending a new timestamp | Active |
| `diary <date>` | Open a specific date (`YYYY-MM-DD`, `MM-DD`, `M/D`) | Active |
| `diary browse` | Open TUI monthly calendar browser | Active |
| `diary search <keyword>` | Search journal Markdown and show a TUI result list | Active, needs generated-file filtering |
| `diary process` | Process recent changed diary files with an LLM and create pending candidates | Active |
| `diary inbox` | Show an AI Inbox summary without forcing item-by-item triage | v0.6.1 active |
| `diary inbox triage` | Promote / dismiss / defer pending AI candidates | v0.6.1 active |
| `diary review` | Compatibility alias for `diary inbox triage` | Compatibility |
| `diary todo` | List active todos | v0.4 active |
| `diary todo done <id>` | Mark a todo done | v0.4 active |
| `diary todo archive <id>` | Archive a todo | v0.4 active |
| `diary dashboard` | Regenerate and open local dashboard HTML | v0.4 active |
| `diary -v` / `--version` | Print version | Active |

### 6.3 TUI Browse

Calendar behavior:

- Monday-first calendar.
- `◆` marks today.
- `●` marks days with journal files.
- Cursor highlights the selected date.
- `hjkl` and arrow keys move.
- `[` or `]` switch months.
- `t` jumps to today.
- `g/G` jump to month start/end.
- `Enter` opens selected day.
- `?` toggles help.
- `q` / `Esc` quits.

### 6.4 Search

Search behavior:

- Uses `rg` when available.
- Falls back to pure Go scanning.
- Shows matching lines in reverse-date order.
- Opens selected result in the configured editor.

Known gap: search currently scans all Markdown files under the diary root, so generated files such as `process/todos.md` and `process/memories.md` can appear in journal search. Search must be constrained to canonical diary files matching `YYYY-MM/YYYY-MM-DD.md`.

### 6.5 Daily Highlights

When opening today's journal, the app can print:

- Current writing streak.
- "On this day" preview from entries in prior years.

The feature is intentionally quiet: if no data exists, it prints nothing.

### 6.6 AI Processing

`diary process` scans recent changed journal files, sends file content to the configured LLM backend, extracts Todo and Memory candidates, stores them in SQLite as `pending`, and writes Markdown / HTML summaries.

Current output locations:

```text
~/.local/share/dear-diary/process.db
~/Documents/dear-diary/process/todos.md
~/Documents/dear-diary/process/memories.md
~/Documents/dear-diary/process/dashboard.html
```

Current implementation uses OpenAI-compatible API settings and reads generic `DIARY_LLM_*` variables first. DeepSeek-compatible variables remain supported for local compatibility:

- DeepSeek is a backend option, not the product boundary.
- OpenAI-compatible APIs should be the first generic interface.
- Local model endpoints or an LLM gateway should be supported by configuration when available.

## 7. Current Status Snapshot

Verified on 2026-06-30 after v0.6.1 AI Inbox deployment:

- Global binary: `/Users/allintech/bin/diary`
- Current build output version: `0.6.1`
- Public health version: `0.6.1-server`
- `go test ./...`: passing
- `go vet ./...`: passing
- `npm --prefix web ci`: passing
- `npm --prefix web run build`: passing
- `make build`: passing
- Browser smoke: desktop and mobile dashboard render without console/API errors or horizontal overflow
- AI extraction now writes pending candidates instead of final todos/memories
- `diary inbox` shows pending candidate counts and a small attention slice
- `diary inbox triage` can promote/dismiss/defer candidates
- `diary todo` can list/done/archive active todos
- Search, scanner, and dashboard share canonical diary-file filtering

### 7.1 Stable Enough for Daily Use

- Opening and writing dated entries.
- Same-day timestamp append.
- Calendar browsing.
- Basic search.
- Streak and on-this-day reminders.
- Local build and tests.

### 7.2 POC / Hardening Needed

- Run the closure loop on real entries for 30 days.
- Add triage edit/merge only if promote/dismiss is too lossy.
- Consider local model or LLM gateway after the provider-neutral boundary is exercised.

## 8. Functional Requirements

### 8.1 Journal Capture

The system must:

- Create a new file for a date if it does not exist.
- Use `# YYYY-MM-DD 周X` as the top-level heading.
- Add an initial `## HH:MM` section.
- Append a new timestamp section when reopening today's entry through default capture.
- Avoid duplicate empty timestamp sections within five minutes.
- Respect editor priority: `$DIARY_EDITOR` > `$EDITOR` > `vim`.
- Keep journal content as plain Markdown.

### 8.2 Journal Navigation

The system must:

- Show a monthly calendar in the terminal.
- Mark today and written days.
- Support Vim-style navigation.
- Open selected dates in the configured editor.
- Re-scan the filesystem when month context changes.

### 8.3 Journal Search

The system must:

- Search only canonical diary entries.
- Exclude generated output directories such as `process/`.
- Support a fast `rg` path and a pure Go fallback.
- Sort results newest first.
- Open selected results in the configured editor.

### 8.4 Writing Stats and Memories

The system must:

- Calculate current streak using the gentle rule: if today is not written yet, count from yesterday.
- Support longest streak and total written day calculations.
- Show prior-year same-date memories only when meaningful content exists.
- Avoid printing empty noise on startup.

### 8.5 AI Extraction

The system must:

- Process only canonical diary files.
- Process incrementally using file content hash and mtime.
- Avoid reprocessing identical input sets after successful runs.
- Store processing state and extracted entities in SQLite.
- Generate human-readable Markdown and HTML summaries.
- Log state transitions for audit.
- Classify fatal and transient failures clearly.

### 8.6 LLM Provider Boundary

The system should move toward provider-neutral configuration:

```bash
DIARY_LLM_PROVIDER=openai-compatible
DIARY_LLM_BASE_URL=https://api.deepseek.com
DIARY_LLM_MODEL=deepseek-chat
DIARY_LLM_API_KEY=...
```

Backward compatibility with existing DeepSeek-specific environment variables may be retained during migration.

The CLI and docs must state that `diary process` sends journal content to the configured LLM provider unless the provider points to a local endpoint.

### 8.7 Reviewable Extraction Lifecycle

The system does not treat AI output as final by default. The implemented v0.4 lifecycle is:

```text
extracted candidate -> pending -> promoted / dismissed -> active Todo / active Memory -> done / archived where applicable
```

Implemented behavior:

- Proposed Todos and Memories remain in AI Inbox before promotion.
- Dismissed candidates are remembered to prevent repeated suggestions.
- Promoted Todos enter the active Todo list.
- Promoted Memories enter the Memory store.
- Promoted items preserve source traceability.

### 8.8 Todo Lifecycle

The Todo system must support:

- List active Todos.
- Mark Todo as done.
- Archive Todo.
- Edit Todo text.
- Preserve source file and creation metadata.
- Keep completed items out of the active list without deleting history.

Implemented CLI:

```bash
diary todo list
diary todo done <id>
diary todo archive <id>
```

Dashboard is intentionally read-only for the current product boundary. Todo completion stays in CLI via `diary todo done <id>` / `diary todo archive <id>`.

### 8.9 Memory Lifecycle

The Memory system should support:

- Topic-based grouping.
- Search by keyword.
- Source-date visibility.
- Merge suggestions for similar topics.
- Avoiding silent discard of same-topic but newer information.

Graph or timeline views are not required for the next hardening phase.

## 9. Non-Functional Requirements

### 9.1 Performance

- `diary` startup should remain fast enough for habitual daily use.
- AI processing must never run implicitly during journal capture.
- Calendar and search should remain responsive over a growing Markdown archive.

### 9.2 Reliability

- Repeated commands must be idempotent when input has not changed.
- Generated files must not be treated as diary source files.
- SQLite migrations must be forward-safe as schema evolves.
- Fatal failures must be visible in durable audit logs.

### 9.3 Privacy and Data Control

- Source journal files remain local Markdown.
- AI processing is explicit via `diary process`.
- The app must disclose which provider receives diary content.
- Local model or gateway endpoints should be supported as first-class configuration paths.
- No cloud sync, hosted account, or background upload is part of the core product.

### 9.4 Maintainability

- Keep the CLI small and dependency-light.
- Avoid adding framework complexity before the lifecycle is closed.
- Add tests for every new package and every schema-sensitive behavior.
- Keep generated artifacts separate from canonical source files.

## 10. Out of Scope for Current Hardening

These are intentionally deferred:

- Weather auto-fill.
- Mood picker.
- Daily reminders / launchd integration.
- Tags.
- Export to HTML / PDF beyond current dashboard generation.
- Encryption.
- Homebrew formula.
- Attachments and images.
- Knowledge graph UI.
- Full LLM gateway service.

These may be revisited after the v0.3 lifecycle is trustworthy.

## 11. Known Product Gaps

| Gap | Impact | Desired Fix |
|---|---|---|
| Generated Markdown appears in dashboard diary list | Dashboard can show generated summaries as diaries | Fixed in v0.4 with canonical diary-file filter |
| Search scans generated Markdown | Search results can mix journal content with AI output | Fixed in v0.4 with shared canonical filter |
| DeepSeek naming is hardcoded in config and docs | Product identity is tied to temporary provider | Fixed in v0.4 with `DIARY_LLM_*` config and compatibility fallback |
| Privacy messaging is incomplete | User may not distinguish local storage from remote LLM processing | Fixed in v0.4 CLI help and README |
| Fatal transitions are not fully persisted | Failed runs are harder to debug | Fixed in v0.4 by persisting forced fatal transition logs |
| Todo has no completion loop | Active Todo list grows into noise | Fixed in v0.4 with done/archive |
| AI output bypasses human review | Low-quality or duplicate items enter permanent store | Fixed in v0.4 with pending candidates |
| Memory duplicate handling is too coarse | Same-topic new information may be discarded | Add merge flow |
| Docs are behind code | Future sessions may misread current state | Keep Project Butler docs synced at session close |

## 12. Recommended Next Milestone

### v0.4.0: Closure Core

Implemented scope:

1. Add one shared canonical diary-file filter.
2. Apply it to search, dashboard, scanner, stats-sensitive paths where relevant.
3. Replace DeepSeek-specific product wording with provider-neutral LLM config.
4. Add clear privacy/provider disclosure to CLI help and README.
5. Persist fatal state transitions in process logs.
6. Add `diary todo`, `diary todo done`, and `diary todo archive`.
7. Add pending/promoted/dismissed lifecycle semantics for AI candidates.
8. Add `diary inbox` and keep `diary review` as compatibility alias.
9. Sync project docs to v0.4.0 status.

Acceptance criteria:

- Dashboard Diary count matches the number of real daily journal files.
- `diary search` does not return `process/todos.md` or `process/memories.md`.
- `diary process` explains the configured provider before sending content.
- Re-running `diary process` does not create duplicates.
- Failed runs have durable logs with reason.
- A generated Todo can be completed or archived from CLI.
- Dismissed AI candidates do not reappear on the next run for identical source content.

## 13. Future Milestones

### Next: Inbox Triage Polish

- Interactive promote/dismiss/merge UI.
- Memory grouping by topic.
- Safer local file links or editor integration.

### v0.5: Provider Flexibility

- OpenAI-compatible provider config finalized.
- Local endpoint documentation.
- Optional LLM gateway integration if multiple providers are actively used.
- Prompt/version metadata per extraction run.

### v0.6+: Convenience Features

- Reminders.
- Tags.
- Exports.
- Weather and mood metadata.
- Optional encryption and release distribution.

## 14. Definition of Done for New Features

A feature is not considered complete unless it has:

- A real command or dashboard entry point.
- Unit tests or focused integration tests.
- Clear failure behavior.
- Documentation in README or project docs.
- No pollution of canonical diary source files.
- Idempotent repeated execution where applicable.
- SQLite migration plan if schema changes.
- A lifecycle path for generated or extracted data.

Features that do not meet this bar should stay in `docs/` planning files or experimental branches, not the main product surface.
