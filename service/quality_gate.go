package service

import "fmt"

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
	body, _ := input.StructuredOutput["body"].(string)
	bodyLength := len(body)

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

	return QualityResult{
		Action:  QualityDecisionPass,
		Message: fmt.Sprintf("body length %d within quality bounds", bodyLength),
	}
}
