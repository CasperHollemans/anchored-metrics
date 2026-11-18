# Roadmap

What comes later, so it isn't lost. Items here are **not agreed behaviours
yet**. They become stories in `docs/stories/` once we've inspected the
transcripts for them and decided what counts.

## Stage 1: sessions.csv (done)
See [stories/sessions.md](stories/sessions.md).

Open: **thinking tokens.** Dropped because we thought there would only be an
estimate. The inspection showed an exact count exists
(`message.usage.output_tokens_details.thinking_tokens`, see
[transcript-format.md](transcript-format.md)). It could come back as
`main_thinking` and `subagent_thinking` columns.

## Stage 2: prompts.csv (in progress)
See [stories/prompts.md](stories/prompts.md): corrections with categories,
interrupts, clarifying questions, file changes and models per prompt.

## Later

### Model judge for corrections
If the word library scores poorly against the labels, add a model that judges
prompts at sync time, behind the same interface. This would need a decision
superseding [0004](decisions/0004-local-and-anonymous.md) for that metric
([0012](decisions/0012-correction-word-library.md)).

### Signals that need more study
Seen in the stage 2 inspection, but not understood well enough to measure
([transcript-format.md](transcript-format.md)):
- **Mid-turn messages.** The user typing while Claude works. `queue-operation`
  isn't it (every prompt is queued), but patterns like enqueue, enqueue or
  `remove` might be.
- **The user changing Claude's edit.** The `userModified` field on edit results.
- **The user refusing a tool.** Not seen, because all sessions ran in auto or
  bypass mode.
- **Undo and rewind.** Branching `parentUuid`s look structural, not like rewinds.
- **Edited files outside Claude.** `attachment.type: "edited_text_file"`.

### Ambiguous prompts
Beyond `asked_question`: a reply that ends in a question without using a tool,
or a correction soon after a short prompt. `words` and `correction` in
`prompts.csv` already allow a first look in the dashboard.
