package sessions

import (
	"os"
	"path/filepath"
	"testing"

	"anchored-metrics/internal/transcript"
)

// A CSV that can't be read is never overwritten: the sync stops with an error
// and the file stays as it was, so no data is lost.
func TestSyncNeverOverwritesACSVItCannotRead(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "repo")
	session := filepath.Join(transcript.ProjectsDir(home), transcript.FolderName(repo), "abc.jsonl")
	if err := os.MkdirAll(filepath.Dir(session), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRaw(t, session, `{"type":"user","timestamp":"2026-11-01T09:00:00.000Z"}`+"\n")
	out := filepath.Join(root, "sessions.csv")
	corrupt := "not,our,columns\n1,2,3\n"
	writeRaw(t, out, corrupt)

	_, err := Sync(home, repo, out)

	after, _ := os.ReadFile(out)
	if err == nil || string(after) != corrupt {
		t.Fatalf("Sync error = %v; CSV after =\n%s\nwant an error and the CSV unchanged", err, after)
	}
}
