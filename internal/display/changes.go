package display

import (
	"reflect"
	"sort"
	"strings"
)

// resolveChanges compares the current and proposed snapshots named by the
// template and returns one ResolvedChange per compared attribute. Either
// snapshot may be absent: a creation has no current state and a deletion has
// no proposed state, in which case the missing side renders as "-".
func resolveChanges(tc *TemplateChanges, data map[string]any) []ResolvedChange {
	current := snapshotAt(data, tc.CurrentPath)
	proposed := snapshotAt(data, tc.ProposedPath)
	if current == nil && proposed == nil {
		return nil
	}

	fields := tc.Fields
	if len(fields) == 0 {
		fields = discoverFields(current, proposed)
	}

	result := make([]ResolvedChange, 0, len(fields))
	for _, f := range fields {
		from := resolvePath(current, f.Path)
		to := resolvePath(proposed, f.Path)
		result = append(result, ResolvedChange{
			Label:   f.Label,
			From:    formatResolved(from, f.Format),
			To:      formatResolved(to, f.Format),
			Changed: !reflect.DeepEqual(from, to),
		})
	}
	return result
}

// snapshotAt resolves a path to an object, or nil when the path is missing or
// does not point at an object. A nil map is safe to pass to resolvePath.
func snapshotAt(data map[string]any, path string) map[string]any {
	val := resolvePath(data, path)
	if val == nil {
		return nil
	}
	m, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	return m
}

// discoverFields builds a field list from the union of leaf paths present in
// either snapshot, sorted for stable output. Labels are derived from the path.
func discoverFields(current, proposed map[string]any) []TemplateField {
	seen := make(map[string]struct{})
	collectLeafPaths(current, "", seen)
	collectLeafPaths(proposed, "", seen)

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	fields := make([]TemplateField, len(paths))
	for i, p := range paths {
		fields[i] = TemplateField{Label: labelFromPath(p), Path: p}
	}
	return fields
}

// collectLeafPaths walks nested objects and records every dot-notation path
// that ends at a non-object value. Arrays are treated as leaves.
func collectLeafPaths(m map[string]any, prefix string, out map[string]struct{}) {
	for k, v := range m {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if child, ok := v.(map[string]any); ok && len(child) > 0 {
			collectLeafPaths(child, path, out)
			continue
		}
		out[path] = struct{}{}
	}
}

// labelFromPath turns "billing.account_id" into "Billing / Account Id".
func labelFromPath(path string) string {
	segments := strings.Split(path, ".")
	for i, seg := range segments {
		words := strings.FieldsFunc(seg, func(r rune) bool { return r == '_' || r == '-' })
		for j, w := range words {
			if w == "" {
				continue
			}
			words[j] = strings.ToUpper(w[:1]) + w[1:]
		}
		segments[i] = strings.Join(words, " ")
	}
	return strings.Join(segments, " / ")
}
