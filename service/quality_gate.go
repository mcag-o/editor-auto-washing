package service

import (
	"fmt"
	"unicode/utf8"
)

const (
	QualityDecisionPass   = "pass"
	QualityDecisionRetry  = "retry_stage"
	QualityDecisionRepair = "route_to_repair"
	QualityDecisionFail   = "fail_run"
)

type QualityInput struct {
	StructuredOutput map[string]any
	MinLength        int
	MaxLength        int
}

type QualityResult struct {
	Action  string
	Message string
}

type QualityGateEngine struct{}

func NewQualityGateEngine() *QualityGateEngine {
	return &QualityGateEngine{}
}

func (g *QualityGateEngine) Evaluate(input QualityInput) QualityResult {
	rawBody, ok := input.StructuredOutput["body"]
	if !ok {
		return QualityResult{
			Action:  QualityDecisionRepair,
			Message: "body is missing",
		}
	}

	body, ok := rawBody.(string)
	if !ok {
		return QualityResult{
			Action:  QualityDecisionRepair,
			Message: "body must be a string",
		}
	}

	bodyLength := utf8.RuneCountInString(body)

	if bodyLength < input.MinLength {
		return QualityResult{
			Action:  QualityDecisionRepair,
			Message: fmt.Sprintf("body too short: got %d characters, need at least %d", bodyLength, input.MinLength),
		}
	}

	if input.MaxLength > 0 && bodyLength > input.MaxLength {
		return QualityResult{
			Action:  QualityDecisionRepair,
			Message: fmt.Sprintf("body too long: got %d characters, need at most %d", bodyLength, input.MaxLength),
		}
	}

	upperBoundMessage := fmt.Sprintf("body length %d within quality bounds", bodyLength)
	if input.MaxLength <= 0 {
		upperBoundMessage = fmt.Sprintf("body length %d within quality bounds (max length unbounded)", bodyLength)
	}

	return QualityResult{
		Action:  QualityDecisionPass,
		Message: upperBoundMessage,
	}
}
