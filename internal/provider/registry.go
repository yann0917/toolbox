package provider

import "fmt"

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func key(provider, name string) string { return provider + "." + name }

func (r *Registry) Register(t Tool) error {
	m := t.Meta()
	k := key(m.Provider, m.Name)
	if _, exists := r.tools[k]; exists {
		return fmt.Errorf("tool %s already registered", k)
	}
	r.tools[k] = t
	return nil
}

func (r *Registry) Get(provider, name string) (Tool, bool) {
	t, ok := r.tools[key(provider, name)]
	return t, ok
}

func (r *Registry) List() []ToolMeta {
	out := make([]ToolMeta, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Meta())
	}
	return out
}
