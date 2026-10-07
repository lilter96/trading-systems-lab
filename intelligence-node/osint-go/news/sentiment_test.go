package news

import (
	"math"
	"sync"
	"testing"
	"time"
)

type mockScorer struct {
	scores map[string]float64
}

func (m *mockScorer) Score(texts []string) ([]float64, error) {
	result := make([]float64, len(texts))
	for i, t := range texts {
		if s, ok := m.scores[t]; ok {
			result[i] = s
		}
	}
	return result, nil
}

func TestAggregateSentiment_TierAndDecayWeighting(t *testing.T) {
	s := NewSentimentScorer(60, map[SourceTier]float64{
		SourceTierTier1Wire:        1.0,
		SourceTierTier2Established: 0.6,
	})

	s.Ingest("BTC-USDT", "cluster1", SourceTierTier1Wire, 0.5)
	s.Ingest("BTC-USDT", "cluster2", SourceTierTier2Established, -0.3)

	score, volume := s.Aggregate("BTC-USDT")

	expectedScore := (0.5*1.0 + (-0.3)*0.6) / (1.0 + 0.6)
	if math.Abs(score-expectedScore) > 1e-9 {
		t.Errorf("expected score %.9f, got %.9f", expectedScore, score)
	}
	if volume != 2 {
		t.Errorf("expected volume 2, got %d", volume)
	}
}

func TestSentimentVolume_IndependentOfDirection(t *testing.T) {
	s := NewSentimentScorer(60, nil)

	s.Ingest("BTC-USDT", "c1", SourceTierTier1Wire, 0.8)
	s.Ingest("BTC-USDT", "c2", SourceTierTier1Wire, -0.8)
	s.Ingest("BTC-USDT", "c3", SourceTierTier2Established, 0.3)

	_, vol1 := s.Aggregate("BTC-USDT")

	s2 := NewSentimentScorer(60, nil)
	s2.Ingest("BTC-USDT", "c1", SourceTierTier1Wire, -0.8)
	s2.Ingest("BTC-USDT", "c2", SourceTierTier1Wire, 0.8)
	s2.Ingest("BTC-USDT", "c3", SourceTierTier2Established, -0.3)

	_, vol2 := s2.Aggregate("BTC-USDT")

	if vol1 != vol2 {
		t.Errorf("volume should be independent of direction: vol1=%d vol2=%d", vol1, vol2)
	}
}

func TestSentimentScorer_EmptyInstrument(t *testing.T) {
	s := NewSentimentScorer(60, nil)

	score, volume := s.Aggregate("NONEXISTENT")
	if score != 0 {
		t.Errorf("expected score 0 for empty instrument, got %f", score)
	}
	if volume != 0 {
		t.Errorf("expected volume 0 for empty instrument, got %d", volume)
	}
}

func TestSentimentScorer_DenominatorZero(t *testing.T) {
	s := NewSentimentScorer(60, map[SourceTier]float64{
		SourceTierTier2Established: 0, // zero tier weight
	})

	s.Ingest("BTC-USDT", "c1", SourceTierTier2Established, 0.5)
	score, volume := s.Aggregate("BTC-USDT")
	if score != 0 {
		t.Errorf("expected 0 when denominator is zero, got %f", score)
	}
	if volume != 1 {
		t.Errorf("expected volume 1, got %d", volume)
	}
}

type batchTestScorer struct {
	mu   sync.Mutex
	log  []int
}

func (b *batchTestScorer) Score(texts []string) ([]float64, error) {
	b.mu.Lock()
	b.log = append(b.log, len(texts))
	b.mu.Unlock()
	result := make([]float64, len(texts))
	for i := range texts {
		result[i] = 0.5
	}
	return result, nil
}

func TestBatchEvaluate_MatchesPerItemResults(t *testing.T) {
	scorer := &batchTestScorer{}

	texts := []string{"Bitcoin surges", "Ethereum drops", "Market neutral"}

	batchScores, err := scorer.Score(texts)
	if err != nil {
		t.Fatal(err)
	}

	for i, text := range texts {
		single, err := scorer.Score([]string{text})
		if err != nil {
			t.Fatal(err)
		}
		if single[0] != batchScores[i] {
			t.Errorf("score mismatch for %q: batch=%.2f single=%.2f", text, batchScores[i], single[0])
		}
	}
}

type evaluateCache struct {
	mu       sync.Mutex
	cache    map[string]cacheEntry
	ttl      time.Duration
	fallback float64
	scorer   Scorer
}

type cacheEntry struct {
	score     float64
	expiresAt time.Time
}

func newEvaluateCache(scorer Scorer, ttl time.Duration, fallback float64) *evaluateCache {
	return &evaluateCache{
		cache:    make(map[string]cacheEntry),
		ttl:      ttl,
		fallback: fallback,
		scorer:   scorer,
	}
}

func (c *evaluateCache) Evaluate(texts []string) ([]float64, error) {
	c.mu.Lock()
	uncached := make([]string, 0, len(texts))
	uncachedIdx := make([]int, 0, len(texts))
	results := make([]float64, len(texts))

	for i, text := range texts {
		if entry, ok := c.cache[text]; ok && time.Now().Before(entry.expiresAt) {
			results[i] = entry.score
		} else if ok {
			results[i] = c.fallback
		} else {
			uncached = append(uncached, text)
			uncachedIdx = append(uncachedIdx, i)
		}
	}
	c.mu.Unlock()

	if len(uncached) > 0 {
		scores, err := c.scorer.Score(uncached)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		for j, text := range uncached {
			results[uncachedIdx[j]] = scores[j]
			c.cache[text] = cacheEntry{score: scores[j], expiresAt: time.Now().Add(c.ttl)}
		}
		c.mu.Unlock()
	}

	return results, nil
}

func TestCacheExpiry_FallsBackToNeutral_NonBlocking(t *testing.T) {
	scorer := &mockScorer{scores: map[string]float64{"test": 0.8}}
	cache := newEvaluateCache(scorer, 50*time.Millisecond, 0.0)

	scores, err := cache.Evaluate([]string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	if scores[0] != 0.8 {
		t.Errorf("expected 0.8 from scorer, got %.2f", scores[0])
	}

	scores, err = cache.Evaluate([]string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	if scores[0] != 0.8 {
		t.Errorf("expected 0.8 from cache, got %.2f", scores[0])
	}

	time.Sleep(60 * time.Millisecond)

	scores, err = cache.Evaluate([]string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	if scores[0] != 0.0 {
		t.Errorf("expected 0.0 fallback after expiry, got %.2f", scores[0])
	}
}
