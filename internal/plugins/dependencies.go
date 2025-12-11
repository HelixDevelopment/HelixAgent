package plugins

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

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
			// Parse version constraints from dependency specification
			// Expected format: "plugin-name@>=1.0.0 <2.0.0"
			depParts := strings.Split(dep, "@")
			if len(depParts) == 2 {
				depName := depParts[0]
				versionConstraint := depParts[1]

				// Get actual version of loaded plugin
				actualVersion := existing.Version()

				// Check version compatibility
				if !d.checkVersionCompatibility(actualVersion, versionConstraint) {
					return fmt.Errorf("version conflict for dependency %s: version %s does not satisfy constraint %s",
						depName, actualVersion, versionConstraint)
				}
			} else {
				// Simple name-based dependency - just check if plugin exists
				// No version constraint specified
				utils.GetLogger().Debugf("No version constraint specified for dependency: %s", dep)
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

// checkVersionCompatibility checks if a version satisfies a constraint
func (d *DependencyResolver) checkVersionCompatibility(version, constraint string) bool {
	// Simple semantic version checking for now
	// Supports: >=1.0.0, <=2.0.0, ~1.2.3, ^1.0.0, 1.0.0, 1.0.x

	// Parse version
	versionParts := strings.Split(version, ".")
	if len(versionParts) < 3 {
		return false
	}

	// Parse constraint
	constraint = strings.TrimSpace(constraint)

	// Check for range constraints
	if strings.HasPrefix(constraint, ">=") {
		minVersion := strings.TrimPrefix(constraint, ">=")
		return d.compareVersions(version, minVersion) >= 0
	} else if strings.HasPrefix(constraint, "<=") {
		maxVersion := strings.TrimPrefix(constraint, "<=")
		return d.compareVersions(version, maxVersion) <= 0
	} else if strings.HasPrefix(constraint, ">") {
		minVersion := strings.TrimPrefix(constraint, ">")
		return d.compareVersions(version, minVersion) > 0
	} else if strings.HasPrefix(constraint, "<") {
		maxVersion := strings.TrimPrefix(constraint, "<")
		return d.compareVersions(version, maxVersion) < 0
	} else if strings.HasPrefix(constraint, "~") {
		// Tilde range: ~1.2.3 means >=1.2.3 <1.3.0
		baseVersion := strings.TrimPrefix(constraint, "~")
		baseParts := strings.Split(baseVersion, ".")
		if len(baseParts) < 3 {
			return false
		}
		nextMinor := fmt.Sprintf("%s.%d.0", baseParts[0], parseInt(baseParts[1])+1)
		return d.compareVersions(version, baseVersion) >= 0 && d.compareVersions(version, nextMinor) < 0
	} else if strings.HasPrefix(constraint, "^") {
		// Caret range: ^1.2.3 means >=1.2.3 <2.0.0
		baseVersion := strings.TrimPrefix(constraint, "^")
		baseParts := strings.Split(baseVersion, ".")
		if len(baseParts) < 3 {
			return false
		}
		nextMajor := fmt.Sprintf("%d.0.0", parseInt(baseParts[0])+1)
		return d.compareVersions(version, baseVersion) >= 0 && d.compareVersions(version, nextMajor) < 0
	} else if strings.Contains(constraint, "x") || strings.Contains(constraint, "*") {
		// Wildcard: 1.2.x or 1.*
		pattern := strings.NewReplacer(".x", ".*", ".*", "\\..*", "*", ".*").Replace(constraint)
		pattern = "^" + regexp.QuoteMeta(pattern) + "$"
		pattern = strings.ReplaceAll(pattern, "\\*\\.\\*", ".*")
		matched, _ := regexp.MatchString(pattern, version)
		return matched
	} else {
		// Exact version match
		return version == constraint
	}
}

// compareVersions compares two semantic versions
func (d *DependencyResolver) compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int
		if i < len(parts1) {
			num1 = parseInt(parts1[i])
		}
		if i < len(parts2) {
			num2 = parseInt(parts2[i])
		}

		if num1 < num2 {
			return -1
		} else if num1 > num2 {
			return 1
		}
	}

	return 0
}

// parseInt safely parses an integer from a string
func parseInt(s string) int {
	var result int
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			result = result*10 + int(ch-'0')
		} else {
			break
		}
	}
	return result
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
