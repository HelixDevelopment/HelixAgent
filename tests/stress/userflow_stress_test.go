package stress

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	uf "digital.vasic.challenges/pkg/userflow"

	"dev.helix.agent/internal/challenges/userflow"
)

const (
	userflowBaseURL         = "http://localhost:7061"
	userflowConcurrency     = 50
	userflowHighConcurrency = 1000
	userflowSequentialCount = 500
	userflowExpectedCount   = 22
)

// TestOrchestrator_NewOrchestrator_ConcurrentCreation
// verifies that 50 goroutines can create NewOrchestrator
// simultaneously without panics, races, or data corruption.
func TestOrchestrator_NewOrchestrator_ConcurrentCreation(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	var wg sync.WaitGroup
	orchestrators := make(
		[]*userflow.Orchestrator,
		userflowConcurrency,
	)
	errs := make(chan error, userflowConcurrency)

	for i := 0; i < userflowConcurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errs <- fmt.Errorf(
						"goroutine %d panicked: %v",
						idx, r,
					)
				}
			}()
			url := fmt.Sprintf(
				"http://localhost:%d", 7061+idx,
			)
			o, oErr := userflow.NewOrchestrator(url)
			if oErr != nil {
				errs <- fmt.Errorf(
					"goroutine %d: %v", idx, oErr,
				)
				return
			}
			orchestrators[idx] = o
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent creation failed: %v", err)
	}

	for i, o := range orchestrators {
		require.NotNil(t, o,
			"orchestrator %d must not be nil", i)
		assert.Equal(t, userflowExpectedCount,
			o.ChallengeCount(),
			"orchestrator %d challenge count", i)
	}
}

// TestOrchestrator_ListChallenges_RapidConcurrent
// hammers ListChallenges() with 1000 concurrent goroutines
// on a shared orchestrator to verify thread safety.
func TestOrchestrator_ListChallenges_RapidConcurrent(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	o, err := userflow.NewOrchestrator(userflowBaseURL)
	require.NoError(t, err)
	require.NotNil(t, o)

	var wg sync.WaitGroup
	var failures atomic.Int64

	for i := 0; i < userflowHighConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					failures.Add(1)
				}
			}()
			ids := o.ListChallenges()
			if len(ids) != userflowExpectedCount {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(0), failures.Load(),
		"all %d ListChallenges calls must succeed",
		userflowHighConcurrency,
	)
}

// TestOrchestrator_ChallengeCount_RapidConcurrent
// hammers ChallengeCount() with 1000 concurrent goroutines
// on a shared orchestrator to verify thread safety.
func TestOrchestrator_ChallengeCount_RapidConcurrent(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	o, err := userflow.NewOrchestrator(userflowBaseURL)
	require.NoError(t, err)
	require.NotNil(t, o)

	var wg sync.WaitGroup
	var failures atomic.Int64

	for i := 0; i < userflowHighConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					failures.Add(1)
				}
			}()
			count := o.ChallengeCount()
			if count != userflowExpectedCount {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(0), failures.Load(),
		"all %d ChallengeCount calls must succeed",
		userflowHighConcurrency,
	)
}

// flowFunc wraps a flow constructor for table-driven
// concurrent testing.
type flowFunc struct {
	name string
	fn   func()
}

// allFlowFunctions returns all 22 user flow constructor
// functions packaged for concurrent invocation.
func allFlowFunctions() []flowFunc {
	return []flowFunc{
		{"HealthCheckFlow", func() {
			userflow.HealthCheckFlow()
		}},
		{"ProviderDiscoveryFlow", func() {
			userflow.ProviderDiscoveryFlow("")
		}},
		{"ChatCompletionFlow", func() {
			userflow.ChatCompletionFlow()
		}},
		{"StreamingCompletionFlow", func() {
			userflow.StreamingCompletionFlow()
		}},
		{"EmbeddingsFlow", func() {
			userflow.EmbeddingsFlow()
		}},
		{"FormattersFlow", func() {
			userflow.FormattersFlow()
		}},
		{"DebateFlow", func() {
			userflow.DebateFlow()
		}},
		{"MonitoringFlow", func() {
			userflow.MonitoringFlow()
		}},
		{"MCPProtocolFlow", func() {
			userflow.MCPProtocolFlow()
		}},
		{"RAGFlow", func() {
			userflow.RAGFlow()
		}},
		{"FeatureFlagsFlow", func() {
			userflow.FeatureFlagsFlow()
		}},
		{"FullSystemFlow", func() {
			userflow.FullSystemFlow()
		}},
		{"AuthenticationFlow", func() {
			userflow.AuthenticationFlow()
		}},
		{"ErrorHandlingFlow", func() {
			userflow.ErrorHandlingFlow()
		}},
		{"ConcurrentUsersFlow", func() {
			userflow.ConcurrentUsersFlow()
		}},
		{"MultiTurnConversationFlow", func() {
			userflow.MultiTurnConversationFlow()
		}},
		{"ToolCallingFlow", func() {
			userflow.ToolCallingFlow()
		}},
		{"ProviderFailoverFlow", func() {
			userflow.ProviderFailoverFlow()
		}},
		{"WebSocketStreamingFlow", func() {
			userflow.WebSocketStreamingFlow()
		}},
		{"GRPCServiceFlow", func() {
			userflow.GRPCServiceFlow()
		}},
		{"RateLimitingFlow", func() {
			userflow.RateLimitingFlow()
		}},
		{"PaginationFlow", func() {
			userflow.PaginationFlow()
		}},
	}
}

// TestFlowConstruction_ConcurrentAllFlows verifies that
// all 22 flow constructors can be invoked concurrently from
// multiple goroutines without panics or data races.
func TestFlowConstruction_ConcurrentAllFlows(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	flows := allFlowFunctions()
	require.Len(t, flows, userflowExpectedCount,
		"must test all %d flow functions",
		userflowExpectedCount,
	)

	const goroutinesPerFlow = 20
	var wg sync.WaitGroup
	var panics atomic.Int64

	for _, ff := range flows {
		for g := 0; g < goroutinesPerFlow; g++ {
			wg.Add(1)
			go func(f flowFunc) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panics.Add(1)
					}
				}()
				f.fn()
			}(ff)
		}
	}
	wg.Wait()

	assert.Equal(t, int64(0), panics.Load(),
		"no flow constructor should panic under "+
			"concurrent invocation")
}

// TestFlowConstruction_RapidRepeated stresses each flow
// constructor by calling it rapidly in a tight loop to
// detect allocation issues or hidden state mutations.
func TestFlowConstruction_RapidRepeated(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	const iterations = 500
	flows := allFlowFunctions()

	for _, ff := range flows {
		t.Run(ff.name, func(t *testing.T) {
			for i := 0; i < iterations; i++ {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf(
								"%s panicked at "+
									"iteration %d: %v",
								ff.name, i, r,
							)
						}
					}()
					ff.fn()
				}()
			}
		})
	}
}

// TestOrchestrator_Sequential_MemoryPressure creates and
// discards 500 orchestrators in sequence to verify there
// are no memory leaks or resource exhaustion.
func TestOrchestrator_Sequential_MemoryPressure(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	for i := 0; i < userflowSequentialCount; i++ {
		url := fmt.Sprintf(
			"http://localhost:%d", 7061+(i%100),
		)
		o, oErr := userflow.NewOrchestrator(url)
		require.NoError(t, oErr,
			"orchestrator %d creation failed", i)
		require.NotNil(t, o,
			"orchestrator %d must not be nil", i)
		assert.Equal(t, userflowExpectedCount,
			o.ChallengeCount(),
			"orchestrator %d challenge count", i)
		// Discard reference to allow GC.
		_ = o
	}

	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	// After creating and discarding 500 orchestrators,
	// heap growth should stay reasonable. Allow up to
	// 256 MB of growth (generous for 500 iterations).
	const maxGrowthBytes = 256 * 1024 * 1024
	growth := int64(0)
	if memAfter.HeapAlloc > memBefore.HeapAlloc {
		growth = int64(
			memAfter.HeapAlloc - memBefore.HeapAlloc,
		)
	}
	assert.Less(t, growth, int64(maxGrowthBytes),
		"heap growth after %d orchestrators should "+
			"stay under %d MB, got %d MB",
		userflowSequentialCount,
		maxGrowthBytes/(1024*1024),
		growth/(1024*1024),
	)

	t.Logf(
		"memory: before=%d MB, after=%d MB, "+
			"growth=%d MB",
		memBefore.HeapAlloc/(1024*1024),
		memAfter.HeapAlloc/(1024*1024),
		growth/(1024*1024),
	)
}

// TestOrchestrator_Summary_ConcurrentAccess verifies that
// multiple goroutines can call Summary() on a shared
// orchestrator without panics or data races.
func TestOrchestrator_Summary_ConcurrentAccess(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	o, err := userflow.NewOrchestrator(userflowBaseURL)
	require.NoError(t, err)
	require.NotNil(t, o)

	const goroutines = 200
	var wg sync.WaitGroup
	var panics atomic.Int64
	var failures atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
				}
			}()
			s := o.Summary()
			if len(s) == 0 {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(0), panics.Load(),
		"Summary() must not panic under "+
			"concurrent access")
	assert.Equal(t, int64(0), failures.Load(),
		"Summary() must return non-empty string")
}

// TestOrchestrator_Challenges_ConcurrentAccess verifies
// that multiple goroutines can call Challenges() on a
// shared orchestrator without panics or data races.
func TestOrchestrator_Challenges_ConcurrentAccess(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	o, err := userflow.NewOrchestrator(userflowBaseURL)
	require.NoError(t, err)
	require.NotNil(t, o)

	const goroutines = 200
	var wg sync.WaitGroup
	var panics atomic.Int64
	var failures atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
				}
			}()
			cs := o.Challenges()
			if len(cs) != userflowExpectedCount {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(0), panics.Load(),
		"Challenges() must not panic")
	assert.Equal(t, int64(0), failures.Load(),
		"Challenges() must return %d items",
		userflowExpectedCount,
	)
}

// TestOrchestrator_MixedConcurrentAccess exercises
// ListChallenges, ChallengeCount, Summary, and Challenges
// simultaneously on a shared orchestrator to detect races
// between different read paths.
func TestOrchestrator_MixedConcurrentAccess(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	o, err := userflow.NewOrchestrator(userflowBaseURL)
	require.NoError(t, err)
	require.NotNil(t, o)

	const perMethod = 100
	var wg sync.WaitGroup
	var panics atomic.Int64

	// ListChallenges goroutines.
	for i := 0; i < perMethod; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
				}
			}()
			_ = o.ListChallenges()
		}()
	}

	// ChallengeCount goroutines.
	for i := 0; i < perMethod; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
				}
			}()
			_ = o.ChallengeCount()
		}()
	}

	// Summary goroutines.
	for i := 0; i < perMethod; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
				}
			}()
			_ = o.Summary()
		}()
	}

	// Challenges goroutines.
	for i := 0; i < perMethod; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
				}
			}()
			_ = o.Challenges()
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(0), panics.Load(),
		"mixed concurrent access must not cause panics")
}

// TestOrchestrator_ConcurrentCreation_UniqueURLs verifies
// that each orchestrator created concurrently maintains its
// own base URL and independent challenge set.
func TestOrchestrator_ConcurrentCreation_UniqueURLs(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	const count = 50
	type result struct {
		summary string
		count   int
		ids     []string
	}

	var wg sync.WaitGroup
	results := make([]result, count)

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			url := fmt.Sprintf(
				"http://host-%d.local:7061", idx,
			)
			o, oErr := userflow.NewOrchestrator(url)
			if oErr != nil {
				return
			}
			results[idx] = result{
				summary: o.Summary(),
				count:   o.ChallengeCount(),
				ids:     o.ListChallenges(),
			}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		expected := fmt.Sprintf(
			"host-%d.local:7061", i,
		)
		assert.Contains(t, r.summary, expected,
			"orchestrator %d summary must contain "+
				"its URL", i)
		assert.Equal(t, userflowExpectedCount, r.count,
			"orchestrator %d challenge count", i)
		assert.Len(t, r.ids, userflowExpectedCount,
			"orchestrator %d challenge ID count", i)
	}
}

// validateAPISteps checks that every step in the slice has
// a non-empty name, method, and path.
func validateAPISteps(steps []uf.APIStep) bool {
	for _, s := range steps {
		if s.Name == "" || s.Method == "" ||
			s.Path == "" {
			return false
		}
	}
	return true
}

// flowValidator wraps a flow constructor that returns step
// count and structural validity.
type flowValidator struct {
	name string
	fn   func() (int, bool)
}

// allFlowValidators returns all 22 flow functions with
// structural validation of the returned steps.
func allFlowValidators() []flowValidator {
	return []flowValidator{
		{"HealthCheckFlow", func() (int, bool) {
			f := userflow.HealthCheckFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"ProviderDiscoveryFlow", func() (int, bool) {
			f := userflow.ProviderDiscoveryFlow("")
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"ChatCompletionFlow", func() (int, bool) {
			f := userflow.ChatCompletionFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"StreamingCompletionFlow", func() (int, bool) {
			f := userflow.StreamingCompletionFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"EmbeddingsFlow", func() (int, bool) {
			f := userflow.EmbeddingsFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"FormattersFlow", func() (int, bool) {
			f := userflow.FormattersFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"DebateFlow", func() (int, bool) {
			f := userflow.DebateFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"MonitoringFlow", func() (int, bool) {
			f := userflow.MonitoringFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"MCPProtocolFlow", func() (int, bool) {
			f := userflow.MCPProtocolFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"RAGFlow", func() (int, bool) {
			f := userflow.RAGFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"FeatureFlagsFlow", func() (int, bool) {
			f := userflow.FeatureFlagsFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"FullSystemFlow", func() (int, bool) {
			f := userflow.FullSystemFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"AuthenticationFlow", func() (int, bool) {
			f := userflow.AuthenticationFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"ErrorHandlingFlow", func() (int, bool) {
			f := userflow.ErrorHandlingFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"ConcurrentUsersFlow", func() (int, bool) {
			f := userflow.ConcurrentUsersFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"MultiTurnConversationFlow",
			func() (int, bool) {
				f := userflow.MultiTurnConversationFlow()
				return len(f.Steps),
					validateAPISteps(f.Steps)
			}},
		{"ToolCallingFlow", func() (int, bool) {
			f := userflow.ToolCallingFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"ProviderFailoverFlow", func() (int, bool) {
			f := userflow.ProviderFailoverFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"WebSocketStreamingFlow",
			func() (int, bool) {
				f := userflow.WebSocketStreamingFlow()
				return len(f.Steps),
					validateAPISteps(f.Steps)
			}},
		{"GRPCServiceFlow", func() (int, bool) {
			f := userflow.GRPCServiceFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"RateLimitingFlow", func() (int, bool) {
			f := userflow.RateLimitingFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
		{"PaginationFlow", func() (int, bool) {
			f := userflow.PaginationFlow()
			return len(f.Steps),
				validateAPISteps(f.Steps)
		}},
	}
}

// TestFlowConstruction_StepIntegrity verifies that flow
// constructors return structurally valid flows even under
// concurrent invocation (steps have names, methods, paths).
func TestFlowConstruction_StepIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")  // SKIP-OK: #legacy-untriaged
	}
	runtime.GOMAXPROCS(2)

	const iterations = 100
	validators := allFlowValidators()
	require.Len(t, validators, userflowExpectedCount,
		"validator count must match flow count")

	var wg sync.WaitGroup
	var invalids atomic.Int64

	for _, v := range validators {
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func(vf flowValidator) {
				defer wg.Done()
				count, valid := vf.fn()
				if !valid || count == 0 {
					invalids.Add(1)
				}
			}(v)
		}
	}
	wg.Wait()

	assert.Equal(t, int64(0), invalids.Load(),
		"all flows must produce valid steps under "+
			"concurrent invocation")
}
