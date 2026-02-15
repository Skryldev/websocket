package wsx

import (
	"sort"
	"strings"
	"sync"
)

type wildcardRoute struct {
	prefix  string
	version string
	handler HandlerFunc
}

type Router struct {
	mu       sync.RWMutex
	exact    map[string]map[string]HandlerFunc
	wildcard []wildcardRoute
}

func NewRouter() *Router {
	return &Router{
		exact: make(map[string]map[string]HandlerFunc),
	}
}

func (r *Router) Handle(topic string, h HandlerFunc) {
	r.HandleVersion(topic, "", h)
}

func (r *Router) HandleVersion(pattern string, version string, h HandlerFunc) {
	if h == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		r.wildcard = append(r.wildcard, wildcardRoute{
			prefix:  prefix,
			version: version,
			handler: h,
		})
		sort.SliceStable(r.wildcard, func(i, j int) bool {
			if len(r.wildcard[i].prefix) == len(r.wildcard[j].prefix) {
				return r.wildcard[i].version > r.wildcard[j].version
			}
			return len(r.wildcard[i].prefix) > len(r.wildcard[j].prefix)
		})
		return
	}

	if _, ok := r.exact[pattern]; !ok {
		r.exact[pattern] = make(map[string]HandlerFunc)
	}
	r.exact[pattern][version] = h
}

func (r *Router) Match(topic string) HandlerFunc {
	return r.MatchVersion(topic, "")
}

func (r *Router) MatchVersion(topic string, version string) HandlerFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if versions, ok := r.exact[topic]; ok {
		if h, ok := versions[version]; ok {
			return h
		}
		if h, ok := versions[""]; ok {
			return h
		}
	}

	for _, route := range r.wildcard {
		if !strings.HasPrefix(topic, route.prefix) {
			continue
		}
		if route.version == version || route.version == "" {
			return route.handler
		}
	}

	return nil
}
