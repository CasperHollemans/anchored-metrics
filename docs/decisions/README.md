# Decisions

One file per decision. Decisions are never edited after they're accepted. To
change one, add a new record and mark the old one `Superseded by NNNN`, so the
reasoning behind every change stays visible.

Template:

```
# NNNN. Title
Status: Proposed | Accepted | Superseded by NNNN
Date: YYYY-MM-DD

## Context
## Decision
## Consequences
```

| # | Decision | Status |
|---|---|---|
| [0001](0001-go-stdlib-only.md) | Go, standard library only | Accepted |
| [0002](0002-acceptance-test-dsl.md) | Acceptance tests as an end-to-end Go DSL | Accepted |
| [0003](0003-parsing-in-one-module.md) | All transcript parsing in one module, format never assumed | Accepted |
| [0004](0004-local-and-anonymous.md) | Local only, anonymous output | Accepted |
| [0005](0005-sync-replaces-changed-sessions.md) | Sync replaces changed sessions, keeps deleted ones | Accepted |
| [0006](0006-one-csv-per-metric.md) | One shared CSV per kind of metric | Accepted |
| [0007](0007-worktree-mapping.md) | Worktrees mapped through Claude Code's worktree folders and git | Accepted |
| [0008](0008-main-and-subagent-tokens.md) | Main and subagent tokens counted separately | Accepted |
| [0009](0009-stage-1-scope.md) | Stage 1 scope | Accepted |
| [0010](0010-models-as-list.md) | Models as a list column | Accepted |
| [0011](0011-one-sync-command.md) | One sync command writes every CSV | Accepted |
| [0012](0012-correction-word-library.md) | Corrections detected locally with a categorised word library | Accepted |
