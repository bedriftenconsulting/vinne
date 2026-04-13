package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/randco/randco-microservices/shared/common/logger"
)

// Router handles HTTP routing using standard library
type Router struct {
	routes      map[string]map[string]HandlerFunc // method -> path -> handler
	middlewares []Middleware
	notFound    http.HandlerFunc
	logger      logger.Logger
	mu          sync.RWMutex
}

// HandlerFunc is our custom handler that can return an error
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Middleware is a function that wraps a handler
type Middleware func(HandlerFunc) HandlerFunc

// Context keys
type contextKey string

const (
	ContextUserID   contextKey = "user_id"
	ContextEmail    contextKey = "email"
	ContextUsername contextKey = "username"
	ContextRoles    contextKey = "roles"
	ContextParams   contextKey = "params"
)

// NewRouter creates a new router
func NewRouter(logger logger.Logger) *Router {
	return &Router{
		routes: make(map[string]map[string]HandlerFunc),
		logger: logger,
		notFound: func(w http.ResponseWriter, r *http.Request) {
			_ = writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "Route not found",
			})
		},
	}
}

// Use adds middleware to the router
func (r *Router) Use(middleware Middleware) {
	r.middlewares = append(r.middlewares, middleware)
}

// GET registers a GET route
func (r *Router) GET(path string, handler HandlerFunc) {
	r.addRoute("GET", path, handler)
}

// POST registers a POST route
func (r *Router) POST(path string, handler HandlerFunc) {
	r.addRoute("POST", path, handler)
}

// PUT registers a PUT route
func (r *Router) PUT(path string, handler HandlerFunc) {
	r.addRoute("PUT", path, handler)
}

// DELETE registers a DELETE route
func (r *Router) DELETE(path string, handler HandlerFunc) {
	r.addRoute("DELETE", path, handler)
}

// PATCH registers a PATCH route
func (r *Router) PATCH(path string, handler HandlerFunc) {
	r.addRoute("PATCH", path, handler)
}

// OPTIONS registers an OPTIONS route
func (r *Router) OPTIONS(path string, handler HandlerFunc) {
	r.addRoute("OPTIONS", path, handler)
}

// addRoute adds a route to the router
func (r *Router) addRoute(method, path string, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.routes[method] == nil {
		r.routes[method] = make(map[string]HandlerFunc)
	}

	// Store the raw handler - middlewares will be applied at request time
	r.routes[method][path] = handler
	r.logger.Info(fmt.Sprintf("Route registered: %s %s", method, path))
}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	var handler HandlerFunc
	var found bool

	// Find handler for method
	if methods, ok := r.routes[req.Method]; ok {
		// First try exact match
		if h, ok := methods[req.URL.Path]; ok {
			handler = h
			found = true
		} else {
			// Then try pattern matching for routes with parameters
			for pattern, h := range methods {
				if params, ok := matchPath(pattern, req.URL.Path); ok {
					// Add params to context
					ctx := context.WithValue(req.Context(), ContextParams, params)
					req = req.WithContext(ctx)
					handler = h
					found = true
					break
				}
			}
		}
	}

	// Make a copy of middlewares to avoid holding lock during execution
	middlewares := make([]Middleware, len(r.middlewares))
	copy(middlewares, r.middlewares)
	r.mu.RUnlock()

	// Determine the handler to use
	var baseHandler HandlerFunc
	if found {
		baseHandler = handler
	} else {
		// No route found - wrap notFound in a HandlerFunc
		baseHandler = func(w http.ResponseWriter, req *http.Request) error {
			r.notFound(w, req)
			return nil
		}
	}

	// Apply middlewares to the handler (including for 404s)
	finalHandler := baseHandler
	for i := len(middlewares) - 1; i >= 0; i-- {
		finalHandler = middlewares[i](finalHandler)
	}
	r.executeHandler(w, req, finalHandler)
}

// executeHandler executes a handler and handles errors
func (r *Router) executeHandler(w http.ResponseWriter, req *http.Request, handler HandlerFunc) {
	if err := handler(w, req); err != nil {
		r.logger.Error("Handler error", "path", req.URL.Path, "error", err)
		_ = writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Internal server error",
		})
	}
}

// matchPath matches a path pattern with parameters
func matchPath(pattern, path string) (map[string]string, bool) {
	// Handle empty paths
	if pattern == "" || path == "" {
		return nil, false
	}

	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	params := make(map[string]string)
	for i, part := range patternParts {
		// Bounds check to prevent panic
		if i >= len(pathParts) {
			return nil, false
		}

		if len(part) > 2 && strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// This is a parameter
			paramName := part[1 : len(part)-1]
			if paramName != "" {
				params[paramName] = pathParts[i]
			}
		} else if part != pathParts[i] {
			// Not a match
			return nil, false
		}
	}

	return params, true
}

// GetParam gets a URL parameter from the request context
func GetParam(r *http.Request, name string) string {
	params, ok := r.Context().Value(ContextParams).(map[string]string)
	if !ok {
		return ""
	}
	return params[name]
}

// GetUserID gets user ID from context
func GetUserID(r *http.Request) string {
	userID, ok := r.Context().Value(ContextUserID).(string)
	if !ok {
		return ""
	}
	return userID
}

// GetEmail gets user email from context
func GetEmail(r *http.Request) string {
	email, ok := r.Context().Value(ContextEmail).(string)
	if !ok {
		return ""
	}
	return email
}

// GetRoles gets user roles from context
func GetRoles(r *http.Request) []string {
	roles, ok := r.Context().Value(ContextRoles).([]string)
	if !ok {
		return nil
	}
	return roles
}

// HasRole checks if user has a specific role
func HasRole(r *http.Request, role string) bool {
	roles := GetRoles(r)
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// Helper functions

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

// readJSON reads JSON from request body
func readJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// WriteJSON exports writeJSON for use in handlers
func WriteJSON(w http.ResponseWriter, status int, data interface{}) error {
	return writeJSON(w, status, data)
}

// ReadJSON exports readJSON for use in handlers
func ReadJSON(r *http.Request, v interface{}) error {
	return readJSON(r, v)
}

// ErrorResponse writes an error response
func ErrorResponse(w http.ResponseWriter, status int, message string) error {
	return writeJSON(w, status, map[string]string{
		"error": message,
	})
}

// SuccessResponse writes a success response
func SuccessResponse(w http.ResponseWriter, data interface{}) error {
	return writeJSON(w, http.StatusOK, data)
}

// Group creates a route group with a prefix
type Group struct {
	router      *Router
	prefix      string
	middlewares []Middleware
}

// Group creates a new route group
func (r *Router) Group(prefix string) *Group {
	return &Group{
		router: r,
		prefix: prefix,
	}
}

// Use adds middleware to the group
func (g *Group) Use(middleware Middleware) {
	g.middlewares = append(g.middlewares, middleware)
}

// GET registers a GET route in the group
func (g *Group) GET(path string, handler HandlerFunc) {
	g.addRoute("GET", path, handler)
}

// POST registers a POST route in the group
func (g *Group) POST(path string, handler HandlerFunc) {
	g.addRoute("POST", path, handler)
}

// PUT registers a PUT route in the group
func (g *Group) PUT(path string, handler HandlerFunc) {
	g.addRoute("PUT", path, handler)
}

// DELETE registers a DELETE route in the group
func (g *Group) DELETE(path string, handler HandlerFunc) {
	g.addRoute("DELETE", path, handler)
}

// PATCH registers a PATCH route in the group
func (g *Group) PATCH(path string, handler HandlerFunc) {
	g.addRoute("PATCH", path, handler)
}

// addRoute adds a route to the group
func (g *Group) addRoute(method, path string, handler HandlerFunc) {
	// Store handler with group middlewares to be applied at request time
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) error {
		// Apply group middlewares
		finalHandler := handler
		for i := len(g.middlewares) - 1; i >= 0; i-- {
			finalHandler = g.middlewares[i](finalHandler)
		}
		return finalHandler(w, r)
	}

	g.router.addRoute(method, g.prefix+path, wrappedHandler)
}
