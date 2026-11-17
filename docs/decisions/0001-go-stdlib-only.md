# 0001. Go, standard library only
Status: Accepted
Date: 2026-11-13

## Context
We needed a language for a local CLI that reads JSONL and writes CSV.
JavaScript/Node was ruled out. Python was acceptable but not preferred. Rust
isn't installed. Swift's `Codable` is strict, which fights a transcript format
that changes.

## Decision
Go, using only the standard library. Tests use `go test`.

## Consequences
- `encoding/json` ignores unknown fields by default, which suits a changing format.
- JSON, CSV, file walking and testing are all in the standard library, so there
  are no dependencies and nothing is fetched from the network.
- The tool ships as one binary.
