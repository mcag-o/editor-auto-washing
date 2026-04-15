package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQualityGateReturnsPassForValidDraftLength(t *testing.T) {
	gate := NewQualityGateEngine()

	decision := gate.Evaluate(QualityInput{
		StructuredOutput: map[string]any{"body": "1234567890"},
		MinLength:        5,
		MaxLength:        20,
	})

	require.Equal(t, QualityDecisionPass, decision.Action)
	require.Contains(t, decision.Message, "within")
}

func TestQualityGateReturnsRepairForTooShortDraft(t *testing.T) {
	gate := NewQualityGateEngine()

	decision := gate.Evaluate(QualityInput{
		StructuredOutput: map[string]any{"body": "123"},
		MinLength:        5,
		MaxLength:        20,
	})

	require.Equal(t, QualityDecisionRepair, decision.Action)
	require.Contains(t, decision.Message, "too short")
}

func TestQualityGateReturnsRepairForTooLongDraft(t *testing.T) {
	gate := NewQualityGateEngine()

	decision := gate.Evaluate(QualityInput{
		StructuredOutput: map[string]any{"body": "123456789012345678901"},
		MinLength:        5,
		MaxLength:        20,
	})

	require.Equal(t, QualityDecisionRepair, decision.Action)
	require.Contains(t, decision.Message, "too long")
}
