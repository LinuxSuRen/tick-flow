package templating

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// AssignmentModeManual is the only P0 assignment mode (issue #4).
const AssignmentModeManual = "manual"

var assignmentModes = map[string]struct{}{
	AssignmentModeManual: {},
}

// Assignment is the dispatch policy.
type Assignment struct {
	Mode string `json:"mode" yaml:"mode"`
}

// ParseAssignment decodes JSON strictly.
func ParseAssignment(raw []byte) (*Assignment, error) {
	a := &Assignment{}
	if err := unmarshalStrict(raw, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Validate checks the mode enum.
func (a *Assignment) Validate() *ValidationError {
	ve := &ValidationError{}
	if _, ok := assignmentModes[a.Mode]; !ok {
		ve.Add("assignment.mode", fmt.Sprintf("unknown mode %q, supported: manual", a.Mode))
	}
	return ve
}

// SLAPolicy holds response/resolve targets. P0 stores the config only;
// timers land in P1 (issue #4).
type SLAPolicy struct {
	FirstResponseMinutes *int  `json:"first_response_minutes" yaml:"first_response_minutes"`
	ResolveMinutes       *int  `json:"resolve_minutes" yaml:"resolve_minutes"`
	Escalation           []any `json:"escalation" yaml:"escalation"`
}

// ParseSLAPolicy decodes JSON strictly.
func ParseSLAPolicy(raw []byte) (*SLAPolicy, error) {
	s := &SLAPolicy{}
	if err := unmarshalStrict(raw, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Validate checks the numeric fields are non-negative integers.
func (s *SLAPolicy) Validate() *ValidationError {
	ve := &ValidationError{}
	ve.Merge(nonNegativeInt("sla_policy.first_response_minutes", s.FirstResponseMinutes))
	ve.Merge(nonNegativeInt("sla_policy.resolve_minutes", s.ResolveMinutes))
	return ve
}

// BuiltinFields are the ticket columns view configs may reference in
// addition to form schema keys (issue #4).
var BuiltinFields = []string{
	"number", "title", "content", "state", "priority", "assignee",
	"creator", "created_at", "updated_at", "closed_at",
}

// PortalConfig is the portal copy of a ticket type. Unlike UI-fixed copy
// (owned by the frontend), these strings are tenant data.
type PortalConfig struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description" yaml:"description"`
	SuccessText string `json:"success_text" yaml:"success_text"`
}

// ColumnConfig references one list column.
type ColumnConfig struct {
	Field string `json:"field" yaml:"field"`
}

// GroupConfig references a detail section.
type GroupConfig struct {
	Title  string   `json:"title" yaml:"title"`
	Fields []string `json:"fields" yaml:"fields"`
}

// ViewConfig drives list columns, detail groups and portal copy.
type ViewConfig struct {
	Portal       PortalConfig  `json:"portal" yaml:"portal"`
	ListColumns  []ColumnConfig `json:"list_columns" yaml:"list_columns"`
	DetailGroups []GroupConfig `json:"detail_groups" yaml:"detail_groups"`
}

// ParseViewConfig decodes JSON strictly.
func ParseViewConfig(raw []byte) (*ViewConfig, error) {
	v := &ViewConfig{}
	if err := unmarshalStrict(raw, v); err != nil {
		return nil, err
	}
	return v, nil
}

// Validate checks every field reference resolves to a builtin field or a
// form schema key (issue #4).
func (v *ViewConfig) Validate(fs *FormSchema) *ValidationError {
	ve := &ValidationError{}
	known := map[string]struct{}{}
	for _, b := range BuiltinFields {
		known[b] = struct{}{}
	}
	if fs != nil {
		for _, f := range fs.Fields {
			known[f.Key] = struct{}{}
		}
	}
	for i, c := range v.ListColumns {
		if _, ok := known[c.Field]; !ok {
			ve.Add(fmt.Sprintf("view_config.list_columns[%d].field", i),
				fmt.Sprintf("references unknown field %q", c.Field))
		}
	}
	for i, g := range v.DetailGroups {
		if g.Title == "" {
			ve.Add(fmt.Sprintf("view_config.detail_groups[%d].title", i), "title is required")
		}
		for j, f := range g.Fields {
			if _, ok := known[f]; !ok {
				ve.Add(fmt.Sprintf("view_config.detail_groups[%d].fields[%d]", i, j),
					fmt.Sprintf("references unknown field %q", f))
			}
		}
	}
	return ve
}

// yamlUnmarshalStrict decodes YAML rejecting unknown keys.
func yamlUnmarshalStrict(raw []byte, v any) error {
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	return dec.Decode(v)
}
