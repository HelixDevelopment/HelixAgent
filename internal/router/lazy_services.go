// Package router provides HTTP routing for HelixAgent.
// This file implements lazy service initialization using sync.Once,
// allowing router handler services to be initialized on first access
// rather than at router setup time.
package router

import (
	"sync"

	"digital.vasic.concurrency/pkg/safe"
)

// LazyService provides thread-safe, on-demand initialization of a service.
// The factory function is called at most once, on the first call to Get().
// Subsequent calls return the cached result without invoking the factory.
type LazyService struct {
	once    sync.Once
	service interface{}
	initErr error
	factory func() (interface{}, error)
}

// NewLazyService creates a LazyService with the given factory function.
// The factory is not called until Get() is invoked.
func NewLazyService(factory func() (interface{}, error)) *LazyService {
	return &LazyService{factory: factory}
}

// Get returns the lazily initialized service. On the first call, the factory
// is invoked; the result (including any error) is cached for all subsequent
// calls. This method is safe for concurrent use.
func (ls *LazyService) Get() (interface{}, error) {
	ls.once.Do(func() {
		ls.service, ls.initErr = ls.factory()
	})
	return ls.service, ls.initErr
}

// IsInitialized reports whether the service has been initialized.
// It does not trigger initialization.
func (ls *LazyService) IsInitialized() bool {
	// A sync.Once that has been executed cannot be introspected directly,
	// but we can check whether the factory has produced a value.
	// After Do completes, service or initErr will be set.
	// Before Do runs, both remain zero-valued.
	// We use a separate atomic-free check: if factory is nil after Do,
	// that would be ambiguous, so instead we track via the service/err.
	//
	// The simplest correct approach: try to see if once has fired by
	// checking if service is non-nil or initErr is non-nil.
	return ls.service != nil || ls.initErr != nil
}

// LazyServiceRegistry holds a collection of named LazyService instances.
// It provides a central place to register and retrieve lazily initialized
// services used by router handlers.
type LazyServiceRegistry struct {
	services *safe.Store[string, *LazyService]
}

// NewLazyServiceRegistry creates a new empty registry.
func NewLazyServiceRegistry() *LazyServiceRegistry {
	return &LazyServiceRegistry{
		services: safe.NewStore[string, *LazyService](),
	}
}

// Register adds a named lazy service to the registry.
func (r *LazyServiceRegistry) Register(name string, svc *LazyService) {
	r.services.Put(name, svc)
}

// Get retrieves a lazy service by name and triggers initialization.
// Returns nil, false if the name is not registered.
func (r *LazyServiceRegistry) Get(name string) (interface{}, bool) {
	ls, ok := r.services.Get(name)
	if !ok {
		return nil, false
	}
	svc, err := ls.Get()
	if err != nil {
		return nil, false
	}
	return svc, true
}

// GetLazy retrieves the LazyService wrapper without triggering initialization.
func (r *LazyServiceRegistry) GetLazy(name string) (*LazyService, bool) {
	return r.services.Get(name)
}

// Names returns all registered service names.
func (r *LazyServiceRegistry) Names() []string {
	return r.services.Keys()
}

// Count returns the number of registered services.
func (r *LazyServiceRegistry) Count() int {
	return r.services.Len()
}
