// Command anchored-metrics reads Claude Code session transcripts and writes
// metrics as CSV. See docs/stories/sessions.md.
//
//	anchored-metrics sessions <repo-path>
//
// writes out/sessions.csv in the current directory.
package main

import (
	"fmt"
	"os"

	"anchored-metrics/internal/sessions"
)

const outPath = "out/sessions.csv"

func main() {
	if len(os.Args) != 3 || os.Args[1] != "sessions" {
		fmt.Fprintln(os.Stderr, "usage: anchored-metrics sessions <repo-path>")
		os.Exit(2)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}
	res, err := sessions.Sync(home, os.Args[2], outPath)
	if err != nil {
		fail(err)
	}
	if res.Skipped > 0 {
		fmt.Fprintf(os.Stderr, "warning: skipped %d unreadable %s\n", res.Skipped, plural(res.Skipped, "line", "lines"))
	}
	if res.Sessions == 0 {
		fmt.Fprintf(os.Stderr, "warning: no Claude Code sessions found for %s; nothing written\n", res.Repo)
		return
	}
	fmt.Printf("synced %d %s for %s to %s\n", res.Sessions, plural(res.Sessions, "session", "sessions"), res.Repo, outPath)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
