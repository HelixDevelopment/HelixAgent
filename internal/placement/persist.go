package placement

import (
	"encoding/json"
	"fmt"
	"os"
)

// WritePlanJSON persists a Plan to disk as JSON. Used so the gateway
// service-registry, the Challenge, and operators all see the same
// authoritative record of which service ended up on which host this
// boot. Path is overwritten atomically (write-then-rename).
func WritePlanJSON(path string, plan *Plan) error {
	if plan == nil {
		return fmt.Errorf("nil plan")
	}
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

// ReadPlanJSON returns the Plan persisted at path. Useful for the
// gateway's startup phase and post-boot Challenges that need to know
// where each service was placed.
func ReadPlanJSON(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &plan, nil
}
