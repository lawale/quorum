// Package display provides template resolution for converting raw payloads
// into human-readable display data using a simple template schema.
package display

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

// Template defines a display template stored on a policy.
// It maps payload fields to human-readable labels with optional formatting.
type Template struct {
	Title  string          `json:"title,omitempty"`
	Fields []TemplateField `json:"fields"`
	Items  *TemplateItems  `json:"items,omitempty"`
}

// TemplateField maps a label to a dot-notation path into the payload.
type TemplateField struct {
	Label  string `json:"label"`
	Path   string `json:"path"`
	Format string `json:"format,omitempty"`
}

// TemplateItems defines how to render a list of items from an array in the payload.
type TemplateItems struct {
	Path      string          `json:"path"`
	LabelPath string          `json:"label_path"`
	Fields    []TemplateField `json:"fields"`
}

// Resolved is the output stored in metadata.display after template resolution.
type Resolved struct {
	Title  string          `json:"title,omitempty"`
	Fields []ResolvedField `json:"fields"`
	Items  []ResolvedItem  `json:"items,omitempty"`
}

// ResolvedField is a label-value pair.
type ResolvedField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ResolvedItem is a titled group of label-value pairs (for batch requests).
type ResolvedItem struct {
	Title  string          `json:"title"`
	Fields []ResolvedField `json:"fields"`
}

var placeholderRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Resolve applies a display template to a payload and returns the resolved output.
// If the template is nil or empty, it returns nil.
func Resolve(tmpl json.RawMessage, payload json.RawMessage) (json.RawMessage, error) {
	if len(tmpl) == 0 || string(tmpl) == "null" {
		return nil, nil
	}

	var t Template
	if err := json.Unmarshal(tmpl, &t); err != nil {
		return nil, fmt.Errorf("parsing display template: %w", err)
	}

	if len(t.Fields) == 0 && t.Title == "" {
		return nil, nil
	}

	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("parsing payload: %w", err)
	}

	resolved := Resolved{
		Title:  resolveTitle(t.Title, data),
		Fields: resolveFields(t.Fields, data),
	}

	if t.Items != nil {
		resolved.Items = resolveItems(t.Items, data)
	}

	out, err := json.Marshal(resolved)
	if err != nil {
		return nil, fmt.Errorf("marshaling resolved display: %w", err)
	}

	return out, nil
}

func resolveTitle(title string, data map[string]any) string {
	if title == "" {
		return ""
	}

	return placeholderRe.ReplaceAllStringFunc(title, func(match string) string {
		inner := strings.TrimSpace(match[2 : len(match)-2])

		// Check for pipe-separated formatter: {{path | format}}
		parts := strings.SplitN(inner, "|", 2)
		path := strings.TrimSpace(parts[0])
		var format string
		if len(parts) == 2 {
			format = strings.TrimSpace(parts[1])
		}

		val := resolvePath(data, path)
		if val == nil {
			return "-"
		}

		if format != "" {
			return formatValue(val, format)
		}
		return fmt.Sprintf("%v", val)
	})
}

func resolveFields(fields []TemplateField, data map[string]any) []ResolvedField {
	result := make([]ResolvedField, len(fields))
	for i, f := range fields {
		val := resolvePath(data, f.Path)
		result[i] = ResolvedField{
			Label: f.Label,
			Value: formatResolved(val, f.Format),
		}
	}
	return result
}

func resolveItems(items *TemplateItems, data map[string]any) []ResolvedItem {
	arr := resolvePathAsSlice(data, items.Path)
	if arr == nil {
		return nil
	}

	result := make([]ResolvedItem, len(arr))
	for i, item := range arr {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		title := "-"
		if items.LabelPath != "" {
			if v := resolvePath(itemMap, items.LabelPath); v != nil {
				title = fmt.Sprintf("%v", v)
			}
		}

		result[i] = ResolvedItem{
			Title:  title,
			Fields: resolveFields(items.Fields, itemMap),
		}
	}

	return result
}

// resolvePath walks a dot-notation path into a nested map.
// Supports paths like "field", "nested.field", "deep.nested.field".
func resolvePath(data map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var current any = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

// resolvePathAsSlice resolves a path and returns it as a slice, or nil.
func resolvePathAsSlice(data map[string]any, path string) []any {
	val := resolvePath(data, path)
	if val == nil {
		return nil
	}

	arr, ok := val.([]any)
	if !ok {
		return nil
	}
	return arr
}

func formatResolved(val any, format string) string {
	if val == nil {
		return "-"
	}
	if format == "" {
		return fmt.Sprintf("%v", val)
	}
	return formatValue(val, format)
}

// formatValue applies a built-in formatter to a value.
// Supported formatters: currency, date, number, truncate.
func formatValue(val any, format string) string {
	switch format {
	case "currency":
		return formatCurrency(val)
	case "date":
		return formatDate(val)
	case "number":
		return formatNumber(val)
	case "truncate":
		return formatTruncate(val, 50)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatCurrency(val any) string {
	switch v := val.(type) {
	case float64:
		if v == math.Trunc(v) {
			return fmt.Sprintf("$%s", formatNumberWithCommas(int64(v)))
		}
		return fmt.Sprintf("$%s", formatFloatWithCommas(v))
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return fmt.Sprintf("$%s", v.String())
		}
		return formatCurrency(f)
	default:
		return fmt.Sprintf("$%v", val)
	}
}

func formatDate(val any) string {
	s, ok := val.(string)
	if !ok {
		return fmt.Sprintf("%v", val)
	}

	// Try common formats
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("Jan 2, 2006")
		}
	}
	return s
}

func formatNumber(val any) string {
	switch v := val.(type) {
	case float64:
		if v == math.Trunc(v) {
			return formatNumberWithCommas(int64(v))
		}
		return formatFloatWithCommas(v)
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return v.String()
		}
		return formatNumber(f)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatTruncate(val any, maxLen int) string {
	s := fmt.Sprintf("%v", val)
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func formatNumberWithCommas(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		return "-" + addCommas(s[1:])
	}
	return addCommas(s)
}

func formatFloatWithCommas(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	parts := strings.SplitN(s, ".", 2)
	integer := parts[0]
	decimal := parts[1]

	if strings.HasPrefix(integer, "-") {
		return "-" + addCommas(integer[1:]) + "." + decimal
	}
	return addCommas(integer) + "." + decimal
}

func addCommas(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}

	var result strings.Builder
	remainder := n % 3
	if remainder > 0 {
		result.WriteString(s[:remainder])
		if n > remainder {
			result.WriteByte(',')
		}
	}

	for i := remainder; i < n; i += 3 {
		if i > remainder {
			result.WriteByte(',')
		}
		result.WriteString(s[i : i+3])
	}

	return result.String()
}
