package templating

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// slugRe is kebab-ish: lower-case letters, digits and dashes, starting with
// a letter. Seed slugs are it-support style, unlike field/state keys which
// stay strictly snake_case.
var slugRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Pack is one seed template pack file (seed/*.yaml, issue #19) and also the
// shape of the admin API payload: the five JSONB bodies plus identity.
type Pack struct {
	Slug       string      `json:"slug" yaml:"slug"`
	Name       string      `json:"name" yaml:"name"`
	Icon       string      `json:"icon" yaml:"icon"`
	FormSchema FormSchema  `json:"form_schema" yaml:"form_schema"`
	Workflow   Workflow    `json:"workflow" yaml:"workflow"`
	SLAPolicy  SLAPolicy   `json:"sla_policy" yaml:"sla_policy"`
	Assignment Assignment  `json:"assignment" yaml:"assignment"`
	ViewConfig ViewConfig  `json:"view_config" yaml:"view_config"`
}

// ParsePackYAML decodes a seed YAML file with unknown keys rejected.
func ParsePackYAML(raw []byte) (*Pack, error) {
	p := &Pack{}
	if err := yamlUnmarshalStrict(raw, p); err != nil {
		return nil, fmt.Errorf("parse pack yaml: %w", err)
	}
	return p, nil
}

// ParsePackJSON decodes an API JSON payload with unknown keys rejected.
func ParsePackJSON(raw []byte) (*Pack, error) {
	p := &Pack{}
	if err := unmarshalStrict(raw, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate runs every section validator (issue #4 / #12).
func (p *Pack) Validate() *ValidationError {
	ve := &ValidationError{}
	if p.Slug == "" {
		ve.Add("slug", "slug is required")
	} else if !slugRe.MatchString(p.Slug) {
		ve.Add("slug", fmt.Sprintf("slug %q must be lowercase letters, digits or dashes", p.Slug))
	}
	if p.Name == "" {
		ve.Add("name", "name is required")
	}
	ve.Merge(p.FormSchema.Validate())
	ve.Merge(p.Workflow.Validate())
	ve.Merge(p.SLAPolicy.Validate())
	ve.Merge(p.Assignment.Validate())
	ve.Merge(p.ViewConfig.Validate(&p.FormSchema))
	return ve
}

// JSONB renders a section as compact JSON for storage in ticket_types.
func jsonb(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
}

// FormSchemaJSON renders the form schema for storage.
func (p *Pack) FormSchemaJSON() json.RawMessage { return jsonb(p.FormSchema) }

// WorkflowJSON renders the workflow for storage.
func (p *Pack) WorkflowJSON() json.RawMessage { return jsonb(p.Workflow) }

// SLAPolicyJSON renders the SLA policy for storage.
func (p *Pack) SLAPolicyJSON() json.RawMessage { return jsonb(p.SLAPolicy) }

// AssignmentJSON renders the assignment for storage.
func (p *Pack) AssignmentJSON() json.RawMessage { return jsonb(p.Assignment) }

// ViewConfigJSON renders the view config for storage.
func (p *Pack) ViewConfigJSON() json.RawMessage { return jsonb(p.ViewConfig) }

// NormalizeSlug lowercases and trims a slug candidate.
func NormalizeSlug(in string) string {
	return strings.ToLower(strings.TrimSpace(in))
}
