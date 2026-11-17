# 0009. Stage 1 scope
Status: Accepted
Date: 2026-11-17

## Context
v0.1 stage 1 delivers `sessions.csv`. The first brief also mentioned thinking
volume and an API-equivalent € price.

## Decision
In scope: session ID, repo, start, end, models, and main and subagent tokens.

Out of scope:
- **Price.** The dashboard computes it; this tool only reports tokens.
- **Thinking volume.** Not valuable enough for now. (Transcripts do carry
  `output_tokens_details.thinking_tokens` if it's ever needed.)
- **Background model calls** that never appear as assistant replies, such as
  title generation. They show up only in `cost-state` records and aren't part
  of the conversation.
- **File changes and per-prompt analysis.** These are later stages.
