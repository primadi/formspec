// Package validation provides cross-field validation and action parameter validation
// for Forma entities.
//
// Cross-field rules validate relationships between multiple fields in the same record:
//   - after:<field> — value must be after another field's value
//   - before:<field> — value must be before another field's value
//   - exists:<resource> — value must reference an existing record
//
// Action param validation validates input parameters against declared action rules.
package validation

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/forma/forma/pkg/spec"
)

// ============================================================================
// Cross-Field Validators
// ============================================================================

// ValidateCrossField checks a cross-field validation rule against a value,
// with access to the full data map for field references.
//
// Supported rule names:
//   - "after": value must be after the field specified in rule.Value
//   - "before": value must be before the field specified in rule.Value
//   - "exists": value references an existing record (by entity+ID in rule.Value)
func ValidateCrossField(fieldName string, val any, rule spec.ValidationRule, data map[string]any) error {
	switch rule.Name {
	case "after", "after_field":
		refField, ok := rule.Value.(string)
		if !ok {
			return fmt.Errorf("after rule for %q: value must be a field name string", fieldName)
		}
		refVal, exists := data[refField]
		if !exists {
			return nil // skip if reference field is empty
		}
		return compareDateTime(fieldName, val, refField, refVal, true)

	case "before", "before_field":
		refField, ok := rule.Value.(string)
		if !ok {
			return fmt.Errorf("before rule for %q: value must be a field name string", fieldName)
		}
		refVal, exists := data[refField]
		if !exists {
			return nil // skip if reference field is empty
		}
		return compareDateTime(fieldName, val, refField, refVal, false)

	case "exists":
		// Format: {exists: billing.product} or just "product" with module prefix
		target, ok := rule.Value.(string)
		if !ok {
			return fmt.Errorf("exists rule for %q: value must be a resource reference", fieldName)
		}
		_ = target
		// TODO(Fase 2): actual DB query to check record existence
		// For now, accept all values (stub)
		return nil
	}
	return nil
}

// compareDateTime compares two datetime fields.
// If after=true, checks that val is after refVal.
// If after=false, checks that val is before refVal.
func compareDateTime(fieldName string, val any, refField string, refVal any, after bool) error {
	valStr, ok := val.(string)
	if !ok {
		return nil // skip non-string values
	}
	refStr, ok := refVal.(string)
	if !ok {
		return nil // skip if reference is not a string
	}

	valTime, err := parseDateTime(valStr)
	if err != nil {
		return nil // skip if can't parse
	}
	refTime, err := parseDateTime(refStr)
	if err != nil {
		return nil // skip if ref can't parse
	}

	if after {
		if !valTime.After(refTime) {
			return fmt.Errorf("%q must be after %q (%s)", fieldName, refField, refStr)
		}
	} else {
		if !valTime.Before(refTime) {
			return fmt.Errorf("%q must be before %q (%s)", fieldName, refField, refStr)
		}
	}
	return nil
}

// parseDateTime attempts to parse a datetime string.
func parseDateTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as datetime", s)
}

// ============================================================================
// Action Param Validation
// ============================================================================

// ValidateActionParams validates action input parameters against their declared rules.
//
// Parameters:
//   - params: the input parameter values
//   - validate: the declared validation rules from action.Params.Validate
//
// Returns a list of validation errors, one per failed rule.
// An empty slice means all validations passed.
func ValidateActionParams(params map[string]any, validate []spec.ParamValidation) []error {
	var errs []error

	for _, pv := range validate {
		val, exists := params[pv.Field]

		for _, rule := range pv.Rules {
			// Handle required/optional presence checks
			if rule.Name == "required" {
				if !exists || val == nil || val == "" {
					errs = append(errs, fmt.Errorf("%s: required", pv.Field))
				}
				continue
			}

			// Skip remaining rules if value is empty
			if !exists || val == nil {
				continue
			}

			// Delegate to same validation rules as field validators
			if err := validateActionParamRule(pv.Field, val, rule); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

// validateActionParamRule validates a single rule against an action parameter value.
// Supports all Core Basic field validation rules (§10.6).
func validateActionParamRule(fieldName string, val any, rule spec.ValidationRule) error {
	switch rule.Name {
	case "min_length", "max_length", "min", "max", "positive", "email", "pattern", "url",
		"precision", "future", "past", "min_items", "max_items":
		// Reuse: delegate to applyInlineRule
		return applyInlineRule(fieldName, val, rule)
	default:
		return fmt.Errorf("unsupported action param rule: %s", rule.Name)
	}
}

// applyInlineRule applies a single rule to a value inline.
func applyInlineRule(fieldName string, val any, rule spec.ValidationRule) error {
	switch rule.Name {
	case "min_length":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%s: must be a string", fieldName)
		}
		minLen := toInt(rule.Value)
		if len(str) < minLen {
			return fmt.Errorf("%s: minimum length %d, got %d", fieldName, minLen, len(str))
		}
	case "max_length":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%s: must be a string", fieldName)
		}
		maxLen := toInt(rule.Value)
		if len(str) > maxLen {
			return fmt.Errorf("%s: maximum length %d, got %d", fieldName, maxLen, len(str))
		}
	case "min":
		num := toFloat(val)
		minVal := toFloat(rule.Value)
		if num < minVal {
			return fmt.Errorf("%s: minimum value %v", fieldName, minVal)
		}
	case "max":
		num := toFloat(val)
		maxVal := toFloat(rule.Value)
		if num > maxVal {
			return fmt.Errorf("%s: maximum value %v", fieldName, maxVal)
		}
	case "positive":
		num := toFloat(val)
		if num <= 0 {
			return fmt.Errorf("%s: must be positive", fieldName)
		}
	case "email":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%s: must be a string", fieldName)
		}
		if !emailRegex.MatchString(str) {
			return fmt.Errorf("%s: invalid email format", fieldName)
		}
	case "pattern":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%s: must be a string", fieldName)
		}
		pattern, ok := rule.Value.(string)
		if !ok {
			return fmt.Errorf("%s: pattern must be a string regex", fieldName)
		}
		matched, err := regexp.MatchString(pattern, str)
		if err != nil {
			return fmt.Errorf("%s: invalid regex: %v", fieldName, err)
		}
		if !matched {
			return fmt.Errorf("%s: does not match pattern %q", fieldName, pattern)
		}
	case "url":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%s: must be a string", fieldName)
		}
		if !urlRegex.MatchString(str) {
			return fmt.Errorf("%s: invalid URL format", fieldName)
		}
	case "precision":
		num := toFloat(val)
		prec := toInt(rule.Value)
		if prec < 0 {
			return fmt.Errorf("%s: precision must be non-negative", fieldName)
		}
		dp := countDecimalPlaces(num)
		if dp > prec {
			return fmt.Errorf("%s: max %d decimal places, got %d", fieldName, prec, dp)
		}
	case "future":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%s: must be a datetime string", fieldName)
		}
		t, err := parseDateTime(str)
		if err != nil {
			return fmt.Errorf("%s: invalid datetime: %v", fieldName, err)
		}
		if !t.After(time.Now().UTC()) {
			return fmt.Errorf("%s: must be in the future", fieldName)
		}
	case "past":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%s: must be a datetime string", fieldName)
		}
		t, err := parseDateTime(str)
		if err != nil {
			return fmt.Errorf("%s: invalid datetime: %v", fieldName, err)
		}
		if !t.Before(time.Now().UTC()) {
			return fmt.Errorf("%s: must be in the past", fieldName)
		}
	case "min_items":
		items, ok := val.([]any)
		if !ok {
			return fmt.Errorf("%s: must be an array", fieldName)
		}
		minLen := toInt(rule.Value)
		if len(items) < minLen {
			return fmt.Errorf("%s: min %d items, got %d", fieldName, minLen, len(items))
		}
	case "max_items":
		items, ok := val.([]any)
		if !ok {
			return fmt.Errorf("%s: must be an array", fieldName)
		}
		maxLen := toInt(rule.Value)
		if len(items) > maxLen {
			return fmt.Errorf("%s: max %d items, got %d", fieldName, maxLen, len(items))
		}
	}
	return nil
}

// toInt converts a value to int.
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// toFloat converts a value to float64.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var urlRegex = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)

// countDecimalPlaces returns the number of decimal places in a float64 value
// using string-based counting to avoid floating-point precision issues.
func countDecimalPlaces(num float64) int {
	s := fmt.Sprintf("%.10f", num)
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	if dotIdx := strings.Index(s, "."); dotIdx >= 0 {
		return len(s) - dotIdx - 1
	}
	return 0
}
