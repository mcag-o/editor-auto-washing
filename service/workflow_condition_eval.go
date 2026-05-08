package service

import (
	"fmt"
	"strconv"
	"strings"
)

type workflowConditionEvaluator struct{}

func newWorkflowConditionEvaluator() workflowConditionEvaluator {
	return workflowConditionEvaluator{}
}

func (workflowConditionEvaluator) Evaluate(ctx *WorkflowExecutionContext, result WorkflowNodeResult, condition string) (bool, error) {
	trimmed := strings.TrimSpace(condition)
	if trimmed == "" || trimmed == "always" {
		return true, nil
	}
	return evalWorkflowConditionExpr(ctx, result, trimmed)
}

func evalWorkflowConditionExpr(ctx *WorkflowExecutionContext, result WorkflowNodeResult, expr string) (bool, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "not ") {
		matched, err := evalWorkflowConditionExpr(ctx, result, strings.TrimSpace(strings.TrimPrefix(expr, "not ")))
		if err != nil {
			return false, err
		}
		return !matched, nil
	}
	if left, right, ok := splitWorkflowCondition(expr, " or "); ok {
		matched, err := evalWorkflowConditionExpr(ctx, result, left)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
		return evalWorkflowConditionExpr(ctx, result, right)
	}
	if left, right, ok := splitWorkflowCondition(expr, " and "); ok {
		matched, err := evalWorkflowConditionExpr(ctx, result, left)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
		return evalWorkflowConditionExpr(ctx, result, right)
	}
	return evalWorkflowComparison(ctx, result, expr)
}

func splitWorkflowCondition(expr, delimiter string) (string, string, bool) {
	idx := strings.Index(expr, delimiter)
	if idx < 0 {
		return "", "", false
	}
	return expr[:idx], expr[idx+len(delimiter):], true
}

func evalWorkflowComparison(ctx *WorkflowExecutionContext, result WorkflowNodeResult, expr string) (bool, error) {
	for _, operator := range []string{"!=", ">=", "<=", "==", ">", "<"} {
		left, right, ok := strings.Cut(expr, operator)
		if !ok {
			continue
		}
		leftValue, err := resolveWorkflowConditionRef(ctx, result, strings.TrimSpace(left))
		if err != nil {
			return false, err
		}
		return compareWorkflowConditionValues(leftValue, strings.TrimSpace(right), operator)
	}
	return false, fmt.Errorf("unsupported workflow condition expression: %s", expr)
}

func resolveWorkflowConditionRef(ctx *WorkflowExecutionContext, result WorkflowNodeResult, ref string) (any, error) {
	namespace, path, ok := strings.Cut(ref, ".")
	if !ok || strings.TrimSpace(namespace) == "" || strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("unsupported workflow condition reference %q", ref)
	}
	var root any
	switch namespace {
	case "input":
		if ctx != nil {
			root = cloneWorkflowPayload(workflowRuntimeSharedInput(ctx))
		}
	case "payload":
		if ctx != nil {
			root = workflowRuntimePayload(ctx)
		}
	case "vars":
		if ctx != nil {
			root = ctx.Variables
		}
	case "metadata":
		if ctx != nil {
			root = cloneWorkflowPayload(ctx.Metadata)
		}
	case "result":
		if ctx != nil && ctx.Result != nil {
			root = ctx.Result
		} else {
			root = result.Output
		}
	case "artifacts":
		if ctx != nil {
			root = ctx.Artifacts
		}
	default:
		return nil, fmt.Errorf("unsupported workflow condition namespace %q", namespace)
	}
	return resolveWorkflowConditionPath(root, strings.Split(path, ".")), nil
}

func resolveWorkflowConditionPath(value any, path []string) any {
	current := value
	for _, segment := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		next, ok := mapping[segment]
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

func compareWorkflowConditionValues(left any, right string, operator string) (bool, error) {
	if leftNumber, ok := workflowConditionNumber(left); ok {
		rightNumber, err := strconv.ParseFloat(right, 64)
		if err == nil {
			switch operator {
			case "==":
				return leftNumber == rightNumber, nil
			case "!=":
				return leftNumber != rightNumber, nil
			case ">":
				return leftNumber > rightNumber, nil
			case ">=":
				return leftNumber >= rightNumber, nil
			case "<":
				return leftNumber < rightNumber, nil
			case "<=":
				return leftNumber <= rightNumber, nil
			}
		}
	}

	leftValue := fmt.Sprint(left)
	switch operator {
	case "==":
		return leftValue == right, nil
	case "!=":
		return leftValue != right, nil
	}
	return false, fmt.Errorf("unsupported workflow condition operator %q for non-numeric comparison", operator)
}

func workflowConditionNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}
