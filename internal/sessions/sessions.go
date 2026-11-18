// Package sessions turns transcripts into sessions.csv rows and keeps the
// CSV up to date (docs/stories/sessions.md).
package sessions

import (
	"cmp"
	"maps"
	"slices"
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
	times := slices.Clone(main.Times)

	row.Main = addReplies(main.Replies, models)
	for _, sub := range subagents {
		times = append(times, sub.Times...)
		row.Subagent = row.Subagent.Add(addReplies(sub.Replies, models))
	}
	if len(times) == 0 {
		return Row{}, false
	}
	row.Start = slices.MinFunc(times, time.Time.Compare)
	row.End = slices.MaxFunc(times, time.Time.Compare)
	row.Models = slices.Sorted(maps.Keys(models))
	return row, true
}

// addReplies sums the replies' usage and records their models.
func addReplies(replies []transcript.Reply, models map[string]bool) transcript.Usage {
	var total transcript.Usage
	for _, r := range replies {
		total = total.Add(r.Usage)
		models[r.Model] = true
	}
	return total
}

// Merge replaces existing rows by the freshly synced ones and keeps every
// other row, so sessions whose transcript is gone are never lost. Rows are
// ordered by start, then session, so the CSV is stable between syncs.
func Merge(existing, synced []Row) []Row {
	byID := map[string]Row{}
	for _, r := range slices.Concat(existing, synced) {
		byID[r.Session] = r
	}
	rows := slices.Collect(maps.Values(byID))
	slices.SortFunc(rows, func(a, b Row) int {
		return cmp.Or(a.Start.Compare(b.Start), strings.Compare(a.Session, b.Session))
	})
	return rows
}
