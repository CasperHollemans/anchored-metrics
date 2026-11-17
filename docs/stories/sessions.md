# Stage 1: sessions.csv

*As a developer, I want a CSV of my Claude Code sessions per repo, so I can
analyse how I work with Claude.*

```
anchored-metrics sessions <repo-path>
```

Writes `out/sessions.csv` in the current directory.

Each behaviour has an ID. Every acceptance test starts its comment with the ID
it covers, and a test enforces that each ID has a test. Don't renumber: retire
an ID and add a new one.

## Columns

| Column | Meaning |
|---|---|
| `session` | Session ID (the transcript's UUID) |
| `repo` | Repo folder name |
| `start` | Earliest prompt or reply timestamp in the session, UTC, to the second (`2026-11-01T08:00:00Z`) |
| `end` | Latest prompt or reply timestamp in the session, UTC, to the second |
| `models` | Models used by the session and its subagents, sorted, `;`-separated |
| `main_input`, `main_output`, `main_cache_read`, `main_cache_write` | Tokens of the main conversation |
| `subagent_input`, `subagent_output`, `subagent_cache_read`, `subagent_cache_write` | Tokens of subagents spawned in the session |

## Behaviours

### Finding sessions
- **S-01** Syncing a repo reads every session transcript in that repo's Claude Code project folder.
- **S-02** Sessions from the repo's worktrees are included as well, but not sessions from a sibling repo with a similar name. ([0007](../decisions/0007-worktree-mapping.md))
- **S-03** Files that aren't session transcripts are ignored: metadata, screenshots, tool results and memory.
- **S-04** Syncing a repo path that has no Claude Code sessions prints a warning, writes nothing and exits normally.

### One row per session
- **S-05** Each session becomes exactly one row, identified by its session ID.
- **S-06** All repos go into one `sessions.csv`, and each row includes the repo name. ([0006](../decisions/0006-one-csv-per-metric.md))
- **S-07** Start is the session's earliest prompt or reply and end is its latest, both in UTC, even when records are out of order.
- **S-08** The row lists every model the session used, including the models its subagents used. ([0010](../decisions/0010-models-as-list.md))
- **S-09** Input, output, cache-read and cache-write tokens are counted once per reply, even though the transcript repeats a reply across several lines.
- **S-10** Main-session and subagent tokens are counted separately, each as input, output, cache read and cache write. ([0008](../decisions/0008-main-and-subagent-tokens.md))
- **S-11** Placeholder replies (`<synthetic>`) add neither a model nor tokens.

### Privacy
- **S-12** The CSV never contains usernames, file paths or prompt text. ([0004](../decisions/0004-local-and-anonymous.md))

### Syncing over time ([0005](../decisions/0005-sync-replaces-changed-sessions.md))
- **S-13** The first sync writes a row for every session.
- **S-14** Syncing again with no changes leaves the CSV exactly as it was.
- **S-15** A session that grew since the last sync gets its row replaced with the new numbers.
- **S-16** A new session gets a new row.
- **S-17** A session whose transcript has since been deleted keeps its row.

### Handling format changes ([0003](../decisions/0003-parsing-in-one-module.md))
- **S-18** A line that can't be read is skipped, the sync carries on, and it reports how many lines it skipped.
- **S-19** Record types and fields the tool doesn't know are ignored.

## Out of scope for this stage
See [0009](../decisions/0009-stage-1-scope.md): price, thinking volume, background
model calls, file changes and per-prompt analysis. What comes next, including
user corrections, ambiguous prompts and file changes, is in
[roadmap.md](../roadmap.md).
