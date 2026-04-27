// Package containers provides an adapter layer between HelixAgent's
// container management and the extracted digital.vasic.containers
// module.
//
// This adapter centralizes all container operations (runtime
// detection, compose up/down, health checking, remote distribution)
// through the Containers module interfaces. All direct exec.Command
// calls to docker/podman should be replaced with adapter methods.
//
// Performance Features:
//   - Lazy runtime detection: Container runtime (Docker/Podman) detection
//     is deferred until first use via sync.Once
//   - Concurrency control: Weighted semaphore limits concurrent container
//     operations (2 * CPU cores, capped 2-10) to prevent system overload
//   - Thread-safe initialization: All lazy initialization is thread-safe
package containers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/distribution"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/envconfig"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/network"
	"digital.vasic.containers/pkg/orchestrator"
	"digital.vasic.containers/pkg/remote"
	containersruntime "digital.vasic.containers/pkg/runtime"
	"digital.vasic.containers/pkg/scheduler"
	"digital.vasic.containers/pkg/volume"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/placement"
	"gopkg.in/yaml.v3"
)

// Adapter bridges HelixAgent's container management with the
// Containers module.
type Adapter struct {
	runtimeInitOnce sync.Once
	runtimeInitErr  error

	runtime       containersruntime.ContainerRuntime
	orchestrator  compose.ComposeOrchestrator
	healthChecker health.HealthChecker
	distributor   distribution.Distributor
	hostManager   remote.HostManager
	executor      remote.RemoteExecutor
	tunnelManager network.TunnelManager
	volumeManager volume.VolumeManager
	logger        logging.Logger
	projectDir    string
	httpClient    *http.Client

	// concurrentOpsSem limits concurrent container operations to prevent
	// overwhelming the system. Default weight is 2 * CPU cores, max 10.
	concurrentOpsSem *semaphore.Weighted
}

// Option configures the Adapter.
type Option func(*Adapter)

// WithRuntime sets the container runtime.
func WithRuntime(r containersruntime.ContainerRuntime) Option {
	return func(a *Adapter) { a.runtime = r }
}

// WithOrchestrator sets the compose orchestrator.
func WithOrchestrator(o compose.ComposeOrchestrator) Option {
	return func(a *Adapter) { a.orchestrator = o }
}

// WithHealthChecker sets the health checker.
func WithHealthChecker(hc health.HealthChecker) Option {
	return func(a *Adapter) { a.healthChecker = hc }
}

// WithDistributor sets the container distributor.
func WithDistributor(d distribution.Distributor) Option {
	return func(a *Adapter) { a.distributor = d }
}

// WithHostManager sets the remote host manager.
func WithHostManager(hm remote.HostManager) Option {
	return func(a *Adapter) { a.hostManager = hm }
}

// WithLogger sets the logger.
func WithLogger(l logging.Logger) Option {
	return func(a *Adapter) { a.logger = l }
}

// WithProjectDir sets the project root directory.
func WithProjectDir(dir string) Option {
	return func(a *Adapter) { a.projectDir = dir }
}

// NewAdapter creates a container Adapter with the given options.
func NewAdapter(opts ...Option) (*Adapter, error) {
	// Calculate max concurrent operations: 2 * CPU cores, capped between 2 and 10
	maxConcurrent := 2 * runtime.NumCPU()
	if maxConcurrent < 2 {
		maxConcurrent = 2
	}
	if maxConcurrent > 10 {
		maxConcurrent = 10
	}

	a := &Adapter{
		httpClient:       &http.Client{Timeout: 10 * time.Second},
		concurrentOpsSem: semaphore.NewWeighted(int64(maxConcurrent)),
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.logger == nil {
		a.logger = logging.NopLogger{}
	}
	if a.projectDir == "" {
		dir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("detect project dir: %w", err)
		}
		a.projectDir = dir
	}
	return a, nil
}

// NewAdapterFromConfig creates an Adapter auto-configured from
// HelixAgent's config and environment. It detects the local
// container runtime, sets up the compose orchestrator, and
// optionally initializes remote distribution if env vars are set.
func NewAdapterFromConfig(cfg *config.Config) (*Adapter, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("detect project dir: %w", err)
	}

	logger := &logrusAdapter{}

	// Calculate max concurrent operations: 2 * CPU cores, capped between 2 and 10
	maxConcurrent := 2 * runtime.NumCPU()
	if maxConcurrent < 2 {
		maxConcurrent = 2
	}
	if maxConcurrent > 10 {
		maxConcurrent = 10
	}

	a := &Adapter{
		logger:           logger,
		projectDir:       projectDir,
		httpClient:       &http.Client{Timeout: 10 * time.Second},
		concurrentOpsSem: semaphore.NewWeighted(int64(maxConcurrent)),
	}

	// Auto-detect local runtime.
	rt, err := containersruntime.AutoDetect(context.Background())
	if err != nil {
		logger.Warn(
			"container runtime not available: %v", err,
		)
	} else {
		a.runtime = rt
		orch, orchErr := compose.NewDefaultOrchestrator(
			projectDir, logger,
		)
		if orchErr != nil {
			logger.Warn(
				"compose orchestrator not available: %v",
				orchErr,
			)
		} else {
			a.orchestrator = orch
		}
	}

	// Set up health checker.
	a.healthChecker = health.NewDefaultChecker()

	// Load Containers/.env as the single source of truth for
	// remote distribution config. Try project-relative path first,
	// then the Containers submodule.
	for _, envPath := range []string{
		filepath.Join(projectDir, "Containers", ".env"),
	} {
		if _, statErr := os.Stat(envPath); statErr == nil {
			if _, loadErr := envconfig.LoadFromFile(
				envPath,
			); loadErr != nil {
				logger.Warn(
					"load %s: %v", envPath, loadErr,
				)
			} else {
				logger.Info(
					"loaded remote config from %s", envPath,
				)
			}
			break
		}
	}

	// Check for remote distribution configuration.
	distCfg := envconfig.LoadFromEnv()
	if distCfg.Enabled {
		if err := a.setupDistribution(distCfg); err != nil {
			logger.Warn(
				"remote distribution setup failed: %v", err,
			)
		}
	}

	return a, nil
}

// setupDistribution configures remote distribution from env config.
func (a *Adapter) setupDistribution(
	cfg *envconfig.DistributionConfig,
) error {
	hosts := cfg.ToRemoteHosts()
	if len(hosts) == 0 {
		return fmt.Errorf("no remote hosts configured")
	}

	// Build SSH executor options from config.
	var sshOpts []remote.Option
	if cfg.ConnectTimeout > 0 {
		sshOpts = append(sshOpts, remote.WithConnectTimeout(
			time.Duration(cfg.ConnectTimeout)*time.Second,
		))
	}
	if cfg.CommandTimeout > 0 {
		sshOpts = append(sshOpts, remote.WithCommandTimeout(
			time.Duration(cfg.CommandTimeout)*time.Second,
		))
	}
	sshOpts = append(sshOpts, remote.WithControlMaster(
		cfg.ControlMasterEnabled,
	))
	if cfg.ControlPersist > 0 {
		sshOpts = append(sshOpts, remote.WithControlPersist(
			time.Duration(cfg.ControlPersist)*time.Second,
		))
	}
	if cfg.MaxConnections > 0 {
		sshOpts = append(sshOpts, remote.WithMaxConnections(
			cfg.MaxConnections,
		))
	}

	executor, execErr := remote.NewSSHExecutor(
		a.logger, sshOpts...,
	)
	if execErr != nil {
		return fmt.Errorf("create SSH executor: %w", execErr)
	}

	// Auto-bootstrap key auth on hosts that need it.
	ctx := context.Background()
	for i := range hosts {
		if executor.NeedsBootstrap(ctx, hosts[i]) {
			a.logger.Info(
				"bootstrapping SSH key auth on %s",
				hosts[i].Name,
			)
			if err := executor.BootstrapKeyAuth(
				ctx, hosts[i],
			); err != nil {
				a.logger.Warn(
					"bootstrap %s failed: %v",
					hosts[i].Name, err,
				)
			}
		}
	}

	hm := remote.NewHostManager(executor, a.logger)

	for _, h := range hosts {
		if err := hm.AddHost(h); err != nil {
			a.logger.Warn("add host %s: %v", h.Name, err)
		}
	}

	strategy := scheduler.StrategyResourceAware
	switch cfg.Scheduler {
	case "round_robin":
		strategy = scheduler.StrategyRoundRobin
	case "affinity":
		strategy = scheduler.StrategyAffinity
	case "spread":
		strategy = scheduler.StrategySpread
	case "bin_pack":
		strategy = scheduler.StrategyBinPack
	}

	sched := scheduler.NewScheduler(hm, a.logger,
		scheduler.WithStrategy(strategy),
	)

	var tm network.TunnelManager
	var vm volume.VolumeManager

	if cfg.PortRangeStart > 0 && cfg.PortRangeEnd > 0 {
		tm = network.NewTunnelManager(hm, a.logger,
			network.WithPortRange(
				cfg.PortRangeStart, cfg.PortRangeEnd,
			),
		)
	} else {
		tm = network.NewTunnelManager(hm, a.logger)
	}

	vm = volume.NewVolumeManager(hm, executor, a.logger)

	a.executor = executor
	a.hostManager = hm
	a.tunnelManager = tm
	a.volumeManager = vm
	a.distributor = distribution.NewDistributor(
		distribution.WithScheduler(sched),
		distribution.WithHostManager(hm),
		distribution.WithExecutor(executor),
		distribution.WithTunnelManager(tm),
		distribution.WithVolumeManager(vm),
		distribution.WithLogger(a.logger),
	)

	a.logger.Info(
		"remote distribution enabled with %d hosts",
		len(hosts),
	)
	return nil
}

// initRuntime lazily initializes the container runtime and orchestrator.
// It uses sync.Once to ensure initialization happens only once.
func (a *Adapter) initRuntime(ctx context.Context) error {
	a.runtimeInitOnce.Do(func() {
		// If runtime already set (e.g., via WithRuntime), skip auto-detection
		if a.runtime != nil {
			a.runtimeInitErr = nil
			return
		}

		// Auto-detect container runtime
		rt, err := containersruntime.AutoDetect(ctx)
		if err != nil {
			a.runtimeInitErr = fmt.Errorf("no container runtime found: %w", err)
			return
		}
		a.runtime = rt

		// Try to create orchestrator if runtime detected
		orch, orchErr := compose.NewDefaultOrchestrator(
			a.projectDir, a.logger,
		)
		if orchErr == nil {
			a.orchestrator = orch
		}
		// Note: orchestrator creation error is not fatal
		a.runtimeInitErr = nil
	})
	return a.runtimeInitErr
}

// DetectRuntime returns the name of the detected container runtime
// (e.g., "docker" or "podman"). If no runtime is available, returns
// an error.
func (a *Adapter) DetectRuntime(
	ctx context.Context,
) (string, error) {
	if a.runtime != nil {
		return a.runtime.Name(), nil
	}
	if err := a.initRuntime(ctx); err != nil {
		return "", err
	}
	if a.runtime == nil {
		return "", fmt.Errorf("container runtime not initialized")
	}
	return a.runtime.Name(), nil
}

// RuntimeAvailable returns true if a container runtime is detected.
func (a *Adapter) RuntimeAvailable(ctx context.Context) bool {
	if a.runtime != nil {
		return a.runtime.IsAvailable(ctx)
	}
	// Try to initialize runtime if not already done
	if err := a.initRuntime(ctx); err != nil {
		return false
	}
	if a.runtime == nil {
		return false
	}
	return a.runtime.IsAvailable(ctx)
}

// ComposeUp starts services from a compose file with the given
// profile.
func (a *Adapter) ComposeUp(
	ctx context.Context, composeFile, profile string,
) error {
	if a.orchestrator == nil {
		return fmt.Errorf("compose orchestrator not available")
	}

	// Acquire semaphore to limit concurrent container operations
	if a.concurrentOpsSem != nil {
		if err := a.concurrentOpsSem.Acquire(ctx, 1); err != nil {
			return err
		}
		defer a.concurrentOpsSem.Release(1)
	}

	absFile := composeFile
	if !filepath.IsAbs(composeFile) {
		absFile = filepath.Join(a.projectDir, composeFile)
	}

	project := compose.ComposeProject{
		File:    absFile,
		Profile: profile,
	}

	a.logger.Info("compose up: %s (profile: %s)",
		composeFile, profile,
	)
	err := a.orchestrator.Up(ctx, project)
	if err != nil {
		a.logger.Error("compose up FAILED: %v", err)
		return err
	}
	a.logger.Info("compose up completed successfully")
	return nil
}

// ComposeDown stops services from a compose file.
func (a *Adapter) ComposeDown(
	ctx context.Context, composeFile, profile string,
) error {
	if a.orchestrator == nil {
		return fmt.Errorf("compose orchestrator not available")
	}

	// Acquire semaphore to limit concurrent container operations
	if a.concurrentOpsSem != nil {
		if err := a.concurrentOpsSem.Acquire(ctx, 1); err != nil {
			return err
		}
		defer a.concurrentOpsSem.Release(1)
	}

	absFile := composeFile
	if !filepath.IsAbs(composeFile) {
		absFile = filepath.Join(a.projectDir, composeFile)
	}

	project := compose.ComposeProject{
		File:    absFile,
		Profile: profile,
	}

	a.logger.Info("compose down: %s", composeFile)
	return a.orchestrator.Down(ctx, project)
}

// ComposeStatus returns the status of services from a compose file.
func (a *Adapter) ComposeStatus(
	ctx context.Context, composeFile string,
) ([]compose.ServiceStatus, error) {
	if a.orchestrator == nil {
		return nil, fmt.Errorf(
			"compose orchestrator not available",
		)
	}

	// Acquire semaphore to limit concurrent container operations
	if a.concurrentOpsSem != nil {
		if err := a.concurrentOpsSem.Acquire(ctx, 1); err != nil {
			return nil, err
		}
		defer a.concurrentOpsSem.Release(1)
	}

	absFile := composeFile
	if !filepath.IsAbs(composeFile) {
		absFile = filepath.Join(a.projectDir, composeFile)
	}

	project := compose.ComposeProject{File: absFile}
	return a.orchestrator.Status(ctx, project)
}

// HealthCheck checks the health of a service target.
func (a *Adapter) HealthCheck(
	ctx context.Context,
	name, host, port, healthPath, healthType string,
	timeout time.Duration,
) (*health.HealthResult, error) {
	if a.healthChecker == nil {
		return nil, fmt.Errorf("health checker not available")
	}

	target := health.HealthTarget{
		Name:    name,
		Host:    host,
		Port:    port,
		Path:    healthPath,
		Type:    health.HealthType(healthType),
		Timeout: timeout,
	}

	result := a.healthChecker.Check(ctx, target)
	return result, nil
}

// HealthCheckHTTP performs an HTTP health check on the given URL.
func (a *Adapter) HealthCheckHTTP(url string) error {
	resp, err := a.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("cannot connect: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"health check failed with status: %d",
			resp.StatusCode,
		)
	}
	return nil
}

// HealthCheckTCP performs a TCP health check.
func (a *Adapter) HealthCheckTCP(
	host string, port int,
) bool {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ToEndpoint converts HelixAgent service config fields to a
// Containers module ServiceEndpoint.
func (a *Adapter) ToEndpoint(
	name, host, port, healthPath, healthType,
	composeFile, serviceName, profile string,
	enabled, required, isRemote bool,
) endpoint.ServiceEndpoint {
	return endpoint.ServiceEndpoint{
		Host:        host,
		Port:        port,
		HealthPath:  healthPath,
		HealthType:  healthType,
		ComposeFile: composeFile,
		ServiceName: serviceName,
		Profile:     profile,
		Enabled:     enabled,
		Required:    required,
		Remote:      isRemote,
	}
}

// BootAll boots all provided endpoints using the Containers module's
// BootManager. It creates a BootManager, registers endpoints, starts
// compose groups, and runs health checks.
func (a *Adapter) BootAll(
	ctx context.Context,
	endpoints map[string]endpoint.ServiceEndpoint,
) (*boot.BootSummary, error) {
	opts := []boot.BootManagerOption{
		boot.WithLogger(a.logger),
	}
	if a.runtime != nil {
		opts = append(opts, boot.WithRuntime(a.runtime))
	}
	if a.orchestrator != nil {
		opts = append(opts, boot.WithOrchestrator(a.orchestrator))
	}
	if a.healthChecker != nil {
		opts = append(opts, boot.WithHealthChecker(a.healthChecker))
	}
	if a.projectDir != "" {
		opts = append(opts, boot.WithProjectDir(a.projectDir))
	}
	if a.distributor != nil {
		if d, ok := a.distributor.(*distribution.DefaultDistributor); ok {
			opts = append(opts, boot.WithDistributor(d))
		}
	}
	if a.hostManager != nil {
		opts = append(opts, boot.WithHostManager(a.hostManager))
	}

	bm := boot.NewBootManager(endpoints, opts...)
	return bm.BootAll(ctx)
}

// ToHealthTarget converts service configuration fields into a
// Containers module HealthTarget.
func (a *Adapter) ToHealthTarget(
	name, host, port, healthPath, healthType string,
	timeout time.Duration, required bool,
) health.HealthTarget {
	return health.HealthTarget{
		Name:     name,
		Host:     host,
		Port:     port,
		Type:     health.HealthType(healthType),
		Path:     healthPath,
		Timeout:  timeout,
		Required: required,
	}
}

// ListContainers returns the status of services from the compose
// file at the given path. Uses the adapter's compose orchestrator.
func (a *Adapter) ListContainers(
	ctx context.Context, composeFile string,
) ([]compose.ServiceStatus, error) {
	return a.ComposeStatus(ctx, composeFile)
}

// HealthCheckAll performs health checks on a list of targets and
// returns errors keyed by target name.
func (a *Adapter) HealthCheckAll(
	ctx context.Context, targets []health.HealthTarget,
) map[string]error {
	errors := make(map[string]error)
	if a.healthChecker == nil {
		return errors
	}
	results := a.healthChecker.CheckAll(ctx, targets)
	for i, result := range results {
		if !result.Healthy {
			errors[targets[i].Name] = fmt.Errorf(
				"health check failed: %s", result.Error,
			)
		}
	}
	return errors
}

// StatusAll returns the status of all running containers.
func (a *Adapter) StatusAll(
	ctx context.Context,
) (map[string]string, error) {
	status := make(map[string]string)
	if a.runtime == nil {
		return status, fmt.Errorf("no container runtime available")
	}

	// Acquire semaphore to limit concurrent container operations
	if a.concurrentOpsSem != nil {
		if err := a.concurrentOpsSem.Acquire(ctx, 1); err != nil {
			return status, err
		}
		defer a.concurrentOpsSem.Release(1)
	}

	containers, err := a.runtime.List(
		ctx, containersruntime.ListFilter{},
	)
	if err != nil {
		return status, fmt.Errorf("list containers: %w", err)
	}
	for _, c := range containers {
		status[c.Name] = string(c.State)
	}
	return status, nil
}

// Distribute distributes containers across local and remote hosts
// using the configured scheduler and remote executor.
func (a *Adapter) Distribute(
	ctx context.Context,
	reqs []scheduler.ContainerRequirements,
) (*distribution.DistributionSummary, error) {
	if a.distributor == nil {
		return nil, fmt.Errorf("distributor not configured")
	}
	return a.distributor.Distribute(ctx, reqs)
}

// Undistribute stops all distributed containers.
func (a *Adapter) Undistribute(ctx context.Context) error {
	if a.distributor == nil {
		return nil
	}
	return a.distributor.Undistribute(ctx)
}

// DistributionStatus returns the current state of all distributed
// containers.
func (a *Adapter) DistributionStatus(
	ctx context.Context,
) []distribution.DistributedContainer {
	if a.distributor == nil {
		return nil
	}
	return a.distributor.Status(ctx)
}

// RemoteEnabled returns true if remote distribution is configured.

// composeService represents a service definition from a compose file
type composeService struct {
	Build interface{} `yaml:"build"` // Can be string or composeBuildConfig
}

// composeFile represents the structure of a docker-compose.yml file
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

// extractBuildContexts parses a compose file and extracts build context paths
func (a *Adapter) extractBuildContexts(composePath string) ([]string, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse compose file: %w", err)
	}

	var contexts []string
	composeDir := filepath.Dir(composePath)

	// Resolve project root once so we can skip any build context that
	// points at the project itself — that's the HelixAgent orchestrator
	// container (build: { context: . }) which should NEVER be shipped
	// to remote workers. The orchestrator runs on the local machine;
	// only the workloads it schedules (postgres, redis, MCP servers,
	// cognee, etc.) get pushed remotely. Without this skip, every
	// remote-distribution boot tried to scp the entire 27 GB project
	// directory (vendor/, releases/, cli_agents/, submodules, models)
	// and hung for 30+ minutes before timing out.
	projectRoot := a.projectDir
	if abs, err := filepath.Abs(projectRoot); err == nil {
		projectRoot = abs
	}

	isProjectRoot := func(p string) bool {
		abs, err := filepath.Abs(p)
		if err != nil {
			return false
		}
		// Filepath.Clean both sides so trailing "/." or "/" don't throw off the compare.
		return filepath.Clean(abs) == filepath.Clean(projectRoot)
	}

	for _, service := range cf.Services {
		if service.Build == nil {
			continue
		}

		// Build can be a simple string (context path) or a map
		switch build := service.Build.(type) {
		case string:
			// Simple string context
			if !filepath.IsAbs(build) {
				build = filepath.Join(composeDir, build)
			}
			if isProjectRoot(build) {
				a.logger.Info(
					"skipping project-root build context (orchestrator stays local): %s",
					build,
				)
				continue
			}
			contexts = append(contexts, build)
		case map[string]interface{}:
			// Map with context and dockerfile
			if ctx, ok := build["context"].(string); ok {
				if !filepath.IsAbs(ctx) {
					ctx = filepath.Join(composeDir, ctx)
				}
				if isProjectRoot(ctx) {
					a.logger.Info(
						"skipping project-root build context (orchestrator stays local): %s",
						ctx,
					)
					continue
				}
				contexts = append(contexts, ctx)

				// Also add dockerfile if specified
				if df, ok := build["dockerfile"].(string); ok {
					if !filepath.IsAbs(df) {
						df = filepath.Join(ctx, df)
					}
					contexts = append(contexts, df)
				}
			}
		}
	}

	return contexts, nil
}

// copyBuildContextsParallelism caps the number of concurrent SCP
// operations per host. Each context can take 5-10 s for the SSH+SCP
// round-trip; serial copies of 12+ contexts dominated boot time
// (Finding #43 — observed 2:18 to 2:33 boot from sequential SCP).
// 4 is a safe default that keeps the host's SSH ControlMaster from
// being overwhelmed; SSH itself multiplexes streams cheaply.
const copyBuildContextsParallelism = 4

// copyBuildContexts copies build context directories and Dockerfiles to
// the remote host with bounded parallelism (Finding #43). Per-context
// failures are collected and returned as an aggregate error; one slow
// context never blocks the others.
func (a *Adapter) copyBuildContexts(
	ctx context.Context,
	host remote.RemoteHost,
	contexts []string,
	remoteDir string,
) error {
	if len(contexts) == 0 {
		return nil
	}

	type copyResult struct {
		err error
	}

	results := make(chan copyResult, len(contexts))
	sem := make(chan struct{}, copyBuildContextsParallelism)
	var wg sync.WaitGroup

	for _, buildCtx := range contexts {
		buildCtx := buildCtx // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results <- copyResult{err: ctx.Err()}
				return
			}
			results <- copyResult{err: a.copyOneBuildContext(ctx, host, buildCtx, remoteDir)}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errors []error
	for r := range results {
		if r.err != nil {
			errors = append(errors, r.err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("copy build contexts failed: %d errors: %v", len(errors), errors)
	}
	return nil
}

// copyOneBuildContext does the original sequential mkdir+stat+copy work
// for a single context. Extracted so copyBuildContexts can fan it out
// across goroutines while keeping the per-context logic readable.
func (a *Adapter) copyOneBuildContext(
	ctx context.Context,
	host remote.RemoteHost,
	buildCtx, remoteDir string,
) error {
	relPath, err := filepath.Rel(a.projectDir, buildCtx)
	if err != nil {
		relPath = filepath.Base(buildCtx)
	}

	remotePath := remoteDir + "/" + relPath
	remoteParent := filepath.Dir(remotePath)

	mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteParent)
	if _, err = a.executor.Execute(ctx, host, mkdirCmd); err != nil {
		a.logger.Warn("Failed to create remote directory %s: %v", remoteParent, err)
		return fmt.Errorf("create remote dir %s: %w", remoteParent, err)
	}

	info, err := os.Stat(buildCtx)
	if err != nil {
		a.logger.Warn("Failed to stat %s: %v", buildCtx, err)
		return fmt.Errorf("stat %s: %w", buildCtx, err)
	}

	if info.IsDir() {
		if err := a.executor.CopyDir(ctx, host, buildCtx, remotePath); err != nil {
			a.logger.Error("Failed to copy directory %s to remote: %v", buildCtx, err)
			return fmt.Errorf("copy dir %s: %w", buildCtx, err)
		}
		a.logger.Info("Copied build context to remote: %s -> %s", buildCtx, remotePath)
		return nil
	}
	if err := a.executor.CopyFile(ctx, host, buildCtx, remotePath); err != nil {
		a.logger.Error("Failed to copy file %s to remote: %v", buildCtx, err)
		return fmt.Errorf("copy file %s: %w", buildCtx, err)
	}
	return nil
}

func (a *Adapter) RemoteEnabled() bool {
	return a.distributor != nil && a.hostManager != nil
}

// deployProfileLabel is the RemoteHost.Labels key whose value,
// when present, overrides the caller-supplied compose profile on a
// per-host basis. This enables label-based service sharding
// (Mode B distribution): give each host a label like
// CONTAINERS_REMOTE_HOST_1_LABELS=deploy_profile=storage and the
// orchestrator will call `compose up --profile storage` on that host.
//
// If no host carries this label, every host receives the caller's
// profile argument — which with a compose file that has no per-service
// profile tags is effectively full-stack replication across hosts
// (Mode A). Mode B activates as soon as (a) hosts are labeled and
// (b) services in the compose file get matching `profiles:` tags.
const deployProfileLabel = "deploy_profile"

// RemoteComposeUp deploys a compose file to ALL registered remote
// hosts, honoring each host's deploy_profile label for Mode B
// sharding. Deployment is sequential (so logs are readable and one
// slow host doesn't block parsing of the others); per-host failures
// are collected and reported as an aggregate error, but a failure
// on one host does NOT abort the others. If no hosts succeed the
// method returns an error; partial success logs a warning and
// returns nil so the boot can proceed.
func (a *Adapter) RemoteComposeUp(
	ctx context.Context, composeFile, profile string,
) error {
	if a.hostManager == nil || a.executor == nil {
		return fmt.Errorf(
			"remote distribution not configured",
		)
	}

	hosts := a.hostManager.ListHosts()
	if len(hosts) == 0 {
		return fmt.Errorf("no remote hosts available")
	}

	absFile := composeFile
	if !filepath.IsAbs(composeFile) {
		absFile = filepath.Join(a.projectDir, composeFile)
	}
	if _, err := os.Stat(absFile); err != nil {
		return fmt.Errorf(
			"compose file not found: %s", absFile,
		)
	}

	// PARTITIONED distribution (CONST-034 / BUGFIXES #52): plan first
	// so every service lands on EXACTLY one host across the
	// registered remote-host set, with depends_on co-location groups
	// kept atomic. Pre-fix, this method broadcast every compose file
	// to every host, producing replicated postgres/redis/etc. with
	// divergent state.
	plan, err := placement.PlanCompose(
		ctx, absFile, profile, a.hostManager,
		scheduler.WithStrategy(scheduler.StrategyResourceAware),
	)
	if err != nil {
		return fmt.Errorf("plan compose: %w", err)
	}
	if len(plan.Assignments) == 0 {
		a.logger.Info(
			"placement plan is empty for %s (profile=%q); nothing to deploy",
			absFile, profile,
		)
		return nil
	}

	planFile := filepath.Join(
		filepath.Dir(absFile),
		fmt.Sprintf(".placement-plan-%s.json",
			sanitizePlanName(filepath.Base(absFile))),
	)
	if err := placement.WritePlanJSON(planFile, plan); err != nil {
		a.logger.Warn("persist placement plan failed: %v", err)
	}

	stageDir, err := os.MkdirTemp(filepath.Dir(absFile), ".placement-")
	if err != nil {
		return fmt.Errorf("create placement staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(stageDir) }()

	var (
		deployFailures []string
		deploySuccess  int
	)

	for _, assign := range plan.Assignments {
		hostName := assign.HostName
		var host remote.RemoteHost
		var found bool
		for _, h := range hosts {
			if h.Name == hostName {
				host, found = h, true
				break
			}
		}
		if !found {
			a.logger.Error(
				"placement targeted unknown host %q for compose %s; skipping",
				hostName, filepath.Base(absFile),
			)
			deployFailures = append(deployFailures,
				fmt.Sprintf("%s: host %q not registered", hostName, hostName))
			continue
		}

		effectiveProfile := profile
		if labelProfile := host.Labels[deployProfileLabel]; labelProfile != "" {
			effectiveProfile = labelProfile
			a.logger.Info(
				"host %s has %s=%s label; overriding deploy profile "+
					"for this host (caller asked %q)",
				host.Name, deployProfileLabel, labelProfile, profile,
			)
		}

		// Emit per-host filtered compose at the SAME relative
		// position in the project tree so build contexts (e.g.
		// `context: ../..` in the MCP compose) resolve correctly on
		// remote.
		stagedAtCanonical := filepath.Join(
			filepath.Dir(absFile),
			fmt.Sprintf(".placement-%s-%s",
				host.Name, filepath.Base(absFile)),
		)
		if _, err := placement.EmitPerHostCompose(
			absFile, assign.ServiceList, stagedAtCanonical,
		); err != nil {
			deployFailures = append(deployFailures,
				fmt.Sprintf("%s: emit per-host compose: %v", host.Name, err))
			continue
		}

		a.logger.Info(
			"placement: deploying %d service(s) to %s via %s",
			len(assign.ServiceList), host.Name,
			filepath.Base(stagedAtCanonical),
		)

		if err := a.deployComposeToHost(
			ctx, host, stagedAtCanonical, effectiveProfile,
		); err != nil {
			a.logger.Error(
				"deploy to %s failed: %v", host.Name, err,
			)
			deployFailures = append(deployFailures,
				fmt.Sprintf("%s: %v", host.Name, err))
			_ = os.Remove(stagedAtCanonical)
			continue
		}
		_ = os.Remove(stagedAtCanonical)

		// Export SVC_<SERVICE>_HOST for the gateway's runtime
		// routing (internal/config picks these up).
		for _, svc := range assign.ServiceList {
			envKey := serviceHostEnvKey(svc)
			hostAddr := host.Address
			if hostAddr == "" {
				hostAddr = host.Name
			}
			_ = os.Setenv(envKey, hostAddr)
		}
		_ = ctx // satisfy any linter
		deploySuccess++
	}

	switch {
	case deploySuccess == 0:
		return fmt.Errorf(
			"deploy to all %d remote hosts failed: %s",
			len(hosts), strings.Join(deployFailures, "; "),
		)
	case len(deployFailures) > 0:
		a.logger.Warn(
			"partial remote deploy: %d/%d hosts ok; failures: %s",
			deploySuccess, len(hosts),
			strings.Join(deployFailures, "; "),
		)
	default:
		a.logger.Info(
			"remote deploy ok on all %d host(s)", len(hosts),
		)
	}
	return nil
}

// deployComposeToHost ships the compose file (plus any build
// contexts) to a single remote host and runs `compose up -d` with
// the given profile. Extracted from RemoteComposeUp so the outer
// loop stays readable.
func (a *Adapter) deployComposeToHost(
	ctx context.Context,
	host remote.RemoteHost,
	absFile, profile string,
) error {
	remoteDir := fmt.Sprintf(
		"/home/%s/helixagent/deploy", host.User,
	)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteDir)
	if _, err := a.executor.Execute(
		ctx, host, mkdirCmd,
	); err != nil {
		return fmt.Errorf("create remote dir: %w", err)
	}

	// Preserve the compose file's position relative to the project
	// root so `context: ../../X` references land on the correct
	// directory on remote. The MCP servers compose lives at
	// `docker/mcp/docker-compose.mcp-servers.yml` and uses
	// `../../MCP-Servers` etc.; flattening to `<remoteDir>/<basename>`
	// would break those references (BUGFIXES.md Issue #51). The main
	// docker-compose.yml at project root is unaffected — its relPath
	// is just the basename.
	relCompose, relErr := filepath.Rel(a.projectDir, absFile)
	if relErr != nil || strings.HasPrefix(relCompose, "..") {
		// Fall back to flat layout for anything outside the project.
		relCompose = filepath.Base(absFile)
	}
	remoteFile := filepath.Join(remoteDir, relCompose)
	remoteFileParent := filepath.Dir(remoteFile)
	if remoteFileParent != remoteDir {
		mkParent := fmt.Sprintf("mkdir -p %s", remoteFileParent)
		if _, err := a.executor.Execute(ctx, host, mkParent); err != nil {
			return fmt.Errorf("create remote compose dir %s: %w", remoteFileParent, err)
		}
	}
	if err := a.executor.CopyFile(
		ctx, host, absFile, remoteFile,
	); err != nil {
		return fmt.Errorf("copy compose file: %w", err)
	}

	contexts, err := a.extractBuildContexts(absFile)
	if err != nil {
		a.logger.Warn(
			"extract build contexts from %s: %v",
			absFile, err,
		)
	} else if len(contexts) > 0 {
		a.logger.Info(
			"copying %d build contexts to %s",
			len(contexts), host.Name,
		)
		if err := a.copyBuildContexts(
			ctx, host, contexts, remoteDir,
		); err != nil {
			a.logger.Warn(
				"partial build-context copy to %s: %v",
				host.Name, err,
			)
		}
	}

	remoteOrch := remote.NewRemoteComposeOrchestrator(
		host, a.executor, a.logger,
	)
	project := compose.ComposeProject{
		File:    remoteFile,
		Profile: profile,
	}

	a.logger.Info(
		"remote compose up on %s: %s (profile: %s)",
		host.Name, remoteFile, profile,
	)
	return remoteOrch.Up(ctx, project)
}

// ListHosts returns all registered remote hosts.
func (a *Adapter) ListHosts() []remote.RemoteHost {
	if a.hostManager == nil {
		return nil
	}
	return a.hostManager.ListHosts()
}

// ProbeHost returns resource info for a specific remote host.
func (a *Adapter) ProbeHost(
	ctx context.Context, name string,
) (*remote.HostResources, error) {
	if a.hostManager == nil {
		return nil, fmt.Errorf("host manager not configured")
	}
	return a.hostManager.ProbeHost(ctx, name)
}

// sanitizePlanName turns a compose-file basename into something safe
// for use in a sibling JSON filename.
func sanitizePlanName(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '-' || c == '_' || c == '.':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// serviceHostEnvKey turns "helixagent-postgres" / "postgres" into
// "SVC_POSTGRES_HOST" matching internal/config's override mechanism.
func serviceHostEnvKey(serviceName string) string {
	name := serviceName
	if strings.HasPrefix(name, "helixagent-") {
		name = name[len("helixagent-"):]
	}
	out := []byte("SVC_")
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z':
			out = append(out, c-32)
		case c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out) + "_HOST"
}

// HostManager exposes the adapter's underlying remote.HostManager so
// callers (e.g. internal/placement.PlanCompose) can run scheduling
// against the live host registry. Returns nil if remote distribution
// is disabled.
func (a *Adapter) HostManager() remote.HostManager {
	return a.hostManager
}

// DeployComposeToHost ships a compose file to ONE specific named host
// and runs `compose up -d` with the given profile. This is the
// partitioned counterpart of RemoteComposeUp (which fans out to every
// host). Used by the placement-aware deploy flow that ships a
// per-host filtered compose file produced by
// internal/placement.EmitPerHostCompose.
func (a *Adapter) DeployComposeToHost(
	ctx context.Context, hostName, composeFile, profile string,
) error {
	if a.hostManager == nil || a.executor == nil {
		return fmt.Errorf("remote distribution not configured")
	}
	hosts := a.hostManager.ListHosts()
	for _, h := range hosts {
		if h.Name == hostName {
			absFile := composeFile
			if !filepath.IsAbs(absFile) {
				absFile = filepath.Join(a.projectDir, absFile)
			}
			if _, err := os.Stat(absFile); err != nil {
				return fmt.Errorf("compose file not found: %s", absFile)
			}
			return a.deployComposeToHost(ctx, h, absFile, profile)
		}
	}
	return fmt.Errorf("host %q not registered", hostName)
}

// Shutdown gracefully shuts down all container operations:
// closes tunnels, unmounts volumes, stops distributed containers.
func (a *Adapter) Shutdown(ctx context.Context) error {
	var errs []string

	if a.distributor != nil {
		if err := a.distributor.Undistribute(ctx); err != nil {
			errs = append(errs, fmt.Sprintf(
				"undistribute: %v", err,
			))
		}
	}

	if a.tunnelManager != nil {
		if err := a.tunnelManager.CloseAll(); err != nil {
			errs = append(errs, fmt.Sprintf(
				"close tunnels: %v", err,
			))
		}
	}

	if a.volumeManager != nil {
		if err := a.volumeManager.UnmountAll(ctx); err != nil {
			errs = append(errs, fmt.Sprintf(
				"unmount volumes: %v", err,
			))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(
			"shutdown errors: %s",
			strings.Join(errs, "; "),
		)
	}
	return nil
}

// Runtime returns the underlying container runtime. May be nil.
func (a *Adapter) Runtime() containersruntime.ContainerRuntime {
	return a.runtime
}

// Orchestrator returns the underlying compose orchestrator. May
// be nil.
func (a *Adapter) Orchestrator() compose.ComposeOrchestrator {
	return a.orchestrator
}

type serviceOrchAdapter struct {
	orch compose.ComposeOrchestrator
}

func (a *serviceOrchAdapter) Up(ctx context.Context, project compose.ComposeProject) error {
	return a.orch.Up(ctx, project)
}

func (a *serviceOrchAdapter) Down(ctx context.Context, project compose.ComposeProject) error {
	return a.orch.Down(ctx, project)
}

type remoteExecAdapter struct {
	exec remote.RemoteExecutor
}

func (a *remoteExecAdapter) Execute(ctx context.Context, host remote.RemoteHost, cmd string) (*remote.CommandResult, error) {
	return a.exec.Execute(ctx, host, cmd)
}

func (a *remoteExecAdapter) CopyDir(ctx context.Context, host remote.RemoteHost, src, dst string) error {
	return a.exec.CopyDir(ctx, host, src, dst)
}

type hostMgrAdapter struct {
	mgr remote.HostManager
}

func (a *hostMgrAdapter) ListHosts() []remote.RemoteHost {
	return a.mgr.ListHosts()
}

func (a *Adapter) NewServiceOrchestrator() *orchestrator.DefaultOrchestrator {
	opts := []orchestrator.Option{
		orchestrator.WithLogger(a.logger),
		orchestrator.WithProjectDir(a.projectDir),
	}
	if a.orchestrator != nil {
		opts = append(opts, orchestrator.WithLocalOrchestrator(&serviceOrchAdapter{orch: a.orchestrator}))
	}
	if a.executor != nil {
		opts = append(opts, orchestrator.WithRemoteExecutor(&remoteExecAdapter{exec: a.executor}))
	}
	if a.hostManager != nil {
		opts = append(opts, orchestrator.WithHostManager(&hostMgrAdapter{mgr: a.hostManager}))
	}
	if a.healthChecker != nil {
		opts = append(opts, orchestrator.WithHealthChecker(a.healthChecker))
	}
	return orchestrator.New(opts...)
}

// logrusAdapter bridges the logging.Logger interface with
// HelixAgent's typical logrus usage.
type logrusAdapter struct{}

func (l *logrusAdapter) Debug(msg string, args ...any) {
	fmt.Printf("[DEBUG] "+msg+"\n", args...)
}

func (l *logrusAdapter) Info(msg string, args ...any) {
	fmt.Printf("[INFO] "+msg+"\n", args...)
}

func (l *logrusAdapter) Warn(msg string, args ...any) {
	fmt.Printf("[WARN] "+msg+"\n", args...)
}

func (l *logrusAdapter) Error(msg string, args ...any) {
	fmt.Printf("[ERROR] "+msg+"\n", args...)
}
