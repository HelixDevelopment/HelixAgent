// Package gptcache provides semantic caching for LLM queries to reduce redundant API calls.
package gptcache

import (
	"math"
)

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical direction,
// 0 means orthogonal, and -1 means opposite direction.
func CosineSimilarity(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) || len(vec1) == 0 {
		return 0
	}

	var dot, norm1, norm2 float64
	for i := range vec1 {
		dot += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}

	norm1 = math.Sqrt(norm1)
	norm2 = math.Sqrt(norm2)

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dot / (norm1 * norm2)
}

// EuclideanDistance computes the Euclidean distance between two vectors.
// Returns the L2 norm of the difference between the vectors.
func EuclideanDistance(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range vec1 {
		diff := vec1[i] - vec2[i]
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

// NormalizeL2 performs L2 normalization on a vector.
// Returns a new vector with unit length (L2 norm = 1).
func NormalizeL2(vec []float64) []float64 {
	if len(vec) == 0 {
		return vec
	}

	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return vec
	}

	result := make([]float64, len(vec))
	for i, v := range vec {
		result[i] = v / norm
	}
	return result
}

// DotProduct computes the dot product of two vectors.
func DotProduct(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) {
		return 0
	}

	var sum float64
	for i := range vec1 {
		sum += vec1[i] * vec2[i]
	}
	return sum
}

// ManhattanDistance computes the Manhattan (L1) distance between two vectors.
func ManhattanDistance(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range vec1 {
		sum += math.Abs(vec1[i] - vec2[i])
	}
	return sum
}

// SimilarityMetric defines the type of similarity metric to use.
type SimilarityMetric string

const (
	// MetricCosine uses cosine similarity (higher is more similar).
	MetricCosine SimilarityMetric = "cosine"
	// MetricEuclidean uses Euclidean distance (lower is more similar).
	MetricEuclidean SimilarityMetric = "euclidean"
	// MetricDotProduct uses dot product (higher is more similar, requires normalized vectors).
	MetricDotProduct SimilarityMetric = "dot_product"
	// MetricManhattan uses Manhattan distance (lower is more similar).
	MetricManhattan SimilarityMetric = "manhattan"
)

// ComputeSimilarity computes similarity based on the specified metric.
// For distance metrics, it converts to similarity score in [0, 1] range.
func ComputeSimilarity(vec1, vec2 []float64, metric SimilarityMetric) float64 {
	switch metric {
	case MetricCosine:
		// Cosine similarity is already in [-1, 1], normalize to [0, 1]
		return (CosineSimilarity(vec1, vec2) + 1) / 2
	case MetricEuclidean:
		// Convert distance to similarity: 1 / (1 + distance)
		dist := EuclideanDistance(vec1, vec2)
		return 1 / (1 + dist)
	case MetricDotProduct:
		// Dot product for normalized vectors is equivalent to cosine
		return (DotProduct(vec1, vec2) + 1) / 2
	case MetricManhattan:
		// Convert distance to similarity: 1 / (1 + distance)
		dist := ManhattanDistance(vec1, vec2)
		return 1 / (1 + dist)
	default:
		return CosineSimilarity(vec1, vec2)
	}
}

// FindMostSimilar finds the most similar vector in a collection.
// Returns the index and similarity score. Returns -1 if collection is empty.
func FindMostSimilar(query []float64, collection [][]float64, metric SimilarityMetric) (int, float64) {
	if len(collection) == 0 {
		return -1, 0
	}

	bestIdx := -1
	bestScore := -math.MaxFloat64

	for i, vec := range collection {
		score := ComputeSimilarity(query, vec, metric)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return bestIdx, bestScore
}

// FindTopK finds the top K most similar vectors in a collection.
// Returns indices and scores sorted by similarity (highest first).
func FindTopK(query []float64, collection [][]float64, metric SimilarityMetric, k int) ([]int, []float64) {
	if len(collection) == 0 || k <= 0 {
		return nil, nil
	}

	type scored struct {
		idx   int
		score float64
	}

	scores := make([]scored, len(collection))
	for i, vec := range collection {
		scores[i] = scored{idx: i, score: ComputeSimilarity(query, vec, metric)}
	}

	// Simple selection sort for top K (efficient for small K)
	for i := 0; i < k && i < len(scores); i++ {
		maxIdx := i
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[maxIdx].score {
				maxIdx = j
			}
		}
		scores[i], scores[maxIdx] = scores[maxIdx], scores[i]
	}

	resultK := k
	if resultK > len(scores) {
		resultK = len(scores)
	}

	indices := make([]int, resultK)
	resultScores := make([]float64, resultK)
	for i := 0; i < resultK; i++ {
		indices[i] = scores[i].idx
		resultScores[i] = scores[i].score
	}

	return indices, resultScores
}
