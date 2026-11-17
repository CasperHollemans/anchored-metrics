# 0010. Models as a list column
Status: Accepted
Date: 2026-11-17

## Context
A session can use several models. One column per model (`opus_tokens`,
`fable_tokens` …) would break every time a new model ships. The real goal is to
see per prompt which model was used, to find where things go wrong.

## Decision
`sessions.csv` has one `models` column listing the models used, `;`-separated.
Per-prompt model tracking belongs in a later `prompts.csv`.

## Consequences
- New models need no schema change.
- Token counts per model aren't available in `sessions.csv`.
