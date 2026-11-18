package worktree

import (
	"slices"
	"testing"
)

func TestParsePorcelainListsWorktreePaths(t *testing.T) {
	out := "worktree /r/shop\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /w/feature\nHEAD def\nbranch refs/heads/feature\n\n" +
		"worktree /w/old\nHEAD 123\ndetached\nprunable gitdir file points to non-existent location\n"

	got := parsePorcelain(out)

	if want := []string{"/r/shop", "/w/feature", "/w/old"}; !slices.Equal(got, want) {
		t.Fatalf("parsePorcelain = %v, want %v", got, want)
	}
}

func TestListOfAFolderThatIsNotAGitRepoIsEmpty(t *testing.T) {
	if got := List(t.TempDir()); len(got) != 0 {
		t.Fatalf("List = %v, want nothing", got)
	}
}
