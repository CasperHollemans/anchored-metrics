package acceptance

// Acceptance tests for docs/stories/prompts.md.

import "testing"

// ----------------------------------------------------------- finding prompts

// P-01 Each prompt the user typed in the main conversation becomes one row.
func TestEachTypedPromptBecomesOneRow(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10")),
		AssistantReply(at("09:11")),
		UserPrompt(at("09:20")),
	)

	s.When.Sync()

	s.Then.Prompts("abc", 3)
}

// P-02 Records Claude Code writes in the user's name aren't prompts: tool
// results, meta records, compaction summaries, task notifications and the
// interrupt marker.
func TestRecordsThatAreNotTypedPromptsAreNotRows(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		EditsFile(at("09:01"), "/repo/a.go", Added(1)),
		ClaudeAsksQuestion(at("09:02")),
		MetaRecord(at("09:03")),
		TaskNotification(at("09:04")),
		Interrupt(at("09:05")),
		CompactSummary(at("09:06")),
		UserPrompt(at("09:10")),
		AssistantReply(at("09:11")),
	)

	s.When.Sync()

	s.Then.Prompts("abc", 2)
}

// P-03 The user never talks to a subagent, so its prompts aren't rows; its
// work counts toward the prompt that spawned it.
func TestSubagentPromptsAreNotRowsButTheirWorkCountsForTheSpawningPrompt(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10")),
		SpawnsSubagent(at("09:11"), "a1"),
		AssistantReply(at("09:20")),
	)
	s.Given.Subagent("abc", "a1",
		UserPrompt(at("09:12")),
		EditsFile(at("09:13"), "/repo/a.go", Added(4)),
		UserPrompt(at("09:14")),
	)

	s.When.Sync()

	s.Then.Prompts("abc", 2)
	s.Then.Prompt("abc", 1).Has(
		FilesChanged(0),
	)
	s.Then.Prompt("abc", 2).Has(
		FilesChanged(1),
		LinesAdded(4),
	)
}

// P-04 Prompts come from the same repos and worktrees as sessions, and every
// repo shares one prompts.csv.
func TestPromptsComeFromWorktreesAndAllReposShareOneCSV(t *testing.T) {
	s := scenario(t)

	s.Given.GitWorktree("feature")
	s.Given.Session("main-work",
		UserPrompt(at("09:00")),
	)
	s.Given.SessionInGitWorktree("feature", "worktree-work",
		UserPrompt(at("10:00")),
	)
	s.Given.SessionIn("blog", "blog-work",
		UserPrompt(at("11:00")),
	)

	s.When.SyncRepo("shop")
	s.When.SyncRepo("blog")

	s.Then.Prompt("main-work", 1).Has(Repo("shop"))
	s.Then.Prompt("worktree-work", 1).Has(Repo("shop"))
	s.Then.Prompt("blog-work", 1).Has(Repo("blog"))
}

// -------------------------------------------------------- what each row says

// P-05 Each row has its session, repo, position, time in UTC and length in
// words; a pasted image doesn't count as words.
func TestEachPromptRowHasItsPositionTimeAndLength(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00 +01:00"), Text("build the login form")),
		AssistantReply(at("09:01 +01:00")),
		UserPrompt(at("09:10 +01:00"), Text("this looks off"), WithImage()),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 1).Has(
		Repo("shop"),
		At("08:00Z"),
		Words(4),
	)
	s.Then.Prompt("abc", 2).Has(
		At("08:10Z"),
		Words(3),
	)
}

// P-06 A prompt lists the models that replied to it, including the models of
// subagents spawned for it, and not those of other prompts.
func TestPromptModelsIncludeItsSubagentsButNotOtherPrompts(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01"), Model("claude-sonnet-5-5")),
		UserPrompt(at("09:10")),
		SpawnsSubagent(at("09:11"), "a1"),
		AssistantReply(at("09:20")),
	)
	s.Given.Subagent("abc", "a1",
		AssistantReply(at("09:12"), Model("claude-haiku-5-5")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 1).Has(
		Models("claude-sonnet-5-5"),
	)
	s.Then.Prompt("abc", 2).Has(
		Models("claude-opus-5-5", "claude-haiku-5-5"),
	)
}

// P-07 When the user interrupts Claude, the prompt it was working on is marked
// interrupted, and the user's next prompt is a row of its own.
func TestAnInterruptMarksThePromptClaudeWasWorkingOn(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		Interrupt(at("09:02")),
		UserPrompt(at("09:03")),
		AssistantReply(at("09:04")),
	)

	s.When.Sync()

	s.Then.Prompts("abc", 2)
	s.Then.Prompt("abc", 1).Has(
		Interrupted(true),
	)
	s.Then.Prompt("abc", 2).Has(
		Interrupted(false),
	)
}

// P-08 A prompt is marked when Claude asked the user a clarifying question
// while working on it.
func TestAPromptIsMarkedWhenClaudeAskedAClarifyingQuestion(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		ClaudeAsksQuestion(at("09:01")),
		AssistantReply(at("09:02")),
		UserPrompt(at("09:10")),
		AssistantReply(at("09:11")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 1).Has(
		AskedQuestion(true),
	)
	s.Then.Prompt("abc", 2).Has(
		AskedQuestion(false),
	)
}

// P-09 files_changed counts each file a prompt created or edited once, however
// often it was touched.
func TestFilesChangedCountsEachFileOnce(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		EditsFile(at("09:01"), "/repo/a.go", Added(1)),
		EditsFile(at("09:02"), "/repo/a.go", Added(1)),
		CreatesFile(at("09:03"), "/repo/b.go", 5),
		UserPrompt(at("09:10")),
		EditsFile(at("09:11"), "/repo/a.go", Added(1)),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 1).Has(
		FilesChanged(2),
	)
	s.Then.Prompt("abc", 2).Has(
		FilesChanged(1),
	)
}

// P-10 Lines added and removed come from each edit's patch, and a newly
// created file counts all its lines as added.
func TestLinesComeFromPatchesAndNewFilesCountAllTheirLines(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		EditsFile(at("09:01"), "/repo/a.go", Added(3), Removed(1)),
		RewritesFile(at("09:02"), "/repo/b.go", Added(2), Removed(2)),
		CreatesFile(at("09:03"), "/repo/c.go", 10),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 1).Has(
		LinesAdded(15),
		LinesRemoved(3),
	)
}

// --------------------------------------------------------------- corrections

// P-11 A prompt is a correction when it contains a library entry, ignoring
// case and matching whole words only.
func TestACorrectionMatchesLibraryEntriesAsWholeWordsIgnoringCase(t *testing.T) {
	s := scenario(t)

	s.Given.CorrectionLibrary(`
		[wrong]
		werkt niet
		bug
	`)
	s.Given.Session("abc",
		UserPrompt(at("09:00"), Text("build the login form")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10"), Text("Het WERKT NIET meer")),
		AssistantReply(at("09:11")),
		UserPrompt(at("09:20"), Text("now debug mode please")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 2).Has(
		Correction(true),
	)
	s.Then.Prompt("abc", 3).Has(
		Correction(false),
	)
}

// P-12 The row lists every category that matched, so kinds of re-steering can
// be followed over time.
func TestACorrectionListsEveryCategoryThatMatched(t *testing.T) {
	s := scenario(t)

	s.Given.CorrectionLibrary(`
		[style]
		rename
		[cleanup]
		clean up
		[revert]
		undo
	`)
	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10"), Text("looks good, but rename the handler and clean up the tests")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 2).Has(
		Correction(true),
		CorrectionCategories("style", "cleanup"),
	)
}

// P-13 An entry ending in * matches any word that starts with it; an entry
// without * still only matches the whole word.
func TestAnEntryEndingInAStarMatchesWordsThatStartWithIt(t *testing.T) {
	s := scenario(t)

	s.Given.CorrectionLibrary(`
		[improve]
		verbeter*
		[wrong]
		fout
	`)
	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10"), Text("kun je de foutmelding verbeteren")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 2).Has(
		CorrectionCategories("improve"),
		CorrectionHits(1),
	)
}

// P-14 The library is a plain-text file with categories as headings, one entry
// per line and # comments, so it can grow without code changes.
func TestTheLibraryIsAPlainTextFile(t *testing.T) {
	s := scenario(t)

	s.Given.CorrectionLibrary(`
		# My own additions
		[naming]
		# entries below
		camelcase
	`)
	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10"), Text("use camelCase here")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 2).Has(
		Correction(true),
		CorrectionCategories("naming"),
	)
}

// P-14 Without a library of its own, sync uses the library that ships in the
// repo (corrections.txt).
func TestSyncUsesTheRepoLibraryByDefault(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10"), Text("dit werkt niet")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 2).Has(
		Correction(true),
		CorrectionCategories("wrong"),
	)
}

// P-22 The row counts how many different library entries matched, so prompts
// that tick many words and categories stand out.
func TestCorrectionHitsCountDifferentEntries(t *testing.T) {
	s := scenario(t)

	s.Given.CorrectionLibrary(`
		[wrong]
		fout
		werkt niet
		[style]
		rename
	`)
	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10"), Text("fout, nog steeds fout, het werkt niet, en rename dit")),
		AssistantReply(at("09:11")),
		UserPrompt(at("09:20"), Text("thanks")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 2).Has(
		CorrectionHits(3),
	)
	s.Then.Prompt("abc", 3).Has(
		Correction(false),
		CorrectionHits(0),
		CorrectionCategories(),
	)
}

// P-23 The first prompt of a session is never a correction: nothing has been
// delivered yet to correct.
func TestTheFirstPromptIsNeverACorrection(t *testing.T) {
	s := scenario(t)

	s.Given.CorrectionLibrary(`
		[wrong]
		bug
	`)
	s.Given.Session("abc",
		UserPrompt(at("09:00"), Text("fix the bug in the login form")),
		AssistantReply(at("09:01")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 1).Has(
		Correction(false),
		CorrectionHits(0),
		CorrectionCategories(),
	)
}

// ------------------------------------------------------------------- privacy

// P-15 prompts.csv never contains prompt text, matched words, file names or
// paths; category names are allowed.
func TestPromptsCSVContainsNoTextNamesOrPaths(t *testing.T) {
	s := scenario(t)

	s.Given.CorrectionLibrary(`
		[wrong]
		werkt niet
	`)
	s.Given.Session("abc",
		UserPrompt(at("09:00"), Text("add invoicing for client X")),
		CreatesFile(at("09:01"), "/home/someone/repo/invoicing.go", 3),
		UserPrompt(at("09:10"), Text("het werkt niet")),
	)

	s.When.Sync()

	s.Then.Prompt("abc", 2).Has(
		CorrectionCategories("wrong"),
	)
	s.Then.CSVsHaveNoPaths()
	s.Then.CSVsDoNotContain("invoicing", "client X", "werkt", "someone")
}

// ------------------------------------------------------------------- syncing

// P-16 One sync command writes both sessions.csv and prompts.csv.
func TestOneSyncWritesSessionsAndPrompts(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
	)

	s.When.Sync()

	s.Then.ExitedNormally()
	s.Then.Sessions("abc")
	s.Then.Prompts("abc", 1)
}

// P-17 A session that grew after a sync gets its prompt rows replaced, so the
// earlier prompts now include their full turn.
func TestPromptRowsOfAGrownSessionAreReplaced(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		EditsFile(at("09:01"), "/repo/a.go", Added(1)),
	)
	s.When.Sync()

	s.Given.MoreRecords("abc",
		EditsFile(at("09:02"), "/repo/b.go", Added(1)),
		UserPrompt(at("09:10")),
	)
	s.When.Sync()

	s.Then.Prompts("abc", 2)
	s.Then.Prompt("abc", 1).Has(
		FilesChanged(2),
	)
}

// P-17 Claude Code deletes old transcripts, so prompt rows of a session whose
// transcript is gone are kept as they were.
func TestPromptRowsOfADeletedTranscriptAreKept(t *testing.T) {
	s := scenario(t)

	s.Given.Session("old",
		UserPrompt(at("09:00"), Text("first")),
		EditsFile(at("09:01"), "/repo/a.go", Added(2)),
		UserPrompt(at("09:10")),
	)
	s.Given.Session("recent", UserPrompt(at("10:00")))
	s.When.Sync()

	s.Given.SessionDeleted("old")
	s.When.Sync()

	s.Then.Prompts("old", 2)
	s.Then.Prompt("old", 1).Has(
		Words(1),
		LinesAdded(2),
	)
}

// -------------------------------------------------------- labelling, scoring

// P-18 Labelling shows a prompt after the last thing Claude said before it,
// so the user knows what the prompt reacts to. First prompts are never shown.
func TestLabellingShowsWhatClaudeSaidBeforeThePrompt(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00"), Text("build the login form")),
		AssistantReply(at("09:05"), Says("I've added the login form and its tests.")),
		UserPrompt(at("09:10"), Text("rename the handler")),
	)

	s.When.Label("n")

	s.Then.ExitedNormally()
	s.Then.ShowedInOrder(
		"I've added the login form and its tests.",
		"rename the handler",
	)
	s.Then.DidNotShow("build the login form")
	s.Then.Label("abc", 2).Is(No())
}

// P-24 After a yes the user picks one or more categories from the library, or
// other when none fits.
func TestAYesLabelRecordsTheChosenCategories(t *testing.T) {
	s := scenario(t)

	s.Given.CorrectionLibrary(`
		[style]
		rename
		[cleanup]
		clean up
	`)
	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10"), Text("rename it and tidy the tests")),
	)

	s.When.Label("y", "style cleanup")

	s.Then.Label("abc", 2).Is(Yes("style", "cleanup"))
}

// P-19 Labels are stored outside the repo by session and prompt position,
// never with prompt text.
func TestLabelsAreStoredWithoutPromptText(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10"), Text("make the invoices page faster")),
	)

	s.When.Label("y", "other")

	s.Then.Label("abc", 2).Is(Yes("other"))
	s.Then.LabelsDoNotContain("invoices", "faster")
}

// P-20 Labelling can be stopped at any time; skipped prompts stay unlabelled,
// and the next run picks up the prompts that are left.
func TestLabellingCanStopAndPickUpWhereItLeftOff(t *testing.T) {
	s := scenario(t)

	s.Given.Session("abc",
		UserPrompt(at("09:00")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10")),
	)
	s.Given.Session("def",
		UserPrompt(at("10:00")),
		AssistantReply(at("10:01")),
		UserPrompt(at("10:10")),
	)

	s.When.Label("s", "q")
	s.Then.LabelledPrompts(0)

	s.When.Label("n", "q")
	s.Then.LabelledPrompts(1)

	s.When.Label("n")
	s.Then.LabelledPrompts(2)

	s.When.Label()
	s.Then.Reported("nothing left to label")
}

// P-21 Scoring compares the library with the labels, overall and per
// category, so the library can be improved where it's weakest.
func TestScoringComparesTheLibraryWithTheLabels(t *testing.T) {
	s := scenario(t)

	s.Given.CorrectionLibrary(`
		[style]
		rename
		[cleanup]
		clean up
	`)
	s.Given.Session("abc",
		UserPrompt(at("09:00"), Text("build it")),
		AssistantReply(at("09:01")),
		UserPrompt(at("09:10"), Text("rename the handler")),
		AssistantReply(at("09:11")),
		UserPrompt(at("09:20"), Text("clean up the tests, thanks")),
		AssistantReply(at("09:21")),
		UserPrompt(at("09:30"), Text("this is not what I asked")),
		AssistantReply(at("09:31")),
		UserPrompt(at("09:40"), Text("rename and clean up please")),
		AssistantReply(at("09:41")),
		UserPrompt(at("09:50"), Text("ship it")),
	)
	s.Given.Labelled("abc", 2, Yes("style"))
	s.Given.Labelled("abc", 3, No())
	s.Given.Labelled("abc", 4, Yes("other"))
	s.Given.Labelled("abc", 5, Yes("style"))

	s.When.Score()

	s.Then.ExitedNormally()
	s.Then.Reported("overall: found 2, missed 1, wrongly flagged 1, precision 67%, recall 67%")
	s.Then.Reported("style: found 2, missed 0, wrongly flagged 0, precision 100%, recall 100%")
	s.Then.Reported("cleanup: found 0, missed 0, wrongly flagged 2, precision 0%, recall n/a")
	s.Then.Reported("other: chosen 1 time")
}
