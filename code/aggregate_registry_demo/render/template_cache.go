package render

import (
	"os"
	"path/filepath"
	"sync"
	"text/template"
)

type templateCache struct {
	mu    sync.RWMutex
	items map[string]*template.Template
}

func (c *templateCache) get(templatePath string) (*template.Template, error) {
	c.mu.RLock()
	tmpl := c.items[templatePath]
	c.mu.RUnlock()
	if tmpl != nil {
		return tmpl, nil
	}

	data, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}

	tmpl, err = template.New(filepath.Base(templatePath)).
		Option("missingkey=error").
		Parse(string(data))
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.items == nil {
		c.items = map[string]*template.Template{}
	}
	if existing := c.items[templatePath]; existing != nil {
		c.mu.Unlock()
		return existing, nil
	}
	c.items[templatePath] = tmpl
	c.mu.Unlock()
	return tmpl, nil
}

var defaultTemplateCache templateCache
