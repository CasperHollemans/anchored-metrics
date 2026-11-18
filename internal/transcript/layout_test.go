package transcript

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFolderNameReplacesNonAlphanumericCharacters(t *testing.T) {
	got := FolderName("/Users/x/.paradev/my_repo")

	if want := "-Users-x--paradev-my-repo"; got != want {
		t.Fatalf("FolderName = %q, want %q", got, want)
	}
}

func TestRepoFoldersFindsOwnClaudeWorktreeAndGitWorktreeFolders(t *testing.T) {
	projects := t.TempDir()
	for _, name := range []string{
		"-r-shop",
		"-r-shop--claude-worktrees-fix",
		"-r-shop-admin",
		"-r-shop-docs",
		"-w-feature",
		"-other",
	} {
		touch(t, filepath.Join(projects, name, "s.jsonl"))
	}

	got, err := RepoFolders(projects, "/r/shop", []string{"/r/shop", "/w/feature", "/w/gone"})

	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(projects, "-r-shop"),
		filepath.Join(projects, "-r-shop--claude-worktrees-fix"),
		filepath.Join(projects, "-w-feature"),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("RepoFolders = %v, want %v", got, want)
	}
}

func TestRepoFoldersIsEmptyWithoutProjectsDir(t *testing.T) {
	got, err := RepoFolders(filepath.Join(t.TempDir(), "missing"), "/r/shop", nil)

	if err != nil || len(got) != 0 {
		t.Fatalf("RepoFolders = %v, %v; want nothing, no error", got, err)
	}
}

func TestRepoFoldersReportsAProjectsDirItCannotRead(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "projects")
	touch(t, notADir)

	if _, err := RepoFolders(notADir, "/r/shop", nil); err == nil {
		t.Fatal("want an error when the projects dir can't be read")
	}
}

func TestSessionsFindsTranscriptsAndTheirSubagents(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "abc.jsonl"))
	touch(t, filepath.Join(dir, "abc", "subagents", "agent-a1.jsonl"))
	touch(t, filepath.Join(dir, "abc", "subagents", "agent-a1.meta.json"))
	touch(t, filepath.Join(dir, "abc", "custom-title.json"))
	touch(t, filepath.Join(dir, "abc", "tool-results", "x.png"))
	touch(t, filepath.Join(dir, "def.jsonl"))
	touch(t, filepath.Join(dir, "x.desktop-released.json"))
	touch(t, filepath.Join(dir, "memory", "MEMORY.md"))

	got, err := Sessions(dir)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2: %+v", len(got), got)
	}
	abc := got[0]
	if abc.ID != "abc" || abc.Main != filepath.Join(dir, "abc.jsonl") ||
		!slices.Equal(abc.Subagents, []string{filepath.Join(dir, "abc", "subagents", "agent-a1.jsonl")}) {
		t.Fatalf("abc = %+v", abc)
	}
	if got[1].ID != "def" || len(got[1].Subagents) != 0 {
		t.Fatalf("def = %+v", got[1])
	}
}
