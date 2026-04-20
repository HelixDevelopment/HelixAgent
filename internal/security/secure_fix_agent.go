// Package security provides autonomous security hardening capabilities
// including vulnerability detection, automated fix generation, and validation.
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// VulnerabilitySeverity represents vulnerability severity
type VulnerabilitySeverity string

const (
	VulnSeverityCritical VulnerabilitySeverity = "critical"
	VulnSeverityHigh     VulnerabilitySeverity = "high"
	VulnSeverityMedium   VulnerabilitySeverity = "medium"
	VulnSeverityLow      VulnerabilitySeverity = "low"
	VulnSeverityInfo     VulnerabilitySeverity = "info"
)

// VulnerabilityCategory represents the category of vulnerability
type VulnerabilityCategory string

const (
	CategoryInjection        VulnerabilityCategory = "injection"
	CategoryXSS              VulnerabilityCategory = "xss"
	CategoryAuthentication   VulnerabilityCategory = "authentication"
	CategoryAuthorization    VulnerabilityCategory = "authorization"
	CategoryCryptographic    VulnerabilityCategory = "cryptographic"
	CategorySensitiveData    VulnerabilityCategory = "sensitive_data"
	CategoryMisconfiguration VulnerabilityCategory = "misconfiguration"
	CategoryDependency       VulnerabilityCategory = "dependency"
	CategoryMemorySafety     VulnerabilityCategory = "memory_safety"
	CategoryRaceCondition    VulnerabilityCategory = "race_condition"
)

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string                `json:"id"`
	Category    VulnerabilityCategory `json:"category"`
	Severity    VulnerabilitySeverity `json:"severity"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	File        string                `json:"file"`
	Line        int                   `json:"line"`
	Column      int                   `json:"column,omitempty"`
	Code        string                `json:"code,omitempty"`
	CWE         string                `json:"cwe,omitempty"`
	CVSS        float64               `json:"cvss,omitempty"`
	Remediation string                `json:"remediation,omitempty"`
	References  []string              `json:"references,omitempty"`
	DetectedAt  time.Time             `json:"detected_at"`
}

// SecurityFix represents a security fix
type SecurityFix struct {
	VulnerabilityID string    `json:"vulnerability_id"`
	OriginalCode    string    `json:"original_code"`
	FixedCode       string    `json:"fixed_code"`
	Explanation     string    `json:"explanation"`
	Validated       bool      `json:"validated"`
	AppliedAt       time.Time `json:"applied_at,omitempty"`
}

// SecurityScanResult represents the result of a security scan
type SecurityScanResult struct {
	Vulnerabilities []*Vulnerability `json:"vulnerabilities"`
	TotalFiles      int              `json:"total_files"`
	ScannedFiles    int              `json:"scanned_files"`
	Duration        time.Duration    `json:"duration"`
	Scanner         string           `json:"scanner"`
}

// MarshalJSON implements custom JSON marshaling
func (r *SecurityScanResult) MarshalJSON() ([]byte, error) {
	type Alias SecurityScanResult
	return json.Marshal(&struct {
		*Alias
		DurationMs int64 `json:"duration_ms"`
	}{
		Alias:      (*Alias)(r),
		DurationMs: r.Duration.Milliseconds(),
	})
}

// SecureFixAgentConfig holds configuration
type SecureFixAgentConfig struct {
	// EnableAutoFix enables automatic fix generation
	EnableAutoFix bool `json:"enable_auto_fix"`
	// RequireValidation requires validation before applying fixes
	RequireValidation bool `json:"require_validation"`
	// MaxConcurrentScans is the max concurrent scans
	MaxConcurrentScans int `json:"max_concurrent_scans"`
	// SeverityThreshold is the minimum severity to report
	SeverityThreshold VulnerabilitySeverity `json:"severity_threshold"`
	// EnableDependencyScanning enables dependency vulnerability scanning
	EnableDependencyScanning bool `json:"enable_dependency_scanning"`
	// Timeout for operations
	Timeout time.Duration `json:"timeout"`
}

// DefaultSecureFixAgentConfig returns default configuration
func DefaultSecureFixAgentConfig() SecureFixAgentConfig {
	return SecureFixAgentConfig{
		EnableAutoFix:            true,
		RequireValidation:        true,
		MaxConcurrentScans:       4,
		SeverityThreshold:        VulnSeverityLow,
		EnableDependencyScanning: true,
		Timeout:                  10 * time.Minute,
	}
}

// VulnerabilityScanner scans code for vulnerabilities
type VulnerabilityScanner interface {
	// Scan scans code for vulnerabilities
	Scan(ctx context.Context, code string, language string) ([]*Vulnerability, error)
	// ScanFile scans a file for vulnerabilities
	ScanFile(ctx context.Context, path string) ([]*Vulnerability, error)
	// Name returns the scanner name
	Name() string
}

// FixGenerator generates fixes for vulnerabilities
type FixGenerator interface {
	// GenerateFix generates a fix for a vulnerability
	GenerateFix(ctx context.Context, vuln *Vulnerability, code string) (*SecurityFix, error)
}

// FixValidator validates security fixes
type FixValidator interface {
	// ValidateFix validates that a fix resolves the vulnerability
	ValidateFix(ctx context.Context, vuln *Vulnerability, fix *SecurityFix) (bool, error)
}

// SecureFixAgent implements autonomous security hardening.
//
// Concurrency model (CONST-029): scanners is a safe.Slice (append-
// only from RegisterScanner, snapshotted for each scan pass).
// vulnerabilities and fixes are safe.Store containers — atomic
// per-key writes during the detect/repair loop. No mutex survives.
type SecureFixAgent struct {
	config          SecureFixAgentConfig
	scanners        *safe.Slice[VulnerabilityScanner]
	generator       FixGenerator
	validator       FixValidator
	vulnerabilities *safe.Store[string, *Vulnerability]
	fixes           *safe.Store[string, *SecurityFix]
	logger          *logrus.Logger
}

// NewSecureFixAgent creates a new SecureFixAgent
func NewSecureFixAgent(config SecureFixAgentConfig, generator FixGenerator, validator FixValidator, logger *logrus.Logger) *SecureFixAgent {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}

	return &SecureFixAgent{
		config:          config,
		scanners:        safe.NewSlice[VulnerabilityScanner](),
		generator:       generator,
		validator:       validator,
		vulnerabilities: safe.NewStore[string, *Vulnerability](),
		fixes:           safe.NewStore[string, *SecurityFix](),
		logger:          logger,
	}
}

// RegisterScanner registers a vulnerability scanner
func (a *SecureFixAgent) RegisterScanner(scanner VulnerabilityScanner) {
	a.scanners.Append(scanner)
}

// DetectRepairValidate implements the full security loop
func (a *SecureFixAgent) DetectRepairValidate(ctx context.Context, code string, language string) (*SecurityResult, error) {
	ctx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()

	startTime := time.Now()

	result := &SecurityResult{
		Vulnerabilities: make([]*Vulnerability, 0),
		Fixes:           make([]*SecurityFix, 0),
	}

	// 1. DETECT - Scan for vulnerabilities
	for _, scanner := range a.scanners.Snapshot() {
		vulns, err := scanner.Scan(ctx, code, language)
		if err != nil {
			a.logger.Warnf("Scanner %s failed: %v", scanner.Name(), err)
			continue
		}

		for _, vuln := range vulns {
			if a.shouldReport(vuln) {
				result.Vulnerabilities = append(result.Vulnerabilities, vuln)
				a.vulnerabilities.Put(vuln.ID, vuln)
			}
		}
	}

	// 2. REPAIR - Generate fixes
	if a.config.EnableAutoFix && a.generator != nil {
		for _, vuln := range result.Vulnerabilities {
			fix, err := a.generator.GenerateFix(ctx, vuln, code)
			if err != nil {
				a.logger.Warnf("Failed to generate fix for %s: %v", vuln.ID, err)
				continue
			}

			// 3. VALIDATE - Validate the fix
			if a.config.RequireValidation && a.validator != nil {
				valid, err := a.validator.ValidateFix(ctx, vuln, fix)
				if err != nil {
					a.logger.Warnf("Fix validation failed for %s: %v", vuln.ID, err)
					continue
				}
				fix.Validated = valid
			} else {
				fix.Validated = true
			}

			if fix.Validated {
				result.Fixes = append(result.Fixes, fix)
				a.fixes.Put(vuln.ID, fix)
			}
		}
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Vulnerabilities) == 0 || len(result.Fixes) == len(result.Vulnerabilities)

	return result, nil
}

// shouldReport checks if a vulnerability should be reported
func (a *SecureFixAgent) shouldReport(vuln *Vulnerability) bool {
	severityOrder := map[VulnerabilitySeverity]int{
		VulnSeverityCritical: 5,
		VulnSeverityHigh:     4,
		VulnSeverityMedium:   3,
		VulnSeverityLow:      2,
		VulnSeverityInfo:     1,
	}

	threshold := severityOrder[a.config.SeverityThreshold]
	vulnSeverity := severityOrder[vuln.Severity]

	return vulnSeverity >= threshold
}

// ApplyFix applies a security fix to code
func (a *SecureFixAgent) ApplyFix(code string, fix *SecurityFix) (string, error) {
	if !fix.Validated {
		return "", fmt.Errorf("fix not validated")
	}

	result := strings.Replace(code, fix.OriginalCode, fix.FixedCode, 1)
	fix.AppliedAt = time.Now()

	return result, nil
}

// GetVulnerability retrieves a vulnerability by ID
func (a *SecureFixAgent) GetVulnerability(id string) (*Vulnerability, bool) {
	return a.vulnerabilities.Get(id)
}

// GetFix retrieves a fix by vulnerability ID
func (a *SecureFixAgent) GetFix(vulnID string) (*SecurityFix, bool) {
	return a.fixes.Get(vulnID)
}

// SecurityResult holds the result of security operations
type SecurityResult struct {
	Vulnerabilities []*Vulnerability `json:"vulnerabilities"`
	Fixes           []*SecurityFix   `json:"fixes"`
	Success         bool             `json:"success"`
	Duration        time.Duration    `json:"duration"`
}

// MarshalJSON implements custom JSON marshaling
func (r *SecurityResult) MarshalJSON() ([]byte, error) {
	type Alias SecurityResult
	return json.Marshal(&struct {
		*Alias
		DurationMs int64 `json:"duration_ms"`
	}{
		Alias:      (*Alias)(r),
		DurationMs: r.Duration.Milliseconds(),
	})
}

// PatternBasedScanner implements pattern-based vulnerability scanning
type PatternBasedScanner struct {
	patterns map[VulnerabilityCategory][]*VulnerabilityPattern
	logger   *logrus.Logger
}

// VulnerabilityPattern defines a vulnerability pattern
type VulnerabilityPattern struct {
	Pattern     *regexp.Regexp
	Category    VulnerabilityCategory
	Severity    VulnerabilitySeverity
	Title       string
	Description string
	CWE         string
	Remediation string
}

// NewPatternBasedScanner creates a new pattern-based scanner
func NewPatternBasedScanner(logger *logrus.Logger) *PatternBasedScanner {
	scanner := &PatternBasedScanner{
		patterns: make(map[VulnerabilityCategory][]*VulnerabilityPattern),
		logger:   logger,
	}

	// Register common vulnerability patterns
	scanner.registerDefaultPatterns()

	return scanner
}

// registerDefaultPatterns registers default vulnerability patterns
func (s *PatternBasedScanner) registerDefaultPatterns() {
	// SQL Injection patterns
	s.patterns[CategoryInjection] = []*VulnerabilityPattern{
		{
			Pattern:     regexp.MustCompile(`(?i)(?:execute|query|exec)\s*\(\s*["'].*\+.*["']`),
			Category:    CategoryInjection,
			Severity:    VulnSeverityCritical,
			Title:       "SQL Injection",
			Description: "String concatenation in SQL query may lead to SQL injection",
			CWE:         "CWE-89",
			Remediation: "Use parameterized queries or prepared statements",
		},
		{
			Pattern:     regexp.MustCompile(`(?i)fmt\.Sprintf\s*\(\s*["'].*(?:SELECT|INSERT|UPDATE|DELETE|WHERE).*%[sv]`),
			Category:    CategoryInjection,
			Severity:    VulnSeverityCritical,
			Title:       "SQL Injection via fmt.Sprintf",
			Description: "Using fmt.Sprintf for SQL queries can lead to SQL injection",
			CWE:         "CWE-89",
			Remediation: "Use database/sql parameterized queries",
		},
	}

	// XSS patterns
	s.patterns[CategoryXSS] = []*VulnerabilityPattern{
		{
			Pattern:     regexp.MustCompile(`(?i)innerHTML\s*=\s*[^"']+`),
			Category:    CategoryXSS,
			Severity:    VulnSeverityHigh,
			Title:       "Cross-Site Scripting (XSS)",
			Description: "Direct assignment to innerHTML may lead to XSS",
			CWE:         "CWE-79",
			Remediation: "Use textContent or sanitize input before setting innerHTML",
		},
		{
			Pattern:     regexp.MustCompile(`(?i)document\.write\s*\(`),
			Category:    CategoryXSS,
			Severity:    VulnSeverityHigh,
			Title:       "DOM-based XSS via document.write",
			Description: "document.write can be exploited for XSS attacks",
			CWE:         "CWE-79",
			Remediation: "Avoid document.write and use DOM manipulation methods",
		},
	}

	// Sensitive Data patterns
	s.patterns[CategorySensitiveData] = []*VulnerabilityPattern{
		{
			Pattern:     regexp.MustCompile(`(?i)(?:password|secret|api_key|apikey|token)\s*[:=]\s*["'][^"']+["']`),
			Category:    CategorySensitiveData,
			Severity:    VulnSeverityHigh,
			Title:       "Hardcoded Credentials",
			Description: "Sensitive credentials should not be hardcoded in source code",
			CWE:         "CWE-798",
			Remediation: "Use environment variables or secure secret management",
		},
	}

	// Cryptographic patterns
	s.patterns[CategoryCryptographic] = []*VulnerabilityPattern{
		{
			Pattern:     regexp.MustCompile(`(?i)(?:md5|sha1)\s*\(`),
			Category:    CategoryCryptographic,
			Severity:    VulnSeverityMedium,
			Title:       "Weak Cryptographic Hash",
			Description: "MD5 and SHA1 are considered cryptographically weak",
			CWE:         "CWE-328",
			Remediation: "Use SHA-256 or stronger hash algorithms",
		},
		{
			Pattern:     regexp.MustCompile(`(?i)(?:DES|3DES|RC4)\b`),
			Category:    CategoryCryptographic,
			Severity:    VulnSeverityHigh,
			Title:       "Weak Encryption Algorithm",
			Description: "DES, 3DES, and RC4 are deprecated encryption algorithms",
			CWE:         "CWE-327",
			Remediation: "Use AES-256 or ChaCha20-Poly1305",
		},
	}

	// Race Condition patterns
	s.patterns[CategoryRaceCondition] = []*VulnerabilityPattern{
		{
			Pattern:     regexp.MustCompile(`(?i)go\s+func\s*\([^)]*\)\s*\{[^}]*(?:map|slice)\[[^]]+\]`),
			Category:    CategoryRaceCondition,
			Severity:    VulnSeverityMedium,
			Title:       "Potential Race Condition",
			Description: "Concurrent access to shared data structure without synchronization",
			CWE:         "CWE-362",
			Remediation: "Use sync.Mutex, sync.RWMutex, or sync.Map for concurrent access",
		},
	}
}

// Name returns the scanner name
func (s *PatternBasedScanner) Name() string {
	return "pattern-based"
}

// Scan scans code for vulnerabilities
func (s *PatternBasedScanner) Scan(ctx context.Context, code string, language string) ([]*Vulnerability, error) {
	vulns := make([]*Vulnerability, 0)

	for category, patterns := range s.patterns {
		for _, pattern := range patterns {
			matches := pattern.Pattern.FindAllStringIndex(code, -1)
			for _, match := range matches {
				line := s.getLineNumber(code, match[0])
				vuln := &Vulnerability{
					ID:          fmt.Sprintf("VULN-%d-%s", time.Now().UnixNano(), category),
					Category:    category,
					Severity:    pattern.Severity,
					Title:       pattern.Title,
					Description: pattern.Description,
					Line:        line,
					Code:        code[match[0]:min(match[1]+50, len(code))],
					CWE:         pattern.CWE,
					Remediation: pattern.Remediation,
					DetectedAt:  time.Now(),
				}
				vulns = append(vulns, vuln)
			}
		}
	}

	return vulns, nil
}

// ScanFile scans a file for vulnerabilities
func (s *PatternBasedScanner) ScanFile(ctx context.Context, path string) ([]*Vulnerability, error) {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Detect language from file extension
	language := detectLanguageFromPath(path)

	// Scan the content
	vulns, err := s.Scan(ctx, string(content), language)
	if err != nil {
		return nil, fmt.Errorf("failed to scan file %s: %w", path, err)
	}

	// Update vulnerability file paths
	for _, vuln := range vulns {
		vuln.File = path
	}

	return vulns, nil
}

// detectLanguageFromPath detects the programming language from file extension
func detectLanguageFromPath(path string) string {
	path = strings.ToLower(path)
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".py"):
		return "python"
	case strings.HasSuffix(path, ".js"):
		return "javascript"
	case strings.HasSuffix(path, ".ts"):
		return "typescript"
	case strings.HasSuffix(path, ".java"):
		return "java"
	case strings.HasSuffix(path, ".rs"):
		return "rust"
	case strings.HasSuffix(path, ".rb"):
		return "ruby"
	case strings.HasSuffix(path, ".php"):
		return "php"
	case strings.HasSuffix(path, ".c") || strings.HasSuffix(path, ".h"):
		return "c"
	case strings.HasSuffix(path, ".cpp") || strings.HasSuffix(path, ".hpp"):
		return "cpp"
	case strings.HasSuffix(path, ".cs"):
		return "csharp"
	case strings.HasSuffix(path, ".swift"):
		return "swift"
	case strings.HasSuffix(path, ".kt"):
		return "kotlin"
	case strings.HasSuffix(path, ".sql"):
		return "sql"
	case strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".bash"):
		return "shell"
	default:
		return "unknown"
	}
}

// getLineNumber gets line number from byte offset
func (s *PatternBasedScanner) getLineNumber(code string, offset int) int {
	line := 1
	for i := 0; i < offset && i < len(code); i++ {
		if code[i] == '\n' {
			line++
		}
	}
	return line
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LLMFixGenerator implements FixGenerator using an LLM
type LLMFixGenerator struct {
	generateFunc func(ctx context.Context, prompt string) (string, error)
	logger       *logrus.Logger
}

// NewLLMFixGenerator creates a new LLM-based fix generator
func NewLLMFixGenerator(generateFunc func(ctx context.Context, prompt string) (string, error), logger *logrus.Logger) *LLMFixGenerator {
	return &LLMFixGenerator{
		generateFunc: generateFunc,
		logger:       logger,
	}
}

// GenerateFix generates a fix for a vulnerability
func (g *LLMFixGenerator) GenerateFix(ctx context.Context, vuln *Vulnerability, code string) (*SecurityFix, error) {
	prompt := fmt.Sprintf(`Security vulnerability detected:
Title: %s
Category: %s
Severity: %s
CWE: %s
Description: %s
Remediation suggestion: %s

Vulnerable code:
%s

Generate a secure fix for this vulnerability. Output ONLY the fixed code, nothing else.`,
		vuln.Title, vuln.Category, vuln.Severity, vuln.CWE, vuln.Description, vuln.Remediation, vuln.Code)

	response, err := g.generateFunc(ctx, prompt)
	if err != nil {
		return nil, err
	}

	fix := &SecurityFix{
		VulnerabilityID: vuln.ID,
		OriginalCode:    vuln.Code,
		FixedCode:       strings.TrimSpace(response),
		Explanation:     fmt.Sprintf("Fixed %s vulnerability", vuln.Title),
	}

	return fix, nil
}

// RescanValidator implements FixValidator by rescanning
type RescanValidator struct {
	scanner VulnerabilityScanner
	logger  *logrus.Logger
}

// NewRescanValidator creates a new rescan-based validator
func NewRescanValidator(scanner VulnerabilityScanner, logger *logrus.Logger) *RescanValidator {
	return &RescanValidator{
		scanner: scanner,
		logger:  logger,
	}
}

// ValidateFix validates that a fix resolves the vulnerability
func (v *RescanValidator) ValidateFix(ctx context.Context, vuln *Vulnerability, fix *SecurityFix) (bool, error) {
	// Scan the fixed code
	vulns, err := v.scanner.Scan(ctx, fix.FixedCode, "")
	if err != nil {
		return false, err
	}

	// Check if original vulnerability is still present
	for _, foundVuln := range vulns {
		if foundVuln.Category == vuln.Category && foundVuln.Title == vuln.Title {
			return false, nil
		}
	}

	return true, nil
}

// FiveRingDefense implements LLMLOOP five-ring defense architecture
type FiveRingDefense struct {
	rings  []DefenseRing
	logger *logrus.Logger
}

// DefenseRing represents a single defense ring
type DefenseRing interface {
	// Check checks input against this defense ring
	Check(ctx context.Context, input string) (bool, string, error)
	// Name returns the ring name
	Name() string
}

// NewFiveRingDefense creates a new five-ring defense
func NewFiveRingDefense(logger *logrus.Logger) *FiveRingDefense {
	return &FiveRingDefense{
		rings:  make([]DefenseRing, 0),
		logger: logger,
	}
}

// AddRing adds a defense ring
func (d *FiveRingDefense) AddRing(ring DefenseRing) {
	d.rings = append(d.rings, ring)
}

// Defend runs input through all defense rings
func (d *FiveRingDefense) Defend(ctx context.Context, input string) (*DefenseResult, error) {
	result := &DefenseResult{
		Passed:      true,
		RingResults: make([]RingResult, 0),
	}

	for _, ring := range d.rings {
		passed, message, err := ring.Check(ctx, input)
		if err != nil {
			d.logger.Warnf("Ring %s failed: %v", ring.Name(), err)
			continue
		}

		ringResult := RingResult{
			Name:    ring.Name(),
			Passed:  passed,
			Message: message,
		}
		result.RingResults = append(result.RingResults, ringResult)

		if !passed {
			result.Passed = false
			result.BlockedBy = ring.Name()
			break
		}
	}

	return result, nil
}

// DefenseResult holds the result of defense check
type DefenseResult struct {
	Passed      bool         `json:"passed"`
	BlockedBy   string       `json:"blocked_by,omitempty"`
	RingResults []RingResult `json:"ring_results"`
}

// RingResult holds the result of a single ring check
type RingResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// InputSanitizationRing implements input sanitization defense
type InputSanitizationRing struct {
	patterns []*regexp.Regexp
}

// NewInputSanitizationRing creates a new input sanitization ring
func NewInputSanitizationRing() *InputSanitizationRing {
	return &InputSanitizationRing{
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)<script[^>]*>`),
			regexp.MustCompile(`(?i)javascript:`),
			regexp.MustCompile(`(?i)on\w+\s*=`),
			regexp.MustCompile(`(?i)(?:union|select|insert|update|delete|drop|truncate)\s+`),
		},
	}
}

// Name returns the ring name
func (r *InputSanitizationRing) Name() string {
	return "input_sanitization"
}

// Check checks input for malicious patterns
func (r *InputSanitizationRing) Check(ctx context.Context, input string) (bool, string, error) {
	for _, pattern := range r.patterns {
		if pattern.MatchString(input) {
			return false, fmt.Sprintf("Malicious pattern detected: %s", pattern.String()), nil
		}
	}
	return true, "Input sanitization passed", nil
}

// RateLimitingRing implements rate limiting defense.
//
// Concurrency model (CONST-029): requests is a *safe.Store whose
// per-key slices are replaced atomically via Update. Each call runs
// the filter-out-expired + append-or-reject step inside the store's
// internal write lock so two concurrent Check calls for the same key
// cannot both skip the cap.
type RateLimitingRing struct {
	requests *safe.Store[string, []time.Time]
	limit    int
	window   time.Duration
}

// NewRateLimitingRing creates a new rate limiting ring
func NewRateLimitingRing(limit int, window time.Duration) *RateLimitingRing {
	return &RateLimitingRing{
		requests: safe.NewStore[string, []time.Time](),
		limit:    limit,
		window:   window,
	}
}

// Name returns the ring name
func (r *RateLimitingRing) Name() string {
	return "rate_limiting"
}

// Check checks if rate limit is exceeded
func (r *RateLimitingRing) Check(ctx context.Context, input string) (bool, string, error) {
	// Use input hash as key (simplified)
	key := fmt.Sprintf("%d", len(input)%100)
	now := time.Now()
	cutoff := now.Add(-r.window)

	var allowed bool
	r.requests.Update(key, func(cur []time.Time, _ bool) ([]time.Time, bool) {
		// Clean old requests.
		valid := make([]time.Time, 0, len(cur))
		for _, t := range cur {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) >= r.limit {
			allowed = false
			return valid, true
		}
		allowed = true
		return append(valid, now), true
	})

	if !allowed {
		return false, "Rate limit exceeded", nil
	}
	return true, "Rate limit check passed", nil
}
