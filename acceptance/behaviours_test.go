package acceptance

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// Every behaviour in docs/stories has an acceptance test, and every acceptance
// test points to a behaviour that exists, so the docs and tests can't drift.
func TestEveryBehaviourHasAnAcceptanceTest(t *testing.T) {
	defined := idsIn(t, "../docs/stories/*.md", regexp.MustCompile(`\*\*([A-Z]-\d+)\*\*`))
	tested := idsIn(t, "*_test.go", regexp.MustCompile(`(?m)^// ([A-Z]-\d+) `))

	for _, id := range sortedKeys(defined) {
		if !tested[id] {
			t.Errorf("behaviour %s has no acceptance test", id)
		}
	}
	for _, id := range sortedKeys(tested) {
		if !defined[id] {
			t.Errorf("an acceptance test refers to %s, which isn't in docs/stories", id)
		}
	}
}

func idsIn(t *testing.T, pattern string, re *regexp.Regexp) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) == 0 {
		t.Fatalf("no files match %s", pattern)
	}
	ids := map[string]bool{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			ids[m[1]] = true
		}
	}
	return ids
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
