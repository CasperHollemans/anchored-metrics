# 0003. All transcript parsing in one module, format never assumed
Status: Accepted
Date: 2026-11-13

## Context
Claude Code's transcript format is undocumented and changes between versions.
The format we found (`docs/transcript-format.md`) has many record types we
don't need, and traps that are easy to miss.

## Decision
- One package owns all knowledge of the transcript format. The rest of the
  code works with domain types (session, reply, usage).
- Nothing about the format is assumed: every field we rely on was seen in real
  transcripts and is recorded in `docs/transcript-format.md`.
- The parser decodes only the fields it needs. It skips unknown record types
  and fields, and skips unreadable lines while counting them.

## Consequences
- When the format changes, we re-run the inspection tool, update
  `docs/transcript-format.md`, then change only the parser and the test helpers.
- Skipped-line counts make a format change visible instead of silent.
