package plugins

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/superagent/superagent/internal/utils"
)

// Version represents a semantic version
type Version struct {
	Major int
	Minor int
	Patch int
}

func ParseVersion(v string) (*Version, error) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid version format: %s", v)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	return &Version{Major: major, Minor: minor, Patch: patch}, nil
}

func (v *Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major > other.Major {
			return 1
		}
		return -1
	}
	if v.Minor != other.Minor {
		if v.Minor > other.Minor {
			return 1
		}
		return -1
	}
	if v.Patch != other.Patch {
		if v.Patch > other.Patch {
			return 1
		}
		return -1
	}
	return 0
}

func (v *Version) Compatible(other *Version) bool {
	// Major version must match for compatibility
	return v.Major == other.Major
}

// VersionManager handles plugin versioning and updates
type VersionManager struct {
	registry *Registry
	versions map[string]*Version
}

func NewVersionManager(registry *Registry) *VersionManager {
	return &VersionManager{
		registry: registry,
		versions: make(map[string]*Version),
	}
}

func (vm *VersionManager) RegisterVersion(pluginName, versionStr string) error {
	version, err := ParseVersion(versionStr)
	if err != nil {
		return fmt.Errorf("invalid version for plugin %s: %w", pluginName, err)
	}

	vm.versions[pluginName] = version
	utils.GetLogger().Infof("Registered version %s for plugin %s", versionStr, pluginName)
	return nil
}

func (vm *VersionManager) GetVersion(pluginName string) (*Version, bool) {
	version, exists := vm.versions[pluginName]
	return version, exists
}

func (vm *VersionManager) CheckCompatibility(pluginName string, requiredVersion string) error {
	current, exists := vm.GetVersion(pluginName)
	if !exists {
		return fmt.Errorf("no version registered for plugin %s", pluginName)
	}

	required, err := ParseVersion(requiredVersion)
	if err != nil {
		return fmt.Errorf("invalid required version: %w", err)
	}

	if !current.Compatible(required) {
		return fmt.Errorf("plugin %s version %s is not compatible with required %s",
			pluginName, current.String(), required.String())
	}

	return nil
}

func (vm *VersionManager) IsUpdateAvailable(pluginName, newVersion string) (bool, error) {
	current, exists := vm.GetVersion(pluginName)
	if !exists {
		return true, nil // No current version, so update is available
	}

	newVer, err := ParseVersion(newVersion)
	if err != nil {
		return false, err
	}

	return current.Compare(newVer) < 0, nil
}

func (vm *VersionManager) UpdateVersion(pluginName, newVersion string) error {
	version, err := ParseVersion(newVersion)
	if err != nil {
		return err
	}

	vm.versions[pluginName] = version
	utils.GetLogger().Infof("Updated plugin %s to version %s", pluginName, newVersion)
	return nil
}

func (vm *VersionManager) GetAllVersions() map[string]string {
	result := make(map[string]string)
	for name, version := range vm.versions {
		result[name] = version.String()
	}
	return result
}

func (vm *VersionManager) ValidateVersionConstraints(constraints map[string]string) error {
	for plugin, constraint := range constraints {
		if err := vm.CheckCompatibility(plugin, constraint); err != nil {
			return err
		}
	}
	return nil
}
