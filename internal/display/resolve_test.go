package display

import (
	"encoding/json"
	"testing"
)

func TestResolve_SimpleFields(t *testing.T) {
	tmpl := json.RawMessage(`{
		"title": "Wire Transfer - {{amount | currency}}",
		"fields": [
			{"label": "From Account", "path": "source_account_id"},
			{"label": "Amount", "path": "amount", "format": "currency"},
			{"label": "Destination", "path": "destination"}
		]
	}`)

	payload := json.RawMessage(`{
		"source_account_id": "ACC-001",
		"amount": 50000,
		"destination": "IBAN-12345"
	}`)

	result, err := Resolve(tmpl, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	var resolved Resolved
	if err := json.Unmarshal(result, &resolved); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if resolved.Title != "Wire Transfer - $50,000" {
		t.Errorf("title = %q, want %q", resolved.Title, "Wire Transfer - $50,000")
	}

	if len(resolved.Fields) != 3 {
		t.Fatalf("got %d fields, want 3", len(resolved.Fields))
	}

	if resolved.Fields[0].Label != "From Account" || resolved.Fields[0].Value != "ACC-001" {
		t.Errorf("field[0] = %+v", resolved.Fields[0])
	}
	if resolved.Fields[1].Label != "Amount" || resolved.Fields[1].Value != "$50,000" {
		t.Errorf("field[1] = %+v, want Amount/$50,000", resolved.Fields[1])
	}
	if resolved.Fields[2].Value != "IBAN-12345" {
		t.Errorf("field[2].Value = %q", resolved.Fields[2].Value)
	}
}

func TestResolve_BatchItems(t *testing.T) {
	tmpl := json.RawMessage(`{
		"title": "Create Profiles",
		"fields": [
			{"label": "Department", "path": "department"}
		],
		"items": {
			"path": "profiles",
			"label_path": "name",
			"fields": [
				{"label": "Email", "path": "email"},
				{"label": "Role", "path": "role"}
			]
		}
	}`)

	payload := json.RawMessage(`{
		"department": "Engineering",
		"profiles": [
			{"name": "John Doe", "email": "john@example.com", "role": "Senior Engineer"},
			{"name": "Jane Smith", "email": "jane@example.com", "role": "Tech Lead"}
		]
	}`)

	result, err := Resolve(tmpl, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resolved Resolved
	if err := json.Unmarshal(result, &resolved); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if resolved.Title != "Create Profiles" {
		t.Errorf("title = %q", resolved.Title)
	}

	if len(resolved.Fields) != 1 {
		t.Fatalf("got %d fields, want 1", len(resolved.Fields))
	}
	if resolved.Fields[0].Value != "Engineering" {
		t.Errorf("field[0].Value = %q", resolved.Fields[0].Value)
	}

	if len(resolved.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(resolved.Items))
	}
	if resolved.Items[0].Title != "John Doe" {
		t.Errorf("item[0].Title = %q", resolved.Items[0].Title)
	}
	if resolved.Items[0].Fields[0].Value != "john@example.com" {
		t.Errorf("item[0].Fields[0].Value = %q", resolved.Items[0].Fields[0].Value)
	}
	if resolved.Items[1].Title != "Jane Smith" {
		t.Errorf("item[1].Title = %q", resolved.Items[1].Title)
	}
}

func TestResolve_MissingFields(t *testing.T) {
	tmpl := json.RawMessage(`{
		"title": "Request for {{missing_field}}",
		"fields": [
			{"label": "Present", "path": "exists"},
			{"label": "Missing", "path": "does_not_exist"}
		]
	}`)

	payload := json.RawMessage(`{"exists": "hello"}`)

	result, err := Resolve(tmpl, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resolved Resolved
	if err := json.Unmarshal(result, &resolved); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if resolved.Title != "Request for -" {
		t.Errorf("title = %q, want %q", resolved.Title, "Request for -")
	}
	if resolved.Fields[0].Value != "hello" {
		t.Errorf("field[0].Value = %q, want %q", resolved.Fields[0].Value, "hello")
	}
	if resolved.Fields[1].Value != "-" {
		t.Errorf("field[1].Value = %q, want %q", resolved.Fields[1].Value, "-")
	}
}

func TestResolve_NestedPath(t *testing.T) {
	tmpl := json.RawMessage(`{
		"fields": [
			{"label": "City", "path": "address.city"},
			{"label": "Deep", "path": "a.b.c"}
		]
	}`)

	payload := json.RawMessage(`{
		"address": {"city": "Lagos"},
		"a": {"b": {"c": "deep value"}}
	}`)

	result, err := Resolve(tmpl, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resolved Resolved
	if err := json.Unmarshal(result, &resolved); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if resolved.Fields[0].Value != "Lagos" {
		t.Errorf("field[0].Value = %q, want %q", resolved.Fields[0].Value, "Lagos")
	}
	if resolved.Fields[1].Value != "deep value" {
		t.Errorf("field[1].Value = %q, want %q", resolved.Fields[1].Value, "deep value")
	}
}

func TestResolve_NilTemplate(t *testing.T) {
	result, err := Resolve(nil, json.RawMessage(`{"foo": "bar"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %s", result)
	}
}

func TestResolve_Formatters(t *testing.T) {
	tmpl := json.RawMessage(`{
		"fields": [
			{"label": "Amount", "path": "amount", "format": "currency"},
			{"label": "Date", "path": "date", "format": "date"},
			{"label": "Count", "path": "count", "format": "number"},
			{"label": "Desc", "path": "desc", "format": "truncate"}
		]
	}`)

	payload := json.RawMessage(`{
		"amount": 1234567.89,
		"date": "2026-03-14",
		"count": 1000000,
		"desc": "This is a very long description that should be truncated because it exceeds the maximum length allowed"
	}`)

	result, err := Resolve(tmpl, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resolved Resolved
	if err := json.Unmarshal(result, &resolved); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if resolved.Fields[0].Value != "$1,234,567.89" {
		t.Errorf("currency = %q, want %q", resolved.Fields[0].Value, "$1,234,567.89")
	}
	if resolved.Fields[1].Value != "Mar 14, 2026" {
		t.Errorf("date = %q, want %q", resolved.Fields[1].Value, "Mar 14, 2026")
	}
	if resolved.Fields[2].Value != "1,000,000" {
		t.Errorf("number = %q, want %q", resolved.Fields[2].Value, "1,000,000")
	}
	if len(resolved.Fields[3].Value) != 50 {
		t.Errorf("truncated length = %d, want 50", len(resolved.Fields[3].Value))
	}
}

func TestValidateTemplate_Valid(t *testing.T) {
	tmpl := json.RawMessage(`{
		"title": "Wire Transfer - {{amount | currency}}",
		"fields": [
			{"label": "From", "path": "source_account_id"},
			{"label": "Amount", "path": "amount", "format": "currency"}
		]
	}`)

	if err := ValidateTemplate(tmpl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateTemplate_NilOrEmpty(t *testing.T) {
	if err := ValidateTemplate(nil); err != nil {
		t.Fatalf("nil template should be valid, got: %v", err)
	}
	if err := ValidateTemplate(json.RawMessage(`null`)); err != nil {
		t.Fatalf("null template should be valid, got: %v", err)
	}
}

func TestValidateTemplate_InvalidJSON(t *testing.T) {
	err := ValidateTemplate(json.RawMessage(`{not json}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateTemplate_EmptyContent(t *testing.T) {
	err := ValidateTemplate(json.RawMessage(`{"fields": []}`))
	if err == nil {
		t.Fatal("expected error for empty template")
	}
}

func TestValidateTemplate_MissingLabel(t *testing.T) {
	err := ValidateTemplate(json.RawMessage(`{
		"fields": [{"label": "", "path": "amount"}]
	}`))
	if err == nil {
		t.Fatal("expected error for missing label")
	}
}

func TestValidateTemplate_MissingPath(t *testing.T) {
	err := ValidateTemplate(json.RawMessage(`{
		"fields": [{"label": "Amount", "path": ""}]
	}`))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestValidateTemplate_UnknownFormat(t *testing.T) {
	err := ValidateTemplate(json.RawMessage(`{
		"fields": [{"label": "Amount", "path": "amount", "format": "bogus"}]
	}`))
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestValidateTemplate_ValidFormats(t *testing.T) {
	for _, format := range []string{"currency", "date", "number", "truncate"} {
		tmpl := json.RawMessage(`{
			"fields": [{"label": "X", "path": "x", "format": "` + format + `"}]
		}`)
		if err := ValidateTemplate(tmpl); err != nil {
			t.Errorf("format %q should be valid, got: %v", format, err)
		}
	}
}

func TestValidateTemplate_ItemsMissingPath(t *testing.T) {
	err := ValidateTemplate(json.RawMessage(`{
		"title": "Test",
		"fields": [],
		"items": {"path": "", "label_path": "name", "fields": [{"label": "X", "path": "x"}]}
	}`))
	if err == nil {
		t.Fatal("expected error for items missing path")
	}
}

func TestValidateTemplate_ItemsFieldValidation(t *testing.T) {
	err := ValidateTemplate(json.RawMessage(`{
		"title": "Test",
		"fields": [],
		"items": {
			"path": "profiles",
			"label_path": "name",
			"fields": [{"label": "", "path": "email"}]
		}
	}`))
	if err == nil {
		t.Fatal("expected error for items field missing label")
	}
}

func TestResolve_TitleInterpolation(t *testing.T) {
	tmpl := json.RawMessage(`{
		"title": "{{type}} - {{amount | currency}} from {{source}}",
		"fields": []
	}`)

	payload := json.RawMessage(`{
		"type": "Wire Transfer",
		"amount": 50000,
		"source": "ACC-001"
	}`)

	result, err := Resolve(tmpl, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resolved Resolved
	if err := json.Unmarshal(result, &resolved); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	want := "Wire Transfer - $50,000 from ACC-001"
	if resolved.Title != want {
		t.Errorf("title = %q, want %q", resolved.Title, want)
	}
}

func TestResolve_ItemsOnly(t *testing.T) {
	tmpl := json.RawMessage(`{
		"items": {
			"path": "profiles",
			"label_path": "name",
			"fields": [
				{"label": "Email", "path": "email"}
			]
		}
	}`)

	payload := json.RawMessage(`{
		"profiles": [
			{"name": "Alice", "email": "alice@example.com"}
		]
	}`)

	result, err := Resolve(tmpl, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for items-only template")
	}

	var resolved Resolved
	if err := json.Unmarshal(result, &resolved); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(resolved.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(resolved.Items))
	}
	if resolved.Items[0].Title != "Alice" {
		t.Errorf("item[0].Title = %q, want %q", resolved.Items[0].Title, "Alice")
	}
}

func TestResolve_NonObjectItems_Skipped(t *testing.T) {
	tmpl := json.RawMessage(`{
		"title": "Mixed Items",
		"fields": [],
		"items": {
			"path": "data",
			"label_path": "name",
			"fields": [
				{"label": "Value", "path": "val"}
			]
		}
	}`)

	payload := json.RawMessage(`{
		"data": [
			{"name": "Valid", "val": "ok"},
			"not-an-object",
			42,
			{"name": "Also Valid", "val": "yes"}
		]
	}`)

	result, err := Resolve(tmpl, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resolved Resolved
	if err := json.Unmarshal(result, &resolved); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(resolved.Items) != 2 {
		t.Fatalf("got %d items, want 2 (non-objects should be skipped)", len(resolved.Items))
	}
	if resolved.Items[0].Title != "Valid" {
		t.Errorf("item[0].Title = %q, want %q", resolved.Items[0].Title, "Valid")
	}
	if resolved.Items[1].Title != "Also Valid" {
		t.Errorf("item[1].Title = %q, want %q", resolved.Items[1].Title, "Also Valid")
	}
}
