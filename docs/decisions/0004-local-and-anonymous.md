# 0004. Local only, anonymous output
Status: Accepted
Date: 2026-11-13

## Context
Transcripts contain prompts, code, usernames and file paths. The tool is
used on client repos.

## Decision
- Nothing leaves the machine: no network calls, and no dependencies fetched at runtime.
- Output contains no usernames, file paths or prompt text. The repo's folder
  name and session IDs are allowed.
- Inspecting transcripts during development prints structure only: record
  types, field names and counts, never content.

## Consequences
- `cwd` and similar path fields are read only to map worktrees, never written out.
- Later metrics, such as prompts, must describe prompts without storing their text.
