package transcript

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ProjectsDir is where Claude Code keeps its project folders.
func ProjectsDir(home string) string { return filepath.Join(home, ".claude", "projects") }

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// FolderName is the project folder name Claude Code uses for a path. The
// encoding is lossy, so folder names are compared, never decoded.
func FolderName(path string) string { return nonAlphanumeric.ReplaceAllString(path, "-") }

// claudeWorktrees is how <repo>/.claude/worktrees/ shows up in a folder name.
var claudeWorktrees = FolderName("/.claude/worktrees/")

// RepoFolders lists the existing project folders that belong to a repo: its
// own, those of Claude Code worktrees inside it, and those of the given git
// worktree paths (docs/decisions/0007-worktree-mapping.md).
func RepoFolders(projectsDir, repoPath string, worktrees []string) ([]string, error) {
	entries, err := os.ReadDir(projectsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	own := FolderName(repoPath)
	wanted := map[string]bool{own: true}
	for _, w := range worktrees {
		wanted[FolderName(w)] = true
	}
	var folders []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() && (wanted[name] || strings.HasPrefix(name, own+claudeWorktrees)) {
			folders = append(folders, filepath.Join(projectsDir, name))
		}
	}
	sort.Strings(folders)
	return folders, nil
}

// Session is one session's transcript files.
type Session struct {
	ID        string
	Main      string
	Subagents []string
}

// Sessions lists the sessions in a project folder: each <id>.jsonl, with the
// subagent transcripts in <id>/subagents/agent-*.jsonl. Everything else is ignored.
func Sessions(folder string) ([]Session, error) {
	mains, err := filepath.Glob(filepath.Join(folder, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(mains)
	var sessions []Session
	for _, main := range mains {
		id := strings.TrimSuffix(filepath.Base(main), ".jsonl")
		subs, err := filepath.Glob(filepath.Join(folder, id, "subagents", "agent-*.jsonl"))
		if err != nil {
			return nil, err
		}
		sort.Strings(subs)
		sessions = append(sessions, Session{ID: id, Main: main, Subagents: subs})
	}
	return sessions, nil
}
