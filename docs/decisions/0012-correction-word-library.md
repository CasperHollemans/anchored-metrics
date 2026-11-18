# 0012. Corrections detected locally with a categorised word library
Status: Accepted
Date: 2026-11-18

## Context
The most valuable metric is how often, and how, the user re-steers Claude. The
transcripts don't mark corrections (`docs/transcript-format.md`). The options
were:
- a categorised word library, run locally at sync time;
- a hook that asks a model to judge every prompt live, which slows down work and
  misses all past sessions;
- a model judging prompts at sync time, which sends prompt text to an API and
  conflicts with [0004](0004-local-and-anonymous.md).

Nobody knows how reliable a word library is.

## Decision
- A plain-text library in the repo, in English and Dutch, with categories (for
  example wrong, redirect, style, cleanup, improve, missing, revert). It matches
  whole words, ignores case, and supports `prefix*`.
- Reliability is measured, not assumed: the user labels a sample of their own
  prompts locally, and a score command compares the library against those
  labels, overall and per category.
- A model judge is not built now. It's reconsidered if the score disappoints,
  behind the same interface, and would need a decision superseding 0004 for
  that metric.

## Consequences
- Fully local and free, and it works on past sessions.
- The library will over- and under-match. The labels and score make this
  visible, and the library improves by editing a text file.
- Labels are personal data about prompts, so they're stored outside the repo,
  without prompt text.
