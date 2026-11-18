# 0011. One sync command writes every CSV
Status: Accepted
Date: 2026-11-18

## Context
Stage 1 has `anchored-metrics sessions <repo-path>`. Stage 2 adds
`prompts.csv`, and later stages will add more. A command per CSV means running
several commands per repo, and the CSVs could drift apart if one is synced and
another isn't.

## Decision
`anchored-metrics sync <repo-path>` reads the transcripts once and writes every
CSV (`sessions.csv` and `prompts.csv`). It replaces `anchored-metrics sessions`.

## Consequences
- The stage 1 acceptance tests run `sessions`. They change to `sync` with the
  user's agreement (`acceptance/` is locked).
- The stage 1 story's command line changes; its behaviours don't.
- Every CSV is always synced from the same moment.
