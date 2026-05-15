package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuditor(t *testing.T) {
	auditor := NewAuditor()
	assert.NotNil(t, auditor)
	assert.Equal(t, 0, auditor.Size())
	assert.Nil(t, auditor.Last())
}

func TestAuditorRecord(t *testing.T) {
	auditor := NewAuditor()

	oldCfg := DefaultConfig()
	newCfg := DefaultConfig()
	newCfg.HTTP.Port = 9090

	changes, err := Diff(oldCfg, newCfg)
	require.NoError(t, err)
	require.True(t, changes.HasChanges())

	auditor.Record(oldCfg, newCfg, changes, "test")

	assert.Equal(t, 1, auditor.Size())
	entry := auditor.Last()
	require.NotNil(t, entry)
	assert.Equal(t, "test", entry.Source)
	assert.True(t, len(entry.Changes) > 0)
}

func TestAuditorMultipleRecords(t *testing.T) {
	auditor := NewAuditor()

	cfg1 := DefaultConfig()
	cfg2 := DefaultConfig()
	cfg2.HTTP.Port = 9090

	changes1, _ := Diff(cfg1, cfg2)
	auditor.Record(cfg1, cfg2, changes1, "first")

	cfg3 := DefaultConfig()
	cfg3.HTTP.Port = 7070

	changes2, _ := Diff(cfg2, cfg3)
	auditor.Record(cfg2, cfg3, changes2, "second")

	assert.Equal(t, 2, auditor.Size())

	last := auditor.Last()
	require.NotNil(t, last)
	assert.Equal(t, "second", last.Source)
}

func TestAuditorDiffDetection(t *testing.T) {
	oldCfg := DefaultConfig()
	newCfg := DefaultConfig()
	newCfg.Log.Level = "debug"
	newCfg.Database.MaxOpenConns = 20

	changes, err := Diff(oldCfg, newCfg)
	require.NoError(t, err)
	require.True(t, changes.HasChanges())

	paths := changes.AffectedPaths()
	assert.Contains(t, paths, "Log.Level")
	assert.Contains(t, paths, "Database.MaxOpenConns")
}

func TestAuditorNoChanges(t *testing.T) {
	cfg := DefaultConfig()

	changes, err := Diff(cfg, cfg)
	require.NoError(t, err)
	assert.False(t, changes.HasChanges())
	assert.Len(t, changes, 0)
}

func TestAuditorHashes(t *testing.T) {
	auditor := NewAuditor()

	oldCfg := DefaultConfig()
	newCfg := DefaultConfig()
	newCfg.HTTP.Port = 8888

	changes, _ := Diff(oldCfg, newCfg)
	auditor.Record(oldCfg, newCfg, changes, "hash-test")

	entry := auditor.Last()
	require.NotNil(t, entry)
	assert.NotEmpty(t, entry.OldHash)
	assert.NotEmpty(t, entry.NewHash)
	assert.NotEqual(t, entry.OldHash, entry.NewHash)
}

func TestChangeString(t *testing.T) {
	tests := []struct {
		change   Change
		expected string
	}{
		{
			change:   Change{Type: "create", Path: "http.port", To: 8123},
			expected: "+ http.port = 8123",
		},
		{
			change:   Change{Type: "delete", Path: "log.format", From: "json"},
			expected: "- log.format (was json)",
		},
		{
			change:   Change{Type: "update", Path: "http.host", From: "0.0.0.0", To: "127.0.0.1"},
			expected: "~ http.host: 0.0.0.0 -> 127.0.0.1",
		},
		{
			change:   Change{Type: "unknown", Path: "something"},
			expected: "? something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.change.Type, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.change.String())
		})
	}
}

func TestChangesSummary(t *testing.T) {
	changes := Changes{
		{Type: "create", Path: "a"},
		{Type: "create", Path: "b"},
		{Type: "update", Path: "c"},
		{Type: "delete", Path: "d"},
	}

	summary := changes.Summary()
	assert.Equal(t, 2, summary["create"])
	assert.Equal(t, 1, summary["update"])
	assert.Equal(t, 1, summary["delete"])
}

func TestChangesAffectedPaths(t *testing.T) {
	changes := Changes{
		{Path: "http.port"},
		{Path: "log.level"},
		{Path: "database.path"},
	}

	paths := changes.AffectedPaths()
	assert.Len(t, paths, 3)
	assert.Contains(t, paths, "http.port")
	assert.Contains(t, paths, "log.level")
	assert.Contains(t, paths, "database.path")
}

func TestAuditLogAddAndLast(t *testing.T) {
	log := &AuditLog{}

	entry1 := AuditEntry{Source: "first"}
	log.Add(entry1)

	entry2 := AuditEntry{Source: "second"}
	log.Add(entry2)

	assert.Equal(t, 2, log.Size())

	last := log.Last()
	require.NotNil(t, last)
	assert.Equal(t, "second", last.Source)
}

func TestAuditLogLastEmpty(t *testing.T) {
	log := &AuditLog{}
	assert.Nil(t, log.Last())
}
