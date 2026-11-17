# 0008. Main and subagent tokens counted separately
Status: Accepted
Date: 2026-11-17

## Context
Subagents are spawned by Claude during a session. They're stored next to the
session and carry the parent's session ID. The user never talks to a subagent,
so conversation-quality analysis (ambiguous prompts, corrections) applies only
to the main chat. Tokens and file changes do matter. Subagents can use a
different model from the main chat.

## Decision
- Subagent transcripts are read as part of their parent session.
- Tokens are reported in two sets of columns, `main_*` and `subagent_*`, each
  with input, output, cache read and cache write. Totals are left to the dashboard.
- Subagent models appear in the session's `models` list.

## Consequences
- The CSV shows how much a session relies on delegation and what it costs.
- Later conversation metrics skip subagent transcripts.
