// Package sessions turns transcripts into sessions.csv rows and keeps the
// CSV up to date (docs/stories/sessions.md).
package sessions

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"anchored-metrics/internal/transcript"
)

// Row is one line of sessions.csv.
type Row struct {
	Session  string
	Repo     string
	Start    time.Time
	End      time.Time
	Models   []string
	Main     transcript.Usage
	Subagent transcript.Usage
}

// Summarize builds a session's row from its main transcript and its subagent
// transcripts. A session without any prompt or reply has no row.
func Summarize(id, repo string, main transcript.Transcript, subagents []transcript.Transcript) (Row, bool) {
	row := Row{Session: id, Repo: repo}
	models := map[string]bool{}
	var times []time.Time

	times = append(times, main.Times...)
	for _, r := range main.Replies {
		row.Main = row.Main.Add(r.Usage)
		models[r.Model] = true
	}
	for _, sub := range subagents {
		times = append(times, sub.Times...)
		for _, r := range sub.Replies {
			row.Subagent = row.Subagent.Add(r.Usage)
			models[r.Model] = true
		}
	}
	if len(times) == 0 {
		return Row{}, false
	}
	row.Start = slices.MinFunc(times, time.Time.Compare)
	row.End = slices.MaxFunc(times, time.Time.Compare)
	for m := range models {
		row.Models = append(row.Models, m)
	}
	slices.Sort(row.Models)
	return row, true
}

// Merge replaces existing rows by the freshly synced ones and keeps every
// other row, so sessions whose transcript is gone are never lost. Rows are
// ordered by start, then session, so the CSV is stable between syncs.
func Merge(existing, synced []Row) []Row {
	byID := map[string]Row{}
	for _, r := range existing {
		byID[r.Session] = r
	}
	for _, r := range synced {
		byID[r.Session] = r
	}
	rows := make([]Row, 0, len(byID))
	for _, r := range byID {
		rows = append(rows, r)
	}
	slices.SortFunc(rows, func(a, b Row) int {
		if c := a.Start.Compare(b.Start); c != 0 {
			return c
		}
		return strings.Compare(a.Session, b.Session)
	})
	return rows
}

var header = []string{
	"session", "repo", "start", "end", "models",
	"main_input", "main_output", "main_cache_read", "main_cache_write",
	"subagent_input", "subagent_output", "subagent_cache_read", "subagent_cache_write",
}

const timeFormat = time.RFC3339 // UTC, to the second

// ReadCSV reads sessions.csv. A missing file has no rows.
func ReadCSV(path string) ([]Row, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	if !slices.Equal(records[0], header) {
		return nil, fmt.Errorf("%s has unexpected columns %v", path, records[0])
	}
	var rows []Row
	for i, rec := range records[1:] {
		row, err := fromRecord(rec)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+2, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// WriteCSV writes sessions.csv atomically: a failed write never damages the
// existing file.
func WriteCSV(path string, rows []Row) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sessions-*.csv")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	w := csv.NewWriter(tmp)
	w.Write(header)
	for _, r := range rows {
		w.Write(toRecord(r))
	}
	w.Flush()
	if err := w.Error(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func toRecord(r Row) []string {
	n := strconv.Itoa
	return []string{
		r.Session, r.Repo,
		r.Start.UTC().Format(timeFormat), r.End.UTC().Format(timeFormat),
		strings.Join(r.Models, ";"),
		n(r.Main.Input), n(r.Main.Output), n(r.Main.CacheRead), n(r.Main.CacheWrite),
		n(r.Subagent.Input), n(r.Subagent.Output), n(r.Subagent.CacheRead), n(r.Subagent.CacheWrite),
	}
}

func fromRecord(rec []string) (Row, error) {
	if len(rec) != len(header) {
		return Row{}, fmt.Errorf("want %d columns, got %d", len(header), len(rec))
	}
	var errs []error
	at := func(s string) time.Time {
		t, err := time.Parse(timeFormat, s)
		errs = append(errs, err)
		return t
	}
	num := func(s string) int {
		v, err := strconv.Atoi(s)
		errs = append(errs, err)
		return v
	}
	row := Row{
		Session:  rec[0],
		Repo:     rec[1],
		Start:    at(rec[2]),
		End:      at(rec[3]),
		Main:     transcript.Usage{Input: num(rec[5]), Output: num(rec[6]), CacheRead: num(rec[7]), CacheWrite: num(rec[8])},
		Subagent: transcript.Usage{Input: num(rec[9]), Output: num(rec[10]), CacheRead: num(rec[11]), CacheWrite: num(rec[12])},
	}
	if rec[4] != "" {
		row.Models = strings.Split(rec[4], ";")
	}
	return row, errors.Join(errs...)
}
