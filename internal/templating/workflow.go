package templating

import (
	"encoding/json"
	"fmt"
	"math"
)

// State types in the workflow definition.
const (
	StateTypeInitial = "initial"
	StateTypeNormal  = "normal"
	StateTypeFinal   = "final"
)

// WorkflowRoleCreator is the relative role resolved against the ticket
// creator at runtime (issue #4); all other roles are tenant roles.
const WorkflowRoleCreator = "creator"

// State is one workflow state.
type State struct {
	Key  string `json:"key" yaml:"key"`
	Name string `json:"name" yaml:"name"`
	Type string `json:"type" yaml:"type"`
}

// Transition is one allowed state change, gated by roles.
type Transition struct {
	From  string   `json:"from" yaml:"from"`
	Action string  `json:"action" yaml:"action"`
	To    string   `json:"to" yaml:"to"`
	Roles []string `json:"roles" yaml:"roles"`
}

// Workflow is the configurable state machine of a ticket type.
type Workflow struct {
	States      []State      `json:"states" yaml:"states"`
	Transitions []Transition `json:"transitions" yaml:"transitions"`
}

// ParseWorkflow decodes JSON with unknown fields rejected.
func ParseWorkflow(raw []byte) (*Workflow, error) {
	w := &Workflow{}
	if err := unmarshalStrict(raw, w); err != nil {
		return nil, err
	}
	return w, nil
}

// Validate enforces the save-time rules of issue #4: exactly one initial
// state, at least one final state, no dangling from/to references, no
// duplicate action within a state and only known roles.
func (w *Workflow) Validate() *ValidationError {
	ve := &ValidationError{}
	if len(w.States) == 0 {
		ve.Add("states", "at least one state is required")
		return ve
	}

	stateKeys := map[string]struct{}{}
	initials := 0
	finals := 0
	for i, s := range w.States {
		p := fmt.Sprintf("states[%d]", i)
		if s.Key == "" {
			ve.Add(p+".key", "key is required")
		} else if !snakeCaseRe.MatchString(s.Key) {
			ve.Add(p+".key", fmt.Sprintf("key %q must be english snake_case", s.Key))
		} else if _, dup := stateKeys[s.Key]; dup {
			ve.Add(p+".key", fmt.Sprintf("duplicate state key %q", s.Key))
		} else {
			stateKeys[s.Key] = struct{}{}
		}
		if s.Name == "" {
			ve.Add(p+".name", "name is required")
		}
		switch s.Type {
		case StateTypeInitial:
			initials++
		case StateTypeNormal, StateTypeFinal:
			if s.Type == StateTypeFinal {
				finals++
			}
		default:
			ve.Add(p+".type", fmt.Sprintf("unknown state type %q", s.Type))
		}
	}
	if initials != 1 {
		ve.Add("states", fmt.Sprintf("exactly one initial state is required, found %d", initials))
	}
	if finals < 1 {
		ve.Add("states", "at least one final state is required")
	}

	actions := map[string]struct{}{}
	for i, t := range w.Transitions {
		p := fmt.Sprintf("transitions[%d]", i)
		if _, ok := stateKeys[t.From]; !ok {
			ve.Add(p+".from", fmt.Sprintf("unknown state %q", t.From))
		}
		if _, ok := stateKeys[t.To]; !ok {
			ve.Add(p+".to", fmt.Sprintf("unknown state %q", t.To))
		}
		if t.Action == "" || !snakeCaseRe.MatchString(t.Action) {
			ve.Add(p+".action", "action must be english snake_case")
		}
		key := t.From + "|" + t.Action
		if _, dup := actions[key]; dup {
			ve.Add(p+".action", fmt.Sprintf("duplicate action %q in state %q", t.Action, t.From))
		}
		actions[key] = struct{}{}
		if len(t.Roles) == 0 {
			ve.Add(p+".roles", "at least one role is required")
		}
		for j, r := range t.Roles {
			switch r {
			case WorkflowRoleCreator, "agent", "tenant_admin":
			default:
				ve.Add(fmt.Sprintf("%s.roles[%d]", p, j), fmt.Sprintf("unknown role %q", r))
			}
		}
	}
	return ve
}

// InitialState returns the unique initial state key.
func (w *Workflow) InitialState() (string, error) {
	for _, s := range w.States {
		if s.Type == StateTypeInitial {
			return s.Key, nil
		}
	}
	return "", fmt.Errorf("workflow has no initial state")
}

// StateName resolves the display name of a state key.
func (w *Workflow) StateName(key string) string {
	for _, s := range w.States {
		if s.Key == key {
			return s.Name
		}
	}
	return key
}

// StateTypeOf resolves the type of a state key, empty when unknown.
func (w *Workflow) StateTypeOf(key string) string {
	for _, s := range w.States {
		if s.Key == key {
			return s.Type
		}
	}
	return ""
}

// TransitionFor finds the transition leaving state with the given action.
func (w *Workflow) TransitionFor(state, action string) (*Transition, bool) {
	for i := range w.Transitions {
		t := &w.Transitions[i]
		if t.From == state && t.Action == action {
			return t, true
		}
	}
	return nil, false
}

// AvailableTransitions lists transitions leaving the given state.
func (w *Workflow) AvailableTransitions(state string) []Transition {
	var out []Transition
	for _, t := range w.Transitions {
		if t.From == state {
			out = append(out, t)
		}
	}
	return out
}

// RolesAllow checks the transition roles against the actor: the tenant role
// plus the creator relative role when the actor created the ticket.
func (t *Transition) RolesAllow(tenantRole string, isCreator bool) bool {
	for _, r := range t.Roles {
		if r == tenantRole {
			return true
		}
		if r == WorkflowRoleCreator && isCreator {
			return true
		}
	}
	return false
}

// MarshalJSONExcept is not needed; workflows persist as the raw JSON they
// were validated from. This helper keeps callers honest when they need to
// re-encode for storage.
func (w *Workflow) ToJSON() (json.RawMessage, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// nonNegativeInt validates optional SLA minutes fields.
func nonNegativeInt(path string, v *int) *ValidationError {
	if v == nil {
		return nil
	}
	if *v < 0 || *v > math.MaxInt32 {
		return &ValidationError{items: []string{path + ": must be a non-negative integer"}}
	}
	return nil
}
