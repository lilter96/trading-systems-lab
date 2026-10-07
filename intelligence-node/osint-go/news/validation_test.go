package news

import (
	"testing"
)

func TestValidationGate_BelowThreshold_BlocksLiveUse(t *testing.T) {
	samples := []Sample{
		{Headline: "a", Label: 0.5},
		{Headline: "b", Label: -0.5},
		{Headline: "c", Label: 0.5},
		{Headline: "d", Label: -0.5},
		{Headline: "e", Label: 0.5},
	}

	noiseScorer := scorerFunc(func(texts []string) ([]float64, error) {
		result := make([]float64, len(texts))
		for i := range texts {
			result[i] = float64(i) * 0.01
		}
		return result, nil
	})

	rho, err := ComputeSpearmanRho(samples, noiseScorer)
	if err != nil {
		t.Fatal(err)
	}

	if rho >= 0.5 {
		t.Fatalf("expected rho < 0.5 for noise scorer, got %.4f", rho)
	}

	gate := NewValidationGate(0.5)

	stored := defaultValidationData
	defer func() { defaultValidationData = stored }()

	defaultValidationData = []byte(`[{"headline":"x","label":0.5},{"headline":"y","label":-0.3},{"headline":"z","label":0.0}]`)

	ready, rho2, err := gate.IsLiveReady(noiseScorer)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatalf("expected gate to block live use for noise scorer (rho=%.4f)", rho2)
	}
}

func TestValidationGate_FallbackLexicon_Auditable(t *testing.T) {
	lex := DefaultLexiconScorer()

	texts := []string{
		"Bitcoin surges to new highs",
		"Ethereum drops amid market uncertainty",
		"Crypto market consolidates in quiet trading",
	}

	scores, err := lex.Score(texts)
	if err != nil {
		t.Fatal(err)
	}

	if scores[0] <= 0 {
		t.Errorf("expected positive score for bullish headline, got %.2f", scores[0])
	}
	if scores[1] >= 0 {
		t.Errorf("expected negative score for bearish headline, got %.2f", scores[1])
	}
	if scores[2] != 0 {
		t.Errorf("expected neutral score for neutral headline, got %.2f", scores[2])
	}

	explanation := lex.Explain("Bitcoin surges to new highs")
	if len(explanation) == 0 {
		t.Fatal("expected at least one matched term")
	}
	found := false
	for _, m := range explanation {
		if len(m) > 6 && m[:6] == "surges" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'surges' in explanation, got %v", explanation)
	}
}

func TestLexiconScorer_Clamping(t *testing.T) {
	lex := NewLexiconScorer(map[string]float64{
		"a": 0.6,
		"b": 0.6,
		"c": 0.6,
	})

	scores, err := lex.Score([]string{"a b c"})
	if err != nil {
		t.Fatal(err)
	}
	if scores[0] > 1.0 || scores[0] < -1.0 {
		t.Errorf("expected clamped score in [-1,1], got %.2f", scores[0])
	}
	if scores[0] != 1.0 {
		t.Errorf("expected 1.0 (clamped from 1.8), got %.2f", scores[0])
	}
}

func TestComputeSpearmanRho_PerfectCorrelation(t *testing.T) {
	samples := []Sample{
		{Headline: "a", Label: -1.0},
		{Headline: "b", Label: 0.0},
		{Headline: "c", Label: 1.0},
	}

	perfectScorer := scorerFunc(func(texts []string) ([]float64, error) {
		result := make([]float64, len(texts))
		for i := range texts {
			result[i] = float64(i-1) * 1.0
		}
		return result, nil
	})

	rho, err := ComputeSpearmanRho(samples, perfectScorer)
	if err != nil {
		t.Fatal(err)
	}
	if rho < 0.99 {
		t.Errorf("expected near-perfect rho for perfect correlation, got %.4f", rho)
	}
}

func TestComputeSpearmanRho_InverseCorrelation(t *testing.T) {
	samples := []Sample{
		{Headline: "a", Label: -1.0},
		{Headline: "b", Label: 0.0},
		{Headline: "c", Label: 1.0},
	}

	inverseScorer := scorerFunc(func(texts []string) ([]float64, error) {
		result := make([]float64, len(texts))
		for i := range texts {
			result[i] = float64(1-i) * 1.0
		}
		return result, nil
	})

	rho, err := ComputeSpearmanRho(samples, inverseScorer)
	if err != nil {
		t.Fatal(err)
	}
	if rho > -0.99 {
		t.Errorf("expected near-perfect negative rho, got %.4f", rho)
	}
}

func TestLoadValidationSet_EmbeddedDefault(t *testing.T) {
	samples, err := LoadValidationSet()
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) < 20 {
		t.Errorf("expected at least 20 validation samples, got %d", len(samples))
	}

	labels := make(map[float64]int)
	for _, s := range samples {
		labels[s.Label]++
	}
	if len(labels) < 3 {
		t.Errorf("expected multiple distinct label values, got %d", len(labels))
	}
}
