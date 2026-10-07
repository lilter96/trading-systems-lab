package news

import (
	"math"
	"sync"
	"time"
)

type Scorer interface {
	Score(texts []string) ([]float64, error)
}

var (
	DefaultHalfLife   = 60.0
	DefaultTierWeights = map[SourceTier]float64{
		SourceTierTier1Wire:        1.0,
		SourceTierTier2Established: 0.6,
		SourceTierTier3Social:      0.25,
	}
)

type scoredCluster struct {
	clusterID string
	score     float64
	tier      SourceTier
	createdAt time.Time
}

type SentimentScorer struct {
	mu           sync.Mutex
	clusters     map[string][]scoredCluster
	halfLifeMin  float64
	tierWeights  map[SourceTier]float64
	volumeWindow float64
}

func NewSentimentScorer(halfLifeMinutes float64, tierWeights map[SourceTier]float64) *SentimentScorer {
	if tierWeights == nil {
		tierWeights = DefaultTierWeights
	}
	if halfLifeMinutes <= 0 {
		halfLifeMinutes = DefaultHalfLife
	}
	return &SentimentScorer{
		clusters:     make(map[string][]scoredCluster),
		halfLifeMin:  halfLifeMinutes,
		tierWeights:  tierWeights,
		volumeWindow: halfLifeMinutes * 5,
	}
}

func (s *SentimentScorer) Ingest(instrument, clusterID string, tier SourceTier, score float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusters[instrument] = append(s.clusters[instrument], scoredCluster{
		clusterID: clusterID,
		score:     score,
		tier:      tier,
		createdAt: time.Now(),
	})
}

func (s *SentimentScorer) Aggregate(instrument string) (score float64, volume int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	clusters, ok := s.clusters[instrument]
	if !ok {
		return 0, 0
	}

	now := time.Now()
	var num, den float64

	for i := 0; i < len(clusters); {
		age := now.Sub(clusters[i].createdAt).Minutes()
		if age < 0 {
			age = 0
		}

		tierWeight := s.tierWeights[clusters[i].tier]
		decay := math.Exp(-age / s.halfLifeMin)

		if age <= s.volumeWindow {
			volume++
		}

		if decay > 0 {
			num += clusters[i].score * tierWeight * decay
			den += tierWeight * decay
		}

		if age > s.volumeWindow {
			last := len(clusters) - 1
			clusters[i] = clusters[last]
			clusters[last] = scoredCluster{}
			clusters = clusters[:last]
		} else {
			i++
		}
	}

	if len(clusters) == 0 {
		delete(s.clusters, instrument)
	} else {
		s.clusters[instrument] = clusters
	}

	if den == 0 {
		return 0, volume
	}
	return num / den, volume
}
