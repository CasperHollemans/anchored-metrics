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
	p := parser{replyIndex: map[string]int{}}
	br := bufio.NewReader(r) // no line-length limit: some records are megabytes
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			p.addLine(line)
		}
		if errors.Is(err, io.EOF) {
			return p.tr, nil
		}
		if err != nil {
			return p.tr, err
		}
	}
}

type parser struct {
	tr         Transcript
	replyIndex map[string]int // message ID -> index in tr.Replies
}

func (p *parser) addLine(line []byte) {
	var rec record
	if err := json.Unmarshal(line, &rec); err != nil {
		p.tr.Skipped++
		return
	}
	if !rec.isPromptOrRealReply() {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, rec.Timestamp)
	if err != nil {
		p.tr.Skipped++
		return
	}
	p.tr.Times = append(p.tr.Times, at.UTC())
	if reply, ok := rec.reply(); ok {
		p.addReply(reply)
	}
}

// addReply keeps one copy per message ID, the latest: Claude Code writes a
// line per content block, each repeating the same usage.
func (p *parser) addReply(reply Reply) {
	if i, seen := p.replyIndex[reply.ID]; seen {
		p.tr.Replies[i] = reply
		return
	}
	p.replyIndex[reply.ID] = len(p.tr.Replies)
	p.tr.Replies = append(p.tr.Replies, reply)
}

func (rec record) isPromptOrRealReply() bool {
	switch rec.Type {
	case "user":
		return true
	case "assistant":
		return rec.Message == nil || rec.Message.Model != syntheticModel
	default:
		return false
	}
}

// reply is the record's reply, if it is an assistant record with an ID and usage.
func (rec record) reply() (Reply, bool) {
	if rec.Type != "assistant" || rec.Message == nil || rec.Message.ID == "" || rec.Message.Usage == nil {
		return Reply{}, false
	}
	u := rec.Message.Usage
	return Reply{
		ID:    rec.Message.ID,
		Model: rec.Message.Model,
		Usage: Usage{u.InputTokens, u.OutputTokens, u.CacheReadInputTokens, u.CacheCreationInputTokens},
	}, true
}
