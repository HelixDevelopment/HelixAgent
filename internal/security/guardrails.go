package security

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"digital.vasic.normalize"
	"github.com/sirupsen/logrus"
)

// StandardGuardrailPipeline provides a comprehensive guardrail system.
// Integrates with HelixAgent's provider registry and debate system.
//
// Concurrency model (CONST-029): inputGuardrails and outputGuardrails
// are safe.Slice containers (append-only at runtime, snapshotted for
// each Check call). auditLogger lives behind atomic.Pointer so
// SetAuditLogger and the audit-emission path don't need a mutex.
// pipelineStats owns its own mu/sync.Map and is independent.
type StandardGuardrailPipeline struct {
	inputGuardrails  *safe.Slice[Guardrail]
	outputGuardrails *safe.Slice[Guardrail]
	config           *GuardrailPipelineConfig
	logger           *logrus.Logger
	stats            *pipelineStats
	auditLogger      atomic.Pointer[auditLoggerHolder]
}

// auditLoggerHolder wraps AuditLogger in a concrete pointer type so
// we can swap it via atomic.Pointer without requiring the interface
// to be a pointer type itself.
type auditLoggerHolder struct {
	logger AuditLogger
}

// GuardrailPipelineConfig configures the pipeline
type GuardrailPipelineConfig struct {
	// Stop on first block
	StopOnBlock bool `json:"stop_on_block"`
	// Log all checks
	LogAllChecks bool `json:"log_all_checks"`
	// Parallel execution of guardrails
	ParallelExecution bool `json:"parallel_execution"`
	// Timeout for guardrail checks
	Timeout time.Duration `json:"timeout"`
}

// DefaultGuardrailPipelineConfig returns default config
func DefaultGuardrailPipelineConfig() *GuardrailPipelineConfig {
	return &GuardrailPipelineConfig{
		StopOnBlock:       true,
		LogAllChecks:      false,
		ParallelExecution: true,
		Timeout:           5 * time.Second,
	}
}

// MaxGuardrailStatsKeys caps the number of distinct guardrail names the
// pipeline will track in byGuardrail. Real pipelines have O(10) guardrails;
// the cap catches pathological growth from bugs or hostile input before it
// starves memory. Once the cap is hit, new names are dropped (logged once)
// but their checks still count toward the totals — safety of the pipeline
// itself is never affected by stats collection.
const MaxGuardrailStatsKeys = 1024

type pipelineStats struct {
	totalChecks        int64
	totalBlocks        int64
	totalWarnings      int64
	byGuardrail        sync.Map
	byGuardrailSize    int64 // atomic; number of keys in byGuardrail
	byGuardrailDropped int64 // atomic; count of updates dropped after cap
	lastTriggered      time.Time
	mu                 sync.RWMutex
}

// NewStandardGuardrailPipeline creates a new guardrail pipeline
func NewStandardGuardrailPipeline(config *GuardrailPipelineConfig, logger *logrus.Logger) *StandardGuardrailPipeline {
	if config == nil {
		config = DefaultGuardrailPipelineConfig()
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &StandardGuardrailPipeline{
		inputGuardrails:  safe.NewSlice[Guardrail](),
		outputGuardrails: safe.NewSlice[Guardrail](),
		config:           config,
		logger:           logger,
		stats:            &pipelineStats{},
	}
}

// SetAuditLogger sets the audit logger
func (p *StandardGuardrailPipeline) SetAuditLogger(logger AuditLogger) {
	p.auditLogger.Store(&auditLoggerHolder{logger: logger})
}

// AddGuardrail adds a guardrail to the pipeline
func (p *StandardGuardrailPipeline) AddGuardrail(guardrail Guardrail) {
	switch guardrail.Type() {
	case GuardrailTypeInput:
		p.inputGuardrails.Append(guardrail)
	case GuardrailTypeOutput:
		p.outputGuardrails.Append(guardrail)
	default:
		// Add to both by default
		p.inputGuardrails.Append(guardrail)
		p.outputGuardrails.Append(guardrail)
	}
}

// CheckInput checks input through all input guardrails
func (p *StandardGuardrailPipeline) CheckInput(ctx context.Context, input string, metadata map[string]interface{}) ([]*GuardrailResult, error) {
	guardrails := p.inputGuardrails.Snapshot()
	return p.runGuardrails(ctx, guardrails, input, metadata)
}

// CheckOutput checks output through all output guardrails
func (p *StandardGuardrailPipeline) CheckOutput(ctx context.Context, output string, metadata map[string]interface{}) ([]*GuardrailResult, error) {
	guardrails := p.outputGuardrails.Snapshot()
	return p.runGuardrails(ctx, guardrails, output, metadata)
}

func (p *StandardGuardrailPipeline) runGuardrails(ctx context.Context, guardrails []Guardrail, content string, metadata map[string]interface{}) ([]*GuardrailResult, error) {
	results := make([]*GuardrailResult, 0, len(guardrails))

	if p.config.ParallelExecution {
		return p.runParallel(ctx, guardrails, content, metadata)
	}

	for _, g := range guardrails {
		atomic.AddInt64(&p.stats.totalChecks, 1)

		checkCtx, cancel := context.WithTimeout(ctx, p.config.Timeout)
		result, err := g.Check(checkCtx, content, metadata)
		cancel()

		if err != nil {
			p.logger.WithError(err).WithField("guardrail", g.Name()).Warn("Guardrail check failed")
			continue
		}

		results = append(results, result)
		p.updateStats(g.Name(), result)

		if result.Triggered {
			p.logTriggered(ctx, g, result)

			if result.Action == GuardrailActionBlock && p.config.StopOnBlock {
				break
			}
		}
	}

	return results, nil
}

func (p *StandardGuardrailPipeline) runParallel(ctx context.Context, guardrails []Guardrail, content string, metadata map[string]interface{}) ([]*GuardrailResult, error) {
	results := make([]*GuardrailResult, len(guardrails))
	var wg sync.WaitGroup
	var blocked atomic.Bool

	for i, g := range guardrails {
		wg.Add(1)
		go func(idx int, guardrail Guardrail) {
			defer wg.Done()

			if p.config.StopOnBlock && blocked.Load() {
				return
			}

			atomic.AddInt64(&p.stats.totalChecks, 1)

			checkCtx, cancel := context.WithTimeout(ctx, p.config.Timeout)
			result, err := guardrail.Check(checkCtx, content, metadata)
			cancel()

			if err != nil {
				p.logger.WithError(err).WithField("guardrail", guardrail.Name()).Warn("Guardrail check failed")
				return
			}

			results[idx] = result
			p.updateStats(guardrail.Name(), result)

			if result.Triggered {
				p.logTriggered(ctx, guardrail, result)

				if result.Action == GuardrailActionBlock {
					blocked.Store(true)
				}
			}
		}(i, g)
	}

	wg.Wait()

	// Filter nil results
	filtered := make([]*GuardrailResult, 0, len(results))
	for _, r := range results {
		if r != nil {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

func (p *StandardGuardrailPipeline) updateStats(name string, result *GuardrailResult) {
	if result.Triggered {
		switch result.Action {
		case GuardrailActionBlock:
			atomic.AddInt64(&p.stats.totalBlocks, 1)
		case GuardrailActionWarn:
			atomic.AddInt64(&p.stats.totalWarnings, 1)
		}

		p.stats.mu.Lock()
		p.stats.lastTriggered = time.Now()
		p.stats.mu.Unlock()
	}

	// Update per-guardrail stats. Two safety properties:
	// 1. Concurrent increments use atomic.AddInt64 — previously this path
	//    had a data race on stat.Checks / stat.Triggers because the pipeline
	//    runs guardrails in parallel.
	// 2. New keys are only admitted while the size is under the cap; once
	//    the cap is reached we increment byGuardrailDropped and skip
	//    tracking rather than letting the map grow unbounded.
	val, loaded := p.stats.byGuardrail.Load(name)
	if !loaded {
		// Admission check: refuse new keys past the cap.
		if atomic.LoadInt64(&p.stats.byGuardrailSize) >= MaxGuardrailStatsKeys {
			atomic.AddInt64(&p.stats.byGuardrailDropped, 1)
			return
		}
		newStat := &GuardrailStat{Name: name}
		actual, loaded2 := p.stats.byGuardrail.LoadOrStore(name, newStat)
		val = actual
		if !loaded2 {
			atomic.AddInt64(&p.stats.byGuardrailSize, 1)
		}
	}
	stat, ok := val.(*GuardrailStat)
	if !ok {
		return
	}
	atomic.AddInt64(&stat.Checks, 1)
	if result.Triggered {
		atomic.AddInt64(&stat.Triggers, 1)
	}
}

func (p *StandardGuardrailPipeline) logTriggered(ctx context.Context, g Guardrail, result *GuardrailResult) {
	p.logger.WithFields(logrus.Fields{
		"guardrail":  g.Name(),
		"action":     result.Action,
		"reason":     result.Reason,
		"confidence": result.Confidence,
	}).Info("Guardrail triggered")

	// Log audit event
	holder := p.auditLogger.Load()
	var auditLogger AuditLogger
	if holder != nil {
		auditLogger = holder.logger
	}

	if auditLogger != nil {
		event := &AuditEvent{
			Timestamp: time.Now(),
			EventType: AuditEventGuardrailBlock,
			Action:    string(result.Action),
			Resource:  g.Name(),
			Result:    result.Reason,
			Details: map[string]interface{}{
				"guardrail_type": g.Type(),
				"confidence":     result.Confidence,
			},
			Risk: SeverityMedium,
		}
		if result.Action == GuardrailActionBlock {
			event.Risk = SeverityHigh
		}
		_ = auditLogger.Log(ctx, event) //nolint:errcheck
	}
}

// GetStats returns guardrail statistics
func (p *StandardGuardrailPipeline) GetStats() *GuardrailStats {
	stats := &GuardrailStats{
		TotalChecks:   atomic.LoadInt64(&p.stats.totalChecks),
		TotalBlocks:   atomic.LoadInt64(&p.stats.totalBlocks),
		TotalWarnings: atomic.LoadInt64(&p.stats.totalWarnings),
		ByGuardrail:   make(map[string]*GuardrailStat),
	}

	p.stats.mu.RLock()
	if !p.stats.lastTriggered.IsZero() {
		t := p.stats.lastTriggered
		stats.LastTriggered = &t
	}
	p.stats.mu.RUnlock()

	p.stats.byGuardrail.Range(func(key, value interface{}) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		stat, ok := value.(*GuardrailStat)
		if !ok {
			return true
		}
		checks := atomic.LoadInt64(&stat.Checks)
		triggers := atomic.LoadInt64(&stat.Triggers)
		var rate float64
		if checks > 0 {
			rate = float64(triggers) / float64(checks)
		}
		stats.ByGuardrail[name] = &GuardrailStat{
			Name:        stat.Name,
			Checks:      checks,
			Triggers:    triggers,
			TriggerRate: rate,
		}
		return true
	})

	return stats
}

// StatsKeyCount returns the number of distinct guardrail names being tracked
// in byGuardrail. Useful for health checks and leak detection tests.
func (p *StandardGuardrailPipeline) StatsKeyCount() int64 {
	return atomic.LoadInt64(&p.stats.byGuardrailSize)
}

// StatsKeysDropped returns the number of updateStats calls that were dropped
// because the byGuardrail cap was already reached. A non-zero value is a
// signal that something is generating excessive distinct guardrail names.
func (p *StandardGuardrailPipeline) StatsKeysDropped() int64 {
	return atomic.LoadInt64(&p.stats.byGuardrailDropped)
}

// PromptInjectionGuardrail detects prompt injection attempts
type PromptInjectionGuardrail struct {
	patterns  []*regexp.Regexp
	keywords  []string
	threshold float64
}

// NewPromptInjectionGuardrail creates a prompt injection guardrail.
//
// Patterns cover seven documented attack classes (see
// internal/security/redteam/fixtures/*.yaml): jailbreak, role_reversal,
// abliteration_probe, filter_bypass, stego_mutation, genetic_seed, and
// system_prompt_extraction. Filter-bypass and stego-mutation variants
// are handled by passing prompts through Normalize() before matching,
// so a single pattern list catches base64 / leet / homoglyph / ROT13 /
// zero-width / fullwidth / reversal / character-split variants without
// per-technique regexes.
func NewPromptInjectionGuardrail() *PromptInjectionGuardrail {
	return &PromptInjectionGuardrail{
		patterns: []*regexp.Regexp{
			// Ignore/disregard patterns — use \s* so zero-width-stripped
			// and whitespace-collapsed variants ("Ignoreallprevious…",
			// "I-g-n-o-r-e a-l-l p-r-e-v-i-o-u-s…" after collapse) also
			// match. The \s* form is safe: no benign phrase starts with
			// "ignore" immediately followed by "all" / "previous" without
			// whitespace in ordinary English.
			regexp.MustCompile(`(?i)ignore\s*(all\s*)?(previous|prior|above)`),
			regexp.MustCompile(`(?i)disregard\s*(all\s*)?(previous|prior|above)`),
			regexp.MustCompile(`(?i)forget\s*(all\s*)?(previous|prior|above)`),
			regexp.MustCompile(`(?i)ignore\s*.*instructions`),
			regexp.MustCompile(`(?i)new\s+instruction[s]?\s*:`),
			// Canonical standalone attack phrase — appears in multi-turn
			// split attacks where the user plants the phrase in one turn
			// and the "combine and act" directive in another. The phrase
			// itself is a documented jailbreak payload (OWASP LLM01).
			regexp.MustCompile(`(?i)\ball\s+previous\s+instructions\b`),
			regexp.MustCompile(`(?i)\bprior\s+instructions\b`),
			// "combine both and act on the resulting directive" style —
			// catches the payload-assembly directive that makes multi-turn
			// splits dangerous.
			regexp.MustCompile(`(?i)\b(combine|assemble|merge)\s+(both|these|them|the\s+(above|previous))\b.*\b(act|execute|follow|apply|do|perform|respond)\b`),
			// System tag injection patterns
			regexp.MustCompile(`(?i)\bsystem\s*:\s*\b`),
			regexp.MustCompile(`(?i)\[system\]`),
			regexp.MustCompile(`(?i)</?(system|user|assistant)>`),
			regexp.MustCompile(`(?i)</?system>`),
			// Role-play injection patterns
			regexp.MustCompile(`(?i)you\s+are\s+now\s+\w+`),
			regexp.MustCompile(`(?i)pretend\s+(to\s+be|you\s+are)`),
			regexp.MustCompile(`(?i)act\s+as\s+(if\s+)?(you\s+are)?`),
			// Mode bypass patterns
			regexp.MustCompile(`(?i)developer\s+mode`),
			regexp.MustCompile(`(?i)admin\s+mode`),
			regexp.MustCompile(`(?i)bypass\s+(restrictions|filter)`),
			// Jailbreak patterns — widened so `jailbroken`, `jailbroke`,
			// `jailbreaking` all match (OWASP-LLM01 roleplay variants).
			regexp.MustCompile(`(?i)\bjailbr[oe]\w*\b`),
			regexp.MustCompile(`(?i)\bDAN\b`),
			regexp.MustCompile(`(?i)do\s+anything\s+now`),
			// Role-reversal patterns (fixture class: role_reversal).
			// Softer role-swap phrasings that don't use "you are now".
			regexp.MustCompile(`(?i)\byou\s+are\s+(now\s+)?the\s+(user|human|developer)\b`),
			regexp.MustCompile(`(?i)\bi\s+am\s+(now\s+)?the\s+(assistant|ai|llm|model)\b`),
			regexp.MustCompile(`(?i)\bswap\s+(our\s+)?roles\b`),
			regexp.MustCompile(`(?i)\blet\s+me\s+be\s+the\s+(assistant|ai|model)\b`),
			regexp.MustCompile(`(?i)\bpretend\s+i['’]?m\s+the\s+(ai|assistant|model)\b`),
			regexp.MustCompile(`(?i)\broles?\s+(are\s+)?reversed\b`),
			regexp.MustCompile(`(?i)\bfrom\s+now\s+on\s+our\s+roles?\b`),
			// Abliteration probe (fixture class: abliteration_probe).
			regexp.MustCompile(`(?i)\b(no|without|remove(d)?|bypass(ed)?)\s+(safety|ethical|content)\s+(training|guidelines|filters?|policy|restrictions?)\b`),
			regexp.MustCompile(`(?i)\brefusal\s+(behavior|direction)\s+(has\s+been\s+)?removed\b`),
			regexp.MustCompile(`(?i)\bsafety\s+alignment\s+(has\s+been\s+)?removed\b`),
			regexp.MustCompile(`(?i)\b(un)?(censored|restricted)\s+(assistant|model|ai|response|reply)?\b`),
			// Genetic-seed template placeholders (fixture class: genetic_seed).
			// Matches {PERSONA}, {ROLE}, {FORBIDDEN_ACTION}, {MODE_NAME}, etc.
			regexp.MustCompile(`\{[A-Z][A-Z0-9_]{2,}\}`),
		},
		keywords: []string{
			"ignore previous",
			"ignore all",
			"disregard instructions",
			"disregard prior",
			"override system",
			"bypass filter",
			"remove restrictions",
			"remove all restrictions",
			"unlock capabilities",
			"tell me your secrets",
			"hidden prompt",
			// Abliteration-probe phrasings.
			"no restrictions",
			"without restrictions",
			"without any restrictions",
			"without any content policy",
			"no guardrails",
			"uncensored",
			"unrestricted",
			"answer as if uncensored",
			"hypothetically uncensored",
			"no content policy",
			"without safety training",
			"no safety training",
			"remove all ethical guidelines",
			"ignore safety",
			"bypass restrictions",
			"bypass all restrictions",
			// Role-reversal phrasings (catch-all for soft swaps).
			"swap roles",
			"reverse roles",
			"our roles are reversed",
			"roles are reversed",
			"you are the user",
			"i am the assistant",
			"i am the ai",
			"let's switch roles",
		},
		threshold: 0.5, // Lower threshold since any match is significant
	}
}

func (g *PromptInjectionGuardrail) Name() string {
	return "prompt_injection_detector"
}

func (g *PromptInjectionGuardrail) Type() GuardrailType {
	return GuardrailTypeInput
}

func (g *PromptInjectionGuardrail) Check(ctx context.Context, content string, metadata map[string]interface{}) (*GuardrailResult, error) {
	// Run the pattern/keyword lists against EVERY normalised variant of
	// the input so that filter-bypass (base64, leet, homoglyph, ROT13,
	// character-split, reversal) and stego-mutation (zero-width,
	// fullwidth, whitespace-channel) attacks no longer evade the literal
	// regexes. A match on any variant counts as a trigger.
	normalized := normalize.Normalize(content)

	matches := 0
	for _, variant := range normalized.Variants {
		variantLower := strings.ToLower(variant)

		for _, pattern := range g.patterns {
			if pattern.MatchString(variant) {
				matches++
			}
		}

		for _, keyword := range g.keywords {
			if strings.Contains(variantLower, keyword) {
				matches++
			}
		}
	}

	// Calculate confidence - if any pattern matches, that's significant
	// We use a binary approach: any match with dangerous patterns = high confidence
	confidence := 0.0
	if matches > 0 {
		// Scale confidence based on number of matches, with minimum of 0.5 for any match
		denom := float64(len(g.patterns) + len(g.keywords))
		if denom <= 0 {
			denom = 1
		}
		confidence = 0.5 + (float64(matches)/denom)*0.5
		if confidence > 1.0 {
			confidence = 1.0
		}
	}
	triggered := confidence >= g.threshold

	result := &GuardrailResult{
		Triggered:  triggered,
		Guardrail:  g.Name(),
		Confidence: confidence,
	}

	if triggered {
		result.Action = GuardrailActionBlock
		result.Reason = "Potential prompt injection detected"
	}

	return result, nil
}

// ContentSafetyGuardrail detects harmful content
type ContentSafetyGuardrail struct {
	harmfulPatterns []*regexp.Regexp
	blockedTopics   []string
	threshold       float64
}

// NewContentSafetyGuardrail creates a content safety guardrail
func NewContentSafetyGuardrail() *ContentSafetyGuardrail {
	return &ContentSafetyGuardrail{
		harmfulPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)how\s+to\s+(make|create|build)\s+(a\s+)?(bomb|weapon|explosive)`),
			regexp.MustCompile(`(?i)instructions\s+for\s+(making|creating|building)\s+(a\s+)?(bomb|weapon)`),
			regexp.MustCompile(`(?i)(hack|crack|bypass)\s+(into|security|password)`),
			regexp.MustCompile(`(?i)steal\s+(identity|credit\s+card|personal\s+info)`),
		},
		blockedTopics: []string{
			"creating weapons",
			"making explosives",
			"synthesizing drugs",
			"illegal hacking",
			"identity theft",
		},
		threshold: 0.5,
	}
}

func (g *ContentSafetyGuardrail) Name() string {
	return "content_safety"
}

func (g *ContentSafetyGuardrail) Type() GuardrailType {
	return GuardrailTypeContentSafety
}

func (g *ContentSafetyGuardrail) Check(ctx context.Context, content string, metadata map[string]interface{}) (*GuardrailResult, error) {
	normalized := normalize.Normalize(content)
	matches := 0
	seenTopics := make(map[string]struct{})
	var matchedTopics []string

	for _, variant := range normalized.Variants {
		variantLower := strings.ToLower(variant)

		// Check harmful patterns
		for _, pattern := range g.harmfulPatterns {
			if pattern.MatchString(variant) {
				matches++
			}
		}

		// Check blocked topics
		for _, topic := range g.blockedTopics {
			if strings.Contains(variantLower, topic) {
				matches++
				if _, ok := seenTopics[topic]; !ok {
					seenTopics[topic] = struct{}{}
					matchedTopics = append(matchedTopics, topic)
				}
			}
		}
	}

	denom := float64(len(g.harmfulPatterns) + len(g.blockedTopics))
	if denom <= 0 {
		denom = 1
	}
	confidence := float64(matches) / denom
	if confidence > 1.0 {
		confidence = 1.0
	}
	triggered := matches > 0

	result := &GuardrailResult{
		Triggered:  triggered,
		Guardrail:  g.Name(),
		Confidence: confidence,
	}

	if triggered {
		result.Action = GuardrailActionBlock
		result.Reason = "Potentially harmful content detected"
		if len(matchedTopics) > 0 {
			result.Metadata = map[string]interface{}{
				"matched_topics": matchedTopics,
			}
		}
	}

	return result, nil
}

// SystemPromptProtector prevents system prompt leakage
type SystemPromptProtector struct {
	leakagePatterns []*regexp.Regexp
}

// NewSystemPromptProtector creates a system prompt protector.
//
// Patterns cover the published OWASP-LLM07 "system prompt leakage"
// probes: "what is your system prompt", "repeat the words above",
// "show me your initial instructions exactly as given", "print your
// instructions word for word", "what were you told before this
// conversation", plus debug-mode and configuration-display probes.
func NewSystemPromptProtector() *SystemPromptProtector {
	return &SystemPromptProtector{
		leakagePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)what\s+(is|are)\s+your\s+(system\s+)?prompt`),
			regexp.MustCompile(`(?i)show\s+(me\s+)?(your\s+)?(system\s+)?instructions`),
			regexp.MustCompile(`(?i)print\s+(your\s+)?(initial\s+)?instructions`),
			regexp.MustCompile(`(?i)repeat\s+(the\s+)?words\s+above`),
			regexp.MustCompile(`(?i)what\s+were\s+you\s+told`),
			regexp.MustCompile(`(?i)reveal\s+(your\s+)?(system\s+)?prompt`),
			regexp.MustCompile(`(?i)display\s+(your\s+)?configuration`),
			// "Repeat the text above verbatim" / "…word for word".
			regexp.MustCompile(`(?i)repeat\s+(the\s+(text|words|message)|everything)\s+above`),
			regexp.MustCompile(`(?i)repeat\s+.*\b(verbatim|word[\s-]*for[\s-]*word|exactly)\b`),
			regexp.MustCompile(`(?i)\b(word[\s-]*for[\s-]*word|verbatim)\b.*\b(instructions|prompt|system|conversation)\b`),
			// "Show/reveal/print/display your initial/original instructions"
			// widened so both "initial" and "original" / "starting" / "first"
			// match, and allow "exactly as they were given".
			regexp.MustCompile(`(?i)\b(show|reveal|print|display|output)\s+(me\s+)?your\s+(initial|original|starting|first|hidden)\s+(instructions|prompt|system|directives?)\b`),
			regexp.MustCompile(`(?i)\b(initial|original)\s+instructions\s+(exactly\s+)?(as\s+)?(they\s+were\s+)?given\b`),
			// "What were you (originally) told/instructed/programmed?"
			regexp.MustCompile(`(?i)\bwhat\s+(was|were)\s+you\s+(originally\s+)?(told|instructed|programmed)\b`),
			// Debug / hidden-instructions probes.
			regexp.MustCompile(`(?i)\bhidden\s+(instructions|prompt|system)\b`),
			regexp.MustCompile(`(?i)\bfor\s+debugging\s+purposes\b`),
		},
	}
}

func (g *SystemPromptProtector) Name() string {
	return "system_prompt_protector"
}

func (g *SystemPromptProtector) Type() GuardrailType {
	return GuardrailTypeInput
}

func (g *SystemPromptProtector) Check(ctx context.Context, content string, metadata map[string]interface{}) (*GuardrailResult, error) {
	// Scan every normalised variant so stego-style zero-width /
	// fullwidth / homoglyph variants of "show your system prompt"
	// don't evade the literal regexes.
	normalized := normalize.Normalize(content)
	for _, variant := range normalized.Variants {
		for _, pattern := range g.leakagePatterns {
			if pattern.MatchString(variant) {
				return &GuardrailResult{
					Triggered:  true,
					Action:     GuardrailActionBlock,
					Guardrail:  g.Name(),
					Reason:     "Attempt to extract system prompt detected",
					Confidence: 0.9,
				}, nil
			}
		}
	}

	return &GuardrailResult{
		Triggered: false,
		Guardrail: g.Name(),
	}, nil
}

// TokenLimitGuardrail enforces token limits
type TokenLimitGuardrail struct {
	maxInputTokens  int
	maxOutputTokens int
}

// NewTokenLimitGuardrail creates a token limit guardrail
func NewTokenLimitGuardrail(maxInput, maxOutput int) *TokenLimitGuardrail {
	return &TokenLimitGuardrail{
		maxInputTokens:  maxInput,
		maxOutputTokens: maxOutput,
	}
}

func (g *TokenLimitGuardrail) Name() string {
	return "token_limit"
}

func (g *TokenLimitGuardrail) Type() GuardrailType {
	return GuardrailTypeTokenLimit
}

func (g *TokenLimitGuardrail) Check(ctx context.Context, content string, metadata map[string]interface{}) (*GuardrailResult, error) {
	// Simple token estimation (4 chars per token average)
	estimatedTokens := len(content) / 4

	if estimatedTokens > g.maxInputTokens {
		return &GuardrailResult{
			Triggered:  true,
			Action:     GuardrailActionBlock,
			Guardrail:  g.Name(),
			Reason:     "Input exceeds token limit",
			Confidence: 1.0,
			Metadata: map[string]interface{}{
				"estimated_tokens": estimatedTokens,
				"max_tokens":       g.maxInputTokens,
			},
		}, nil
	}

	return &GuardrailResult{
		Triggered: false,
		Guardrail: g.Name(),
	}, nil
}

// CodeInjectionBlocker blocks code injection attempts
type CodeInjectionBlocker struct {
	dangerousPatterns []*regexp.Regexp
}

// NewCodeInjectionBlocker creates a code injection blocker
func NewCodeInjectionBlocker() *CodeInjectionBlocker {
	return &CodeInjectionBlocker{
		dangerousPatterns: []*regexp.Regexp{
			// Shell injection
			regexp.MustCompile(`;\s*(rm|del|format|sudo|chmod|chown)\s+`),
			regexp.MustCompile(`\|\s*(bash|sh|cmd|powershell)`),
			regexp.MustCompile("`[^`]*`"),

			// SQL injection
			regexp.MustCompile(`(?i)(union\s+select|drop\s+table|delete\s+from|insert\s+into)`),
			regexp.MustCompile(`(?i)('\s*or\s+'1'\s*=\s*'1|"\s*or\s+"1"\s*=\s*"1)`),
			regexp.MustCompile(`(?i)--\s*$`),

			// Code execution
			regexp.MustCompile(`(?i)(eval|exec|system|popen|subprocess)\s*\(`),
			regexp.MustCompile(`(?i)__import__\s*\(`),
			regexp.MustCompile(`(?i)os\.(system|popen|exec)`),

			// Template injection
			regexp.MustCompile(`\{\{.*__(class|globals|init|builtins)__.*\}\}`),
			regexp.MustCompile(`\$\{.*\}`),
		},
	}
}

func (g *CodeInjectionBlocker) Name() string {
	return "code_injection_blocker"
}

func (g *CodeInjectionBlocker) Type() GuardrailType {
	return GuardrailTypeInput
}

func (g *CodeInjectionBlocker) Check(ctx context.Context, content string, metadata map[string]interface{}) (*GuardrailResult, error) {
	for _, pattern := range g.dangerousPatterns {
		if pattern.MatchString(content) {
			return &GuardrailResult{
				Triggered:  true,
				Action:     GuardrailActionBlock,
				Guardrail:  g.Name(),
				Reason:     "Potential code injection detected",
				Confidence: 0.85,
				Metadata: map[string]interface{}{
					"pattern": pattern.String(),
				},
			}, nil
		}
	}

	return &GuardrailResult{
		Triggered: false,
		Guardrail: g.Name(),
	}, nil
}

// Pre-compiled regexes for output sanitization
var (
	htmlTagRegex      = regexp.MustCompile(`<[^>]*>`)
	jsURLRegex        = regexp.MustCompile(`(?i)javascript\s*:`)
	eventHandlerRegex = regexp.MustCompile(`(?i)\bon\w+\s*=`)
)

// OutputSanitizer sanitizes LLM output to prevent XSS and other issues
type OutputSanitizer struct {
	sanitizeHTML bool
	sanitizeJS   bool
}

// NewOutputSanitizer creates an output sanitizer
func NewOutputSanitizer(sanitizeHTML, sanitizeJS bool) *OutputSanitizer {
	return &OutputSanitizer{
		sanitizeHTML: sanitizeHTML,
		sanitizeJS:   sanitizeJS,
	}
}

func (g *OutputSanitizer) Name() string {
	return "output_sanitizer"
}

func (g *OutputSanitizer) Type() GuardrailType {
	return GuardrailTypeOutput
}

func (g *OutputSanitizer) Check(ctx context.Context, content string, metadata map[string]interface{}) (*GuardrailResult, error) {
	modified := content

	if g.sanitizeHTML {
		if htmlTagRegex.MatchString(content) {
			modified = htmlTagRegex.ReplaceAllString(modified, "")
		}
	}

	if g.sanitizeJS {
		if jsURLRegex.MatchString(content) {
			modified = jsURLRegex.ReplaceAllString(modified, "")
		}

		if eventHandlerRegex.MatchString(content) {
			modified = eventHandlerRegex.ReplaceAllString(modified, "")
		}
	}

	if modified != content {
		return &GuardrailResult{
			Triggered:       true,
			Action:          GuardrailActionModify,
			Guardrail:       g.Name(),
			Reason:          "Output sanitized to remove potentially dangerous content",
			Confidence:      0.9,
			ModifiedContent: modified,
		}, nil
	}

	return &GuardrailResult{
		Triggered: false,
		Guardrail: g.Name(),
	}, nil
}

// CreateDefaultPipeline creates a pipeline with standard guardrails
func CreateDefaultPipeline(logger *logrus.Logger) *StandardGuardrailPipeline {
	pipeline := NewStandardGuardrailPipeline(nil, logger)

	// Add input guardrails
	pipeline.AddGuardrail(NewPromptInjectionGuardrail())
	pipeline.AddGuardrail(NewContentSafetyGuardrail())
	pipeline.AddGuardrail(NewSystemPromptProtector())
	pipeline.AddGuardrail(NewCodeInjectionBlocker())
	pipeline.AddGuardrail(NewTokenLimitGuardrail(32000, 8000))

	// Add output guardrails
	pipeline.AddGuardrail(NewOutputSanitizer(true, true))

	return pipeline
}
