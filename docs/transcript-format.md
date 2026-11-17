# Claude Code transcript format: observed

What we saw in real transcripts. The format is undocumented and changes, so
treat this as observation, not specification. Re-inspect and update this file
whenever the parser meets something new ([0003](decisions/0003-parsing-in-one-module.md)).

## Inspection log

| Date | Claude Code versions | Sample | Method |
|---|---|---|---|
| 2026-11-13 | 2.1.44 – 2.1.322 | 1 repo + 1 of its worktrees: 5 sessions, 1 to 2,592 records each | Structure only: record types, field names and types, counts |
| 2026-11-17 | 2.1.286 – 2.1.295 | 1 session from another repo: 5 subagent transcripts | Same |

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
