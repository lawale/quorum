package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lawale/quorum/internal/model"
)

// validateScope enforces the stage's scope rules against the checker's
// identity attributes. Each rule names an attribute the checker must hold with
// a value equal to the maker's attribute of the same name, or, when
// payload_path is set, to the value at that path in the request payload.
//
// Rules fail closed: if the expected value cannot be determined, or the checker
// lacks the attribute and the rule does not allow unscoped checkers, the
// checker is out of scope.
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

		var expected string
		var ok bool
		if rule.PayloadPath != "" {
			if !payloadParsed {
				payloadParsed = true
				if err := json.Unmarshal(req.Payload, &payload); err != nil {
					payload = nil
				}
			}
			expected, ok = scalarAtPath(payload, rule.PayloadPath)
		} else {
			expected, ok = req.MakerAttributes[rule.Attribute]
			ok = ok && expected != ""
		}
		if !ok {
			return fmt.Errorf("%w: request has no %q to match against", ErrOutOfScope, rule.Attribute)
		}

		if checkerValue != expected {
			return fmt.Errorf("%w: %s mismatch", ErrOutOfScope, rule.Attribute)
		}
	}
	return nil
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
