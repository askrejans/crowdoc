package ast

import (
	"sort"
	"strings"
	"sync"
)

// Resource is an embedded binary asset.
type Resource struct {
	Name      string // unique name, e.g. "media/image1.png"
	MediaType string // e.g. "image/png"; may be empty
	Data      []byte
}

// Resources is a concurrency-safe set of embedded assets keyed by name.
type Resources struct {
	mu    sync.RWMutex
	items map[string]*Resource
}

// NewResources returns an empty resource set.
func NewResources() *Resources { return &Resources{items: map[string]*Resource{}} }

// Add stores data under name and returns the "res:" reference to use as an
// Image source. If name is already taken by different data, a unique name is
// derived.
func (r *Resources) Add(name, mediaType string, data []byte) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.items == nil {
		r.items = map[string]*Resource{}
	}
	name = strings.TrimPrefix(strings.ReplaceAll(name, `\`, "/"), "/")
	if name == "" {
		name = "resource"
	}
	base, ext := name, ""
	if i := strings.LastIndexByte(name, '.'); i > strings.LastIndexByte(name, '/') {
		base, ext = name[:i], name[i:]
	}
	for n := 2; ; n++ {
		existing, ok := r.items[name]
		if !ok {
			break
		}
		if string(existing.Data) == string(data) {
			return "res:" + name
		}
		name = base + "-" + itoa(n) + ext
	}
	r.items[name] = &Resource{Name: name, MediaType: mediaType, Data: data}
	return "res:" + name
}

// Get returns the resource with the given name (with or without "res:").
func (r *Resources) Get(name string) (*Resource, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	res, ok := r.items[strings.TrimPrefix(name, "res:")]
	return res, ok
}

// Names returns all resource names in sorted order.
func (r *Resources) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.items))
	for k := range r.items {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Len returns the number of resources.
func (r *Resources) Len() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
