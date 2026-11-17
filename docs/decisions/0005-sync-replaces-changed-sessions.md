# 0005. Sync replaces changed sessions, keeps deleted ones
Status: Accepted
Date: 2026-11-17

## Context
Old data must never be lost. By default Claude Code deletes transcripts after
about 30 days (`cleanupPeriodDays`), so the CSV must outlive the transcripts.
Sessions keep growing after a sync: one inspected session was written to over
three days. A row is a summary of a whole session, so a row written halfway
through would be wrong forever if it couldn't change.

We considered:
- **A.** Replace rows of changed sessions.
- **B.** Only write sessions that have been idle for N hours.
- **C.** Append the new part as an extra row.
- **D.** Append a snapshot row per sync, where the latest row wins.

## Decision
A. Each sync re-reads the transcripts that changed since the last sync and
replaces their rows. A row whose transcript no longer exists is kept unchanged.

## Consequences
- Rows can change between syncs, but only towards newer, more complete numbers.
- File modified times can't be trusted: content from October showed November
  modified times. Re-reading an unchanged session gives an identical row, so
  an unneeded re-read costs time but never correctness.
- There is no sync history. Option D could add it later if needed.
