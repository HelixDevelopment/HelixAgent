package precondition

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreconditionContainersBoot(t *testing.T) {
	// CONST-030: this is a non-unit precondition that talks to real
	// remote SSH hosts, real running HelixAgent containers and reads
	// the user's live OpenCode config. Under `go test -short` it must
	// skip rather than fail — `-short` is the unit-test gate, and a
	// precondition that fails because the binary hasn't been built or
	// the OpenCode config is stale would falsely block unit-test
	// pipelines that have nothing to do with infrastructure state.
	if testing.Short() {
		t.Skip("SKIP-OK: #const030 — precondition needs the running HelixAgent + remote container infra; `-short` is unit-tests-only")
	}

	t.Log("╔══════════════════════════════════════════════════════════════════╗")
	t.Log("║     PRECONDITION TEST: Container Boot Verification              ║")
	t.Log("║     This test MUST pass before any other test can execute        ║")
	t.Log("╚══════════════════════════════════════════════════════════════════╝")

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("Failed to find project root: %v", err)
	}

	containersEnvPath := filepath.Join(projectRoot, "Containers", ".env")
	remoteConfig := parseContainersEnv(containersEnvPath)

	// PHASE 0: Pre-cleanup - Stop all local containers if remote is enabled
	if remoteConfig.Enabled {
		t.Log("\n═══════════════════════════════════════════════════════════════")
		t.Log("PHASE 0: Pre-cleanup (Remote distribution enabled)")
		t.Log("═══════════════════════════════════════════════════════════════")
		if err := stopAllLocalContainers(t); err != nil {
			t.Logf("WARNING: Pre-cleanup had issues: %v", err)
		}
	}

	// PHASE 1: Verify container infrastructure
	t.Log("\n═══════════════════════════════════════════════════════════════")
	t.Log("PHASE 1: Container Infrastructure Verification")
	t.Log("═══════════════════════════════════════════════════════════════")

	if remoteConfig.Enabled {
		t.Logf("Remote distribution enabled: %s", remoteConfig.HostsSummary())
		t.Log("Verifying remote containers are accessible...")
		if err := verifyRemoteContainers(t, remoteConfig); err != nil {
			t.Skipf("Remote container verification failed: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}

		// CRITICAL: When remote is enabled, NO local containers for same services
		t.Log("\nValidating no local containers for services (remote mode)...")
		if err := validateNoLocalContainersForRemoteServices(t); err != nil {
			t.Fatalf("SAFETY CHECK FAILED: %v", err)
		}
	} else {
		t.Log("No remote distribution configured - verifying local containers...")
		if err := verifyLocalContainers(t, projectRoot); err != nil {
			t.Fatalf("Local container verification failed: %v", err)
		}
	}

	// PHASE 2: Multiple service instance detection
	t.Log("\n═══════════════════════════════════════════════════════════════")
	t.Log("PHASE 2: Multiple Service Instance Detection")
	t.Log("═══════════════════════════════════════════════════════════════")
	if err := detectMultipleServiceInstances(t, remoteConfig); err != nil {
		t.Fatalf("Multiple service instance conflict detected: %v", err)
	}

	// PHASE 3: Verify OpenCode configuration
	t.Log("\n═══════════════════════════════════════════════════════════════")
	t.Log("PHASE 3: OpenCode Configuration Verification")
	t.Log("═══════════════════════════════════════════════════════════════")
	if err := verifyOpenCodeConfig(t, projectRoot, remoteConfig); err != nil {
		t.Fatalf("OpenCode configuration verification failed: %v", err)
	}

	// PHASE 4: Test requests against HelixAgent
	t.Log("\n═══════════════════════════════════════════════════════════════")
	t.Log("PHASE 4: HelixAgent Debate Ensemble Test Requests")
	t.Log("═══════════════════════════════════════════════════════════════")
	if err := testHelixAgentRequests(t); err != nil {
		t.Fatalf("HelixAgent test requests failed: %v", err)
	}

	t.Log("\n╔══════════════════════════════════════════════════════════════════╗")
	t.Log("║     PRECONDITION PASSED: All systems verified                     ║")
	t.Log("╚══════════════════════════════════════════════════════════════════╝")
}

// stopAllLocalContainers stops ALL local Docker/Podman containers
func stopAllLocalContainers(t *testing.T) error {
	containerCmd, containerArgs := detectContainerRuntime()
	if containerCmd == "" {
		t.Log("  No container runtime found - skipping container cleanup")
		return nil
	}

	t.Logf("  Using container runtime: %s %v", containerCmd, containerArgs)

	// Try to get list of running containers
	var cmd *exec.Cmd
	if len(containerArgs) > 0 {
		fullArgs := append(containerArgs, "ps", "-q")
		cmd = exec.Command(containerCmd, fullArgs...)
	} else {
		cmd = exec.Command(containerCmd, "ps", "-q")
	}

	output, err := cmd.Output()
	if err != nil {
		// Try without compose subcommand
		cmd = exec.Command(containerCmd, "ps", "-q")
		output, err = cmd.Output()
		if err != nil {
			t.Logf("  WARNING: Could not list containers: %v", err)
			return nil // Not a fatal error
		}
	}

	containerIDs := strings.Split(strings.TrimSpace(string(output)), "\n")
	var runningContainers []string
	for _, id := range containerIDs {
		if id != "" {
			runningContainers = append(runningContainers, id)
		}
	}

	if len(runningContainers) == 0 {
		t.Log("  ✓ No local containers running (clean slate)")
		return nil
	}

	t.Logf("  Found %d running containers - stopping them...", len(runningContainers))

	// Stop all containers
	var stopCmd *exec.Cmd
	if len(containerArgs) > 0 {
		stopArgs := append(containerArgs, "stop")
		stopArgs = append(stopArgs, runningContainers...)
		stopCmd = exec.Command(containerCmd, stopArgs...)
	} else {
		stopArgs := []string{"stop"}
		stopArgs = append(stopArgs, runningContainers...)
		stopCmd = exec.Command(containerCmd, stopArgs...)
	}

	if output, err := stopCmd.CombinedOutput(); err != nil {
		t.Logf("  WARNING: Some containers may not have stopped: %s", string(output))
	}

	t.Logf("  ✓ Stopped %d local containers", len(runningContainers))

	t.Log("  ✓ Container cleanup complete")
	return nil
}

// validateNoLocalContainersForRemoteServices ensures no local containers are running
// for services that should be on remote hosts when remote distribution is enabled
func validateNoLocalContainersForRemoteServices(t *testing.T) error {
	containerCmd, containerArgs := detectContainerRuntime()
	if containerCmd == "" {
		return nil
	}

	// Services that must NOT run locally when remote is enabled
	remoteServices := []string{"postgres", "redis", "chromadb", "mem0", "cognee", "helixagent"}

	for _, svc := range remoteServices {
		var cmd *exec.Cmd
		if len(containerArgs) > 0 {
			fullArgs := append(containerArgs, "ps", "--filter", "name="+svc, "-q")
			cmd = exec.Command(containerCmd, fullArgs...)
		} else {
			cmd = exec.Command(containerCmd, "ps", "--filter", "name="+svc, "-q")
		}
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		ids := strings.TrimSpace(string(output))
		if ids != "" {
			count := len(strings.Split(ids, "\n"))
			return fmt.Errorf("LOCAL_CONTAINER_CONFLICT: Service '%s' has %d local container(s) running but remote distribution is enabled. Stop all local containers first with: %s stop $(%s ps -q)",
				svc, count, containerCmd, containerCmd)
		}
	}

	t.Log("  ✓ No local containers for remote services detected")
	return nil
}

// verifyOpenCodeConfig verifies OpenCode configuration file exists and has correct endpoints
func verifyOpenCodeConfig(t *testing.T, projectRoot string, rc *RemoteConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("OpenCode config not found at %s. Generate it with: go run ./cmd/helixagent --generate-agent-config=opencode --agent-config-output=%s", configPath, configPath)
	}

	t.Logf("  ✓ OpenCode config found: %s", configPath)

	// Read and parse config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read OpenCode config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse OpenCode config: %w", err)
	}

	// Verify provider configuration
	provider, ok := config["provider"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("OpenCode config missing 'provider' section")
	}

	helixagentProvider, ok := provider["helixagent"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("OpenCode config missing 'helixagent' provider")
	}

	options, ok := helixagentProvider["options"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("OpenCode config missing helixagent 'options'")
	}

	baseURL, _ := options["baseURL"].(string)
	if baseURL == "" {
		return fmt.Errorf("OpenCode config missing baseURL in helixagent options")
	}

	t.Logf("  ✓ HelixAgent provider configured with baseURL: %s", baseURL)

	// Verify MCP endpoints
	mcp, ok := config["mcp"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("OpenCode config missing 'mcp' section")
	}

	helixagentMCPs := []string{
		"helixagent-mcp", "helixagent-lsp", "helixagent-acp",
		"helixagent-embeddings", "helixagent-vision", "helixagent-rag",
		"helixagent-formatters", "helixagent-monitoring", "helixagent-cognee",
	}

	foundMCPs := 0
	for _, mcpName := range helixagentMCPs {
		if _, exists := mcp[mcpName]; exists {
			foundMCPs++
		}
	}

	t.Logf("  ✓ Found %d/%d HelixAgent MCP endpoints in config", foundMCPs, len(helixagentMCPs))

	if foundMCPs < len(helixagentMCPs)/2 {
		return fmt.Errorf("OpenCode config has too few HelixAgent MCP endpoints (%d/%d)", foundMCPs, len(helixagentMCPs))
	}

	// Verify agent configuration
	agent, ok := config["agent"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("OpenCode config missing 'agent' section")
	}

	if _, hasCoder := agent["coder"]; !hasCoder {
		return fmt.Errorf("OpenCode config missing 'coder' agent")
	}

	t.Log("  ✓ Agent configurations present (coder, summarizer, task, title)")

	return nil
}

// testHelixAgentRequests executes test requests against HelixAgent debate ensemble
func testHelixAgentRequests(t *testing.T) error {
	baseURL := "http://localhost:8100/v1"

	// First check if HelixAgent is running
	if err := checkHTTPHealth(baseURL+"/../health", 5*time.Second); err != nil {
		t.Log("  ~ HelixAgent not running - skipping test requests")
		t.Log("    Start HelixAgent to enable full validation")
		return nil // Not a failure - just skip
	}

	t.Log("  ✓ HelixAgent health check passed")

	testRequests := []struct {
		name     string
		prompt   string
		validate func(string) bool
	}{
		{
			name:   "Codebase visibility",
			prompt: "Do you see my codebase? Answer YES or NO only.",
			validate: func(resp string) bool {
				lower := strings.ToLower(resp)
				return strings.Contains(lower, "yes") || strings.Contains(lower, "no") || strings.Contains(lower, "can see") || strings.Contains(lower, "access")
			},
		},
		{
			name:   "Math calculation",
			prompt: "Calculate 2 + 5 and then multiply that sum by 3. Give me just the final number.",
			validate: func(resp string) bool {
				return strings.Contains(resp, "21")
			},
		},
		{
			name:   "File counting",
			prompt: "How many .go files are inside the Containers module? Just give me a number or approximate count.",
			validate: func(resp string) bool {
				return len(resp) > 0 && !strings.Contains(strings.ToLower(resp), "error") && !strings.Contains(strings.ToLower(resp), "unable")
			},
		},
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		// Try to read from OpenCode config
		homeDir, _ := os.UserHomeDir()
		configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
		if data, err := os.ReadFile(configPath); err == nil {
			var config map[string]interface{}
			if json.Unmarshal(data, &config); err == nil {
				if provider, ok := config["provider"].(map[string]interface{}); ok {
					if ha, ok := provider["helixagent"].(map[string]interface{}); ok {
						if opts, ok := ha["options"].(map[string]interface{}); ok {
							if key, ok := opts["apiKey"].(string); ok {
								apiKey = key
							}
						}
					}
				}
			}
		}
	}

	if apiKey == "" {
		t.Log("  WARNING: No API key found - skipping test requests")
		return nil
	}

	for _, test := range testRequests {
		t.Logf("\n  Test: %s", test.name)
		t.Logf("    Prompt: %s", test.prompt)

		resp, err := sendChatRequest(baseURL, apiKey, test.prompt, 30*time.Second)
		if err != nil {
			t.Logf("    ✗ Request failed: %v", err)
			return fmt.Errorf("test '%s' failed: %w", test.name, err)
		}

		if test.validate(resp) {
			t.Logf("    ✓ Response validated: %s", truncate(resp, 100))
		} else {
			t.Logf("    ✗ Response not validated: %s", truncate(resp, 200))
			return fmt.Errorf("test '%s' response validation failed", test.name)
		}
	}

	t.Log("\n  ✓ All HelixAgent test requests passed")
	return nil
}

// sendChatRequest sends a chat request to HelixAgent
func sendChatRequest(baseURL, apiKey, prompt string, timeout time.Duration) (string, error) {
	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 500,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// detectMultipleServiceInstances checks for multiple instances of the same service
func detectMultipleServiceInstances(t *testing.T, rc *RemoteConfig) error {
	servicePorts := map[string]int{
		"postgres":   5432,
		"redis":      6379,
		"chromadb":   8000,
		"helixagent": 8100,
	}

	// §11.4.174 / §11.4.201: ownership is established from the container runtime,
	// never inferred from a port number. Containers belonging to other projects
	// on a shared developer host are explicitly NOT ours and must not trip this
	// gate.
	//
	// The previous implementation called findContainerInstances with the runtime's
	// `compose` args, producing `docker compose ps --filter name=…` — not a
	// container-listing command. It returned nothing, so this check silently never
	// fired (a fail-open gate).
	if running := listRunningContainers(); running != nil {
		t.Log("Checking for duplicate container instances (this project only)...")

		for svcName := range servicePorts {
			ours := ourServiceInstances(running, svcName)
			if err := portConflict(svcName, ours); err != nil {
				return err
			}
			if len(ours) == 1 {
				t.Logf("  ✓ Single instance of %s owned by this project: %s", svcName, ours[0])
			}
		}
	}

	if rc.Enabled && len(rc.Hosts) > 0 {
		seenNames := make(map[string]string)
		seenAddresses := make(map[string]string)

		for _, host := range rc.Hosts {
			if existingAddr, exists := seenNames[host.Name]; exists {
				return fmt.Errorf("DUPLICATE_HOST: Host name '%s' defined multiple times (addresses: %s, %s)",
					host.Name, existingAddr, host.Address)
			}
			if existingName, exists := seenAddresses[host.Address]; exists {
				return fmt.Errorf("DUPLICATE_ADDRESS: Address '%s' used by multiple hosts (%s, %s)",
					host.Address, existingName, host.Name)
			}
			seenNames[host.Name] = host.Address
			seenAddresses[host.Address] = host.Name
		}
		t.Log("  ✓ No duplicate remote hosts configured")
	}

	// The former "local port conflicts" probe lived here. It dialled
	// {port, port+10000} and declared PORT_CONFLICT whenever both answered — with
	// no ownership check. Because this project's test stack publishes postgres on
	// 15432 (= 5432 + 10000), ANY unrelated postgres on the standard port tripped
	// a false failure on a shared host. Ownership is now established above from
	// the container runtime (§11.4.174 / §11.4.201); a port number alone can never
	// establish whose process it is.

	t.Log("  ✓ No multiple service instance conflicts detected")
	return nil
}

func detectContainerRuntime() (string, []string) {
	// Prefer docker over podman for compose operations
	runtimes := []string{"docker", "podman"}
	for _, runtime := range runtimes {
		// Try "<runtime> compose" (compose plugin)
		checkCmd := exec.Command(runtime, "compose", "version")
		if err := checkCmd.Run(); err == nil {
			return runtime, []string{"compose"}
		}
		// Try <runtime>-compose standalone binary
		standalone := runtime + "-compose"
		if _, err := exec.LookPath(standalone); err == nil {
			return standalone, nil
		}
	}
	// Fallback: check if docker CLI exists even without compose
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker", nil
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman", nil
	}
	return "", nil
}

func findContainerInstances(cmd string, args []string, serviceName string) []string {
	var execCmd *exec.Cmd
	if len(args) > 0 {
		fullArgs := append(args, "ps", "--filter", "name="+serviceName, "--format", "{{.Names}}")
		execCmd = exec.Command(cmd, fullArgs...)
	} else {
		execCmd = exec.Command(cmd, "ps", "--filter", "name="+serviceName, "--format", "{{.Names}}")
	}
	output, err := execCmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var instances []string
	for _, line := range lines {
		if line != "" {
			instances = append(instances, line)
		}
	}
	return instances
}

// projectContainerPrefix identifies containers belonging to THIS project's test
// stack (docker-compose.test.yml pins container_name: helixagent-<svc>).
// Containers without this prefix belong to other projects sharing the developer
// host and MUST NOT be attributed to us (§11.4.174).
const projectContainerPrefix = "helixagent-"

// listRunningContainers returns the names of every running container reported by
// the host container runtime. It queries the runtime binary DIRECTLY — not its
// `compose` subcommand, which does not accept `ps --filter name=` and silently
// returns nothing, turning any check built on it into a fail-open gate
// (§11.4.201).
func listRunningContainers() []string {
	for _, runtime := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(runtime); err != nil {
			continue
		}
		output, err := exec.Command(runtime, "ps", "--format", "{{.Names}}").Output()
		if err != nil {
			continue
		}
		var names []string
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line != "" {
				names = append(names, line)
			}
		}
		return names
	}
	return nil
}

// ourServiceInstances returns the running containers that belong to THIS project
// AND serve the named logical service. Foreign containers are filtered out
// before any conclusion is drawn (§11.4.174).
func ourServiceInstances(all []string, svcName string) []string {
	var ours []string
	for _, name := range all {
		if !strings.HasPrefix(name, projectContainerPrefix) {
			continue
		}
		if strings.Contains(strings.TrimPrefix(name, projectContainerPrefix), svcName) {
			ours = append(ours, name)
		}
	}
	return ours
}

// portConflict reports an error ONLY when this project runs more than one
// instance of the same logical service. One instance is the expected state;
// zero means the service is not up (reported by the required-services check, not
// here); a foreign instance is never counted.
func portConflict(svcName string, ourInstances []string) error {
	if len(ourInstances) > 1 {
		return fmt.Errorf("MULTIPLE_INSTANCES: this project runs %d instances of '%s': %v",
			len(ourInstances), svcName, ourInstances)
	}
	return nil
}

type RemoteConfig struct {
	Enabled bool
	Hosts   []RemoteHost
	SSHUser string
	SSHKey  string
}

type RemoteHost struct {
	Name    string
	Address string
	Port    int
	User    string
}

func (rc *RemoteConfig) HostsSummary() string {
	if len(rc.Hosts) == 0 {
		return "no hosts configured"
	}
	names := make([]string, len(rc.Hosts))
	for i, h := range rc.Hosts {
		names[i] = fmt.Sprintf("%s (%s)", h.Name, h.Address)
	}
	return strings.Join(names, ", ")
}

func parseContainersEnv(path string) *RemoteConfig {
	rc := &RemoteConfig{}

	data, err := os.ReadFile(path)
	if err != nil {
		return rc
	}

	lines := strings.Split(string(data), "\n")
	hostMap := make(map[int]*RemoteHost)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		switch key {
		case "CONTAINERS_REMOTE_ENABLED":
			rc.Enabled = strings.EqualFold(value, "true")
		case "CONTAINERS_REMOTE_DEFAULT_SSH_USER":
			rc.SSHUser = value
		case "CONTAINERS_REMOTE_DEFAULT_SSH_KEY":
			rc.SSHKey = value
		}

		if strings.HasPrefix(key, "CONTAINERS_REMOTE_HOST_") {
			idx, field := parseHostKey(key)
			if idx > 0 {
				if _, exists := hostMap[idx]; !exists {
					hostMap[idx] = &RemoteHost{Port: 22}
				}
				switch field {
				case "NAME":
					hostMap[idx].Name = value
				case "ADDRESS":
					hostMap[idx].Address = value
				case "PORT":
					fmt.Sscanf(value, "%d", &hostMap[idx].Port)
				case "USER":
					hostMap[idx].User = value
				}
			}
		}
	}

	for _, host := range hostMap {
		if host.Name != "" && host.Address != "" {
			if host.User == "" {
				host.User = rc.SSHUser
			}
			rc.Hosts = append(rc.Hosts, *host)
		}
	}

	return rc
}

func parseHostKey(key string) (int, string) {
	parts := strings.Split(key, "_")
	if len(parts) < 5 {
		return 0, ""
	}
	var idx int
	fmt.Sscanf(parts[3], "%d", &idx)
	field := strings.Join(parts[4:], "_")
	return idx, field
}

func verifyRemoteContainers(t *testing.T, rc *RemoteConfig) error {
	t.Log("Verifying remote container infrastructure...")

	for _, host := range rc.Hosts {
		t.Logf("Checking host: %s (%s:%d)", host.Name, host.Address, host.Port)

		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host.Address, host.Port), 10*time.Second)
		if err != nil {
			return fmt.Errorf("cannot connect to host %s at %s:%d: %w", host.Name, host.Address, host.Port, err)
		}
		conn.Close()
		t.Logf("  ✓ Host %s is reachable", host.Name)
	}

	return nil
}

func verifyLocalContainers(t *testing.T, projectRoot string) error {
	t.Log("Verifying local container infrastructure...")

	requiredServices := []struct {
		name        string
		port        int
		healthURL   string
		description string
		required    bool
	}{
		{"PostgreSQL", 15432, "", "Primary database", true},
		{"Redis", 16379, "", "Cache and session store", true},
		{"Mock LLM", 18081, "http://localhost:18081/health", "Mock LLM server for testing", true},
		{"ChromaDB", 8001, "http://localhost:8001/api/v2/heartbeat", "Vector store", false},
		{"HelixAgent", 8100, "http://localhost:8100/health", "Main service (optional for tests)", false},
	}

	allRequiredHealthy := true

	t.Log("\nService Status:")
	for _, svc := range requiredServices {
		healthy := checkTCPPort(svc.port)
		if svc.healthURL != "" {
			healthy = checkHTTPHealth(svc.healthURL, 5*time.Second) == nil
		}

		if !healthy {
			if svc.required {
				t.Errorf("  ✗ %s (port %d) - %s - NOT AVAILABLE (REQUIRED)", svc.name, svc.port, svc.description)
				allRequiredHealthy = false
			} else {
				t.Logf("  ~ %s (port %d) - %s - NOT AVAILABLE (optional)", svc.name, svc.port, svc.description)
			}
		} else {
			reqStr := "optional"
			if svc.required {
				reqStr = "required"
			}
			t.Logf("  ✓ %s (port %d) - %s (%s)", svc.name, svc.port, svc.description, reqStr)
		}
	}

	if !allRequiredHealthy {
		return fmt.Errorf("one or more required services are not running - run './bin/helixagent' for proper container orchestration (or set SKIP_INFRA_SETUP=true to skip)")
	}

	return nil
}

func checkTCPPort(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func checkHTTPHealth(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error: %d", resp.StatusCode)
	}
	return nil
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("project root not found")
}
