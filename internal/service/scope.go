package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lawale/quorum/internal/model"
)

// validateScope enforces the stage's scope rules against the checker's
// identity attributes. Each rule names an attribute and one or more sources of
// acceptable values: the maker's attribute of the same name, a path into the
// payload, and fixed values. The checker passes a rule when their attribute
// equals any value the sources produce. All rules must pass.
//
// Rules fail closed: if no source yields a value, or the checker lacks the
// attribute and the rule does not allow unscoped checkers, the checker is out
// of scope.
func validateScope(stage *model.ApprovalStage, req *model.Request, checkerAttributes map[string]string) error {
	if len(stage.Scope) == 0 {
		return nil
	}

	var payload map[string]any
	payloadParsed := false

	for _, rule := range stage.Scope {
		if rule.Attribute == "" {
			continue
		}

		checkerValue, checkerHas := checkerAttributes[rule.Attribute]
		if !checkerHas || checkerValue == "" {
			if rule.AllowUnscopedCheckers {
				continue
			}
			return fmt.Errorf("%w: checker has no %q attribute", ErrOutOfScope, rule.Attribute)
		}

		if !payloadParsed && rule.PayloadPath != "" {
			payloadParsed = true
			if err := json.Unmarshal(req.Payload, &payload); err != nil {
				payload = nil
			}
		}
		expected := scopeExpectedValues(rule, req, payload)

		if len(expected) == 0 {
			return fmt.Errorf("%w: request has no %q to match against", ErrOutOfScope, rule.Attribute)
		}

		matched := false
		for _, v := range expected {
			if v == checkerValue {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%w: %s mismatch", ErrOutOfScope, rule.Attribute)
		}
	}
	return nil
}

// scopeExpectedValues resolves the values a checker's attribute may take for
// this rule and request: the maker's attribute, the payload value, and any
// fixed values. A rule with no explicit source matches the maker.
func scopeExpectedValues(rule model.ScopeRule, req *model.Request, payload map[string]any) []string {
	var expected []string
	if rule.MatchMaker || !rule.HasSource() {
		if v, ok := req.MakerAttributes[rule.Attribute]; ok && v != "" {
			expected = append(expected, v)
		}
	}
	if rule.PayloadPath != "" {
		if v, ok := scalarAtPath(payload, rule.PayloadPath); ok {
			expected = append(expected, v)
		}
	}
	return append(expected, rule.Values...)
}

// scalarAtPath walks a dot-notation path into a decoded JSON object and
// returns the scalar found there as a string. Objects and arrays yield false.
func scalarAtPath(data map[string]any, path string) (string, bool) {
	if data == nil {
		return "", false
	}
	var current any = data
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = m[part]
		if !ok {
			return "", false
		}
	}
	switch v := current.(type) {
	case string:
		return v, v != ""
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(v), true
	default:
		return "", false
	}
}
