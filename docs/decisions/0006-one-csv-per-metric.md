# 0006. One shared CSV per kind of metric
Status: Accepted
Date: 2026-11-17

## Context
The tool runs on a few client repos, and more CSVs (such as prompts) will follow.

## Decision
One file per kind of metric (`out/sessions.csv`, later `out/prompts.csv`), shared
by all repos, with a `repo` column.

## Consequences
- The dashboard has one source per metric and filters by repo.
- The number of files doesn't grow with repos × metrics.
- Syncing one repo must leave other repos' rows untouched.
