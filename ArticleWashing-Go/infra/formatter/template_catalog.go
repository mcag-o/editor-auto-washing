package formatter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TemplateCatalog struct {
	roots []string
}

func NewTemplateCatalog(roots []string) *TemplateCatalog {
	cleaned := make([]string, 0, len(roots))
	seen := map[string]struct{}{}
	for _, root := range roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	return &TemplateCatalog{roots: cleaned}
}

func (c *TemplateCatalog) ReadTemplate(name string) (string, error) {
	for _, candidate := range c.candidates(name) {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return string(data), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("read template %s: %w", candidate, err)
		}
	}
	return "", fmt.Errorf("template %s not found in configured roots", name)
}

func (c *TemplateCatalog) ListTemplates() ([]string, error) {
	seen := map[string]struct{}{}
	for _, root := range c.roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read template root %s: %w", root, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			if ext != ".html" && ext != ".tmpl" {
				continue
			}
			seen[strings.TrimSuffix(name, ext)] = struct{}{}
		}
	}
	templates := make([]string, 0, len(seen))
	for name := range seen {
		templates = append(templates, name)
	}
	sort.Strings(templates)
	return templates, nil
}

func (c *TemplateCatalog) candidates(name string) []string {
	base := strings.TrimSpace(name)
	if base == "" {
		return nil
	}
	variants := []string{base}
	if filepath.Ext(base) == "" {
		variants = append(variants, base+".html", base+".tmpl")
	}
	paths := make([]string, 0, len(c.roots)*len(variants))
	for _, root := range c.roots {
		for _, variant := range variants {
			paths = append(paths, filepath.Join(root, variant))
		}
	}
	return paths
}
