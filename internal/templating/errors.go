package templating

import (
	"fmt"
	"strings"
)

// ValidationError carries field-attributed validation failures. The HTTP
// layer maps it to VALIDATION_FAILED with a per-field English message
// (issue #12).
type ValidationError struct {
	items []string
}

// Add records a failure for a field path.
func (ve *ValidationError) Add(field, reason string) {
	ve.items = append(ve.items, fmt.Sprintf("%s: %s", field, reason))
}

// Merge absorbs another ValidationError.
func (ve *ValidationError) Merge(other *ValidationError) {
	if other != nil {
		ve.items = append(ve.items, other.items...)
	}
}

// Empty reports whether no failure was recorded.
func (ve *ValidationError) Empty() bool {
	return len(ve.items) == 0
}

// Error implements error with one line per failure.
func (ve *ValidationError) Error() string {
	return strings.Join(ve.items, "; ")
}
