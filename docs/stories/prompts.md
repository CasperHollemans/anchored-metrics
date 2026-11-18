# Stage 2: prompts.csv

*As a developer, I want to see which of my prompts corrected or re-steered
Claude, in which way, and what each prompt changed, so I can find where working
with Claude goes wrong and how that changes over time.*

```
anchored-metrics sync <repo-path>
```

Writes `out/sessions.csv` and `out/prompts.csv` in the current directory
([0011](../decisions/0011-one-sync-command.md)).

Behaviour IDs use the prefix `P-`. Don't renumber: retire an ID and add a new one.

## Columns

| Column | Meaning |
|---|---|
| `session` | Session ID |
| `repo` | Repo folder name |
| `prompt` | Position of the prompt in the session: 1, 2, 3 … |
| `at` | When the prompt was sent, UTC, to the second |
| `words` | Length of the prompt in words |
| `models` | Models that replied to this prompt, including its subagents, sorted, `;`-separated |
| `interrupted` | `true` if the user interrupted Claude while it worked on this prompt |
| `asked_question` | `true` if Claude asked the user a clarifying question while working on this prompt |
| `files_changed` | Distinct files Claude created or edited for this prompt, including its subagents |
| `lines_added`, `lines_removed` | Lines added and removed by those edits |
| `correction` | `true` if the prompt re-steers Claude according to the correction library |
| `correction_categories` | Which kinds of re-steering matched, sorted, `;`-separated, e.g. `cleanup;style` |
| `correction_hits` | How many different library entries matched; the more hits, the more worth a look |

A prompt's "turn" is everything from that prompt until the next prompt.

## What counts as a correction
Any prompt that says the result didn't meet the user's expectation, or asks for
more after Claude delivered something:
- it's wrong or broken ("this doesn't work", "werkt niet");
- not what I meant, do it differently ("instead", "liever");
- style, naming or formatting isn't right ("looks good, but rename …");
- clean it up or improve it ("also clean up the code", "verbeter");
- something is missing or forgotten ("you forgot", "ontbreekt");
- undo it ("revert", "terugdraaien").

Positive feedback on its own ("great, thanks") is not a correction.

## Behaviours

### Finding prompts
- **P-01** Each prompt the user typed in the main conversation becomes one row.
- **P-02** Tool results, meta records, compaction summaries, background-task notifications and the interrupt marker are not prompts.
- **P-03** Subagent transcripts never produce prompt rows; their work counts toward the prompt that spawned them.
- **P-04** Prompts come from the same repos and worktrees as sessions, and all repos share one `prompts.csv`.

### What each row says
- **P-05** Each row has its session, repo, position in the session, time in UTC and length in words.
- **P-06** The row lists the models that replied to the prompt, including the models of subagents spawned for it.
- **P-07** A prompt is marked `interrupted` when the user interrupted Claude while it worked on that prompt; the user's next prompt is a row of its own.
- **P-08** A prompt is marked `asked_question` when Claude asked the user a clarifying question while working on it.
- **P-09** `files_changed` counts the distinct files Claude created or edited for the prompt, including through subagents.
- **P-10** Lines added and removed come from each edit's patch; a newly created file counts all its lines as added.

### Corrections ([0012](../decisions/0012-correction-word-library.md))
- **P-11** A prompt is marked `correction` when it contains an entry from the correction library, in English or Dutch, ignoring case and matching whole words.
- **P-12** Every library entry belongs to a category, and the row lists every category that matched, so kinds of re-steering can be followed over time.
- **P-13** An entry ending in `*` matches any word that starts with it, so `verbeter*` matches "verbeter", "verbeteren" and "verbetering".
- **P-22** The row counts how many different library entries matched, so prompts that tick many words and categories stand out.
- **P-23** The first prompt of a session is never a correction, because nothing has been delivered yet to correct.
- **P-14** The library is a plain-text file in the repo, with categories as headings and one entry per line, so it can grow without code changes.

### Privacy
- **P-15** `prompts.csv` never contains prompt text, matched words, file names or paths. Category names are allowed.

### Syncing
- **P-16** One `sync` command writes both `sessions.csv` and `prompts.csv`.
- **P-17** Prompt rows follow the same rules as session rows: rows of synced sessions are replaced, and rows whose transcript is gone are kept.

### Labelling and scoring
- **P-18** `anchored-metrics label <repo>` shows unlabelled prompts (never a session's first prompt, see P-23) one at a time in the user's own terminal, after the last thing Claude said before the prompt (usually its summary of what it did), and asks: correction yes, no, skip or quit.
- **P-24** After a yes, the user picks one or more categories from the library, or `other` when none fits, so missing categories show up.
- **P-19** Labels are stored outside the repo, in `~/.anchored-metrics/labels.csv`, by session and prompt position, with the chosen categories, never with prompt text.
- **P-20** Labelling can be stopped at any time and picks up where it left off.
- **P-21** `anchored-metrics score` compares the library with the labels: overall found, missed and wrongly flagged, with precision and recall, and the same per category, plus how often labels used `other`.
