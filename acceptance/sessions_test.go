package acceptance

// Acceptance tests for docs/stories/sessions.md.

import "testing"

// ---------------------------------------------------------- finding sessions

// S-01 Syncing a repo reads every session transcript in that repo's Claude
// Code project folder.
func TestSyncReadsEverySessionOfTheRepo(t *testing.T) {
	s := scenario(t)

	s.Given.Session("first",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
	)
	s.Given.Session("second",
		UserPrompt(at("10:00")),
		AssistantReply(at("10:01")),
	)

	s.When.Sync()

	s.Then.Sessions("first", "second")
}

// S-02 Sessions from a git worktree of the repo are included, wherever the
// worktree lives, so work done in a worktree isn't lost.
func TestSessionsFromGitWorktreesAreIncluded(t *testing.T) {
	s := scenario(t)

	s.Given.GitWorktree("feature")
	s.Given.Session("main-work",
		UserPrompt(at("09:00")),
	)
	s.Given.SessionInGitWorktree("feature", "worktree-work",
		UserPrompt(at("10:00")),
	)

	s.When.Sync()

	s.Then.Sessions("main-work", "worktree-work")
	s.Then.Session("worktree-work").Has(
		Repo("shop"),
	)
}

// S-02 Sessions from worktrees Claude Code made inside the repo are included.
func TestSessionsFromClaudeCodeWorktreesAreIncluded(t *testing.T) {
	s := scenario(t)

	s.Given.Session("main-work",
		UserPrompt(at("09:00")),
	)
	s.Given.SessionInClaudeWorktree("quick-fix", "worktree-work",
		UserPrompt(at("10:00")),
	)

	s.When.Sync()

	s.Then.Sessions("main-work", "worktree-work")
}

// S-02 A sibling repo whose name starts the same way is a different repo, so
// its sessions are left out.
func TestSessionsFromASiblingRepoWithASimilarNameAreLeftOut(t *testing.T) {
	s := scenario(t)

	s.Given.Session("shop-work",
		UserPrompt(at("09:00")),
	)
	s.Given.SessionIn("shop-admin", "admin-work",
		UserPrompt(at("10:00")),
	)

	s.When.Sync()

	s.Then.Sessions("shop-work")
}

// S-03 Files that aren't session transcripts are ignored.
func TestFilesThatAreNotTranscriptsAreIgnored(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
	)
	s.Given.FileInProjectFolder("memory/MEMORY.md", "- some memory\n")
	s.Given.FileInProjectFolder("d82f7ca6.desktop-released.json", `{"released":true}`)
	s.Given.FileInProjectFolder("abc/custom-title.json", `{"title":"x"}`)
	s.Given.FileInProjectFolder("abc/tool-results/screenshot.png", "not really a png")

	s.When.Sync()

	s.Then.Sessions("abc")
}

// S-04 A repo without Claude Code sessions gives a warning, not an error, and
// writes nothing.
func TestARepoWithoutSessionsWarnsAndWritesNothing(t *testing.T) {
	s := scenario(t)

	s.When.Sync()

	s.Then.ExitedNormally()
	s.Then.Warned("no Claude Code sessions")
	s.Then.NoCSVWritten()
}

// ------------------------------------------------------- one row per session

// S-05 A session with subagents is still one row: subagents belong to it.
func TestEachSessionIsOneRowEvenWithSubagents(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
	)
	s.Given.Subagent("abc", "a1",
		UserPrompt(at("09:02")),
		AssistantReply(at("09:03")),
	)
	s.Given.Subagent("abc", "a2",
		UserPrompt(at("09:04")),
		AssistantReply(at("09:05")),
	)

	s.When.Sync()

	s.Then.Sessions("abc")
}

// S-06 Syncing a second repo adds its rows to the same sessions.csv and leaves
// the first repo's rows as they were.
func TestAllReposShareOneCSVWithARepoColumn(t *testing.T) {
	s := scenario(t)

	s.Given.Session("shop-work",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"), Tokens(Out(20))),
	)
	s.Given.SessionIn("blog", "blog-work",
		UserPrompt(at("10:00")),
	)

	s.When.SyncRepo("shop")
	s.When.SyncRepo("blog")

	s.Then.Sessions("shop-work", "blog-work")
	s.Then.Session("shop-work").Has(
		Repo("shop"),
		MainOutput(20),
	)
	s.Then.Session("blog-work").Has(
		Repo("blog"),
	)
}

// S-07 When a session spans time zones, start and end are still written in
// UTC, so sessions from different machines sort and compare correctly.
func TestSessionStartAndEndAreWrittenInUTC(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00 +01:00")),
		AssistantReply(at("09:05 +01:00")),
	)

	s.When.Sync()

	s.Then.Session("abc").Has(
		Start("08:00Z"),
		End("08:05Z"),
	)
}

// S-07 Records aren't always in time order, so start and end are the earliest
// and latest moments, not the first and last lines.
func TestStartAndEndAreEarliestAndLatestEvenOutOfOrder(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		AssistantReply(at("09:30")),
		UserPrompt(at("09:00")),
		AssistantReply(at("10:15")),
		UserPrompt(at("10:00")),
	)

	s.When.Sync()

	s.Then.Session("abc").Has(
		Start("09:00Z"),
		End("10:15Z"),
	)
}

// S-08 The models column lists every model used, including models only a
// subagent used.
func TestModelsIncludeThoseUsedBySubagents(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"), Model("claude-opus-5-5")),
		AssistantReply(at("09:02"), Model("claude-opus-5-5")),
	)
	s.Given.Subagent("abc", "a1",
		AssistantReply(at("09:03"), Model("claude-haiku-5-5")),
	)

	s.When.Sync()

	s.Then.Session("abc").Has(
		Models("claude-opus-5-5", "claude-haiku-5-5"),
	)
}

// S-09 Claude Code writes a reply as one line per content block, each repeating
// the same usage. A reply's tokens are counted once, not once per line.
func TestRepliesSplitIntoBlocksAreCountedOnce(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"), Blocks(3),
			Tokens(In(10), Out(200), CacheRead(5000), CacheWrite(300)),
		),
		AssistantReply(at("09:02"),
			Tokens(In(5), Out(50), CacheRead(1000), CacheWrite(100)),
		),
	)

	s.When.Sync()

	s.Then.Session("abc").Has(
		MainInput(15),
		MainOutput(250),
		MainCacheRead(6000),
		MainCacheWrite(400),
	)
}

// S-10 Tokens from the main conversation and from subagents are reported
// separately, to show how much a session relies on delegation.
func TestMainAndSubagentTokensAreCountedSeparately(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"),
			Tokens(In(10), Out(100), CacheRead(1000), CacheWrite(50)),
		),
	)
	s.Given.Subagent("abc", "a1",
		AssistantReply(at("09:02"), Blocks(2),
			Tokens(In(3), Out(30), CacheRead(300), CacheWrite(20)),
		),
	)
	s.Given.Subagent("abc", "a2",
		AssistantReply(at("09:03"),
			Tokens(In(4), Out(40), CacheRead(400), CacheWrite(10)),
		),
	)

	s.When.Sync()

	s.Then.Session("abc").Has(
		MainInput(10),
		MainOutput(100),
		MainCacheRead(1000),
		MainCacheWrite(50),
		SubagentInput(7),
		SubagentOutput(70),
		SubagentCacheRead(700),
		SubagentCacheWrite(30),
	)
}

// S-11 Placeholder replies with model <synthetic> aren't real model calls, so
// they add no model and no tokens.
func TestSyntheticRepliesAreNotCounted(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"), Tokens(Out(20))),
		SyntheticReply(at("09:02")),
	)

	s.When.Sync()

	s.Then.Session("abc").Has(
		Models("claude-opus-5-5"),
		MainOutput(20),
	)
}

// ------------------------------------------------------------------ privacy

// S-12 The CSV never contains usernames, file paths or prompt text, even
// though the transcripts do.
func TestCSVContainsNoPathsOrPromptText(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00"), Text("refactor the billing module for client X")),
		AssistantReply(at("09:01")),
	)
	s.Given.Subagent("abc", "a1",
		UserPrompt(at("09:02"), Text("search the invoices folder")),
	)

	s.When.Sync()

	s.Then.CSVHasNoPaths()
	s.Then.CSVDoesNotContain("billing", "client X", "invoices")
}

// -------------------------------------------------------- syncing over time

// S-13 The first sync writes a row for every session.
func TestFirstSyncWritesEverySession(t *testing.T) {
	s := scenario(t)

	s.Given.Session("a", UserPrompt(at("09:00")))
	s.Given.Session("b", UserPrompt(at("10:00")))
	s.Given.Session("c", UserPrompt(at("11:00")))

	s.When.Sync()

	s.Then.ExitedNormally()
	s.Then.Sessions("a", "b", "c")
}

// S-14 Syncing again without changes leaves sessions.csv exactly as it was.
func TestSyncingAgainWithoutChangesChangesNothing(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"), Tokens(Out(20))),
	)
	s.When.Sync()

	s.When.Sync()

	s.Then.CSVUnchangedByLastSync()
}

// S-15 A session that kept running after a sync gets its row replaced with the
// newer numbers, not a second row.
func TestASessionThatGrewGetsItsRowReplaced(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"), Tokens(Out(20))),
	)
	s.When.Sync()

	s.Given.MoreRecords("abc",
		UserPrompt(at("11:00")),
		AssistantReply(at("11:05"), Tokens(Out(30))),
	)
	s.When.Sync()

	s.Then.Sessions("abc")
	s.Then.Session("abc").Has(
		Start("09:00Z"),
		End("11:05Z"),
		MainOutput(50),
	)
}

// S-16 A session that started after the last sync gets a new row.
func TestANewSessionGetsANewRow(t *testing.T) {
	s := scenario(t)

	s.Given.Session("old", UserPrompt(at("09:00")))
	s.When.Sync()

	s.Given.Session("new", UserPrompt(at("10:00")))
	s.When.Sync()

	s.Then.Sessions("old", "new")
}

// S-17 Claude Code deletes old transcripts, so a session whose transcript is
// gone keeps its row exactly as it was.
func TestASessionWhoseTranscriptWasDeletedKeepsItsRow(t *testing.T) {
	s := scenario(t)

	s.Given.Session("old",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"), Tokens(Out(20))),
	)
	s.Given.Session("recent", UserPrompt(at("10:00")))
	s.When.Sync()

	s.Given.SessionDeleted("old")
	s.When.Sync()

	s.Then.Sessions("old", "recent")
	s.Then.Session("old").Has(
		Start("09:00Z"),
		End("09:01Z"),
		MainOutput(20),
	)
}

// ------------------------------------------------- handling format changes

// S-18 A line that can't be read is skipped and reported, and the rest of the
// session still counts.
func TestUnreadableLinesAreSkippedAndReported(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		BrokenLine(),
		AssistantReply(at("09:01"), Tokens(Out(20))),
	)

	s.When.Sync()

	s.Then.ExitedNormally()
	s.Then.Reported("skipped 1 unreadable line")
	s.Then.Session("abc").Has(
		MainOutput(20),
	)
}

// S-19 Record types and fields the tool doesn't know are ignored, so a newer
// Claude Code version doesn't break the sync.
func TestUnknownRecordTypesAndFieldsAreIgnored(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"), Tokens(Out(20)), ExtraField("futureField", "x")),
		UnknownRecord(at("12:00")),
	)

	s.When.Sync()

	s.Then.ExitedNormally()
	s.Then.Session("abc").Has(
		End("09:01Z"),
		MainOutput(20),
	)
}
