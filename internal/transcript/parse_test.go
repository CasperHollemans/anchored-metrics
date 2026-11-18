package transcript

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func parse(t *testing.T, lines ...string) Transcript {
	t.Helper()
	tr, err := Parse(strings.NewReader(strings.Join(lines, "\n") + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

const (
	userAt9 = `{"type":"user","timestamp":"2026-11-01T09:00:00.000Z","message":{"role":"user","content":"hi"}}`
	reply1  = `{"type":"assistant","timestamp":"2026-11-01T09:01:00.000Z","message":{"id":"msg_1","model":"claude-opus-5-5","content":[{"type":"text","text":"x"}],"usage":{"input_tokens":1,"output_tokens":2,"cache_read_input_tokens":3,"cache_creation_input_tokens":4}}}`
)

func TestParseReadsPromptAndReplyTimes(t *testing.T) {
	tr := parse(t, userAt9, reply1)

	want := []time.Time{
		time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 1, 9, 1, 0, 0, time.UTC),
	}
	if len(tr.Times) != 2 || !tr.Times[0].Equal(want[0]) || !tr.Times[1].Equal(want[1]) {
		t.Fatalf("Times = %v, want %v", tr.Times, want)
	}
}

func TestParseReadsTimestampOffsets(t *testing.T) {
	tr := parse(t, `{"type":"user","timestamp":"2026-11-01T09:00:00.000+01:00"}`)

	if want := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC); len(tr.Times) != 1 || !tr.Times[0].Equal(want) {
		t.Fatalf("Times = %v, want %v", tr.Times, want)
	}
}

func TestParseReadsReplyModelAndUsage(t *testing.T) {
	tr := parse(t, reply1)

	want := Reply{ID: "msg_1", Model: "claude-opus-5-5", Usage: Usage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}}
	if len(tr.Replies) != 1 || tr.Replies[0] != want {
		t.Fatalf("Replies = %+v, want [%+v]", tr.Replies, want)
	}
}

func TestParseCountsAReplyRepeatedPerContentBlockOnce(t *testing.T) {
	tr := parse(t, reply1, reply1, reply1)

	if len(tr.Replies) != 1 {
		t.Fatalf("got %d replies, want 1", len(tr.Replies))
	}
}

func TestParseKeepsTheLatestCopyOfARepeatedReply(t *testing.T) {
	tr := parse(t,
		`{"type":"assistant","timestamp":"2026-11-01T09:01:00.000Z","message":{"id":"msg_1","model":"m","usage":{"output_tokens":1}}}`,
		`{"type":"assistant","timestamp":"2026-11-01T09:01:00.000Z","message":{"id":"msg_1","model":"m","usage":{"output_tokens":7}}}`,
	)

	if len(tr.Replies) != 1 || tr.Replies[0].Usage.Output != 7 {
		t.Fatalf("Replies = %+v, want one reply with the latest usage (7)", tr.Replies)
	}
}

func TestParseCountsTheTimeButNoReplyWhenIDOrUsageIsMissing(t *testing.T) {
	tr := parse(t,
		`{"type":"assistant","timestamp":"2026-11-01T09:01:00.000Z","message":{"model":"m","usage":{"output_tokens":5}}}`,
		`{"type":"assistant","timestamp":"2026-11-01T09:02:00.000Z","message":{"id":"msg_2","model":"m"}}`,
	)

	if len(tr.Replies) != 0 || len(tr.Times) != 2 {
		t.Fatalf("Replies = %+v, Times = %v; want no replies and both times", tr.Replies, tr.Times)
	}
}

func TestParseFileOfAMissingFileIsAnError(t *testing.T) {
	if _, err := ParseFile(filepath.Join(t.TempDir(), "missing.jsonl")); err == nil {
		t.Fatal("want an error for a missing file")
	}
}

func TestParseSkipsSyntheticReplies(t *testing.T) {
	tr := parse(t, `{"type":"assistant","timestamp":"2026-11-01T09:01:00.000Z","message":{"id":"msg_s","model":"<synthetic>","usage":{"output_tokens":0}}}`)

	if len(tr.Replies) != 0 || len(tr.Times) != 0 {
		t.Fatalf("got %+v, want no replies and no times", tr)
	}
}

func TestParseCountsUnreadableLinesAndKeepsGoing(t *testing.T) {
	tr := parse(t, userAt9, `{"type":"assistant","message":{"id":"msg_trunc`, reply1)

	if tr.Skipped != 1 || len(tr.Replies) != 1 {
		t.Fatalf("Skipped = %d, replies = %d; want 1 and 1", tr.Skipped, len(tr.Replies))
	}
}

func TestParseCountsABadTimestampAsUnreadable(t *testing.T) {
	tr := parse(t, `{"type":"user","timestamp":"yesterday"}`)

	if tr.Skipped != 1 || len(tr.Times) != 0 {
		t.Fatalf("Skipped = %d, Times = %v; want 1 and none", tr.Skipped, tr.Times)
	}
}

func TestParseIgnoresOtherRecordTypesAndBlankLines(t *testing.T) {
	tr := parse(t,
		`{"type":"attachment","timestamp":"2026-11-01T12:00:00.000Z"}`,
		`{"type":"future-record-type","timestamp":"2026-11-01T12:00:00.000Z","payload":{"x":[1]}}`,
		``,
		userAt9,
	)

	if len(tr.Times) != 1 || tr.Skipped != 0 {
		t.Fatalf("Times = %v, Skipped = %d; want only the prompt and nothing skipped", tr.Times, tr.Skipped)
	}
}

func TestParseHandlesVeryLongLines(t *testing.T) {
	long := `{"type":"user","timestamp":"2026-11-01T09:00:00.000Z","message":{"content":"` + strings.Repeat("x", 5_000_000) + `"}}`

	tr := parse(t, long)

	if len(tr.Times) != 1 || tr.Skipped != 0 {
		t.Fatalf("Times = %v, Skipped = %d", tr.Times, tr.Skipped)
	}
}
