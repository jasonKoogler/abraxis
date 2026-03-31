package gateway

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// RouteMetadata stores information about a service route
type RouteMetadata struct {
	Path           string            `json:"path"`                    // URL path pattern
	Method         string            `json:"method"`                  // HTTP method (GET, POST, etc.)
	ServiceID      string            `json:"service_id"`              // ID of the service that handles this route
	ServiceName    string            `json:"service_name"`            // Name of the service
	ServiceURL     string            `json:"service_url"`             // Base URL of the service
	Public         bool              `json:"public"`                  // true = no auth required, false = auth required
	RequiredScopes []string          `json:"required_scopes,omitempty"` // Required permission scopes (if any)
	Priority       int               `json:"priority"`                // Higher priority routes are matched first
	Tags           map[string]string `json:"tags,omitempty"`          // Additional metadata tags
}

// RoutePattern represents a parsed URL path pattern
type RoutePattern struct {
	Original    string        // Original pattern string
	Segments    []PathSegment // Parsed path segments
	HasWildcard bool          // Whether this pattern contains a wildcard
	NumParams   int           // Number of path parameters
}

// PathSegment represents a segment in a path pattern
type PathSegment struct {
	Type  SegmentType // Type of segment (Static, Param, Wildcard)
	Value string      // Value of the segment (static text or param name)
}

// SegmentType defines the type of path segment
type SegmentType int

const (
	StaticSegment SegmentType = iota
	ParamSegment
	WildcardSegment
)

// ParsePattern parses a URL path pattern into a RoutePattern
func ParsePattern(pattern string) RoutePattern {
	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	result := RoutePattern{
		Original: pattern,
		Segments: make([]PathSegment, 0, len(segments)),
	}

	for _, seg := range segments {
		if seg == "" {
			continue
		}

		if seg == "*" {
			result.Segments = append(result.Segments, PathSegment{
				Type:  WildcardSegment,
				Value: "",
			})
			result.HasWildcard = true
			break // Wildcard must be the last segment
		} else if strings.HasPrefix(seg, ":") {
			paramName := seg[1:]
			result.Segments = append(result.Segments, PathSegment{
				Type:  ParamSegment,
				Value: paramName,
			})
			result.NumParams++
		} else {
			result.Segments = append(result.Segments, PathSegment{
				Type:  StaticSegment,
				Value: seg,
			})
		}
	}

	return result
}

// Match attempts to match a concrete path against this pattern
func (rp RoutePattern) Match(path string) (map[string]string, bool) {
	pathSegments := strings.Split(strings.Trim(path, "/"), "/")
	params := make(map[string]string)

	// Fast path: if no params or wildcards, just compare segment counts
	if rp.NumParams == 0 && !rp.HasWildcard {
		if len(pathSegments) != len(rp.Segments) {
			return nil, false
		}

		for i, seg := range rp.Segments {
			if seg.Value != pathSegments[i] {
				return nil, false
			}
		}
		return params, true
	}

	// With wildcard, path must have at least as many segments as pattern minus wildcard
	if rp.HasWildcard {
		if len(pathSegments) < len(rp.Segments)-1 {
			return nil, false
		}
	} else {
		// Without wildcard, path must have exactly the same number of segments
		if len(pathSegments) != len(rp.Segments) {
			return nil, false
		}
	}

	// Match each segment
	for i, segment := range rp.Segments {
		if segment.Type == WildcardSegment {
			// Collect all remaining path segments
			wildcard := strings.Join(pathSegments[i:], "/")
			params["*"] = wildcard
			return params, true
		}

		// If we've run out of path segments, this is a mismatch
		if i >= len(pathSegments) {
			return nil, false
		}

		switch segment.Type {
		case StaticSegment:
			if segment.Value != pathSegments[i] {
				return nil, false
			}
		case ParamSegment:
			params[segment.Value] = pathSegments[i]
		}
	}

	return params, true
}

// RouteEntry combines metadata with parsed pattern
type RouteEntry struct {
	Metadata RouteMetadata // Route metadata
	Pattern  RoutePattern  // Parsed path pattern
}

// RoutingTable manages service routes with improved pattern matching
type RoutingTable struct {
	routes map[string][]RouteEntry // Method -> Routes
	mu     sync.RWMutex            // Mutex for concurrent access
}

// NewRoutingTable creates a new routing table
func NewRoutingTable() *RoutingTable {
	return &RoutingTable{
		routes: make(map[string][]RouteEntry),
	}
}

// AddRoute adds a route to the routing table
func (rt *RoutingTable) AddRoute(route RouteMetadata) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Validate inputs
	if route.Path == "" {
		return fmt.Errorf("route path cannot be empty")
	}
	if route.Method == "" {
		return fmt.Errorf("route method cannot be empty")
	}
	if route.ServiceID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}

	pattern := ParsePattern(route.Path)
	entry := RouteEntry{
		Metadata: route,
		Pattern:  pattern,
	}

	// Ensure method map exists
	if _, ok := rt.routes[route.Method]; !ok {
		rt.routes[route.Method] = make([]RouteEntry, 0)
	}

	// Add the route
	rt.routes[route.Method] = append(rt.routes[route.Method], entry)

	// Sort routes by priority, specificity, and path length
	rt.sortRoutes(route.Method)

	return nil
}

// sortRoutes sorts routes for a specific method by priority and specificity
func (rt *RoutingTable) sortRoutes(method string) {
	if routes, ok := rt.routes[method]; ok {
		// Use stable sort to maintain order for equal priority routes
		// Sort by:
		// 1. Higher priority first
		// 2. Static routes before parameterized routes before wildcard routes
		// 3. More specific routes (fewer params) before less specific ones
		// 4. Longer paths before shorter paths (for equal specificity)
		sort.SliceStable(routes, func(i, j int) bool {
			ri, rj := routes[i], routes[j]

			// Higher priority first
			if ri.Metadata.Priority != rj.Metadata.Priority {
				return ri.Metadata.Priority > rj.Metadata.Priority
			}

			// Static routes before parameterized routes before wildcard routes
			if ri.Pattern.HasWildcard != rj.Pattern.HasWildcard {
				return !ri.Pattern.HasWildcard
			}

			if ri.Pattern.NumParams != rj.Pattern.NumParams {
				return ri.Pattern.NumParams < rj.Pattern.NumParams
			}

			// Longer paths first
			return len(ri.Pattern.Segments) > len(rj.Pattern.Segments)
		})

		rt.routes[method] = routes
	}
}

// RemoveRoute removes a route by service ID, method and path
func (rt *RoutingTable) RemoveRoute(serviceID, method, path string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if routes, ok := rt.routes[method]; ok {
		newRoutes := make([]RouteEntry, 0, len(routes))
		for _, entry := range routes {
			if !(entry.Metadata.ServiceID == serviceID && entry.Metadata.Path == path) {
				newRoutes = append(newRoutes, entry)
			}
		}
		rt.routes[method] = newRoutes
	}
}

// RemoveServiceRoutes removes all routes for a specific service
func (rt *RoutingTable) RemoveServiceRoutes(serviceID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	for method, routes := range rt.routes {
		newRoutes := make([]RouteEntry, 0, len(routes))
		for _, entry := range routes {
			if entry.Metadata.ServiceID != serviceID {
				newRoutes = append(newRoutes, entry)
			}
		}
		if len(newRoutes) > 0 {
			rt.routes[method] = newRoutes
		} else {
			delete(rt.routes, method)
		}
	}
}

// LookupRoute finds the appropriate route for a request
func (rt *RoutingTable) LookupRoute(method, path string) (RouteMetadata, map[string]string, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	// Check if we have routes for this method
	if entries, ok := rt.routes[method]; ok {
		// Routes are already sorted by priority and specificity
		for _, entry := range entries {
			params, matched := entry.Pattern.Match(path)
			if matched {
				return entry.Metadata, params, true
			}
		}
	}

	// If method-specific route not found, try wildcard method "*" if it exists
	if entries, ok := rt.routes["*"]; ok {
		for _, entry := range entries {
			params, matched := entry.Pattern.Match(path)
			if matched {
				return entry.Metadata, params, true
			}
		}
	}

	return RouteMetadata{}, nil, false
}

// GetRoutes returns all routes
func (rt *RoutingTable) GetRoutes() []RouteMetadata {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var routes []RouteMetadata
	for _, methodRoutes := range rt.routes {
		for _, entry := range methodRoutes {
			routes = append(routes, entry.Metadata)
		}
	}
	return routes
}

// GetPublicRoutes returns all public routes
func (rt *RoutingTable) GetPublicRoutes() []RouteMetadata {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var publicRoutes []RouteMetadata
	for _, methodRoutes := range rt.routes {
		for _, entry := range methodRoutes {
			if entry.Metadata.Public {
				publicRoutes = append(publicRoutes, entry.Metadata)
			}
		}
	}
	return publicRoutes
}

// GetProtectedRoutes returns all protected routes
func (rt *RoutingTable) GetProtectedRoutes() []RouteMetadata {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var protectedRoutes []RouteMetadata
	for _, methodRoutes := range rt.routes {
		for _, entry := range methodRoutes {
			if !entry.Metadata.Public {
				protectedRoutes = append(protectedRoutes, entry.Metadata)
			}
		}
	}
	return protectedRoutes
}

// GetServiceRoutes returns all routes for a specific service
func (rt *RoutingTable) GetServiceRoutes(serviceID string) []RouteMetadata {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var serviceRoutes []RouteMetadata
	for _, methodRoutes := range rt.routes {
		for _, entry := range methodRoutes {
			if entry.Metadata.ServiceID == serviceID {
				serviceRoutes = append(serviceRoutes, entry.Metadata)
			}
		}
	}
	return serviceRoutes
}
