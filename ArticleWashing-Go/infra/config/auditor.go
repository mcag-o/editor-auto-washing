package config

import (
	"fmt"
	"time"

	"github.com/r3labs/diff/v3"
)

type Change struct {
	Type      string      `json:"type"`
	Path      string      `json:"path"`
	From      interface{} `json:"from,omitempty"`
	To        interface{} `json:"to,omitempty"`
	Timestamp string      `json:"timestamp"`
}

type Changes []Change

type AuditLog struct {
	Entries []AuditEntry `json:"entries"`
}

type AuditEntry struct {
	Timestamp string  `json:"timestamp"`
	OldHash   string  `json:"old_hash"`
	NewHash   string  `json:"new_hash"`
	Changes   Changes `json:"changes"`
	Source    string  `json:"source"`
}

func Diff(old, new Config) (Changes, error) {
	changelog, err := diff.Diff(old, new)
	if err != nil {
		return nil, fmt.Errorf("failed to compute diff: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var changes Changes
	for _, c := range changelog {
		change := Change{
			Type:      c.Type,
			Path:      buildPath(c.Path),
			Timestamp: now,
		}
		if c.From != nil {
			change.From = c.From
		}
		if c.To != nil {
			change.To = c.To
		}
		changes = append(changes, change)
	}

	return changes, nil
}

func buildPath(path []string) string {
	result := ""
	for i, p := range path {
		if i > 0 {
			result += "."
		}
		result += p
	}
	return result
}

func CreateAuditEntry(old, new Config, changes Changes, source string) AuditEntry {
	oldHash, _ := old.Hash()
	newHash, _ := new.Hash()

	return AuditEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		OldHash:   oldHash,
		NewHash:   newHash,
		Changes:   changes,
		Source:    source,
	}
}

func (a *AuditLog) Add(entry AuditEntry) {
	a.Entries = append(a.Entries, entry)
}

func (a *AuditLog) Last() *AuditEntry {
	if len(a.Entries) == 0 {
		return nil
	}
	return &a.Entries[len(a.Entries)-1]
}

func (a *AuditLog) Size() int {
	return len(a.Entries)
}

func (c Changes) HasChanges() bool {
	return len(c) > 0
}

func (c Changes) summary() map[string]int {
	counts := make(map[string]int)
	for _, ch := range c {
		counts[ch.Type]++
	}
	return counts
}

func (c Changes) AffectedPaths() []string {
	paths := make([]string, len(c))
	for i, ch := range c {
		paths[i] = ch.Path
	}
	return paths
}

func (c Change) String() string {
	switch c.Type {
	case "create":
		return fmt.Sprintf("+ %s = %v", c.Path, c.To)
	case "delete":
		return fmt.Sprintf("- %s (was %v)", c.Path, c.From)
	case "update":
		return fmt.Sprintf("~ %s: %v -> %v", c.Path, c.From, c.To)
	default:
		return fmt.Sprintf("? %s", c.Path)
	}
}
