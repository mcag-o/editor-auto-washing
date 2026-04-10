package plugin

import (
	"fmt"
	"sort"
	"strings"
)

type Registry struct {
	plugins map[string]SourcePlugin
	aliases map[string]string
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: map[string]SourcePlugin{},
		aliases: map[string]string{},
	}
}

func (r *Registry) Register(p SourcePlugin) error {
	if p == nil {
		return fmt.Errorf("register plugin: nil plugin")
	}

	id := normalizeKey(p.SourceID())
	if id == "" {
		return fmt.Errorf("register plugin: empty source id")
	}
	if _, exists := r.plugins[id]; exists {
		return fmt.Errorf("register plugin %s: already registered", id)
	}

	aliasValues := append([]string{id}, p.Aliases()...)
	pendingAliases := make(map[string]string, len(aliasValues))
	for _, alias := range aliasValues {
		key := normalizeKey(alias)
		if key == "" {
			continue
		}
		if existing, exists := pendingAliases[key]; exists && existing != id {
			return fmt.Errorf("register alias %s: already mapped to %s", key, existing)
		}
		if existing, exists := r.aliases[key]; exists && existing != id {
			return fmt.Errorf("register alias %s: already mapped to %s", key, existing)
		}
		pendingAliases[key] = id
	}

	r.plugins[id] = p
	for key, canonical := range pendingAliases {
		r.aliases[key] = canonical
	}

	return nil
}

func (r *Registry) Get(idOrAlias string) (SourcePlugin, error) {
	key := normalizeKey(idOrAlias)
	if canonical, ok := r.aliases[key]; ok {
		return r.plugins[canonical], nil
	}
	if p, ok := r.plugins[key]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("plugin not registered: %s", idOrAlias)
}

func (r *Registry) List() []SourcePlugin {
	ids := make([]string, 0, len(r.plugins))
	for id := range r.plugins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	items := make([]SourcePlugin, 0, len(ids))
	for _, id := range ids {
		items = append(items, r.plugins[id])
	}
	return items
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
