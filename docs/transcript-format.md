# Claude Code transcript format: observed

What we saw in real transcripts. The format is undocumented and changes, so
treat this as observation, not specification. Re-inspect and update this file
whenever the parser meets something new ([0003](decisions/0003-parsing-in-one-module.md)).

## Inspection log

| Date | Claude Code versions | Sample | Method |
|---|---|---|---|
| 2026-11-13 | 2.1.44 – 2.1.322 | 1 repo + 1 of its worktrees: 5 sessions, 1 to 2,592 records each | Structure only: record types, field names and types, counts |
| 2026-11-17 | 2.1.286 – 2.1.295 | 1 session from another repo: 5 subagent transcripts | Same |
| 2026-11-18 | 2.1.44 – 2.1.322 | 5 main sessions across 3 repos, including one with a known interrupt | Stage 2 signals: prompts, tools, file edits, denials. Only fixed system markers were matched in text; counted, never printed |

Message text was never printed. Paths were masked.

## Layout

```
~/.claude/projects/<encoded-cwd>/
  <sessionId>.jsonl                      one session (main conversation)
  <sessionId>/subagents/agent-<id>.jsonl one subagent of that session
  <sessionId>/subagents/agent-<id>.meta.json
  <sessionId>/subagents/agent-<id>.prefix.json   (not always present)
  <sessionId>/custom-title.json          ignore
  <sessionId>/tool-results/*             screenshots etc., ignore
  *.desktop-released.json                ignore
  memory/                                Claude's memory files, ignore
```

- `<encoded-cwd>` is the working directory with non-alphanumeric characters
  replaced by `-`: `/Users/x/.paradev/a` becomes `-Users-x--paradev-a`. This is
  lossy, so encode the repo path and compare; never decode a folder name.
- A worktree has its own `<encoded-cwd>` folder ([0007](decisions/0007-worktree-mapping.md)).
- A resumed session keeps appending to the same `<sessionId>.jsonl`.

## Records

One JSON object per line. Every record has `type`. Seen in one large session:

| `type` | Count | Used for sessions.csv |
|---|---|---|
| `assistant` | 538 | **yes**: model and usage |
| `user` | 366 | **yes**: timestamps |
| `attachment`, `queue-operation`, `last-prompt`, `custom-title`, `agent-name`, `file-history-snapshot`, `file-history-delta`, `atis-latch`, `api-request`, `api-request-blob`, `api-request-shape`, `system`, `tools-used`, `cost-state`, `mode` | 2–315 each | no |

Common fields on `user` and `assistant` records: `sessionId`, `uuid`,
`parentUuid`, `timestamp`, `cwd`, `gitBranch`, `isSidechain`, `version`,
`entrypoint`, `userType`.

- `sessionId` always equals the file name, for both main and subagent transcripts.
- `timestamp` was UTC with milliseconds in every record checked (shape checked
  in the largest session): `2026-11-11T15:31:02.123Z`. Still parse the offset;
  don't assume `Z`.
- Not every record type has a `timestamp`.

### Assistant record (fields we use)

```
message.id                                  reply ID, repeated across lines (see traps)
message.model                               e.g. claude-opus-5-5, or <synthetic>
message.usage.input_tokens
message.usage.output_tokens
message.usage.cache_read_input_tokens
message.usage.cache_creation_input_tokens   = 5m + 1h below
message.usage.cache_creation.ephemeral_5m_input_tokens
message.usage.cache_creation.ephemeral_1h_input_tokens
```

Also present but unused: `message.usage.output_tokens_details.thinking_tokens`,
`message.usage.iterations[]`, `server_tool_use`, `service_tier`, `speed`,
`inference_geo`, `message.stop_reason`, `message.content[]` (block types
`text`, `thinking`, `tool_use`, `tool_result`, `image`).

## Subagents

- Records have `isSidechain: true`, an `agentId`, and the **parent's** `sessionId`.
- `.meta.json` fields: `agentType`, `description`, `toolUseId`, `spawnDepth`,
  `requestShape`, `requestNonInteractive`, `model` (missing in one file).
- Link to the parent: `.meta.json` `toolUseId` matches the parent's spawning
  tool call, and the parent's tool result has `toolUseResult.agentId`.
- A subagent can use a different model from the main chat. One ran entirely on
  `claude-haiku-5-5` under an Opus session.

## Prompts, tools and file edits (stage 2)

### What a `user` record is
A `user` record is one of four things:

| Kind | How to tell | Seen |
|---|---|---|
| A prompt typed by the user | `message.content` is text, not `tool_result` blocks; not `isMeta`; not `isCompactSummary` | 3–95 per session |
| A tool result | `message.content` holds `tool_result` blocks | most `user` records |
| Meta | `isMeta: true` | a few |
| Compaction summary | `isCompactSummary: true` | 1 |

- Newer prompts carry `turnOrigin: "human"`, `origin.kind: "human"` and
  `promptSource: "sdk"`. Older sessions don't have these fields.
- Some "prompts" aren't typed by a human: text starting with
  `<task-notification>` (a background task finished, 3–5 per session) or
  `<bash-input>` (the user ran a shell command). Classify them by their leading tag.
- Prompt length varied from 1 to 1,307 words, with a median of 14–18.

- A prompt's `message.content` is a plain string, or a list with `image` and
  `text` blocks when the user pasted an image (3 of ~220 prompts). Its text is
  the string, or the `text` blocks joined.

### Interrupts
- The interrupt marker's `message.content` is a list with one `text` block
  (not a plain string), and it has no `isMeta`.
- A user interrupt is a prompt-kind record whose text is exactly
  `[Request interrupted by user]`, or starts with
  `[Request interrupted by user for tool use]`. Found once, in the session with a
  known interrupt; none in the others.
- `toolUseResult.interrupted` (on Bash results) is a different thing: a tool
  run that was cut off.

### Tool calls
- `tool_use` blocks in assistant content have `id` and `name`. The matching
  `tool_result` in a later user record has `tool_use_id` and optionally `is_error`.
- Names seen: `Bash`, `Edit`, `Write`, `Read`, `Glob`, `TodoWrite`, `Agent`,
  `Skill`, `AskUserQuestion`, `WebSearch`, `WebFetch`, `ToolSearch`, `TaskStop`,
  and `mcp__<server>__<tool>`.
- `AskUserQuestion` (Claude asking the user a question) appeared once. Its
  `toolUseResult` has `questions` and `answers`.
- `is_error` is mostly on `Bash`, which is a command failing, not the user correcting.

### Denials
- `permissionDecision.decision` is `accept` or `reject`, and
  `permissionDecision.source` is `config` or `hook`. `toolDenialKind:
  "permission-rule"` was seen.
- Every `reject` seen came from a hook or a permission rule, not from the user
  saying no. All sessions ran in `permissionMode` `auto` or `bypassPermissions`,
  so a user clicking "deny" was never seen.

### File edits
- `Edit` results: `toolUseResult` has `filePath`, `oldString`, `newString`,
  `originalFile`, `replaceAll`, `structuredPatch`, `userModified`, and sometimes
  `contentNotInModelContext` or `staleRecovered`.
- `Write` results: `content`, `filePath`, `originalFile`, `structuredPatch`,
  `type`, `userModified`, and sometimes `memdirStamped`. `userModified` was
  missing in the oldest session.
- `toolUseResult` is sometimes a plain string instead of an object (1–3 per
  session, probably an error message).
- `structuredPatch` is a list of hunks with `oldStart`, `oldLines`, `newStart`,
  `newLines` and `lines`. Each entry in `lines` starts with `+` (added), `-`
  (removed) or a space (context). Count the `+` and `-` lines.
- A `Write` that creates a file has `type: "create"`, `originalFile: null` and an
  **empty** `structuredPatch` (92 of 92 creates seen). Its added lines must be
  counted from `content`. A `Write` with `type: "update"` has hunks like an `Edit`.
- `filePath` is a path and must never be written out.
- `userModified` may mean the user changed Claude's proposed edit, a possible
  correction signal. Not verified.
- `file-history-snapshot` and `file-history-delta` records track backups:
  `messageId`, `trackingPath` (a path), and `backup.version` and `backupTime`.
- `attachment.type: "edited_text_file"` appeared 8–9 times per session. It
  probably means a file changed outside Claude. Not verified.

### Other
- `queue-operation` records (`enqueue`, `dequeue`, `remove`) are **not** a
  mid-turn signal: in the desktop app *every* prompt goes enqueue → dequeue →
  prompt. Patterns such as enqueue, enqueue (queued while Claude was busy) and
  `remove` may mean something, but this isn't understood yet.
- An interrupt marker is immediately followed by the user's next prompt (seen as
  two prompt-kind records in a row).
- An `Agent` tool result carries `toolUseResult.agentId`, linking the spawning
  tool call to `subagents/agent-<agentId>.jsonl`. Other keys seen: `agentType`,
  `status`, `content`, `prompt`, `resolvedModel`, `usage`, `totalTokens`,
  `totalToolUseCount`, `totalDurationMs`, `toolStats` and a few `harness*` and
  `handback*` fields.
- In nearly every session, many records share a `parentUuid` with a sibling.
  This looks structural (parallel tool calls), so branching is **not** a clean
  rewind or undo signal without further study.
- `mode` records (`mode: "normal"`) and `permissionMode` on prompts track the mode
  the session ran in.

## Traps

1. **Usage is repeated per content block.** Each content block of a reply is
   its own line (always 1 block per line), and every line carries the same
   `message.id` and **identical** usage. Summing all lines overcounts up to
   about 3×. Count each `message.id` once. Verified: deduplicated totals matched
   Claude Code's own `cost-state` totals exactly in one session.
2. **`<synthetic>` model.** Local placeholder replies with zero usage. Skip them.
3. **Zero-usage replies.** One session had 5 assistant replies, all with zero usage.
4. **Timestamps aren't always in order.** One subagent file had them out of
   order. Use the earliest and latest timestamp, never the first and last line.
5. **File modified times are unreliable.** Subagent files with October content
   had November modified times.
6. **`cost-state` isn't a session total.** It appears a few times per session
   and resets when the process restarts (it didn't match the deduplicated
   totals in a resumed session). It also includes background calls, such as Haiku,
   that never appear as assistant records. Don't use it.
7. **`cwd` varies within a session.** It changes when Claude changes directory
   inside the repo, for example to `<repo>/docs/stories`.
