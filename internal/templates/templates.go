package templates

import (
	"embed"
	"html/template"
	"sync"
)

//go:embed *.html
var templateFiles embed.FS

// TemplateManager manages HTML templates
type TemplateManager struct {
	templates map[string]*template.Template
	mutex     sync.RWMutex
}

// NewTemplateManager creates a new template manager
func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		templates: make(map[string]*template.Template),
	}
}

// LoadTemplate loads a template by name, caching it for future use
func (tm *TemplateManager) LoadTemplate(name string) (*template.Template, error) {
	tm.mutex.RLock()
	tmpl, exists := tm.templates[name]
	tm.mutex.RUnlock()
	
	if exists {
		return tmpl, nil
	}
	
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	// Double-check after acquiring write lock
	if tmpl, exists := tm.templates[name]; exists {
		return tmpl, nil
	}
	
	// Load template from embedded files
	content, err := templateFiles.ReadFile(name + ".html")
	if err != nil {
		return nil, err
	}
	
	tmpl, err = template.New(name).Parse(string(content))
	if err != nil {
		return nil, err
	}
	
	tm.templates[name] = tmpl
	return tmpl, nil
}

// GetTemplate gets a cached template or loads it if not cached
func (tm *TemplateManager) GetTemplate(name string) (*template.Template, error) {
	return tm.LoadTemplate(name)
}

// Global template manager instance
var globalTemplateManager = NewTemplateManager()

// GetTemplate is a convenience function to get templates from the global manager
func GetTemplate(name string) (*template.Template, error) {
	return globalTemplateManager.GetTemplate(name)
}