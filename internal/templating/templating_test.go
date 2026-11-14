package templating_test

import (
	"testing"

	"github.com/LinuxSuRen/tick-flow/internal/templating"
	"github.com/LinuxSuRen/tick-flow/seed"
)

// Seed packs double as the validator fixtures (issue #19): every embedded
// pack must parse strictly and validate cleanly.
func TestSeedPacksValidate(t *testing.T) {
	packs, err := seed.Packs()
	if err != nil {
		t.Fatalf("load seed packs: %v", err)
	}
	if len(packs) < 4 {
		t.Fatalf("expected at least 4 seed packs, got %d", len(packs))
	}
	seen := map[string]bool{}
	for _, p := range packs {
		if seen[p.Slug] {
			t.Errorf("duplicate seed slug %q", p.Slug)
		}
		seen[p.Slug] = true
		if ve := p.Validate(); !ve.Empty() {
			t.Errorf("seed pack %q invalid: %v", p.Slug, ve)
		}
		if _, err := p.Workflow.InitialState(); err != nil {
			t.Errorf("seed pack %q has no initial state", p.Slug)
		}
	}
}

func TestFormSchemaRejectsUnknownType(t *testing.T) {
	fs := &templating.FormSchema{Fields: []templating.Field{
		{Key: "a", Label: "A", Type: "richtext"},
	}}
	ve := fs.Validate()
	assertHas(t, ve, "unknown field type")
}

func TestFormSchemaRejectsDuplicateKey(t *testing.T) {
	fs := &templating.FormSchema{Fields: []templating.Field{
		{Key: "a", Label: "A", Type: "string"},
		{Key: "a", Label: "A2", Type: "string"},
	}}
	assertHas(t, fs.Validate(), "duplicate key")
}

func TestFormSchemaRejectsMisplacedValidation(t *testing.T) {
	maxLen := 10
	fs := &templating.FormSchema{Fields: []templating.Field{
		{Key: "n", Label: "N", Type: "number", MaxLength: &maxLen},
	}}
	assertHas(t, fs.Validate(), "max_length only applies")
}

func TestFormSchemaRejectsNonSnakeKey(t *testing.T) {
	fs := &templating.FormSchema{Fields: []templating.Field{
		{Key: "BadKey", Label: "B", Type: "string"},
	}}
	assertHas(t, fs.Validate(), "snake_case")
}

func TestFormSchemaRejectsSelectWithoutOptions(t *testing.T) {
	fs := &templating.FormSchema{Fields: []templating.Field{
		{Key: "s", Label: "S", Type: "select"},
	}}
	assertHas(t, fs.Validate(), "options")
}

func TestWorkflowRequiresSingleInitial(t *testing.T) {
	w := &templating.Workflow{States: []templating.State{
		{Key: "a", Name: "A", Type: "final"},
	}}
	assertHas(t, w.Validate(), "exactly one initial")
}

func TestWorkflowRequiresFinal(t *testing.T) {
	w := &templating.Workflow{States: []templating.State{
		{Key: "a", Name: "A", Type: "initial"},
	}}
	assertHas(t, w.Validate(), "at least one final")
}

func TestWorkflowRejectsDanglingReference(t *testing.T) {
	w := &templating.Workflow{States: []templating.State{
		{Key: "a", Name: "A", Type: "initial"},
		{Key: "b", Name: "B", Type: "final"},
	}, Transitions: []templating.Transition{
		{From: "a", Action: "go", To: "ghost", Roles: []string{"agent"}},
	}}
	assertHas(t, w.Validate(), `unknown state "ghost"`)
}

func TestWorkflowRejectsDuplicateActionInState(t *testing.T) {
	w := &templating.Workflow{States: []templating.State{
		{Key: "a", Name: "A", Type: "initial"},
		{Key: "b", Name: "B", Type: "final"},
	}, Transitions: []templating.Transition{
		{From: "a", Action: "go", To: "b", Roles: []string{"agent"}},
		{From: "a", Action: "go", To: "a", Roles: []string{"agent"}},
	}}
	assertHas(t, w.Validate(), "duplicate action")
}

func TestWorkflowRejectsUnknownRole(t *testing.T) {
	w := &templating.Workflow{States: []templating.State{
		{Key: "a", Name: "A", Type: "initial"},
		{Key: "b", Name: "B", Type: "final"},
	}, Transitions: []templating.Transition{
		{From: "a", Action: "go", To: "b", Roles: []string{"superuser"}},
	}}
	assertHas(t, w.Validate(), "unknown role")
}

func TestWorkflowRuntime(t *testing.T) {
	w := &templating.Workflow{States: []templating.State{
		{Key: "pending", Name: "待受理", Type: "initial"},
		{Key: "closed", Name: "已关闭", Type: "final"},
	}, Transitions: []templating.Transition{
		{From: "pending", Action: "close", To: "closed", Roles: []string{"creator", "agent"}},
	}}
	if init, _ := w.InitialState(); init != "pending" {
		t.Fatalf("initial state = %q", init)
	}
	tr, ok := w.TransitionFor("pending", "close")
	if !ok {
		t.Fatal("expected transition found")
	}
	if !tr.RolesAllow("agent", false) {
		t.Error("agent should be allowed")
	}
	if !tr.RolesAllow("requester", true) {
		t.Error("creator should be allowed")
	}
	if tr.RolesAllow("requester", false) {
		t.Error("plain requester should be denied")
	}
	if got := w.StateName("closed"); got != "已关闭" {
		t.Fatalf("state name = %q", got)
	}
}

func TestViewConfigRejectsUnknownFieldReference(t *testing.T) {
	fs := &templating.FormSchema{Fields: []templating.Field{
		{Key: "device_sn", Label: "设备编号", Type: "string"},
	}}
	vc := &templating.ViewConfig{
		ListColumns:  []templating.ColumnConfig{{Field: "ghost_field"}},
		DetailGroups: []templating.GroupConfig{{Title: "基础", Fields: []string{"device_sn"}}},
	}
	assertHas(t, vc.Validate(fs), `unknown field "ghost_field"`)
}

func TestViewConfigAllowsBuiltinAndSchemaFields(t *testing.T) {
	fs := &templating.FormSchema{Fields: []templating.Field{
		{Key: "device_sn", Label: "设备编号", Type: "string"},
	}}
	vc := &templating.ViewConfig{
		ListColumns:  []templating.ColumnConfig{{Field: "number"}, {Field: "device_sn"}},
		DetailGroups: []templating.GroupConfig{{Title: "基础", Fields: []string{"title", "state"}}},
	}
	if ve := vc.Validate(fs); !ve.Empty() {
		t.Fatalf("expected clean validation, got %v", ve)
	}
}

func TestAssignmentRejectsUnknownMode(t *testing.T) {
	a := &templating.Assignment{Mode: "round_robin"}
	assertHas(t, a.Validate(), "unknown mode")
}

func TestFormSchemaValidateData(t *testing.T) {
	maxLen := 5
	maxCount := 2
	fs := &templating.FormSchema{Fields: []templating.Field{
		{Key: "name", Label: "名称", Type: "string", Required: true, MaxLength: &maxLen},
		{Key: "count", Label: "数量", Type: "number"},
		{Key: "photos", Label: "照片", Type: "attachments", MaxCount: &maxCount},
	}}

	if ve := fs.ValidateData(map[string]any{"name": "ab", "count": 2}); !ve.Empty() {
		t.Fatalf("valid data rejected: %v", ve)
	}
	if ve := fs.ValidateData(map[string]any{"count": 2}); ve.Empty() {
		t.Fatal("missing required field accepted")
	}
	if ve := fs.ValidateData(map[string]any{"name": "ab", "count": "x"}); ve.Empty() {
		t.Fatal("wrong type accepted")
	}
	if ve := fs.ValidateData(map[string]any{"name": "ab", "ghost": 1}); ve.Empty() {
		t.Fatal("unknown field accepted")
	}
	if ve := fs.ValidateData(map[string]any{"name": "ab", "photos": []any{1.0, 2.0, 3.0}}); ve.Empty() {
		t.Fatal("max_count violation accepted")
	}
}

func assertHas(t *testing.T, ve *templating.ValidationError, want string) {
	t.Helper()
	if ve == nil || ve.Empty() {
		t.Fatalf("expected validation error containing %q, got none", want)
	}
	if !contains(ve.Error(), want) {
		t.Fatalf("expected error containing %q, got %q", want, ve.Error())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
