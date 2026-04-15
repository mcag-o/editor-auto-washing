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

func TestQualityGateReturnsPassForExactMinLength(t *testing.T) {
	gate := NewQualityGateEngine()

	decision := gate.Evaluate(QualityInput{
		StructuredOutput: map[string]any{"body": "12345"},
		MinLength:        5,
		MaxLength:        20,
	})

	require.Equal(t, QualityDecisionPass, decision.Action)
}

func TestQualityGateReturnsPassForExactMaxLength(t *testing.T) {
	gate := NewQualityGateEngine()

	decision := gate.Evaluate(QualityInput{
		StructuredOutput: map[string]any{"body": "1234567890"},
		MinLength:        1,
		MaxLength:        10,
	})

	require.Equal(t, QualityDecisionPass, decision.Action)
}

func TestQualityGateReturnsRepairWhenBodyIsMissing(t *testing.T) {
	gate := NewQualityGateEngine()

	decision := gate.Evaluate(QualityInput{
		StructuredOutput: map[string]any{},
		MinLength:        1,
		MaxLength:        10,
	})

	require.Equal(t, QualityDecisionRepair, decision.Action)
	require.Equal(t, "body is missing", decision.Message)
}

func TestQualityGateReturnsRepairWhenBodyIsNotAString(t *testing.T) {
	gate := NewQualityGateEngine()

	decision := gate.Evaluate(QualityInput{
		StructuredOutput: map[string]any{"body": 123},
		MinLength:        1,
		MaxLength:        10,
	})

	require.Equal(t, QualityDecisionRepair, decision.Action)
	require.Equal(t, "body must be a string", decision.Message)
}

func TestQualityGateCountsRunesInsteadOfBytes(t *testing.T) {
	gate := NewQualityGateEngine()

	decision := gate.Evaluate(QualityInput{
		StructuredOutput: map[string]any{"body": "你好世界"},
		MinLength:        4,
		MaxLength:        4,
	})

	require.Equal(t, QualityDecisionPass, decision.Action)
	require.Contains(t, decision.Message, "4")
}

func TestQualityGateTreatsNonPositiveMaxLengthAsUnbounded(t *testing.T) {
	gate := NewQualityGateEngine()

	decision := gate.Evaluate(QualityInput{
		StructuredOutput: map[string]any{"body": "123456789012345678901"},
		MinLength:        5,
		MaxLength:        0,
	})

	require.Equal(t, QualityDecisionPass, decision.Action)
	require.Contains(t, decision.Message, "unbounded")
}
