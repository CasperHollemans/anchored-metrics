package sessions

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"anchored-metrics/internal/transcript"
)

func clock(h, m int) time.Time { return time.Date(2026, 11, 1, h, m, 0, 0, time.UTC) }

func TestSummarizeSplitsMainAndSubagentUsageAndCollectsModels(t *testing.T) {
	main := transcript.Transcript{
		Times:   []time.Time{clock(9, 30), clock(9, 0)},
		Replies: []transcript.Reply{{ID: "m1", Model: "opus", Usage: transcript.Usage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}}},
	}
	subs := []transcript.Transcript{
		{Times: []time.Time{clock(10, 15)}, Replies: []transcript.Reply{{ID: "s1", Model: "haiku", Usage: transcript.Usage{Output: 10}}}},
		{Times: []time.Time{clock(9, 45)}, Replies: []transcript.Reply{{ID: "s2", Model: "opus", Usage: transcript.Usage{Output: 5}}}},
	}

	row, ok := Summarize("abc", "shop", main, subs)

	want := Row{
		Session: "abc", Repo: "shop", Start: clock(9, 0), End: clock(10, 15),
		Models:   []string{"haiku", "opus"},
		Main:     transcript.Usage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4},
		Subagent: transcript.Usage{Output: 15},
	}
	if !ok || !reflect.DeepEqual(row, want) {
		t.Fatalf("Summarize = %+v, %v\nwant %+v", row, ok, want)
	}
}

func TestSummarizeOfASessionWithoutAnyMomentsIsNotARow(t *testing.T) {
	if _, ok := Summarize("abc", "shop", transcript.Transcript{}, nil); ok {
		t.Fatal("want no row for a session with no prompts or replies")
	}
}

func TestMergeReplacesSyncedSessionsAndKeepsTheRest(t *testing.T) {
	existing := []Row{
		{Session: "kept", Start: clock(8, 0)},
		{Session: "grew", Start: clock(9, 0), Main: transcript.Usage{Output: 1}},
	}
	synced := []Row{
		{Session: "grew", Start: clock(9, 0), Main: transcript.Usage{Output: 2}},
		{Session: "new", Start: clock(10, 0)},
	}

	got := Merge(existing, synced)

	want := []Row{
		{Session: "kept", Start: clock(8, 0)},
		{Session: "grew", Start: clock(9, 0), Main: transcript.Usage{Output: 2}},
		{Session: "new", Start: clock(10, 0)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge = %+v\nwant %+v", got, want)
	}
}

func TestMergeOrdersRowsByStartThenSession(t *testing.T) {
	got := Merge(nil, []Row{
		{Session: "b", Start: clock(9, 0)},
		{Session: "c", Start: clock(8, 0)},
		{Session: "a", Start: clock(9, 0)},
	})

	var ids []string
	for _, r := range got {
		ids = append(ids, r.Session)
	}
	if want := []string{"c", "a", "b"}; !slices.Equal(ids, want) {
		t.Fatalf("order = %v, want %v", ids, want)
	}
}

func TestCSVRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out", "sessions.csv")
	rows := []Row{{
		Session: "abc", Repo: "shop", Start: clock(9, 0), End: clock(10, 15),
		Models:   []string{"haiku", "opus"},
		Main:     transcript.Usage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4},
		Subagent: transcript.Usage{Input: 5, Output: 6, CacheRead: 7, CacheWrite: 8},
	}}

	if err := WriteCSV(path, rows); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCSV(path)

	if err != nil || !reflect.DeepEqual(got, rows) {
		t.Fatalf("ReadCSV = %+v, %v\nwant %+v", got, err, rows)
	}
}

func TestCSVFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.csv")
	rows := []Row{{
		Session: "abc", Repo: "shop", Start: clock(9, 0), End: clock(10, 15).Add(500 * time.Millisecond),
		Models: []string{"haiku", "opus"}, Main: transcript.Usage{Output: 2},
	}}

	if err := WriteCSV(path, rows); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)

	want := "session,repo,start,end,models,main_input,main_output,main_cache_read,main_cache_write," +
		"subagent_input,subagent_output,subagent_cache_read,subagent_cache_write\n" +
		"abc,shop,2026-11-01T09:00:00Z,2026-11-01T10:15:00Z,haiku;opus,0,2,0,0,0,0,0,0\n"
	if string(b) != want {
		t.Fatalf("CSV =\n%s\nwant\n%s", b, want)
	}
}

func writeRaw(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadCSVRefusesUnexpectedColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.csv")
	renamed := strings.Replace(strings.Join(header, ","), "main_output", "output", 1)
	writeRaw(t, path, renamed+"\n"+
		"abc,shop,2026-11-01T09:00:00Z,2026-11-01T09:01:00Z,m,0,0,0,0,0,0,0,0\n")

	if _, err := ReadCSV(path); err == nil {
		t.Fatal("want an error for a CSV whose columns are named differently")
	}
}

func TestReadCSVRefusesARowWithTheWrongNumberOfColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.csv")
	writeRaw(t, path, strings.Join(header, ",")+"\nabc,shop\n")

	if _, err := ReadCSV(path); err == nil {
		t.Fatal("want an error for a row with too few columns")
	}
}

func TestReadCSVRefusesValuesItCannotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.csv")
	writeRaw(t, path, strings.Join(header, ",")+"\n"+
		"abc,shop,2026-11-01T09:00:00Z,2026-11-01T09:01:00Z,m,lots,0,0,0,0,0,0,0\n")

	if _, err := ReadCSV(path); err == nil {
		t.Fatal("want an error for a token count that isn't a number")
	}
}

func TestAFailedWriteLeavesTheExistingCSVIntact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.csv")
	old := []Row{{Session: "old", Repo: "shop", Start: clock(9, 0), End: clock(9, 1)}}
	if err := WriteCSV(path, old); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := os.Chmod(dir, 0o555); err != nil { // no new files can be made here
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := WriteCSV(path, []Row{{Session: "new", Repo: "shop", Start: clock(10, 0), End: clock(10, 1)}})

	after, _ := os.ReadFile(path)
	if err == nil || string(after) != string(before) {
		t.Fatalf("WriteCSV error = %v; CSV after =\n%s\nwant an error and the CSV unchanged:\n%s", err, after, before)
	}
}

func TestReadCSVOfAMissingFileIsEmpty(t *testing.T) {
	got, err := ReadCSV(filepath.Join(t.TempDir(), "missing.csv"))

	if err != nil || len(got) != 0 {
		t.Fatalf("ReadCSV = %v, %v; want nothing, no error", got, err)
	}
}
