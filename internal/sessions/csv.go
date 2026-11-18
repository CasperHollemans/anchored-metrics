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

var header = []string{
	"session", "repo", "start", "end", "models",
	"main_input", "main_output", "main_cache_read", "main_cache_write",
	"subagent_input", "subagent_output", "subagent_cache_read", "subagent_cache_write",
}

const timeFormat = time.RFC3339 // UTC, to the second

// ReadCSV reads sessions.csv. A missing file has no rows. A file it can't
// fully understand is an error, so it is never overwritten by mistake.
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

// WriteCSV writes sessions.csv to a temporary file and renames it into place,
// so a failed write never damages the existing file.
func WriteCSV(path string, rows []Row) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".sessions-*.csv")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	w := csv.NewWriter(tmp)
	w.Write(header)
	for _, r := range rows {
		w.Write(toRecord(r))
	}
	w.Flush()
	if err := errors.Join(w.Error(), tmp.Close()); err != nil {
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
	usage := func(cols []string) transcript.Usage {
		return transcript.Usage{Input: num(cols[0]), Output: num(cols[1]), CacheRead: num(cols[2]), CacheWrite: num(cols[3])}
	}
	row := Row{
		Session:  rec[0],
		Repo:     rec[1],
		Start:    at(rec[2]),
		End:      at(rec[3]),
		Main:     usage(rec[5:9]),
		Subagent: usage(rec[9:13]),
	}
	if rec[4] != "" {
		row.Models = strings.Split(rec[4], ";")
	}
	return row, errors.Join(errs...)
}
