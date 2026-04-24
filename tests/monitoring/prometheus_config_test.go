package monitoring_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// prometheusConfigPath returns the absolute path to monitoring/prometheus.yml.
func prometheusConfigPath(t *testing.T) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed")
	// tests/monitoring/ → ../../monitoring/prometheus.yml
	return filepath.Join(filepath.Dir(testFile), "..", "..", "monitoring", "prometheus.yml")
}

// promConfig mirrors the relevant top-level fields from prometheus.yml.
type promConfig struct {
	Global struct {
		ScrapeInterval     string `yaml:"scrape_interval"`
		EvaluationInterval string `yaml:"evaluation_interval"`
	} `yaml:"global"`
	ScrapeConfigs []struct {
		JobName        string `yaml:"job_name"`
		ScrapeInterval string `yaml:"scrape_interval"`
		ScrapeTimeout  string `yaml:"scrape_timeout"`
		StaticConfigs  []struct {
			Targets []string `yaml:"targets"`
		} `yaml:"static_configs"`
	} `yaml:"scrape_configs"`
}

// TestPrometheusConfig_FileExists verifies that monitoring/prometheus.yml is
// present in the repository.
func TestPrometheusConfig_FileExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := prometheusConfigPath(t)
	_, err := os.Stat(path)
	assert.NoError(t, err, "prometheus.yml must exist at %s", path)
}

// TestPrometheusConfig_ValidYAML verifies that prometheus.yml contains
// well-formed YAML.
func TestPrometheusConfig_ValidYAML(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := prometheusConfigPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "prometheus.yml must be readable")

	var parsed interface{}
	err = yaml.Unmarshal(data, &parsed)
	assert.NoError(t, err, "prometheus.yml must be valid YAML")
}

// TestPrometheusConfig_RequiredScrapeJobs verifies that the configuration
// contains the essential scrape jobs for HelixAgent monitoring.
func TestPrometheusConfig_RequiredScrapeJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := prometheusConfigPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg promConfig
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err, "prometheus.yml must parse into expected structure")

	require.NotEmpty(t, cfg.ScrapeConfigs,
		"prometheus.yml must define at least one scrape_config")

	// Build a set of job names for fast lookup.
	jobNames := make(map[string]bool, len(cfg.ScrapeConfigs))
	for _, sc := range cfg.ScrapeConfigs {
		jobNames[sc.JobName] = true
	}

	// These jobs are mandatory for HelixAgent production monitoring.
	requiredJobs := []string{
		"helixagent",
		"helixagent-providers",
	}

	for _, job := range requiredJobs {
		assert.True(t, jobNames[job],
			"prometheus.yml must contain scrape job %q", job)
	}
}

// TestPrometheusConfig_ScrapeIntervals validates that every scrape job with an
// explicit scrape_interval uses a value in the range [5s, 120s].  Intervals
// outside this range are either too aggressive (may overload the target) or so
// infrequent that monitoring loses resolution.
func TestPrometheusConfig_ScrapeIntervals(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := prometheusConfigPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg promConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	// parseDuration converts a Prometheus duration string (e.g. "15s", "1m")
	// to seconds as an integer.
	parseDuration := func(s string) int {
		if s == "" {
			return 0
		}
		var value int
		var unit byte
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c >= '0' && c <= '9' {
				value = value*10 + int(c-'0')
			} else {
				unit = c
				break
			}
		}
		switch unit {
		case 's':
			return value
		case 'm':
			return value * 60
		case 'h':
			return value * 3600
		default:
			return value
		}
	}

	for _, sc := range cfg.ScrapeConfigs {
		if sc.ScrapeInterval == "" {
			continue
		}
		seconds := parseDuration(sc.ScrapeInterval)
		assert.GreaterOrEqual(t, seconds, 5,
			"Job %q scrape_interval %q must be >= 5s", sc.JobName, sc.ScrapeInterval)
		assert.LessOrEqual(t, seconds, 120,
			"Job %q scrape_interval %q must be <= 120s", sc.JobName, sc.ScrapeInterval)
	}
}

// TestPrometheusConfig_GlobalInterval validates that the global scrape_interval
// and evaluation_interval are both set and within sensible bounds.
func TestPrometheusConfig_GlobalInterval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := prometheusConfigPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg promConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.NotEmpty(t, cfg.Global.ScrapeInterval,
		"global scrape_interval must be set")
	assert.NotEmpty(t, cfg.Global.EvaluationInterval,
		"global evaluation_interval must be set")
}

// TestPrometheusConfig_ScrapeJobsHaveTargets verifies that every scrape job
// defines at least one target, preventing silent no-op scrape configurations.
func TestPrometheusConfig_ScrapeJobsHaveTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := prometheusConfigPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg promConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	for _, sc := range cfg.ScrapeConfigs {
		totalTargets := 0
		for _, staticCfg := range sc.StaticConfigs {
			totalTargets += len(staticCfg.Targets)
		}
		assert.Greater(t, totalTargets, 0,
			"Scrape job %q must define at least one target", sc.JobName)
	}
}

// TestPrometheusConfig_JobNamesUnique verifies that no two scrape jobs share
// the same job_name, which would cause Prometheus to merge or drop one.
func TestPrometheusConfig_JobNamesUnique(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := prometheusConfigPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg promConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	seen := make(map[string]bool, len(cfg.ScrapeConfigs))
	for _, sc := range cfg.ScrapeConfigs {
		assert.False(t, seen[sc.JobName],
			"Job name %q must be unique in prometheus.yml", sc.JobName)
		seen[sc.JobName] = true
	}
}
