package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lawale/quorum/internal/model"
)

var bypassOps = map[string]bool{"eq": true, "ne": true, "lt": true, "lte": true, "gt": true, "gte": true, "in": true}

// validateBypassRules normalises and checks a policy's bypass rules. A rule
// must constrain something, every payload condition needs a path and a known
// operator, ordering operators need a numeric value, and "in" needs an array.
func validateBypassRules(rules []model.BypassRule) error {
	for i := range rules {
		r := &rules[i]
		r.Name = strings.TrimSpace(r.Name)
		r.MakerRoles = trimList(r.MakerRoles)
		r.MakerPermissions = trimList(r.MakerPermissions)
		for attr, vals := range r.MakerAttributes {
			cleaned := trimList(vals)
			if strings.TrimSpace(attr) == "" || len(cleaned) == 0 {
				return fmt.Errorf("%w: rule %d: maker_attributes entries need a name and at least one value", ErrInvalidBypassRule, i)
			}
			r.MakerAttributes[attr] = cleaned
		}
		if !r.HasCondition() {
			return fmt.Errorf("%w: rule %d has no conditions and would bypass every request", ErrInvalidBypassRule, i)
		}
		for j := range r.Payload {
			c := &r.Payload[j]
			c.Path = strings.TrimSpace(c.Path)
			c.Op = strings.ToLower(strings.TrimSpace(c.Op))
			if c.Path == "" {
				return fmt.Errorf("%w: rule %d, condition %d: path is required", ErrInvalidBypassRule, i, j)
			}
			if !bypassOps[c.Op] {
				return fmt.Errorf("%w: rule %d, condition %d: unknown op %q (valid: eq, ne, lt, lte, gt, gte, in)", ErrInvalidBypassRule, i, j, c.Op)
			}
			if len(c.Value) == 0 {
				return fmt.Errorf("%w: rule %d, condition %d: value is required", ErrInvalidBypassRule, i, j)
			}
			var v any
			if err := json.Unmarshal(c.Value, &v); err != nil {
				return fmt.Errorf("%w: rule %d, condition %d: value is not valid JSON", ErrInvalidBypassRule, i, j)
			}
			switch c.Op {
			case "lt", "lte", "gt", "gte":
				if _, ok := v.(float64); !ok {
					return fmt.Errorf("%w: rule %d, condition %d: %s requires a numeric value", ErrInvalidBypassRule, i, j, c.Op)
				}
			case "in":
				if _, ok := v.([]any); !ok {
					return fmt.Errorf("%w: rule %d, condition %d: in requires an array value", ErrInvalidBypassRule, i, j)
				}
			}
		}
	}
	return nil
}

func trimList(in []string) []string {
	out := in[:0]
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// matchBypass returns the first rule the maker and payload satisfy, or nil.
func matchBypass(policy *model.Policy, req *model.Request, makerRoles, makerPermissions []string, makerAttributes map[string]string) *model.BypassRule {
	if len(policy.BypassRules) == 0 {
		return nil
	}

	var payload map[string]any
	payloadParsed := false

	for i := range policy.BypassRules {
		rule := &policy.BypassRules[i]
		if !rule.HasCondition() {
			continue // never bypass on an unconstrained rule, whatever is stored
		}
		if len(rule.MakerRoles) > 0 && !anyIn(rule.MakerRoles, makerRoles) {
			continue
		}
		if len(rule.MakerPermissions) > 0 && !anyIn(rule.MakerPermissions, makerPermissions) {
			continue
		}
		attrsOK := true
		for attr, allowed := range rule.MakerAttributes {
			v, ok := makerAttributes[attr]
			if !ok || !slices.Contains(allowed, v) {
				attrsOK = false
				break
			}
		}
		if !attrsOK {
			continue
		}
		if len(rule.Payload) > 0 {
			if !payloadParsed {
				payloadParsed = true
				if err := json.Unmarshal(req.Payload, &payload); err != nil {
					payload = nil
				}
			}
			if !allConditionsHold(rule.Payload, payload) {
				continue
			}
		}
		return rule
	}
	return nil
}

func anyIn(wanted, have []string) bool {
	for _, w := range wanted {
		if slices.Contains(have, w) {
			return true
		}
	}
	return false
}

func allConditionsHold(conds []model.BypassCondition, payload map[string]any) bool {
	for _, c := range conds {
		if !conditionHolds(c, payload) {
			return false
		}
	}
	return true
}

// conditionHolds evaluates one payload condition. A missing path never
// satisfies a condition, including "ne": bypass must be earned by a value
// that is present.
func conditionHolds(c model.BypassCondition, payload map[string]any) bool {
	actual, ok := valueAtPath(payload, c.Path)
	if !ok {
		return false
	}
	var expected any
	if err := json.Unmarshal(c.Value, &expected); err != nil {
		return false
	}

	switch c.Op {
	case "eq":
		return scalarEqual(actual, expected)
	case "ne":
		return !scalarEqual(actual, expected)
	case "in":
		list, ok := expected.([]any)
		if !ok {
			return false
		}
		for _, item := range list {
			if scalarEqual(actual, item) {
				return true
			}
		}
		return false
	case "lt", "lte", "gt", "gte":
		a, aok := actual.(float64)
		e, eok := expected.(float64)
		if !aok || !eok {
			return false
		}
		switch c.Op {
		case "lt":
			return a < e
		case "lte":
			return a <= e
		case "gt":
			return a > e
		default:
			return a >= e
		}
	}
	return false
}

// scalarEqual compares two decoded JSON scalars. Objects and arrays never match.
func scalarEqual(a, b any) bool {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case nil:
		return b == nil
	}
	return false
}

// valueAtPath walks a dot-notation path into a decoded JSON object.
func valueAtPath(data map[string]any, path string) (any, bool) {
	if data == nil {
		return nil, false
	}
	var current any = data
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
