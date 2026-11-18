// Package transcript is the only code that knows Claude Code's transcript
// format and folder layout (docs/decisions/0003-parsing-in-one-module.md).
// Every field it relies on is recorded in docs/transcript-format.md.
package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

// Usage is a reply's token usage.
type Usage struct {
	Input, Output, CacheRead, CacheWrite int
}

// Add returns the sum of two usages.
func (u Usage) Add(o Usage) Usage {
	return Usage{u.Input + o.Input, u.Output + o.Output, u.CacheRead + o.CacheRead, u.CacheWrite + o.CacheWrite}
}

// Reply is one model reply, counted once however many lines it spans.
type Reply struct {
	ID    string
	Model string
	Usage Usage
}

// Transcript is what one transcript file tells us.
type Transcript struct {
	Times   []time.Time // every prompt and reply moment
	Replies []Reply     // real model replies, deduplicated by message ID
	Skipped int         // unreadable lines
}

// syntheticModel marks local placeholder replies that are not model calls.
const syntheticModel = "<synthetic>"

// record holds only the fields we use; encoding/json ignores the rest.
type record struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   *struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ParseFile reads one transcript file.
func ParseFile(path string) (Transcript, error) {
	f, err := os.Open(path)
	if err != nil {
		return Transcript{}, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads a transcript in JSONL format. Unreadable lines are counted and
// skipped; unknown record types and fields are ignored.
func Parse(r io.Reader) (Transcript, error) {
	var tr Transcript
	replyIndex := map[string]int{}
	br := bufio.NewReader(r) // no line-length limit: some records are megabytes
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			tr.add(line, replyIndex)
		}
		if errors.Is(err, io.EOF) {
			return tr, nil
		}
		if err != nil {
			return tr, err
		}
	}
}

func (tr *Transcript) add(line []byte, replyIndex map[string]int) {
	var rec record
	if err := json.Unmarshal(line, &rec); err != nil {
		tr.Skipped++
		return
	}
	if rec.Type != "user" && rec.Type != "assistant" {
		return
	}
	if rec.Type == "assistant" && rec.Message != nil && rec.Message.Model == syntheticModel {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, rec.Timestamp)
	if err != nil {
		tr.Skipped++
		return
	}
	tr.Times = append(tr.Times, at.UTC())

	if rec.Type != "assistant" || rec.Message == nil || rec.Message.ID == "" || rec.Message.Usage == nil {
		return
	}
	u := rec.Message.Usage
	reply := Reply{
		ID:    rec.Message.ID,
		Model: rec.Message.Model,
		Usage: Usage{u.InputTokens, u.OutputTokens, u.CacheReadInputTokens, u.CacheCreationInputTokens},
	}
	// One line per content block, each repeating the usage: keep one, the latest.
	if i, seen := replyIndex[reply.ID]; seen {
		tr.Replies[i] = reply
		return
	}
	replyIndex[reply.ID] = len(tr.Replies)
	tr.Replies = append(tr.Replies, reply)
}
