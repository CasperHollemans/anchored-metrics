# 0002. Acceptance tests as an end-to-end Go DSL
Status: Accepted
Date: 2026-11-13

## Context
We work BDD then TDD: behaviours are written as executable acceptance tests
first, then implemented test-first. The tests must be readable by a human who
doesn't read Go, and runnable. We considered txtar data files with a runner,
raw JSONL/CSV strings in tests, tables, a mini transcript language and string
assertions. The domain-language DSL read best.

## Decision
Acceptance tests are Go functions in this shape:

```go
// S-07 When a session spans time zones, start and end are still written in
// UTC, so sessions from different machines sort and compare correctly.
func TestSessionStartAndEndAreWrittenInUTC(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00 +01:00")),
		AssistantReply(at("09:05 +01:00")),
	)

	s.When.Sync()

	s.Then.Session("abc").Has(
		Start("08:00Z"),
		End("08:05Z"),
	)
}
```

- Each test has a plain-English comment of one or two lines, starting with the
  behaviour ID from `docs/stories/`.
- Given / When / Then blocks are on separate lines, and times are wrapped in `at(...)`.
- `Has(...)` checks only the columns listed.
- `When.Sync()` builds and runs the real binary against a temporary fake
  `~/.claude/projects`.
- Helpers such as `UserPrompt` and `AssistantReply` build real transcript
  records. They are the only test code that knows the transcript format, and
  they follow `docs/transcript-format.md`.

## Consequences
- A format change touches the helpers and the parser, never the behaviour tests.
- A test fails if a behaviour ID in `docs/stories/` has no test, or a test
  references an unknown ID.
