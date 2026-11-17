# 0007. Worktrees mapped through Claude Code's worktree folders and git
Status: Accepted
Date: 2026-11-17

## Context
Claude Code stores each working directory's sessions in its own project folder,
named after the path with non-alphanumeric characters replaced by `-`. A worktree
therefore gets its own folder. Worktrees can be made by Claude Code (inside the
repo, under `.claude/worktrees/` as far as we know, not yet verified), by
editors such as paradev (`~/.paradev/worktrees/<repo>/<name>`), or by custom
scripts placed anywhere. Transcripts only record the worktree's `cwd` and
`gitBranch`, not the main repo.

This is a nice-to-have: useful, but not critical.

## Decision
A repo's sessions come from:
1. its own project folder;
2. project folders for Claude Code's own worktrees inside the repo: the
   encoded repo path followed by `--claude-worktrees-`;
3. project folders of every path listed by `git worktree list` in the repo.

A plain name-prefix match is not used. Because the encoding is lossy, the repo
`shop` would also match a sibling repo `shop-admin` (`…-shop` is a prefix of
`…-shop-admin`).

There is no tool-specific logic, such as for paradev. Paths are encoded the way Claude
Code does; folder names are never decoded back, because the encoding is lossy.

## Consequences
- The tool runs one local git command.
- Sessions started in a subfolder of the repo (for example `<repo>/docs`) get
  their own project folder and are not included, for the same lossy-encoding
  reason. This is a known limitation.
- A deleted worktree drops out of `git worktree list`. Its sessions stay in the
  CSV if they were synced while it existed ([0005](0005-sync-replaces-changed-sessions.md)).
