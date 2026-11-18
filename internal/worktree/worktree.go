// Package worktree asks git which worktrees a repo has.
package worktree

import (
	"os/exec"
	"strings"
)

// List returns the paths of the repo's git worktrees, or nothing if git
// can't tell (not a git repo, git not installed). It runs git locally only.
func List(repoPath string) []string {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parsePorcelain(string(out))
}

func parsePorcelain(out string) []string {
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			paths = append(paths, path)
		}
	}
	return paths
}
