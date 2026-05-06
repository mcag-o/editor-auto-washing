package domain

import (
	"testing"
	"time"
)

func TestWorkflowDefinitionValidateRequiresEntryNodeAndNodes(t *testing.T) {
	wf := WorkflowDefinition{}
	if err := wf.Validate(); err == nil {
		t.Fatal("expected validate to reject empty workflow definition")
	}
}

func TestWorkflowDefinitionValidateRejectsMissingEntryNodeReference(t *testing.T) {
	wf := WorkflowDefinition{
		ID:          "wf-1",
		Name:        "rewrite",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "missing",
		Nodes: []WorkflowNode{{
			ID:   "start",
			Type: "action",
			Name: "Start",
		}},
		UpdatedBy: "tester",
		UpdatedAt: time.Now().UTC(),
	}

	if err := wf.Validate(); err == nil {
		t.Fatal("expected validate to reject missing entry node reference")
	}
}

func TestWorkflowDefinitionValidateAcceptsDefaultWorkflowDefinition(t *testing.T) {
	wf := DefaultWorkflowDefinition()
	if err := wf.Validate(); err != nil {
		t.Fatalf("expected default workflow definition to be valid: %v", err)
	}
}

func TestTemplateDefinitionValidateRequiresNameTypeContent(t *testing.T) {
	tpl := TemplateDefinition{}
	if err := tpl.Validate(); err == nil {
		t.Fatal("expected validate to reject empty template definition")
	}
}

func TestTemplateDefinitionValidateAcceptsMinimalDefinition(t *testing.T) {
	tpl := TemplateDefinition{
		ID:            "tpl-1",
		Name:          "rewrite-system",
		Type:          "prompt",
		Version:       "v1",
		Enabled:       true,
		Content:       "You are a rewriter.",
		VariablesJSON: "{\"tone\":\"formal\"}",
		UpdatedBy:     "tester",
		UpdatedAt:     time.Now().UTC(),
	}

	if err := tpl.Validate(); err != nil {
		t.Fatalf("expected template definition to be valid: %v", err)
	}
}
