# Roadmap

What comes after stage 1, so it isn't lost. Items here are **not agreed
behaviours yet**. They become stories in `docs/stories/` once we've inspected
the transcripts for them and decided what counts.

## Stage 1: sessions.csv (in progress)
See [stories/sessions.md](stories/sessions.md).

Open: **thinking tokens.** Dropped because we thought there would only be an
estimate. The inspection showed an exact count exists
(`message.usage.output_tokens_details.thinking_tokens`, see
[transcript-format.md](transcript-format.md)). It could come back as
`main_thinking` and `subagent_thinking` columns.

## Stage 2: prompts.csv (most valuable)
One row per user prompt in the main conversation. Subagents are excluded from
conversation metrics, because the user never talks to them
([0008](decisions/0008-main-and-subagent-tokens.md)). Subagent file changes
still count toward the prompt that spawned them.

Priorities, in order of value:

### 1. User corrections
Did the user have to correct or redirect Claude after this prompt?
Signals to investigate:
- the user interrupting Claude mid-reply;
- rejected tool calls (`toolDenialKind` and `permissionDecision` were seen, not yet checked in detail);
- undo or rewind;
- a follow-up prompt that redirects ("no", "that's wrong", "instead …").

### 2. Ambiguous prompts
Was the prompt unclear enough that Claude had to ask, or got it wrong?
Signals to investigate:
- Claude asking a clarifying question (an `AskUserQuestion` call, or a reply
  ending in a question with no tool use);
- a correction soon after the prompt.

### 3. File changes
What did this prompt change? Files touched, and lines added and removed.
Signals to investigate:
- edit and write tool calls;
- `file-history-delta` records.

This is likely the easiest of the three.

### Also per prompt
- **Model per prompt**, to see whether things go wrong more often with certain
  models ([0010](decisions/0010-models-as-list.md)).

### Open questions before stage 2
- **Local versus meaning.** Reliably telling a correction or an ambiguous prompt
  from the text needs understanding it. A model could judge that, but it would
  mean sending prompts to an API, which [0004](decisions/0004-local-and-anonymous.md)
  rules out. The options: structural signals plus simple local text patterns (less
  precise, private), or revisit 0004 for this metric only.
- **Prompt text.** Pattern matching reads the text locally, but the CSV must
  never contain it.

### First step of stage 2
Inspect transcripts for the signals above, structure only, the same way as in
stage 1. Record the findings in `transcript-format.md`, then decide together
what counts as a correction or an ambiguous prompt.
