# anchored-metrics

A local CLI that reads Claude Code session transcripts and writes metrics as CSV.
v0.1 stage 1: `anchored-metrics sessions <repo-path>` writes `out/sessions.csv`.

## Docs
- `docs/stories/`: user stories and behaviours with IDs (`S-01` …). This is the contract.
- `docs/decisions/`: decision records (ADRs). Read the index before changing behaviour.
- `docs/transcript-format.md`: what we observed in real transcripts, including traps.
- `docs/roadmap.md`: later stages (prompts.csv: corrections, ambiguous prompts,
  file changes). Not agreed behaviour yet. Read it before starting a new stage.

## Workflow
New stages and behaviour changes follow the `metrics-stage` skill
(`.claude/skills/metrics-stage/SKILL.md`): inspect, stories, acceptance tests,
lock, plan, TDD, review. Load it before starting.

## Rules
- **Acceptance tests are locked.** Never change anything under `acceptance/`
  without explicit agreement first, not with Edit or Write and not through the
  shell. If a test seems wrong, stop and propose the change.
- **Behaviour changes update the docs in the same commit:** the story, a new
  decision record (superseding the old one, never editing it), and the tests.
- **Transcript parsing lives in one package** ([0003](docs/decisions/0003-parsing-in-one-module.md)).
  Never assume the format: rely only on fields recorded in
  `docs/transcript-format.md`, and inspect real files when something new shows up.
- **Local and anonymous** ([0004](docs/decisions/0004-local-and-anonymous.md)):
  no network, and no usernames, paths or prompt text in output. Inspect
  transcripts by structure only, never print message text.
- Go standard library only ([0001](docs/decisions/0001-go-stdlib-only.md)).
