package placement

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EmitPerHostCompose reads `composeFile`, retains only the services
// listed in `keepServices`, and writes the filtered project to
// `outFile`. Networks, volumes, and top-level keys are preserved.
//
// Used by the boot orchestrator after scheduling: each host receives a
// compose file that contains ONLY the services it was scheduled to
// run, so `compose up -d` on that host won't try to start anything
// else. Critically, intra-compose `depends_on` references stay valid
// because services in the same co-location group are kept together.
//
// Returns the absolute path of the written file.
func EmitPerHostCompose(composeFile string, keepServices []string, outFile string) (string, error) {
	src, err := os.ReadFile(composeFile)
	if err != nil {
		return "", fmt.Errorf("read source compose: %w", err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return "", fmt.Errorf("parse source compose: %w", err)
	}

	keep := make(map[string]bool, len(keepServices))
	for _, s := range keepServices {
		keep[s] = true
	}

	servicesRaw, ok := doc["services"]
	if !ok {
		return "", fmt.Errorf("source compose has no `services` block")
	}

	switch services := servicesRaw.(type) {
	case map[string]interface{}:
		for name := range services {
			if !keep[name] {
				delete(services, name)
			}
		}
	case map[interface{}]interface{}:
		// PyYAML / older yaml.v3 returns this shape for some inputs.
		for name := range services {
			s, ok := name.(string)
			if !ok {
				continue
			}
			if !keep[s] {
				delete(services, name)
			}
		}
	default:
		return "", fmt.Errorf("unexpected services type %T", servicesRaw)
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal filtered compose: %w", err)
	}

	abs, err := filepath.Abs(outFile)
	if err != nil {
		abs = outFile
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("mkdir for output: %w", err)
	}
	if err := os.WriteFile(abs, out, 0o644); err != nil {
		return "", fmt.Errorf("write filtered compose: %w", err)
	}
	return abs, nil
}
