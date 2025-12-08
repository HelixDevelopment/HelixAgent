package plugins

import (
	"fmt"
	"sort"

	"github.com/superagent/superagent/internal/utils"
)

// DependencyResolver handles plugin dependency resolution and conflict detection
type DependencyResolver struct {
	registry *Registry
	deps     map[string][]string // plugin -> dependencies
}

func NewDependencyResolver(registry *Registry) *DependencyResolver {
	return &DependencyResolver{
		registry: registry,
		deps:     make(map[string][]string),
	}
}

func (d *DependencyResolver) AddDependency(pluginName string, dependencies []string) error {
	// Check for circular dependencies
	if d.hasCircularDependency(pluginName, dependencies) {
		return fmt.Errorf("circular dependency detected for plugin %s", pluginName)
	}

	// Check for conflicts
	if err := d.checkConflicts(pluginName, dependencies); err != nil {
		return err
	}

	d.deps[pluginName] = dependencies
	utils.GetLogger().Infof("Added dependencies for plugin %s: %v", pluginName, dependencies)
	return nil
}

func (d *DependencyResolver) ResolveLoadOrder(plugins []string) ([]string, error) {
	// Topological sort for dependency resolution
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	var order []string

	var visit func(string) error
	visit = func(plugin string) error {
		if recStack[plugin] {
			return fmt.Errorf("circular dependency detected")
		}
		if visited[plugin] {
			return nil
		}

		visited[plugin] = true
		recStack[plugin] = true

		// Visit dependencies first
		for _, dep := range d.deps[plugin] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		recStack[plugin] = false
		order = append(order, plugin)
		return nil
	}

	for _, plugin := range plugins {
		if !visited[plugin] {
			if err := visit(plugin); err != nil {
				return nil, err
			}
		}
	}

	// Reverse the order for loading (dependencies first)
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}

	return order, nil
}

func (d *DependencyResolver) hasCircularDependency(plugin string, deps []string) bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(p string) bool {
		visited[p] = true
		recStack[p] = true

		for _, dep := range d.deps[p] {
			if !visited[dep] && hasCycle(dep) {
				return true
			} else if recStack[dep] {
				return true
			}
		}

		recStack[p] = false
		return false
	}

	// Check new dependencies
	for _, dep := range deps {
		if hasCycle(dep) {
			return true
		}
	}

	return false
}

func (d *DependencyResolver) checkConflicts(plugin string, deps []string) error {
	// Check if any dependency conflicts with existing plugins
	for _, dep := range deps {
		if existing, exists := d.registry.Get(dep); exists {
			// Check version compatibility (simplified)
			if existing.Version() != "1.0.0" { // TODO: Implement proper version checking
				return fmt.Errorf("version conflict for dependency %s", dep)
			}
		}
	}

	// Check for capability conflicts
	pluginCaps := d.getPluginCapabilities(plugin)
	for _, dep := range deps {
		depCaps := d.getPluginCapabilities(dep)
		if d.hasCapabilityConflict(pluginCaps, depCaps) {
			return fmt.Errorf("capability conflict between %s and %s", plugin, dep)
		}
	}

	return nil
}

func (d *DependencyResolver) getPluginCapabilities(name string) *map[string]interface{} {
	if plugin, exists := d.registry.Get(name); exists {
		caps := plugin.Capabilities()
		result := make(map[string]interface{})
		result["streaming"] = caps.SupportsStreaming
		result["function_calling"] = caps.SupportsFunctionCalling
		result["vision"] = caps.SupportsVision
		return &result
	}
	return nil
}

func (d *DependencyResolver) hasCapabilityConflict(caps1, caps2 *map[string]interface{}) bool {
	if caps1 == nil || caps2 == nil {
		return false
	}

	// Simple conflict detection - plugins with conflicting capabilities
	for cap, val1 := range *caps1 {
		if val2, exists := (*caps2)[cap]; exists && val1 != val2 {
			return true
		}
	}

	return false
}

func (d *DependencyResolver) GetDependencies(plugin string) []string {
	if deps, exists := d.deps[plugin]; exists {
		return deps
	}
	return []string{}
}

func (d *DependencyResolver) GetDependents(plugin string) []string {
	var dependents []string
	for p, deps := range d.deps {
		for _, dep := range deps {
			if dep == plugin {
				dependents = append(dependents, p)
				break
			}
		}
	}
	sort.Strings(dependents)
	return dependents
}
