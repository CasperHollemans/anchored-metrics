package sessions

import (
	"path/filepath"

	"anchored-metrics/internal/transcript"
	"anchored-metrics/internal/worktree"
)

// Result tells what a sync did.
type Result struct {
	Repo     string // repo name, as written in the CSV
	Sessions int    // sessions found and synced
	Skipped  int    // unreadable transcript lines
}

// Sync reads every session of the repo (and its worktrees) and updates the CSV
// at outPath (docs/decisions/0005-sync-replaces-changed-sessions.md). With no
// sessions it writes nothing.
//
// Every transcript is re-read on each sync. Re-reading an unchanged transcript
// gives an identical row, so this is simple and always correct; skipping
// unchanged files is an optimisation for later, if ever needed.
func Sync(home, repoPath, outPath string) (Result, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return Result{}, err
	}
	res := Result{Repo: filepath.Base(repoPath)}
	folders, err := transcript.RepoFolders(transcript.ProjectsDir(home), repoPath, worktree.List(repoPath))
	if err != nil {
		return res, err
	}

	var rows []Row
	for _, folder := range folders {
		found, err := transcript.Sessions(folder)
		if err != nil {
			return res, err
		}
		for _, s := range found {
			row, ok, skipped, err := readSession(s, res.Repo)
			if err != nil {
				return res, err
			}
			res.Sessions++
			res.Skipped += skipped
			if ok {
				rows = append(rows, row)
			}
		}
	}
	if res.Sessions == 0 {
		return res, nil
	}

	existing, err := ReadCSV(outPath)
	if err != nil {
		return res, err
	}
	return res, WriteCSV(outPath, Merge(existing, rows))
}

// readSession parses a session's main and subagent transcripts into a row.
func readSession(s transcript.Session, repo string) (row Row, ok bool, skipped int, err error) {
	main, err := transcript.ParseFile(s.Main)
	if err != nil {
		return Row{}, false, 0, err
	}
	skipped = main.Skipped
	var subs []transcript.Transcript
	for _, path := range s.Subagents {
		sub, err := transcript.ParseFile(path)
		if err != nil {
			return Row{}, false, 0, err
		}
		skipped += sub.Skipped
		subs = append(subs, sub)
	}
	row, ok = Summarize(s.ID, repo, main, subs)
	return row, ok, skipped, nil
}
