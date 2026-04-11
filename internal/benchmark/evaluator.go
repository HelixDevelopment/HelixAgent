// Package benchmark provides evaluation framework for agent performance
// Inspired by OpenHands and SWE-agent evaluation frameworks
package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// Task represents a single evaluation task
type Task struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`       // code, test, reasoning, multi-step
	Difficulty  string                 `json:"difficulty"` // easy, medium, hard
	Input       map[string]interface{} `json:"input"`
	Expected    interface{}            `json:"expected"`
	Timeout     time.Duration          `json:"timeout"`
	Tags        []string               `json:"tags"`
}

// Result represents the result of evaluating a task
type Result struct {
	TaskID    string             `json:"task_id"`
	Success   bool               `json:"success"`
	Score     float64            `json:"score"` // 0.0 - 1.0
	Output    interface{}        `json:"output,omitempty"`
	Error     string             `json:"error,omitempty"`
	Duration  time.Duration      `json:"duration"`
	Metrics   map[string]float64 `json:"metrics,omitempty"`
	Timestamp time.Time          `json:"timestamp"`
}

// Evaluator evaluates agent performance on tasks
type Evaluator struct {
	tasks      []Task
	results    []Result
	logger     *logrus.Logger
	outputPath string
}

// NewEvaluator creates a new evaluator
func NewEvaluator(logger *logrus.Logger) *Evaluator {
	if logger == nil {
		logger = logrus.New()
	}
	return &Evaluator{
		tasks:      make([]Task, 0),
		results:    make([]Result, 0),
		logger:     logger,
		outputPath: "",
	}
}

// WithOutput sets the output path for results
func (e *Evaluator) WithOutput(path string) *Evaluator {
	e.outputPath = path
	return e
}

// AddTask adds a task to the evaluation
func (e *Evaluator) AddTask(task Task) {
	if task.Timeout == 0 {
		task.Timeout = 5 * time.Minute
	}
	e.tasks = append(e.tasks, task)
}

// LoadTasksFromFile loads tasks from a JSON file
func (e *Evaluator) LoadTasksFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read tasks file: %w", err)
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return fmt.Errorf("unmarshal tasks: %w", err)
	}

	for _, task := range tasks {
		e.AddTask(task)
	}

	e.logger.WithField("count", len(tasks)).Info("Loaded tasks from file")
	return nil
}

// SaveResults saves results to a JSON file
func (e *Evaluator) SaveResults(path string) error {
	data, err := json.MarshalIndent(e.results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write results: %w", err)
	}

	return nil
}

// Run executes all tasks and collects results
func (e *Evaluator) Run(ctx context.Context, agent Agent) (*Summary, error) {
	if len(e.tasks) == 0 {
		return nil, fmt.Errorf("no tasks to evaluate")
	}

	e.logger.WithField("count", len(e.tasks)).Info("Starting evaluation")

	for _, task := range e.tasks {
		result := e.evaluateTask(ctx, agent, task)
		e.results = append(e.results, result)
	}

	summary := e.generateSummary()

	if e.outputPath != "" {
		if err := e.SaveResults(e.outputPath); err != nil {
			e.logger.WithError(err).Warn("Failed to save results")
		}
	}

	return summary, nil
}

// RunParallel executes tasks in parallel
func (e *Evaluator) RunParallel(ctx context.Context, agent Agent, concurrency int) (*Summary, error) {
	if len(e.tasks) == 0 {
		return nil, fmt.Errorf("no tasks to evaluate")
	}

	if concurrency <= 0 {
		concurrency = 4
	}

	e.logger.WithFields(logrus.Fields{
		"count":       len(e.tasks),
		"concurrency": concurrency,
	}).Info("Starting parallel evaluation")

	// Create task queue
	taskChan := make(chan Task, len(e.tasks))
	resultChan := make(chan Result, len(e.tasks))

	for _, task := range e.tasks {
		taskChan <- task
	}
	close(taskChan)

	// Start workers
	for i := 0; i < concurrency; i++ {
		go func() {
			for task := range taskChan {
				result := e.evaluateTask(ctx, agent, task)
				resultChan <- result
			}
		}()
	}

	// Collect results
	for i := 0; i < len(e.tasks); i++ {
		result := <-resultChan
		e.results = append(e.results, result)
	}

	summary := e.generateSummary()

	if e.outputPath != "" {
		if err := e.SaveResults(e.outputPath); err != nil {
			e.logger.WithError(err).Warn("Failed to save results")
		}
	}

	return summary, nil
}

// evaluateTask evaluates a single task
func (e *Evaluator) evaluateTask(ctx context.Context, agent Agent, task Task) Result {
	start := time.Now()

	// Apply timeout
	taskCtx, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()

	result := Result{
		TaskID:    task.ID,
		Timestamp: start,
	}

	// Execute task
	output, err := agent.Execute(taskCtx, task)
	result.Duration = time.Since(start)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.Score = 0.0
	} else {
		// Compare output with expected
		result.Output = output
		result.Score = e.compareOutput(output, task.Expected)
		result.Success = result.Score >= 0.8 // 80% threshold
	}

	// Collect additional metrics
	result.Metrics = agent.Metrics()

	e.logger.WithFields(logrus.Fields{
		"task":     task.ID,
		"success":  result.Success,
		"score":    result.Score,
		"duration": result.Duration,
	}).Debug("Task evaluated")

	return result
}

// compareOutput compares actual output with expected
func (e *Evaluator) compareOutput(actual, expected interface{}) float64 {
	if actual == nil && expected == nil {
		return 1.0
	}
	if actual == nil || expected == nil {
		return 0.0
	}

	// Try exact match first
	if actual == expected {
		return 1.0
	}

	// Try string comparison
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	if actualStr == expectedStr {
		return 1.0
	}

	// Calculate similarity (Jaccard-like for strings)
	return stringSimilarity(actualStr, expectedStr)
}

// stringSimilarity calculates string similarity (0.0 - 1.0)
func stringSimilarity(a, b string) float64 {
	if a == "" && b == "" {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	// Simple word-based Jaccard similarity
	wordsA := tokenize(a)
	wordsB := tokenize(b)

	intersection := 0
	for word := range wordsA {
		if wordsB[word] {
			intersection++
		}
	}

	union := len(wordsA) + len(wordsB) - intersection
	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// tokenize splits string into word set
func tokenize(s string) map[string]bool {
	words := make(map[string]bool)
	start := 0
	for i, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '.' {
			if i > start {
				word := s[start:i]
				if len(word) > 0 {
					words[word] = true
				}
			}
			start = i + 1
		}
	}
	if start < len(s) {
		words[s[start:]] = true
	}
	return words
}

// generateSummary generates evaluation summary
func (e *Evaluator) generateSummary() *Summary {
	if len(e.results) == 0 {
		return &Summary{}
	}

	summary := &Summary{
		TotalTasks:   len(e.results),
		ByType:       make(map[string]int),
		ByDifficulty: make(map[string]int),
	}

	var totalScore float64
	var totalDuration time.Duration

	for _, result := range e.results {
		if result.Success {
			summary.Passed++
		} else {
			summary.Failed++
		}

		totalScore += result.Score
		totalDuration += result.Duration

		// Find task info
		for _, task := range e.tasks {
			if task.ID == result.TaskID {
				summary.ByType[task.Type]++
				summary.ByDifficulty[task.Difficulty]++
				break
			}
		}
	}

	summary.AverageScore = totalScore / float64(len(e.results))
	summary.AverageDuration = totalDuration / time.Duration(len(e.results))
	summary.SuccessRate = float64(summary.Passed) / float64(summary.TotalTasks)

	return summary
}

// GetResults returns all results
func (e *Evaluator) GetResults() []Result {
	return e.results
}

// Clear clears all tasks and results
func (e *Evaluator) Clear() {
	e.tasks = e.tasks[:0]
	e.results = e.results[:0]
}

// Agent interface for evaluation
type Agent interface {
	Execute(ctx context.Context, task Task) (interface{}, error)
	Metrics() map[string]float64
}

// Summary represents evaluation summary
type Summary struct {
	TotalTasks      int            `json:"total_tasks"`
	Passed          int            `json:"passed"`
	Failed          int            `json:"failed"`
	SuccessRate     float64        `json:"success_rate"`
	AverageScore    float64        `json:"average_score"`
	AverageDuration time.Duration  `json:"average_duration"`
	ByType          map[string]int `json:"by_type"`
	ByDifficulty    map[string]int `json:"by_difficulty"`
}

// String returns formatted summary
func (s *Summary) String() string {
	return fmt.Sprintf(
		"Evaluation Summary:\n"+
			"  Total Tasks: %d\n"+
			"  Passed: %d\n"+
			"  Failed: %d\n"+
			"  Success Rate: %.2f%%\n"+
			"  Average Score: %.2f\n"+
			"  Average Duration: %v",
		s.TotalTasks,
		s.Passed,
		s.Failed,
		s.SuccessRate*100,
		s.AverageScore,
		s.AverageDuration,
	)
}

// LoadTasksFromDir loads all task files from a directory
func (e *Evaluator) LoadTasksFromDir(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := filepath.Ext(file.Name())
		if ext != ".json" {
			continue
		}

		path := filepath.Join(dir, file.Name())
		if err := e.LoadTasksFromFile(path); err != nil {
			e.logger.WithError(err).WithField("file", file.Name()).Warn("Failed to load tasks")
		}
	}

	return nil
}

// FilterTasks filters tasks by tags
func (e *Evaluator) FilterTasks(tags ...string) []Task {
	if len(tags) == 0 {
		return e.tasks
	}

	var filtered []Task
	for _, task := range e.tasks {
		if hasAnyTag(task.Tags, tags) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// hasAnyTag checks if task has any of the specified tags
func hasAnyTag(taskTags, filterTags []string) bool {
	tagSet := make(map[string]bool)
	for _, tag := range taskTags {
		tagSet[tag] = true
	}

	for _, tag := range filterTags {
		if tagSet[tag] {
			return true
		}
	}
	return false
}

// GetTasksByType returns tasks of a specific type
func (e *Evaluator) GetTasksByType(taskType string) []Task {
	var filtered []Task
	for _, task := range e.tasks {
		if task.Type == taskType {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// GetTasksByDifficulty returns tasks of a specific difficulty
func (e *Evaluator) GetTasksByDifficulty(difficulty string) []Task {
	var filtered []Task
	for _, task := range e.tasks {
		if task.Difficulty == difficulty {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// Compare compares two evaluation results
func (e *Evaluator) Compare(other *Evaluator) *Comparison {
	comparison := &Comparison{
		This:  e.generateSummary(),
		Other: other.generateSummary(),
	}

	// Calculate improvements
	if comparison.Other.AverageScore > 0 {
		comparison.ScoreImprovement = comparison.This.AverageScore - comparison.Other.AverageScore
	}
	if comparison.Other.SuccessRate > 0 {
		comparison.SuccessRateImprovement = comparison.This.SuccessRate - comparison.Other.SuccessRate
	}

	return comparison
}

// Comparison represents comparison between two evaluations
type Comparison struct {
	This                   *Summary `json:"this"`
	Other                  *Summary `json:"other"`
	ScoreImprovement       float64  `json:"score_improvement"`
	SuccessRateImprovement float64  `json:"success_rate_improvement"`
}
