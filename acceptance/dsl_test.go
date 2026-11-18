package acceptance

// The acceptance test DSL (docs/decisions/0002-acceptance-test-dsl.md).
//
// Record builders (UserPrompt, AssistantReply, ...) are the only test code that
// knows the transcript format. They follow docs/transcript-format.md. When the
// format changes, change them here, never in the behaviour tests.

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	defaultDay    = "2026-11-01"
	defaultRepo   = "shop"
	defaultModel  = "claude-opus-5-5"
	claudeVersion = "2.1.322"
)

var binary string

// TestMain builds the real binary once; every scenario runs it end to end.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "anchored-metrics-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "anchored-metrics")
	build := exec.Command("go", "build", "-o", binary, "../cmd/anchored-metrics")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building anchored-metrics failed:", err)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// ---------------------------------------------------------------- scenario

// Scenario is one isolated world: a fake home with ~/.claude/projects, a repos
// folder, and a working directory where out/*.csv is written.
type Scenario struct {
	t     *testing.T
	root  string
	home  string
	work  string
	Given *given
	When  *when
	Then  *then

	sessions  map[string]sessionFile
	library   string // path of a custom correction library, if given
	run       runResult
	outBefore map[string][]byte // out/ files before the last command
}

type sessionFile struct {
	path string
	cwd  string
}

type runResult struct {
	ran    bool
	stdout string
	stderr string
	code   int
}

func scenario(t *testing.T) *Scenario {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &Scenario{
		t:        t,
		root:     root,
		home:     filepath.Join(root, "home"),
		work:     filepath.Join(root, "work"),
		sessions: map[string]sessionFile{},
	}
	mustMkdir(t, filepath.Join(s.home, ".claude", "projects"))
	mustMkdir(t, s.work)
	mustMkdir(t, s.repoPath(defaultRepo))
	s.Given, s.When, s.Then = &given{s}, &when{s}, &then{s}
	return s
}

func (s *Scenario) repoPath(repo string) string     { return filepath.Join(s.root, "repos", repo) }
func (s *Scenario) worktreePath(name string) string { return filepath.Join(s.root, "worktrees", name) }
func (s *Scenario) outDir() string                  { return filepath.Join(s.work, "out") }
func (s *Scenario) csvPath(name string) string      { return filepath.Join(s.outDir(), name) }
func (s *Scenario) labelsPath() string {
	return filepath.Join(s.home, ".anchored-metrics", "labels.csv")
}

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// projectDir mirrors how Claude Code names a project folder after its cwd.
func (s *Scenario) projectDir(cwd string) string {
	return filepath.Join(s.home, ".claude", "projects", nonAlphanumeric.ReplaceAllString(cwd, "-"))
}

func (s *Scenario) env() []string {
	return []string{
		"HOME=" + s.home,
		"PATH=" + os.Getenv("PATH"),
		"GIT_CONFIG_NOSYSTEM=1",
	}
}

func (s *Scenario) writeSession(cwd, id string, records []Record) {
	s.t.Helper()
	f := sessionFile{path: filepath.Join(s.projectDir(cwd), id+".jsonl"), cwd: cwd}
	s.sessions[id] = f
	appendLines(s.t, f.path, render(recordContext{sessionID: id, cwd: cwd}, records))
}

func (s *Scenario) session(id string) sessionFile {
	s.t.Helper()
	f, ok := s.sessions[id]
	if !ok {
		s.t.Fatalf("test bug: session %q was never given", id)
	}
	return f
}

// ------------------------------------------------------------------- given

type given struct{ s *Scenario }

// Session is a session in the default repo.
func (g *given) Session(id string, records ...Record) {
	g.s.t.Helper()
	g.SessionIn(defaultRepo, id, records...)
}

// SessionIn is a session in the named repo.
func (g *given) SessionIn(repo, id string, records ...Record) {
	g.s.t.Helper()
	mustMkdir(g.s.t, g.s.repoPath(repo))
	g.s.writeSession(g.s.repoPath(repo), id, records)
}

// SessionInClaudeWorktree is a session in a worktree Claude Code made inside
// the default repo, at <repo>/.claude/worktrees/<name>.
func (g *given) SessionInClaudeWorktree(name, id string, records ...Record) {
	g.s.t.Helper()
	cwd := filepath.Join(g.s.repoPath(defaultRepo), ".claude", "worktrees", name)
	mustMkdir(g.s.t, cwd)
	g.s.writeSession(cwd, id, records)
}

// GitWorktree makes the default repo a git repo and adds a worktree outside
// it, the way an editor or a custom script would.
func (g *given) GitWorktree(name string) {
	g.s.t.Helper()
	repo := g.s.repoPath(defaultRepo)
	if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
		g.git(repo, "init", "-q", "-b", "main")
		g.git(repo, "-c", "user.email=test@example.com", "-c", "user.name=test",
			"commit", "-q", "--allow-empty", "-m", "init")
	}
	mustMkdir(g.s.t, filepath.Dir(g.s.worktreePath(name)))
	g.git(repo, "worktree", "add", "-q", "-b", name, g.s.worktreePath(name))
}

// SessionInGitWorktree is a session in a worktree made with GitWorktree.
func (g *given) SessionInGitWorktree(name, id string, records ...Record) {
	g.s.t.Helper()
	g.s.writeSession(g.s.worktreePath(name), id, records)
}

// Subagent is a subagent transcript stored next to its parent session. Link
// it to a prompt with SpawnsSubagent in the parent session.
func (g *given) Subagent(sessionID, agentID string, records ...Record) {
	g.s.t.Helper()
	parent := g.s.session(sessionID)
	dir := filepath.Join(filepath.Dir(parent.path), sessionID, "subagents")
	ctx := recordContext{sessionID: sessionID, cwd: parent.cwd, agentID: agentID}
	appendLines(g.s.t, filepath.Join(dir, "agent-"+agentID+".jsonl"), render(ctx, records))
	meta := mustJSON(map[string]any{
		"agentType": "general-purpose", "description": "helper task",
		"toolUseId": newID("toolu"), "spawnDepth": 1,
		"requestShape": "default", "requestNonInteractive": true,
	})
	writeFile(g.s.t, filepath.Join(dir, "agent-"+agentID+".meta.json"), meta)
}

// MoreRecords appends to an existing session, as when it keeps running.
func (g *given) MoreRecords(sessionID string, records ...Record) {
	g.s.t.Helper()
	f := g.s.session(sessionID)
	appendLines(g.s.t, f.path, render(recordContext{sessionID: sessionID, cwd: f.cwd}, records))
}

// SessionDeleted removes a session's transcript, as Claude Code's cleanup does.
func (g *given) SessionDeleted(id string) {
	g.s.t.Helper()
	f := g.s.session(id)
	if err := os.Remove(f.path); err != nil {
		g.s.t.Fatal(err)
	}
	os.RemoveAll(strings.TrimSuffix(f.path, ".jsonl"))
}

// FileInProjectFolder is any other file in the default repo's project folder.
func (g *given) FileInProjectFolder(relPath, content string) {
	g.s.t.Helper()
	writeFile(g.s.t, filepath.Join(g.s.projectDir(g.s.repoPath(defaultRepo)), relPath), content)
}

// CorrectionLibrary replaces the built-in correction library for this
// scenario. Lines are trimmed, so the text can be indented.
func (g *given) CorrectionLibrary(text string) {
	g.s.t.Helper()
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(text), "\n") {
		lines = append(lines, strings.TrimSpace(l))
	}
	g.s.library = filepath.Join(g.s.root, "corrections.txt")
	writeFile(g.s.t, g.s.library, strings.Join(lines, "\n")+"\n")
}

// Labelled is a label the user gave earlier, stored in the labels file.
func (g *given) Labelled(session string, prompt int, l label) {
	g.s.t.Helper()
	path := g.s.labelsPath()
	if _, err := os.Stat(path); err != nil {
		appendLines(g.s.t, path, []string{"session,prompt,correction,categories"})
	}
	appendLines(g.s.t, path, []string{fmt.Sprintf("%s,%d,%s,%s", session, prompt, l.correction, l.categories)})
}

func (g *given) git(dir string, args ...string) {
	g.s.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = g.s.env()
	if out, err := cmd.CombinedOutput(); err != nil {
		g.s.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// -------------------------------------------------------------------- when

type when struct{ s *Scenario }

// Sync runs `anchored-metrics sync <default repo>`.
func (w *when) Sync() {
	w.s.t.Helper()
	w.SyncRepo(defaultRepo)
}

// SyncRepo runs `anchored-metrics sync <repo>`.
func (w *when) SyncRepo(repo string) {
	w.s.t.Helper()
	w.s.runCommand("sync", repo, "")
}

// Label runs `anchored-metrics label <default repo>`, typing each answer on
// its own line: y, n, s or q, and after a y the categories, e.g. "style cleanup".
func (w *when) Label(answers ...string) {
	w.s.t.Helper()
	w.s.runCommand("label", defaultRepo, strings.Join(answers, "\n")+"\n")
}

// Score runs `anchored-metrics score <default repo>`.
func (w *when) Score() {
	w.s.t.Helper()
	w.s.runCommand("score", defaultRepo, "")
}

func (s *Scenario) runCommand(command, repo, stdin string) {
	s.t.Helper()
	s.outBefore = s.outFiles()
	args := []string{command}
	if s.library != "" {
		args = append(args, "--corrections", s.library)
	}
	args = append(args, s.repoPath(repo))
	cmd := exec.Command(binary, args...)
	cmd.Dir = s.work
	cmd.Env = s.env()
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			s.t.Fatalf("running anchored-metrics: %v", err)
		}
		code = exit.ExitCode()
	}
	s.run = runResult{ran: true, stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func (s *Scenario) outFiles() map[string][]byte {
	files := map[string][]byte{}
	entries, _ := os.ReadDir(s.outDir())
	for _, e := range entries {
		if b, err := os.ReadFile(filepath.Join(s.outDir(), e.Name())); err == nil {
			files[e.Name()] = b
		}
	}
	return files
}

// -------------------------------------------------------------------- then

type then struct{ s *Scenario }

// Session checks the one sessions.csv row for this session. Has(...) checks
// only the columns it lists.
func (th *then) Session(id string) *row {
	return &row{s: th.s, file: "sessions.csv", what: "session " + id,
		match: func(r map[string]string) bool { return r["session"] == id }}
}

// Prompt checks the one prompts.csv row for the n-th prompt of a session.
func (th *then) Prompt(session string, n int) *row {
	return &row{s: th.s, file: "prompts.csv", what: fmt.Sprintf("prompt %d of session %s", n, session),
		match: func(r map[string]string) bool { return r["session"] == session && r["prompt"] == strconv.Itoa(n) }}
}

type row struct {
	s     *Scenario
	file  string
	what  string
	match func(map[string]string) bool
}

func (r *row) Has(columns ...Column) {
	r.s.t.Helper()
	var found []map[string]string
	for _, rec := range r.s.rows(r.file) {
		if r.match(rec) {
			found = append(found, rec)
		}
	}
	if len(found) != 1 {
		r.s.t.Fatalf("want exactly 1 row for %s in %s, got %d%s", r.what, r.file, len(found), r.s.runInfo())
	}
	for _, c := range columns {
		got, ok := found[0][c.name]
		if !ok {
			r.s.t.Errorf("%s has no column %q", r.file, c.name)
			continue
		}
		if got != c.value {
			r.s.t.Errorf("%s: %s = %q, want %q", r.what, c.name, got, c.value)
		}
	}
}

// Sessions checks that sessions.csv holds exactly these sessions, one row each.
func (th *then) Sessions(ids ...string) {
	th.s.t.Helper()
	var got []string
	for _, r := range th.s.rows("sessions.csv") {
		got = append(got, r["session"])
	}
	slices.Sort(got)
	want := slices.Sorted(slices.Values(ids))
	if !slices.Equal(got, want) {
		th.s.t.Fatalf("sessions in sessions.csv = %v, want %v%s", got, want, th.s.runInfo())
	}
}

// Prompts checks that prompts.csv holds exactly n prompts for the session,
// numbered 1 to n.
func (th *then) Prompts(session string, n int) {
	th.s.t.Helper()
	var got []string
	for _, r := range th.s.rows("prompts.csv") {
		if r["session"] == session {
			got = append(got, r["prompt"])
		}
	}
	slices.Sort(got)
	var want []string
	for i := 1; i <= n; i++ {
		want = append(want, strconv.Itoa(i))
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		th.s.t.Fatalf("prompts of session %s in prompts.csv = %v, want %v%s", session, got, want, th.s.runInfo())
	}
}

// NoCSVWritten checks that no CSV was written at all.
func (th *then) NoCSVWritten() {
	th.s.t.Helper()
	for _, name := range []string{"sessions.csv", "prompts.csv"} {
		if _, err := os.Stat(th.s.csvPath(name)); err == nil {
			th.s.t.Errorf("want no %s, but it was written", name)
		}
	}
}

func (th *then) ExitedNormally() {
	th.s.t.Helper()
	th.s.mustHaveRun()
	if th.s.run.code != 0 {
		th.s.t.Fatalf("exit code = %d, want 0%s", th.s.run.code, th.s.runInfo())
	}
}

// Warned checks that the last command printed a warning containing text.
func (th *then) Warned(text string) {
	th.s.t.Helper()
	th.s.mustHaveRun()
	if !strings.Contains(strings.ToLower(th.s.run.stderr), "warning") || !strings.Contains(th.s.run.stderr, text) {
		th.s.t.Fatalf("want a warning containing %q on stderr%s", text, th.s.runInfo())
	}
}

// Reported checks that the last command printed text, on stdout or stderr.
func (th *then) Reported(text string) {
	th.s.t.Helper()
	th.s.mustHaveRun()
	if !strings.Contains(th.s.run.stdout+th.s.run.stderr, text) {
		th.s.t.Fatalf("want output containing %q%s", text, th.s.runInfo())
	}
}

// ShowedInOrder checks that the last command printed these texts, in this order.
func (th *then) ShowedInOrder(texts ...string) {
	th.s.t.Helper()
	th.s.mustHaveRun()
	out := th.s.run.stdout
	pos := 0
	for _, text := range texts {
		i := strings.Index(out[pos:], text)
		if i < 0 {
			th.s.t.Fatalf("want %q shown after the earlier texts %q%s", text, texts, th.s.runInfo())
		}
		pos += i + len(text)
	}
}

// DidNotShow checks that the last command never printed text.
func (th *then) DidNotShow(text string) {
	th.s.t.Helper()
	th.s.mustHaveRun()
	if strings.Contains(th.s.run.stdout, text) {
		th.s.t.Fatalf("want %q not shown%s", text, th.s.runInfo())
	}
}

// CSVsUnchangedByLastSync checks that the last command left every CSV in out/
// byte for byte as it was.
func (th *then) CSVsUnchangedByLastSync() {
	th.s.t.Helper()
	after := th.s.outFiles()
	if len(after) == 0 {
		th.s.t.Fatalf("no CSVs in out/%s", th.s.runInfo())
	}
	for name, b := range after {
		if !bytes.Equal(th.s.outBefore[name], b) {
			th.s.t.Errorf("%s changed.\nbefore:\n%s\nafter:\n%s", name, th.s.outBefore[name], b)
		}
	}
}

// CSVsHaveNoPaths checks that no file path (and so no username) leaks into any CSV.
func (th *then) CSVsHaveNoPaths() {
	th.s.t.Helper()
	for name, content := range th.s.csvContents() {
		if strings.Contains(content, "/") {
			th.s.t.Errorf("%s contains a path:\n%s", name, content)
		}
	}
}

// CSVsDoNotContain checks that none of these texts appear in any CSV.
func (th *then) CSVsDoNotContain(texts ...string) {
	th.s.t.Helper()
	for name, content := range th.s.csvContents() {
		for _, text := range texts {
			if strings.Contains(strings.ToLower(content), strings.ToLower(text)) {
				th.s.t.Errorf("%s contains %q:\n%s", name, text, content)
			}
		}
	}
}

// Label checks the stored label of the n-th prompt of a session.
func (th *then) Label(session string, prompt int) *labelCheck {
	return &labelCheck{s: th.s, session: session, prompt: prompt}
}

type labelCheck struct {
	s       *Scenario
	session string
	prompt  int
}

func (c *labelCheck) Is(want label) {
	c.s.t.Helper()
	got, ok := c.s.labels()[fmt.Sprintf("%s/%d", c.session, c.prompt)]
	if !ok {
		c.s.t.Fatalf("prompt %d of session %s has no label%s", c.prompt, c.session, c.s.runInfo())
	}
	if got != want {
		c.s.t.Fatalf("label of prompt %d of session %s = %+v, want %+v", c.prompt, c.session, got, want)
	}
}

func (c *labelCheck) IsMissing() {
	c.s.t.Helper()
	if _, ok := c.s.labels()[fmt.Sprintf("%s/%d", c.session, c.prompt)]; ok {
		c.s.t.Fatalf("want no label for prompt %d of session %s", c.prompt, c.session)
	}
}

// LabelledPrompts checks how many prompts have a label, whichever they are.
func (th *then) LabelledPrompts(n int) {
	th.s.t.Helper()
	got := 0
	if _, err := os.Stat(th.s.labelsPath()); err == nil {
		got = len(th.s.labels())
	}
	if got != n {
		th.s.t.Fatalf("labelled prompts = %d, want %d%s", got, n, th.s.runInfo())
	}
}

// LabelsDoNotContain checks that none of these texts appear in the labels file.
func (th *then) LabelsDoNotContain(texts ...string) {
	th.s.t.Helper()
	b, err := os.ReadFile(th.s.labelsPath())
	if err != nil {
		th.s.t.Fatalf("reading labels: %v%s", err, th.s.runInfo())
	}
	for _, text := range texts {
		if strings.Contains(strings.ToLower(string(b)), strings.ToLower(text)) {
			th.s.t.Errorf("labels file contains %q:\n%s", text, b)
		}
	}
}

func (s *Scenario) mustHaveRun() {
	s.t.Helper()
	if !s.run.ran {
		s.t.Fatal("test bug: no When step ran")
	}
}

func (s *Scenario) runInfo() string {
	if !s.run.ran {
		return ""
	}
	return fmt.Sprintf("\nexit code: %d\nstdout:\n%s\nstderr:\n%s", s.run.code, s.run.stdout, s.run.stderr)
}

func (s *Scenario) csvContents() map[string]string {
	s.t.Helper()
	files := s.outFiles()
	if len(files) == 0 {
		s.t.Fatalf("no CSVs in out/%s", s.runInfo())
	}
	contents := map[string]string{}
	for name, b := range files {
		contents[name] = string(b)
	}
	return contents
}

func (s *Scenario) rows(file string) []map[string]string {
	s.t.Helper()
	b, err := os.ReadFile(s.csvPath(file))
	if err != nil {
		s.t.Fatalf("reading %s: %v%s", file, err, s.runInfo())
	}
	return parseCSV(s.t, file, b)
}

func (s *Scenario) labels() map[string]label {
	s.t.Helper()
	b, err := os.ReadFile(s.labelsPath())
	if err != nil {
		s.t.Fatalf("reading labels: %v%s", err, s.runInfo())
	}
	labels := map[string]label{}
	for _, r := range parseCSV(s.t, "labels.csv", b) {
		labels[r["session"]+"/"+r["prompt"]] = label{correction: r["correction"], categories: r["categories"]}
	}
	return labels
}

func parseCSV(t *testing.T, name string, b []byte) []map[string]string {
	t.Helper()
	records, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	if len(records) == 0 {
		t.Fatalf("%s is empty", name)
	}
	header := records[0]
	var rows []map[string]string
	for _, rec := range records[1:] {
		r := map[string]string{}
		for i, col := range header {
			if i < len(rec) {
				r[col] = rec[i]
			}
		}
		rows = append(rows, r)
	}
	return rows
}

// ----------------------------------------------------------------- columns

// Column is one expected value in a CSV row.
type Column struct{ name, value string }

func Repo(name string) Column  { return Column{"repo", name} }
func Start(when string) Column { return Column{"start", utc(when)} }
func End(when string) Column   { return Column{"end", utc(when)} }

// Models lists the expected models in any order; the CSV sorts them.
func Models(names ...string) Column { return Column{"models", sortedList(names)} }

func MainInput(n int) Column          { return Column{"main_input", strconv.Itoa(n)} }
func MainOutput(n int) Column         { return Column{"main_output", strconv.Itoa(n)} }
func MainCacheRead(n int) Column      { return Column{"main_cache_read", strconv.Itoa(n)} }
func MainCacheWrite(n int) Column     { return Column{"main_cache_write", strconv.Itoa(n)} }
func SubagentInput(n int) Column      { return Column{"subagent_input", strconv.Itoa(n)} }
func SubagentOutput(n int) Column     { return Column{"subagent_output", strconv.Itoa(n)} }
func SubagentCacheRead(n int) Column  { return Column{"subagent_cache_read", strconv.Itoa(n)} }
func SubagentCacheWrite(n int) Column { return Column{"subagent_cache_write", strconv.Itoa(n)} }

// prompts.csv columns
func At(when string) Column           { return Column{"at", utc(when)} }
func Words(n int) Column              { return Column{"words", strconv.Itoa(n)} }
func Interrupted(b bool) Column       { return Column{"interrupted", strconv.FormatBool(b)} }
func AskedQuestion(b bool) Column     { return Column{"asked_question", strconv.FormatBool(b)} }
func FilesChanged(n int) Column       { return Column{"files_changed", strconv.Itoa(n)} }
func LinesAdded(n int) Column         { return Column{"lines_added", strconv.Itoa(n)} }
func LinesRemoved(n int) Column       { return Column{"lines_removed", strconv.Itoa(n)} }
func Correction(b bool) Column        { return Column{"correction", strconv.FormatBool(b)} }
func CorrectionHits(n int) Column     { return Column{"correction_hits", strconv.Itoa(n)} }
func CorrectionCategories(names ...string) Column {
	return Column{"correction_categories", sortedList(names)}
}

func sortedList(names []string) string {
	return strings.Join(slices.Sorted(slices.Values(names)), ";")
}

// ------------------------------------------------------------------ labels

type label struct{ correction, categories string }

// Yes is a correction label with its categories (any order).
func Yes(categories ...string) label { return label{"yes", sortedList(categories)} }

// No is a not-a-correction label.
func No() label { return label{"no", ""} }

// ------------------------------------------------------------------- times

type moment struct{ t time.Time }

// at reads "09:00", "09:00 +01:00", "08:00Z" or "2026-11-02 09:00 +01:00".
// Without a date it is on defaultDay; without an offset it is UTC.
func at(s string) moment { return moment{parseMoment(s)} }

var hasDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} `)

func parseMoment(s string) time.Time {
	v := s
	if !hasDate.MatchString(v) {
		v = defaultDay + " " + v
	}
	for _, layout := range []string{"2006-01-02 15:04 -07:00", "2006-01-02 15:04Z07:00", "2006-01-02 15:04"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t
		}
	}
	panic(fmt.Sprintf("test bug: can't read time %q", s))
}

func utc(s string) string { return parseMoment(s).UTC().Format(time.RFC3339) }

func stamp(m moment) string { return m.t.Format("2006-01-02T15:04:05.000Z07:00") }

// ----------------------------------------------------------------- records

type recordContext struct {
	sessionID string
	cwd       string
	agentID   string // set for subagent transcripts
}

// Record renders to one or more transcript lines.
type Record func(c recordContext) []string

func render(c recordContext, records []Record) []string {
	var lines []string
	for _, r := range records {
		lines = append(lines, r(c)...)
	}
	return lines
}

// base holds the fields common to user and assistant records.
func base(c recordContext, typ string, m moment) map[string]any {
	r := map[string]any{
		"type":        typ,
		"sessionId":   c.sessionID,
		"uuid":        newID("uuid"),
		"parentUuid":  nil,
		"timestamp":   stamp(m),
		"cwd":         c.cwd,
		"gitBranch":   "main",
		"isSidechain": c.agentID != "",
		"userType":    "external",
		"entrypoint":  "cli",
		"version":     claudeVersion,
	}
	if c.agentID != "" {
		r["agentId"] = c.agentID
	}
	return r
}

type prompt struct {
	text  string
	image bool
}

type PromptOption func(*prompt)

func Text(text string) PromptOption { return func(p *prompt) { p.text = text } }

// WithImage sends the prompt as a pasted image plus text, as Claude Code
// does when the user pastes a screenshot.
func WithImage() PromptOption { return func(p *prompt) { p.image = true } }

// UserPrompt is a prompt typed by the user.
func UserPrompt(m moment, opts ...PromptOption) Record {
	p := prompt{text: "please help"}
	for _, o := range opts {
		o(&p)
	}
	return func(c recordContext) []string {
		r := base(c, "user", m)
		r["promptId"] = newID("prompt")
		r["turnOrigin"] = "human"
		r["promptSource"] = "sdk"
		var content any = p.text
		if p.image {
			content = []any{
				map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": "image/png", "data": "iVBORw0KGgo="}},
				map[string]any{"type": "text", "text": p.text},
			}
		}
		r["message"] = map[string]any{"role": "user", "content": content}
		return []string{mustJSON(r)}
	}
}

// Interrupt is the marker Claude Code writes when the user interrupts Claude.
func Interrupt(m moment) Record {
	return func(c recordContext) []string {
		r := base(c, "user", m)
		r["message"] = map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": "[Request interrupted by user]"},
		}}
		return []string{mustJSON(r)}
	}
}

// TaskNotification is the message Claude Code adds when a background task finishes.
func TaskNotification(m moment) Record {
	return func(c recordContext) []string {
		r := base(c, "user", m)
		r["message"] = map[string]any{"role": "user",
			"content": "<task-notification>\n<status>completed</status>\n</task-notification>"}
		return []string{mustJSON(r)}
	}
}

// MetaRecord is a user record Claude Code adds itself, marked isMeta.
func MetaRecord(m moment) Record {
	return func(c recordContext) []string {
		r := base(c, "user", m)
		r["isMeta"] = true
		r["message"] = map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": "Caveat: generated by the harness"},
		}}
		return []string{mustJSON(r)}
	}
}

// CompactSummary is the summary Claude Code writes when it compacts a session.
func CompactSummary(m moment) Record {
	return func(c recordContext) []string {
		r := base(c, "user", m)
		r["isCompactSummary"] = true
		r["message"] = map[string]any{"role": "user", "content": "This session is being continued from a previous conversation."}
		return []string{mustJSON(r)}
	}
}

type reply struct {
	model                          string
	in, out, cacheRead, cacheWrite int
	blocks                         int
	text                           string
	extra                          map[string]any
}

type ReplyOption func(*reply)
type TokenOption func(*reply)

func Model(name string) ReplyOption { return func(r *reply) { r.model = name } }

func Tokens(opts ...TokenOption) ReplyOption {
	return func(r *reply) {
		for _, o := range opts {
			o(r)
		}
	}
}

func In(n int) TokenOption         { return func(r *reply) { r.in = n } }
func Out(n int) TokenOption        { return func(r *reply) { r.out = n } }
func CacheRead(n int) TokenOption  { return func(r *reply) { r.cacheRead = n } }
func CacheWrite(n int) TokenOption { return func(r *reply) { r.cacheWrite = n } }

// Blocks writes the reply as n lines, one per content block, each repeating the
// same message id and usage, as Claude Code does.
func Blocks(n int) ReplyOption { return func(r *reply) { r.blocks = n } }

// Says sets the reply's text, what Claude tells the user.
func Says(text string) ReplyOption { return func(r *reply) { r.text = text } }

// ExtraField adds a field the tool doesn't know about.
func ExtraField(name string, value any) ReplyOption {
	return func(r *reply) { r.extra[name] = value }
}

// AssistantReply is one reply from Claude.
func AssistantReply(m moment, opts ...ReplyOption) Record {
	r := reply{model: defaultModel, blocks: 1, text: "done", extra: map[string]any{}}
	for _, o := range opts {
		o(&r)
	}
	return func(c recordContext) []string {
		id := newID("msg")
		kinds := []string{"text"}
		if r.blocks > 1 {
			kinds = []string{"thinking", "text", "tool_use"}
		}
		var lines []string
		for i := 0; i < r.blocks; i++ {
			var block map[string]any
			switch kinds[i%len(kinds)] {
			case "thinking":
				block = map[string]any{"type": "thinking", "thinking": "considering", "signature": "sig"}
			case "tool_use":
				block = toolUse(newID("toolu"), "Read", map[string]any{"file_path": "README.md"})
			default:
				block = map[string]any{"type": "text", "text": r.text}
			}
			rec := assistantLine(c, m, id, r.model, block, usage(r.in, r.out, r.cacheRead, r.cacheWrite))
			for k, v := range r.extra {
				rec[k] = v
			}
			lines = append(lines, mustJSON(rec))
		}
		return lines
	}
}

// SyntheticReply is a local placeholder reply: model <synthetic>, no usage.
func SyntheticReply(m moment) Record {
	return AssistantReply(m, Model("<synthetic>"))
}

// ClaudeAsksQuestion is Claude asking the user a clarifying question with the
// AskUserQuestion tool, and the user's answer coming back as its tool result.
func ClaudeAsksQuestion(m moment) Record {
	questions := []any{map[string]any{"question": "Which approach?", "header": "Approach",
		"options": []any{map[string]any{"label": "A"}, map[string]any{"label": "B"}}, "multiSelect": false}}
	return toolCall(m, "AskUserQuestion", map[string]any{"questions": questions},
		map[string]any{"questions": questions, "answers": map[string]any{"Which approach?": "A"}})
}

type edit struct{ added, removed int }

type EditOption func(*edit)

func Added(n int) EditOption   { return func(e *edit) { e.added = n } }
func Removed(n int) EditOption { return func(e *edit) { e.removed = n } }

// EditsFile is Claude changing a file with the Edit tool.
func EditsFile(m moment, path string, opts ...EditOption) Record {
	e := edit{}
	for _, o := range opts {
		o(&e)
	}
	return toolCall(m, "Edit",
		map[string]any{"file_path": path, "old_string": "old", "new_string": "new"},
		map[string]any{
			"filePath": path, "oldString": "old", "newString": "new",
			"originalFile": "old\n", "replaceAll": false, "userModified": false,
			"structuredPatch": patch(e),
		})
}

// RewritesFile is Claude overwriting an existing file with the Write tool.
func RewritesFile(m moment, path string, opts ...EditOption) Record {
	e := edit{}
	for _, o := range opts {
		o(&e)
	}
	return toolCall(m, "Write",
		map[string]any{"file_path": path, "content": "new\n"},
		map[string]any{
			"type": "update", "filePath": path, "content": "new\n", "originalFile": "old\n",
			"userModified": false, "structuredPatch": patch(e),
		})
}

// CreatesFile is Claude creating a new file of n lines with the Write tool.
// A created file has an empty patch (docs/transcript-format.md).
func CreatesFile(m moment, path string, lines int) Record {
	var content strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&content, "line %d\n", i+1)
	}
	return toolCall(m, "Write",
		map[string]any{"file_path": path, "content": content.String()},
		map[string]any{
			"type": "create", "filePath": path, "content": content.String(), "originalFile": nil,
			"userModified": false, "structuredPatch": []any{},
		})
}

// SpawnsSubagent is Claude starting a subagent with the Agent tool. Give the
// subagent's own records with Given.Subagent and the same agent ID.
func SpawnsSubagent(m moment, agentID string) Record {
	return toolCall(m, "Agent",
		map[string]any{"description": "helper task", "prompt": "do the task", "subagent_type": "general-purpose"},
		map[string]any{"agentId": agentID, "agentType": "general-purpose", "status": "completed",
			"content": []any{map[string]any{"type": "text", "text": "finished"}}, "totalToolUseCount": 1})
}

func patch(e edit) []any {
	lines := []any{" context"}
	for i := 0; i < e.removed; i++ {
		lines = append(lines, fmt.Sprintf("-old %d", i+1))
	}
	for i := 0; i < e.added; i++ {
		lines = append(lines, fmt.Sprintf("+new %d", i+1))
	}
	return []any{map[string]any{
		"oldStart": 1, "oldLines": e.removed + 1, "newStart": 1, "newLines": e.added + 1, "lines": lines,
	}}
}

// toolCall is an assistant line calling a tool and the user line carrying its result.
func toolCall(m moment, name string, input, result map[string]any) Record {
	return func(c recordContext) []string {
		toolID := newID("toolu")
		call := assistantLine(c, m, newID("msg"), defaultModel, toolUse(toolID, name, input), usage(0, 0, 0, 0))
		res := base(c, "user", m)
		res["message"] = map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": toolID, "content": "ok"},
		}}
		res["toolUseResult"] = result
		res["sourceToolAssistantUUID"] = call["uuid"]
		return []string{mustJSON(call), mustJSON(res)}
	}
}

func toolUse(id, name string, input map[string]any) map[string]any {
	return map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}
}

func assistantLine(c recordContext, m moment, id, model string, block map[string]any, u map[string]any) map[string]any {
	rec := base(c, "assistant", m)
	rec["requestId"] = "req_" + id
	rec["message"] = map[string]any{
		"id": id, "type": "message", "role": "assistant", "model": model,
		"content": []any{block}, "stop_reason": "end_turn", "usage": u,
	}
	return rec
}

func usage(in, out, cacheRead, cacheWrite int) map[string]any {
	return map[string]any{
		"input_tokens":                in,
		"output_tokens":               out,
		"cache_read_input_tokens":     cacheRead,
		"cache_creation_input_tokens": cacheWrite,
		"cache_creation": map[string]any{
			"ephemeral_5m_input_tokens": 0,
			"ephemeral_1h_input_tokens": cacheWrite,
		},
		"output_tokens_details": map[string]any{"thinking_tokens": 0},
		"service_tier":          "standard",
	}
}

// UnknownRecord is a record type the tool has never seen.
func UnknownRecord(m moment) Record {
	return func(c recordContext) []string {
		return []string{mustJSON(map[string]any{
			"type":      "future-record-type",
			"sessionId": c.sessionID,
			"timestamp": stamp(m),
			"payload":   map[string]any{"nested": []int{1, 2}},
		})}
	}
}

// BrokenLine is a line that isn't valid JSON, like a half-written record.
func BrokenLine() Record {
	return func(recordContext) []string {
		return []string{`{"type":"assistant","message":{"id":"msg_trunc`}
	}
}

// ----------------------------------------------------------------- helpers

var idCounter atomic.Int64

func newID(prefix string) string {
	return fmt.Sprintf("%s_%06d", prefix, idCounter.Add(1))
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendLines(t *testing.T, path string, lines []string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}
