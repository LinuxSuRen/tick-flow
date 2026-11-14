// Package templating validates and interprets the template pack payloads
// stored in ticket_types JSONB columns (issue #4). Both the admin API and
// the seed importer run every pack through Validate before persisting, and
// the ticket service uses the same types at runtime for form validation and
// the workflow state machine.
package templating

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Field types of the form schema subset (issue #4).
const (
	TypeString      = "string"
	TypeTextarea    = "textarea"
	TypeNumber      = "number"
	TypeSelect      = "select"
	TypeMultiselect = "multiselect"
	TypeBoolean     = "boolean"
	TypeDate        = "date"
	TypeDatetime    = "datetime"
	TypeAttachments = "attachments"
)

var fieldTypes = map[string]struct{}{
	TypeString: {}, TypeTextarea: {}, TypeNumber: {}, TypeSelect: {},
	TypeMultiselect: {}, TypeBoolean: {}, TypeDate: {}, TypeDatetime: {},
	TypeAttachments: {},
}

var snakeCaseRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// FieldOption is one selectable option.
type FieldOption struct {
	Value string `json:"value" yaml:"value"`
	Label string `json:"label" yaml:"label"`
}

// Field is one form field definition.
type Field struct {
	Key         string        `json:"key" yaml:"key"`
	Label       string        `json:"label" yaml:"label"`
	Type        string        `json:"type" yaml:"type"`
	Placeholder string        `json:"placeholder" yaml:"placeholder"`
	Required    bool          `json:"required" yaml:"required"`
	MaxLength   *int          `json:"max_length" yaml:"max_length"`
	Min         *float64      `json:"min" yaml:"min"`
	Max         *float64      `json:"max" yaml:"max"`
	Pattern     string        `json:"pattern" yaml:"pattern"`
	Options     []FieldOption `json:"options" yaml:"options"`
	MaxCount    *int          `json:"max_count" yaml:"max_count"`
}

// FormSchema is the restricted JSON-Schema-like form definition.
type FormSchema struct {
	Fields []Field `json:"fields" yaml:"fields"`
}

// ParseFormSchema decodes JSON with unknown fields rejected.
func ParseFormSchema(raw []byte) (*FormSchema, error) {
	fs := &FormSchema{}
	if err := unmarshalStrict(raw, fs); err != nil {
		return nil, err
	}
	return fs, nil
}

// Validate checks the schema itself (issue #4 save-time rules).
func (fs *FormSchema) Validate() *ValidationError {
	ve := &ValidationError{}
	seen := map[string]struct{}{}
	for i, f := range fs.Fields {
		p := fmt.Sprintf("fields[%d]", i)
		if f.Key == "" {
			ve.Add(p+".key", "key is required")
		} else if !snakeCaseRe.MatchString(f.Key) {
			ve.Add(p+".key", fmt.Sprintf("key %q must be english snake_case", f.Key))
		} else if _, dup := seen[f.Key]; dup {
			ve.Add(p+".key", fmt.Sprintf("duplicate key %q", f.Key))
		} else {
			seen[f.Key] = struct{}{}
		}
		if f.Label == "" {
			ve.Add(p+".label", "label is required")
		}
		if _, ok := fieldTypes[f.Type]; !ok {
			ve.Add(p+".type", fmt.Sprintf("unknown field type %q", f.Type))
		}
		switch f.Type {
		case TypeSelect, TypeMultiselect:
			if len(f.Options) == 0 {
				ve.Add(p+".options", "select fields require options")
			}
			values := map[string]struct{}{}
			for j, opt := range f.Options {
				if opt.Value == "" {
					ve.Add(fmt.Sprintf("%s.options[%d].value", p, j), "option value is required")
					continue
				}
				if _, dup := values[opt.Value]; dup {
					ve.Add(fmt.Sprintf("%s.options[%d].value", p, j), fmt.Sprintf("duplicate option value %q", opt.Value))
				}
				values[opt.Value] = struct{}{}
			}
		case TypeNumber:
			if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
				ve.Add(p+".min", "min must not exceed max")
			}
		}
		// validation keys are only legal for relevant types
		if f.MaxLength != nil && f.Type != TypeString && f.Type != TypeTextarea {
			ve.Add(p+".max_length", "max_length only applies to string and textarea fields")
		}
		if (f.Min != nil || f.Max != nil) && f.Type != TypeNumber {
			ve.Add(p+".min", "min and max only apply to number fields")
		}
		if f.Pattern != "" && f.Type != TypeString && f.Type != TypeTextarea {
			ve.Add(p+".pattern", "pattern only applies to string and textarea fields")
		}
		if f.MaxCount != nil && f.Type != TypeAttachments {
			ve.Add(p+".max_count", "max_count only applies to attachments fields")
		}
		if f.Pattern != "" {
			if _, err := regexp.Compile(f.Pattern); err != nil {
				ve.Add(p+".pattern", fmt.Sprintf("invalid pattern: %v", err))
			}
		}
	}
	return ve
}

// FieldByKey returns the field definition for key.
func (fs *FormSchema) FieldByKey(key string) *Field {
	for i := range fs.Fields {
		if fs.Fields[i].Key == key {
			return &fs.Fields[i]
		}
	}
	return nil
}

// ValidateData validates submitted custom field values against the schema.
// Missing optional keys are fine; unknown keys are rejected so the stored
// JSONB never drifts from the schema (issue #2 constraint 3).
func (fs *FormSchema) ValidateData(data map[string]any) *ValidationError {
	ve := &ValidationError{}
	for key, val := range data {
		field := fs.FieldByKey(key)
		if field == nil {
			ve.Add("custom_fields."+key, "unknown field for this ticket type")
			continue
		}
		ve.Merge(validateValue("custom_fields."+key, field, val))
	}
	for _, f := range fs.Fields {
		if _, present := data[f.Key]; !present && f.Required {
			ve.Add("custom_fields."+f.Key, "field is required")
		}
	}
	return ve
}

func validateValue(path string, f *Field, val any) *ValidationError {
	ve := &ValidationError{}
	if val == nil {
		if f.Required {
			ve.Add(path, "field is required")
		}
		return ve
	}
	switch f.Type {
	case TypeString, TypeTextarea:
		s, ok := val.(string)
		if !ok {
			ve.Add(path, "must be a string")
			return ve
		}
		if f.Required && strings.TrimSpace(s) == "" {
			ve.Add(path, "field is required")
		}
		if f.MaxLength != nil && len([]rune(s)) > *f.MaxLength {
			ve.Add(path, fmt.Sprintf("length must not exceed %d", *f.MaxLength))
		}
		if f.Pattern != "" && s != "" {
			re, err := regexp.Compile(f.Pattern)
			if err == nil && !re.MatchString(s) {
				ve.Add(path, fmt.Sprintf("must match pattern %s", f.Pattern))
			}
		}
	case TypeNumber:
		n, ok := toFloat(val)
		if !ok {
			ve.Add(path, "must be a number")
			return ve
		}
		if f.Min != nil && n < *f.Min {
			ve.Add(path, fmt.Sprintf("must be >= %v", *f.Min))
		}
		if f.Max != nil && n > *f.Max {
			ve.Add(path, fmt.Sprintf("must be <= %v", *f.Max))
		}
	case TypeSelect:
		s, ok := val.(string)
		if !ok {
			ve.Add(path, "must be a string")
			return ve
		}
		if !hasOption(f.Options, s) {
			ve.Add(path, fmt.Sprintf("value %q is not a valid option", s))
		}
	case TypeMultiselect:
		arr, ok := val.([]any)
		if !ok {
			ve.Add(path, "must be an array of strings")
			return ve
		}
		for _, item := range arr {
			s, ok := item.(string)
			if !ok || !hasOption(f.Options, s) {
				ve.Add(path, fmt.Sprintf("value %v is not a valid option", item))
			}
		}
	case TypeBoolean:
		if _, ok := val.(bool); !ok {
			ve.Add(path, "must be a boolean")
		}
	case TypeDate:
		s, ok := val.(string)
		if !ok {
			ve.Add(path, "must be a date string")
			return ve
		}
		if !dateRe.MatchString(s) {
			ve.Add(path, "must be formatted as yyyy-MM-dd")
		}
	case TypeDatetime:
		s, ok := val.(string)
		if !ok {
			ve.Add(path, "must be a datetime string")
			return ve
		}
		if !datetimeRe.MatchString(s) {
			ve.Add(path, "must be an ISO 8601 datetime")
		}
	case TypeAttachments:
		arr, ok := val.([]any)
		if !ok {
			ve.Add(path, "must be an array of attachment ids")
			return ve
		}
		if f.MaxCount != nil && len(arr) > *f.MaxCount {
			ve.Add(path, fmt.Sprintf("must not contain more than %d items", *f.MaxCount))
		}
		for _, item := range arr {
			if _, ok := item.(float64); !ok {
				ve.Add(path, "attachment references must be numeric ids")
			}
		}
	}
	return ve
}

var (
	dateRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	datetimeRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:\d{2})?)?$`)
)

func hasOption(opts []FieldOption, value string) bool {
	for _, o := range opts {
		if o.Value == value {
			return true
		}
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, false
		}
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// unmarshalStrict decodes JSON rejecting unknown keys, so the "unknown
// validation key" rule of issue #4 is enforced at parse time.
func unmarshalStrict(raw []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("strict json decode: %w", err)
	}
	return nil
}
