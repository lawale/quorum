package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
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
		for j := range r.Limits {
			l := &r.Limits[j]
			l.Window = strings.TrimSpace(l.Window)
			if d, err := parseWindow(l.Window); err != nil || d <= 0 {
				return fmt.Errorf("%w: rule %d, limit %d: window must be a positive duration such as 24h or 7d", ErrInvalidBypassRule, i, j)
			}
			if l.MaxCount == nil && l.MaxSum == nil {
				return fmt.Errorf("%w: rule %d, limit %d: set max_count, max_sum, or both", ErrInvalidBypassRule, i, j)
			}
			if l.MaxCount != nil && *l.MaxCount < 1 {
				return fmt.Errorf("%w: rule %d, limit %d: max_count must be at least 1", ErrInvalidBypassRule, i, j)
			}
			if l.MaxSum != nil {
				l.MaxSum.Path = strings.TrimSpace(l.MaxSum.Path)
				if l.MaxSum.Path == "" || l.MaxSum.Value < 0 {
					return fmt.Errorf("%w: rule %d, limit %d: max_sum needs a path and a non-negative value", ErrInvalidBypassRule, i, j)
				}
			}
			l.GroupBy = trimList(l.GroupBy)
			for _, g := range l.GroupBy {
				if g != "maker" && !strings.HasPrefix(g, "payload:") {
					return fmt.Errorf("%w: rule %d, limit %d: group_by entries must be \"maker\" or \"payload:<path>\"", ErrInvalidBypassRule, i, j)
				}
				if strings.HasPrefix(g, "payload:") && strings.TrimPrefix(g, "payload:") == "" {
					return fmt.Errorf("%w: rule %d, limit %d: payload group_by needs a path", ErrInvalidBypassRule, i, j)
				}
			}
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

// matchBypass returns the first rule whose conditions and limits the request
// satisfies. When a rule's conditions match but a limit stops it, the first
// such hit is returned alongside a nil rule so the caller can surface it to
// reviewers. Limit evaluation needs recent history; errors there fail safe,
// treating the rule as not matched.
func matchBypass(ctx context.Context, requests store.RequestStore, policy *model.Policy, req *model.Request, makerRoles, makerPermissions []string, makerAttributes map[string]string) (*model.BypassRule, *model.BypassLimitExceeded) {
	var firstHit *model.BypassLimitExceeded
	for _, rule := range matchingBypassRules(policy, req, makerRoles, makerPermissions, makerAttributes) {
		hit, err := limitsExceeded(ctx, requests, rule, req)
		if err != nil {
			slog.WarnContext(ctx, "bypass limit check failed; routing to review", "error", err, "rule", rule.Name, "request_type", req.Type)
			continue
		}
		if hit == nil {
			return rule, firstHit
		}
		if firstHit == nil {
			firstHit = hit
		}
	}
	return nil, firstHit
}

// matchingBypassRules returns, in policy order, every rule whose maker and
// payload conditions hold. Limits are not considered here.
func matchingBypassRules(policy *model.Policy, req *model.Request, makerRoles, makerPermissions []string, makerAttributes map[string]string) []*model.BypassRule {
	if len(policy.BypassRules) == 0 {
		return nil
	}

	var payload map[string]any
	payloadParsed := false
	var matched []*model.BypassRule

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
		matched = append(matched, rule)
	}
	return matched
}

// limitsExceeded measures the maker's recent activity against each limit on
// the rule, including the request being created. It returns the first limit
// that would be exceeded, or nil when all pass.
func limitsExceeded(ctx context.Context, requests store.RequestStore, rule *model.BypassRule, req *model.Request) (*model.BypassLimitExceeded, error) {
	if len(rule.Limits) == 0 {
		return nil, nil
	}

	var newPayload map[string]any
	_ = json.Unmarshal(req.Payload, &newPayload)

	now := time.Now().UTC()
	for _, limit := range rule.Limits {
		window, err := parseWindow(limit.Window)
		if err != nil {
			return nil, fmt.Errorf("limit window %q: %w", limit.Window, err)
		}
		groupBy := limit.GroupBy
		if len(groupBy) == 0 {
			groupBy = []string{"maker"}
		}
		makerFilter := ""
		if slices.Contains(groupBy, "maker") {
			makerFilter = req.MakerID
		}

		recent, err := requests.ListRecentByType(ctx, req.Type, now.Add(-window), makerFilter)
		if err != nil {
			return nil, fmt.Errorf("listing recent requests: %w", err)
		}

		count := 1 // the request being created
		sum := 0.0
		if limit.MaxSum != nil {
			sum = numberAt(newPayload, limit.MaxSum.Path)
		}
		for i := range recent {
			if recent[i].ID == req.ID {
				continue
			}
			var prior map[string]any
			if err := json.Unmarshal(recent[i].Payload, &prior); err != nil {
				prior = nil
			}
			if !sameGroup(groupBy, newPayload, prior) {
				continue
			}
			count++
			if limit.MaxSum != nil {
				sum += numberAt(prior, limit.MaxSum.Path)
			}
		}

		if limit.MaxCount != nil && count > *limit.MaxCount {
			return &model.BypassLimitExceeded{
				Rule: rule.Name, Window: limit.Window, Measure: "count",
				Observed: float64(count), Limit: float64(*limit.MaxCount), GroupBy: groupBy,
			}, nil
		}
		if limit.MaxSum != nil && sum > limit.MaxSum.Value {
			return &model.BypassLimitExceeded{
				Rule: rule.Name, Window: limit.Window, Measure: "sum", Path: limit.MaxSum.Path,
				Observed: sum, Limit: limit.MaxSum.Value, GroupBy: groupBy,
			}, nil
		}
	}
	return nil, nil
}

// sameGroup reports whether a prior request falls in the same group as the
// new one for every payload:<path> key. The maker key is applied in the query.
func sameGroup(groupBy []string, current, prior map[string]any) bool {
	for _, g := range groupBy {
		if !strings.HasPrefix(g, "payload:") {
			continue
		}
		path := strings.TrimPrefix(g, "payload:")
		a, aok := valueAtPath(current, path)
		b, bok := valueAtPath(prior, path)
		if !aok || !bok || !scalarEqual(a, b) {
			return false
		}
	}
	return true
}

// numberAt returns the numeric value at a payload path, or 0 when absent or
// not a number. Numeric strings are accepted since amounts are often quoted.
func numberAt(data map[string]any, path string) float64 {
	v, ok := valueAtPath(data, path)
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
			return f
		}
	}
	return 0
}

// parseWindow parses a Go duration, additionally accepting a trailing "d"
// for whole days (e.g. "7d", "30d").
func parseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty window")
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid day window %q", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return d, nil
}

// stampLimitExceeded records a tripped bypass limit in request metadata so
// the reviewer sees why a normally-bypassed request needs their decision.
func stampLimitExceeded(req *model.Request, hit *model.BypassLimitExceeded) {
	meta := map[string]json.RawMessage{}
	if len(req.Metadata) > 0 {
		if err := json.Unmarshal(req.Metadata, &meta); err != nil {
			meta = map[string]json.RawMessage{}
		}
	}
	b, err := json.Marshal(hit)
	if err != nil {
		return
	}
	meta["bypass_limit_exceeded"] = b
	if merged, err := json.Marshal(meta); err == nil {
		req.Metadata = merged
	}
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
