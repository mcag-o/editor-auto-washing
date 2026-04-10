package plugin_test

import (
	"context"
	"testing"
	"time"

	"content-hub/collector/plugin"
	"content-hub/collector/plugin/sources"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_ResolvesRegisteredPlugin(t *testing.T) {
	reg := plugin.NewRegistry()
	p, err := sources.NewBaidu()
	require.NoError(t, err)
	require.NoError(t, reg.Register(p))

	p, err = reg.Get("baidu")
	require.NoError(t, err)
	assert.Equal(t, "baidu", p.SourceID())
}

func TestRegistry_ResolvesAliases(t *testing.T) {
	reg := plugin.NewRegistry()
	p, err := sources.NewStackOverflow()
	require.NoError(t, err)
	require.NoError(t, reg.Register(p))

	p, err = reg.Get("StackOverflow")
	require.NoError(t, err)
	assert.Equal(t, "stackoverflow", p.SourceID())
}

func TestRegistry_RejectsDuplicatePluginID(t *testing.T) {
	reg := plugin.NewRegistry()
	p, err := sources.NewGitHub()
	require.NoError(t, err)
	require.NoError(t, reg.Register(p))

	duplicate, err := sources.NewGitHub()
	require.NoError(t, err)
	err = reg.Register(duplicate)

	require.Error(t, err)
	assert.ErrorContains(t, err, "github")
}

func TestRegistry_RejectsAliasConflicts(t *testing.T) {
	reg := plugin.NewRegistry()
	require.NoError(t, reg.Register(newStubPlugin("github", "github")))

	err := reg.Register(newStubPlugin("other", "github"))

	require.Error(t, err)
	assert.ErrorContains(t, err, "alias")
}

func TestRegistry_RegisterIsAtomicOnAliasConflict(t *testing.T) {
	reg := plugin.NewRegistry()
	require.NoError(t, reg.Register(newStubPlugin("github", "github")))

	err := reg.Register(newStubPlugin("other", "unique", "github"))

	require.Error(t, err)
	_, getErr := reg.Get("other")
	require.Error(t, getErr)
	_, aliasErr := reg.Get("unique")
	require.Error(t, aliasErr)
	items := reg.List()
	require.Len(t, items, 1)
	assert.Equal(t, "github", items[0].SourceID())
}

type stubPlugin struct {
	sourceID string
	aliases  []string
}

func newStubPlugin(sourceID string, aliases ...string) plugin.SourcePlugin {
	return stubPlugin{sourceID: sourceID, aliases: aliases}
}

func (s stubPlugin) SourceID() string    { return s.sourceID }
func (s stubPlugin) DisplayName() string { return s.sourceID }
func (s stubPlugin) Aliases() []string   { return append([]string(nil), s.aliases...) }
func (s stubPlugin) FetchHotlist(context.Context, plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	return nil, nil
}
func (s stubPlugin) FetchArticle(context.Context, plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	return nil, nil
}
func (s stubPlugin) NormalizeHotEntry(any) (plugin.HotEntry, error) { return plugin.HotEntry{}, nil }
func (s stubPlugin) NormalizeArticle(any) (*plugin.NormalizedArticle, error) {
	return nil, nil
}
func (s stubPlugin) HealthCheck(context.Context) (plugin.SourceHealth, error) {
	return plugin.SourceHealth{SourceID: s.sourceID, OK: true, CheckedAt: time.Now().UTC()}, nil
}
func (s stubPlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{SupportsHotlist: true}
}
