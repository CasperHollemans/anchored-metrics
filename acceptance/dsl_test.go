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
// folder, and a working directory where out/sessions.csv is written.
type Scenario struct {
	t     *testing.T
	root  string
	home  string
	work  string
	Given *given
	When  *when
	Then  *then

	sessions  map[string]sessionFile
	run       runResult
	csvBefore []byte
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
func (s *Scenario) csvPath() string                 { return filepath.Join(s.work, "out", "sessions.csv") }

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

// Subagent is a subagent transcript stored next to its parent session.
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

// Sync runs `anchored-metrics sessions <default repo>`.
func (w *when) Sync() {
	w.s.t.Helper()
	w.SyncRepo(defaultRepo)
}

// SyncRepo runs `anchored-metrics sessions <repo>`.
func (w *when) SyncRepo(repo string) {
	s := w.s
	s.t.Helper()
	s.csvBefore, _ = os.ReadFile(s.csvPath())
	cmd := exec.Command(binary, "sessions", s.repoPath(repo))
	cmd.Dir = s.work
	cmd.Env = s.env()
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

// -------------------------------------------------------------------- then

type then struct{ s *Scenario }

// Session checks the one row for this session. Has(...) checks only the
// columns it lists.
func (th *then) Session(id string) *sessionRow {
	return &sessionRow{s: th.s, id: id}
}

type sessionRow struct {
	s  *Scenario
	id string
}

func (r *sessionRow) Has(columns ...Column) {
	r.s.t.Helper()
	var found []map[string]string
	for _, row := range r.s.rows() {
		if row["session"] == r.id {
			found = append(found, row)
		}
	}
	if len(found) != 1 {
		r.s.t.Fatalf("want exactly 1 row for session %q, got %d%s", r.id, len(found), r.s.runInfo())
	}
	for _, c := range columns {
		got, ok := found[0][c.name]
		if !ok {
			r.s.t.Errorf("sessions.csv has no column %q", c.name)
			continue
		}
		if got != c.value {
			r.s.t.Errorf("session %q: %s = %q, want %q", r.id, c.name, got, c.value)
		}
	}
}

// Sessions checks that sessions.csv holds exactly these sessions, one row each.
func (th *then) Sessions(ids ...string) {
	th.s.t.Helper()
	var got []string
	for _, row := range th.s.rows() {
		got = append(got, row["session"])
	}
	slices.Sort(got)
	want := slices.Sorted(slices.Values(ids))
	if !slices.Equal(got, want) {
		th.s.t.Fatalf("sessions in sessions.csv = %v, want %v%s", got, want, th.s.runInfo())
	}
}

func (th *then) NoCSVWritten() {
	th.s.t.Helper()
	if _, err := os.Stat(th.s.csvPath()); err == nil {
		th.s.t.Fatalf("want no sessions.csv, but it was written")
	}
}

func (th *then) ExitedNormally() {
	th.s.t.Helper()
	th.s.mustHaveRun()
	if th.s.run.code != 0 {
		th.s.t.Fatalf("exit code = %d, want 0%s", th.s.run.code, th.s.runInfo())
	}
}

// Warned checks that the last sync printed a warning containing text.
func (th *then) Warned(text string) {
	th.s.t.Helper()
	th.s.mustHaveRun()
	if !strings.Contains(strings.ToLower(th.s.run.stderr), "warning") || !strings.Contains(th.s.run.stderr, text) {
		th.s.t.Fatalf("want a warning containing %q on stderr%s", text, th.s.runInfo())
	}
}

// Reported checks that the last sync printed text, on stdout or stderr.
func (th *then) Reported(text string) {
	th.s.t.Helper()
	th.s.mustHaveRun()
	if !strings.Contains(th.s.run.stdout+th.s.run.stderr, text) {
		th.s.t.Fatalf("want output containing %q%s", text, th.s.runInfo())
	}
}

func (th *then) CSVUnchangedByLastSync() {
	th.s.t.Helper()
	after, err := os.ReadFile(th.s.csvPath())
	if err != nil {
		th.s.t.Fatalf("reading sessions.csv: %v%s", err, th.s.runInfo())
	}
	if !bytes.Equal(th.s.csvBefore, after) {
		th.s.t.Fatalf("sessions.csv changed.\nbefore:\n%s\nafter:\n%s", th.s.csvBefore, after)
	}
}

// CSVHasNoPaths checks that no file path (and so no username) leaks into the CSV.
func (th *then) CSVHasNoPaths() {
	th.s.t.Helper()
	content := th.s.csvContent()
	if strings.Contains(content, "/") {
		th.s.t.Fatalf("sessions.csv contains a path:\n%s", content)
	}
}

func (th *then) CSVDoesNotContain(texts ...string) {
	th.s.t.Helper()
	content := th.s.csvContent()
	for _, text := range texts {
		if strings.Contains(content, text) {
			th.s.t.Errorf("sessions.csv contains %q:\n%s", text, content)
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

func (s *Scenario) csvContent() string {
	s.t.Helper()
	b, err := os.ReadFile(s.csvPath())
	if err != nil {
		s.t.Fatalf("reading sessions.csv: %v%s", err, s.runInfo())
	}
	return string(b)
}

func (s *Scenario) rows() []map[string]string {
	s.t.Helper()
	records, err := csv.NewReader(strings.NewReader(s.csvContent())).ReadAll()
	if err != nil {
		s.t.Fatalf("parsing sessions.csv: %v", err)
	}
	if len(records) == 0 {
		s.t.Fatalf("sessions.csv is empty%s", s.runInfo())
	}
	header := records[0]
	var rows []map[string]string
	for _, rec := range records[1:] {
		row := map[string]string{}
		for i, name := range header {
			if i < len(rec) {
				row[name] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// ----------------------------------------------------------------- columns

// Column is one expected value in a sessions.csv row.
type Column struct{ name, value string }

func Repo(name string) Column  { return Column{"repo", name} }
func Start(when string) Column { return Column{"start", utc(when)} }
func End(when string) Column   { return Column{"end", utc(when)} }

// Models lists the expected models in any order; the CSV sorts them.
func Models(names ...string) Column {
	return Column{"models", strings.Join(slices.Sorted(slices.Values(names)), ";")}
}

func MainInput(n int) Column          { return Column{"main_input", strconv.Itoa(n)} }
func MainOutput(n int) Column         { return Column{"main_output", strconv.Itoa(n)} }
func MainCacheRead(n int) Column      { return Column{"main_cache_read", strconv.Itoa(n)} }
func MainCacheWrite(n int) Column     { return Column{"main_cache_write", strconv.Itoa(n)} }
func SubagentInput(n int) Column      { return Column{"subagent_input", strconv.Itoa(n)} }
func SubagentOutput(n int) Column     { return Column{"subagent_output", strconv.Itoa(n)} }
func SubagentCacheRead(n int) Column  { return Column{"subagent_cache_read", strconv.Itoa(n)} }
func SubagentCacheWrite(n int) Column { return Column{"subagent_cache_write", strconv.Itoa(n)} }

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
		"timestamp":   m.t.Format("2006-01-02T15:04:05.000Z07:00"),
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

type prompt struct{ text string }

type PromptOption func(*prompt)

func Text(text string) PromptOption { return func(p *prompt) { p.text = text } }

// UserPrompt is a prompt typed by the user.
func UserPrompt(m moment, opts ...PromptOption) Record {
	p := prompt{text: "please help"}
	for _, o := range opts {
		o(&p)
	}
	return func(c recordContext) []string {
		r := base(c, "user", m)
		r["promptId"] = newID("prompt")
		r["message"] = map[string]any{"role": "user", "content": p.text}
		return []string{mustJSON(r)}
	}
}

type reply struct {
	model                          string
	in, out, cacheRead, cacheWrite int
	blocks                         int
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

// ExtraField adds a field the tool doesn't know about.
func ExtraField(name string, value any) ReplyOption {
	return func(r *reply) { r.extra[name] = value }
}

// AssistantReply is one reply from Claude.
func AssistantReply(m moment, opts ...ReplyOption) Record {
	r := reply{model: defaultModel, blocks: 1, extra: map[string]any{}}
	for _, o := range opts {
		o(&r)
	}
	return func(c recordContext) []string {
		id := newID("msg")
		usage := map[string]any{
			"input_tokens":                r.in,
			"output_tokens":               r.out,
			"cache_read_input_tokens":     r.cacheRead,
			"cache_creation_input_tokens": r.cacheWrite,
			"cache_creation": map[string]any{
				"ephemeral_5m_input_tokens": 0,
				"ephemeral_1h_input_tokens": r.cacheWrite,
			},
			"output_tokens_details": map[string]any{"thinking_tokens": 0},
			"service_tier":          "standard",
		}
		kinds := []string{"thinking", "text", "tool_use"}
		var lines []string
		for i := 0; i < r.blocks; i++ {
			rec := base(c, "assistant", m)
			rec["requestId"] = "req_" + id
			rec["message"] = map[string]any{
				"id":          id,
				"type":        "message",
				"role":        "assistant",
				"model":       r.model,
				"content":     []any{contentBlock(kinds[i%len(kinds)])},
				"stop_reason": "end_turn",
				"usage":       usage,
			}
			for k, v := range r.extra {
				rec[k] = v
			}
			lines = append(lines, mustJSON(rec))
		}
		return lines
	}
}

func contentBlock(kind string) map[string]any {
	switch kind {
	case "thinking":
		return map[string]any{"type": "thinking", "thinking": "considering", "signature": "sig"}
	case "tool_use":
		return map[string]any{"type": "tool_use", "id": newID("toolu"), "name": "Read",
			"input": map[string]any{"file_path": "README.md"}}
	default:
		return map[string]any{"type": "text", "text": "done"}
	}
}

// SyntheticReply is a local placeholder reply: model <synthetic>, no usage.
func SyntheticReply(m moment) Record {
	return AssistantReply(m, Model("<synthetic>"))
}

// UnknownRecord is a record type the tool has never seen.
func UnknownRecord(m moment) Record {
	return func(c recordContext) []string {
		return []string{mustJSON(map[string]any{
			"type":      "future-record-type",
			"sessionId": c.sessionID,
			"timestamp": m.t.Format("2006-01-02T15:04:05.000Z07:00"),
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
