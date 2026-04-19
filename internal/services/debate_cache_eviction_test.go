package services

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDebateService_IntentCache_BoundedSize(t *testing.T) {
	ds := &DebateService{logger: logrus.New()}
	ds.initCaches()

	// Fill beyond max
	for i := 0; i < maxIntentCacheSize+500; i++ {
		ds.intentCache.Put(fmt.Sprintf("topic-%d", i), &IntentClassificationResult{
			Intent: "test",
		})
	}

	// Trigger eviction
	ds.evictIntentCacheIfNeeded()

	size := ds.intentCache.Len()
	assert.LessOrEqual(t, size, maxIntentCacheSize,
		"cache should be bounded to maxIntentCacheSize after eviction")
}

func TestDebateService_IntentCache_NoEvictionUnderLimit(t *testing.T) {
	ds := &DebateService{logger: logrus.New()}
	ds.initCaches()

	for i := 0; i < 100; i++ {
		ds.intentCache.Put(fmt.Sprintf("topic-%d", i), &IntentClassificationResult{
			Intent: "test",
		})
	}

	ds.evictIntentCacheIfNeeded()

	assert.Equal(t, 100, ds.intentCache.Len(), "cache under limit should not be evicted")
}
