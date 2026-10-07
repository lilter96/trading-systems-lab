package news

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

//go:embed validation_set_default.json
var defaultValidationData []byte

type Sample struct {
	Headline string  `json:"headline"`
	Label    float64 `json:"label"`
}

func LoadValidationSet() ([]Sample, error) {
	path := os.Getenv("VALIDATION_SET_PATH")
	if path == "" {
		return parseSamples(defaultValidationData)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading custom validation set: %w", err)
	}
	return parseSamples(data)
}

func parseSamples(data []byte) ([]Sample, error) {
	var samples []Sample
	if err := json.Unmarshal(data, &samples); err != nil {
		return nil, fmt.Errorf("parsing validation set: %w", err)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("validation set is empty")
	}
	return samples, nil
}

func ComputeSpearmanRho(samples []Sample, scorer Scorer) (float64, error) {
	texts := make([]string, len(samples))
	for i, s := range samples {
		texts[i] = s.Headline
	}

	scores, err := scorer.Score(texts)
	if err != nil {
		return 0, fmt.Errorf("scoring validation set: %w", err)
	}

	n := len(samples)
	type pair struct {
		model float64
		label float64
	}
	pairs := make([]pair, n)
	for i := range samples {
		pairs[i] = pair{model: scores[i], label: samples[i].Label}
	}

	modelRanks := rank(pairs, func(p pair) float64 { return p.model })
	labelRanks := rank(pairs, func(p pair) float64 { return p.label })

	var sumSqDiff float64
	for i := 0; i < n; i++ {
		diff := modelRanks[i] - labelRanks[i]
		sumSqDiff += diff * diff
	}

	denom := float64(n) * (float64(n)*float64(n) - 1)
	if denom == 0 {
		return 0, nil
	}
	rho := 1 - (6*sumSqDiff)/denom

	if rho < -1 {
		rho = -1
	}
	if rho > 1 {
		rho = 1
	}
	return rho, nil
}

func rank[T any](items []T, keyFn func(T) float64) []float64 {
	n := len(items)
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}

	sort.Slice(indices, func(i, j int) bool {
		return keyFn(items[indices[i]]) < keyFn(items[indices[j]])
	})

	ranks := make([]float64, n)
	for i := 0; i < n; {
		j := i
		for j < n && keyFn(items[indices[j]]) == keyFn(items[indices[i]]) {
			j++
		}
		avgRank := float64(i+1+j) / 2.0
		for k := i; k < j; k++ {
			ranks[indices[k]] = avgRank
		}
		i = j
	}
	return ranks
}

type ValidationGate struct {
	Threshold float64
}

func NewValidationGate(threshold float64) *ValidationGate {
	return &ValidationGate{Threshold: threshold}
}

func (g *ValidationGate) IsLiveReady(scorer Scorer) (bool, float64, error) {
	samples, err := LoadValidationSet()
	if err != nil {
		return false, 0, fmt.Errorf("loading validation set: %w", err)
	}

	rho, err := ComputeSpearmanRho(samples, scorer)
	if err != nil {
		return false, 0, fmt.Errorf("computing spearman rho: %w", err)
	}

	return rho >= g.Threshold, rho, nil
}

type LexiconScorer struct {
	words map[string]float64
}

func NewLexiconScorer(words map[string]float64) *LexiconScorer {
	lower := make(map[string]float64, len(words))
	for word, score := range words {
		lower[strings.ToLower(word)] = score
	}
	return &LexiconScorer{words: lower}
}

func DefaultLexiconScorer() *LexiconScorer {
	return NewLexiconScorer(map[string]float64{
		"surge":     0.35,
		"surges":    0.35,
		"surged":    0.35,
		"soar":      0.30,
		"soars":     0.30,
		"soared":    0.30,
		"rally":     0.25,
		"rallies":   0.25,
		"rallied":   0.25,
		"bullish":   0.30,
		"breakout":  0.25,
		"break":     0.20,
		"pump":      0.15,
		"adopt":     0.20,
		"adopts":    0.20,
		"adopted":   0.20,
		"adoption":  0.20,
		"approval":  0.25,
		"approve":   0.25,
		"approved":  0.25,
		"positive":  0.15,
		"gain":      0.15,
		"gains":     0.15,
		"upgrade":   0.20,
		"upgrades":  0.20,
		"upgraded":  0.20,
		"launch":    0.10,
		"launches":  0.10,
		"launched":  0.10,
		"partners":  0.15,
		"partnership": 0.15,

		"plunge":    -0.35,
		"plunges":   -0.35,
		"plunged":   -0.35,
		"crash":     -0.40,
		"crashes":   -0.40,
		"crashed":   -0.40,
		"dump":      -0.25,
		"dumps":     -0.25,
		"dumped":    -0.25,
		"bearish":   -0.30,
		"slump":     -0.25,
		"slumps":    -0.25,
		"slumped":   -0.25,
		"decline":   -0.20,
		"declines":  -0.20,
		"declined":  -0.20,
		"drop":      -0.20,
		"drops":     -0.20,
		"dropped":   -0.20,
		"fall":      -0.20,
		"falls":     -0.20,
		"fell":      -0.20,
		"fear":      -0.25,
		"panic":     -0.30,
		"ban":       -0.30,
		"bans":      -0.30,
		"banned":    -0.30,
		"crackdown": -0.35,
		"fraud":     -0.40,
		"hack":      -0.35,
		"hacks":     -0.35,
		"hacked":    -0.35,
		"liquidation": -0.25,
		"negative":  -0.15,
		"loss":      -0.20,
		"losses":    -0.20,
		"uncertain": -0.10,
		"volatile":  -0.10,
	})
}

func (l *LexiconScorer) Score(texts []string) ([]float64, error) {
	result := make([]float64, len(texts))
	for i, text := range texts {
		lower := strings.ToLower(text)
		var total float64
		for word, bias := range l.words {
			if strings.Contains(lower, word) {
				total += bias
			}
		}
		if total > 1 {
			total = 1
		} else if total < -1 {
			total = -1
		}
		result[i] = total
	}
	return result, nil
}

func (l *LexiconScorer) Explain(text string) []string {
	lower := strings.ToLower(text)
	var matches []string
	for word, bias := range l.words {
		if strings.Contains(lower, word) {
			matches = append(matches, fmt.Sprintf("%s:%+.2f", word, bias))
		}
	}
	sort.Strings(matches)
	return matches
}

var _ Scorer = (*LexiconScorer)(nil)

type scorerFunc func(texts []string) ([]float64, error)

func (f scorerFunc) Score(texts []string) ([]float64, error) {
	return f(texts)
}

func Clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func RoundTo(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}
