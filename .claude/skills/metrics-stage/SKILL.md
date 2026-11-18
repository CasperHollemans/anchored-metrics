---
name: metrics-stage
description: The anchored-metrics workflow for building a stage or a new behaviour (a new CSV, new columns, changed behaviour). Use when starting a new stage, adding or changing behaviour, or when unsure which step comes next. Covers inspecting transcripts, user stories, acceptance tests and the lock, the implementation plan, TDD and the review.
---

# Building a stage

Seven steps, in order. Steps marked **gate** wait for the user's explicit
approval before the next step starts. Never skip a gate, and never treat
silence as approval.

**Never commit without the user's explicit permission for that commit.**
Propose the message and contents, then wait.

## 1. Inspect the real transcripts
Before any story: find out what the transcripts actually contain for the new
metric.
- Structure only: record types, field names, value types and counts. Never
  print message text; mask paths ([0004](../../../docs/decisions/0004-local-and-anonymous.md)).
- Look for traps: repeated records, placeholders, ordering and missing fields.
- Record the findings in `docs/transcript-format.md`, with the date, the Claude Code
  versions seen and the sample size.

## 2. Build the user stories (**gate**)
Together with the user, in conversation:
- Write `docs/stories/<stage>.md`: the user story, the columns, and numbered
  behaviours (`S-01` …, a new prefix per stage). Behaviours are plain sentences.
- Write a decision record in `docs/decisions/` for every decision. Never edit an
  accepted one: supersede it.
- Move anything left out to `docs/roadmap.md`.
- Ask, don't assume. Raise ambiguities before writing the stories.

## 3. Write the acceptance tests
- One or more tests per behaviour, in the DSL of
  [0002](../../../docs/decisions/0002-acceptance-test-dsl.md). Each comment starts
  with its behaviour ID.
- Extend `acceptance/dsl_test.go` where needed. Record builders follow
  `docs/transcript-format.md` and nothing else.
- Run them: every new test must fail for the right reason (an assertion, not a
  build or setup error). `TestEveryBehaviourHasAnAcceptanceTest` must pass.
- Propose a commit ("Add failing acceptance tests for …") and wait for permission.

## 4. Review and lock (**gate**)
The user reviews the tests and locks `acceptance/` (a local deny rule in
`.claude/settings.local.json`). From here on:
- Never change anything under `acceptance/`, not with Edit or Write, and not through
  the shell.
- If a test turns out wrong while implementing, **stop**. Explain the problem,
  propose the change, and wait. The user unlocks the folder or makes the edit.
  If behaviour changes, add a decision record.

## 5. Implementation spec and plan (**gate**)
Write `docs/plans/<stage>.md`:
- the packages and types involved, and what each change does;
- the order of work, as a list of unit tests to write (one line each);
- what stays inside `internal/transcript` (all format knowledge, [0003](../../../docs/decisions/0003-parsing-in-one-module.md));
- risks and open questions.

The user approves before any implementation code is written.

## 6. Implement with TDD
Strict cycle, one test at a time:
1. **Red**: write one unit test and run it. It must fail for the right reason.
2. **Green**: write the least code that makes it pass, and run all tests.
3. **Refactor**: clean up with all tests green, and run them again.

Rules:
- No production code without a failing test that asks for it. That includes
  error paths and guards.
- If code was written without a test that failed first, prove the test can fail:
  break the code on purpose, watch the test fail, then restore it.
- At green points, propose a commit and wait. The acceptance tests go green as the work completes.
- Run `go vet ./...` and `gofmt -l .` before every commit.

## 7. Review (**gate**)
- Walk the user through the code: the flow, where each behaviour lives, and the
  trade-offs.
- Run it on real transcripts (output to the scratchpad, never into the repo) and
  check the numbers against step 1.
- Report honestly: what deviates from the docs, what wasn't TDD, and what's
  unverified.
- The user decides whether to merge.
