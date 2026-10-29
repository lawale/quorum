package display

import (
	"encoding/json"
	"testing"
)

func resolveChangesFor(t *testing.T, tmpl, payload string) Resolved {
	t.Helper()
	out, err := Resolve(json.RawMessage(tmpl), json.RawMessage(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var r Resolved
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return r
}

func TestResolve_Changes_ExplicitFields(t *testing.T) {
	tmpl := `{
		"title": "Update Branch {{current.code}}",
		"changes": {
			"current_path": "current",
			"proposed_path": "proposed",
			"fields": [
				{"label": "Name", "path": "name"},
				{"label": "Limit", "path": "limit", "format": "currency"},
				{"label": "Status", "path": "status"}
			]
		}
	}`
	payload := `{
		"current":  {"code": "LAG-01", "name": "Lagos Island", "limit": 500000, "status": "active"},
		"proposed": {"code": "LAG-01", "name": "Lagos Island", "limit": 750000, "status": "inactive"}
	}`

	r := resolveChangesFor(t, tmpl, payload)

	if r.Title != "Update Branch LAG-01" {
		t.Errorf("title = %q", r.Title)
	}
	if len(r.Changes) != 3 {
		t.Fatalf("got %d changes, want 3", len(r.Changes))
	}
	if r.Changes[0].Changed || r.Changes[0].From != "Lagos Island" || r.Changes[0].To != "Lagos Island" {
		t.Errorf("name change = %+v, want unchanged", r.Changes[0])
	}
	if !r.Changes[1].Changed || r.Changes[1].From != "$500,000" || r.Changes[1].To != "$750,000" {
		t.Errorf("limit change = %+v", r.Changes[1])
	}
	if !r.Changes[2].Changed || r.Changes[2].From != "active" || r.Changes[2].To != "inactive" {
		t.Errorf("status change = %+v", r.Changes[2])
	}
}

func TestResolve_Changes_AutoDiscovery(t *testing.T) {
	tmpl := `{"changes": {"current_path": "before", "proposed_path": "after"}}`
	payload := `{
		"before": {"name": "Ada", "address": {"city": "Lagos", "zip": "100001"}, "tags": ["a"]},
		"after":  {"name": "Ada", "address": {"city": "Abuja", "zip": "100001"}, "tags": ["a", "b"], "email": "ada@example.com"}
	}`

	r := resolveChangesFor(t, tmpl, payload)

	want := []ResolvedChange{
		{Label: "Address / City", From: "Lagos", To: "Abuja", Changed: true},
		{Label: "Address / Zip", From: "100001", To: "100001", Changed: false},
		{Label: "Email", From: "-", To: "ada@example.com", Changed: true},
		{Label: "Name", From: "Ada", To: "Ada", Changed: false},
		{Label: "Tags", From: "[a]", To: "[a b]", Changed: true},
	}
	if len(r.Changes) != len(want) {
		t.Fatalf("got %d changes, want %d: %+v", len(r.Changes), len(want), r.Changes)
	}
	for i := range want {
		if r.Changes[i] != want[i] {
			t.Errorf("changes[%d] = %+v, want %+v", i, r.Changes[i], want[i])
		}
	}
}

func TestResolve_Changes_CreateHasNoCurrent(t *testing.T) {
	tmpl := `{"changes": {"current_path": "current", "proposed_path": "proposed", "fields": [{"label": "Code", "path": "code"}]}}`
	payload := `{"proposed": {"code": "ABJ-02"}}`

	r := resolveChangesFor(t, tmpl, payload)

	if len(r.Changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(r.Changes))
	}
	if r.Changes[0] != (ResolvedChange{Label: "Code", From: "-", To: "ABJ-02", Changed: true}) {
		t.Errorf("change = %+v", r.Changes[0])
	}
}

func TestResolve_Changes_DeleteHasNoProposed(t *testing.T) {
	tmpl := `{"changes": {"current_path": "current", "proposed_path": "proposed"}}`
	payload := `{"current": {"code": "ABJ-02"}, "proposed": null}`

	r := resolveChangesFor(t, tmpl, payload)

	if len(r.Changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(r.Changes))
	}
	if r.Changes[0] != (ResolvedChange{Label: "Code", From: "ABJ-02", To: "-", Changed: true}) {
		t.Errorf("change = %+v", r.Changes[0])
	}
}

func TestResolve_Changes_BothSnapshotsMissing(t *testing.T) {
	tmpl := `{"changes": {"current_path": "current", "proposed_path": "proposed"}}`
	payload := `{"something": "else"}`

	r := resolveChangesFor(t, tmpl, payload)

	if r.Changes != nil {
		t.Errorf("expected no changes, got %+v", r.Changes)
	}
}

func TestResolve_Changes_NonObjectSnapshotIgnored(t *testing.T) {
	tmpl := `{"changes": {"current_path": "current", "proposed_path": "proposed", "fields": [{"label": "Code", "path": "code"}]}}`
	payload := `{"current": "not-an-object", "proposed": {"code": "X"}}`

	r := resolveChangesFor(t, tmpl, payload)

	if len(r.Changes) != 1 || r.Changes[0].From != "-" || r.Changes[0].To != "X" {
		t.Errorf("changes = %+v", r.Changes)
	}
}

func TestResolve_Changes_FieldsStillEmptyArray(t *testing.T) {
	// The front ends gate on metadata.display.fields being an array, so a
	// changes-only template must still emit "fields": [].
	out, err := Resolve(
		json.RawMessage(`{"changes": {"current_path": "a", "proposed_path": "b"}}`),
		json.RawMessage(`{"a": {"x": 1}, "b": {"x": 2}}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["fields"]) != "[]" {
		t.Errorf("fields = %s, want []", raw["fields"])
	}
}

func TestValidateTemplate_Changes(t *testing.T) {
	cases := []struct {
		name    string
		tmpl    string
		wantErr bool
	}{
		{"valid minimal", `{"changes": {"current_path": "a", "proposed_path": "b"}}`, false},
		{"valid with fields", `{"changes": {"current_path": "a", "proposed_path": "b", "fields": [{"label": "L", "path": "p", "format": "date"}]}}`, false},
		{"missing current_path", `{"changes": {"proposed_path": "b"}}`, true},
		{"missing proposed_path", `{"changes": {"current_path": "a"}}`, true},
		{"same paths", `{"changes": {"current_path": "a", "proposed_path": "a"}}`, true},
		{"field missing label", `{"changes": {"current_path": "a", "proposed_path": "b", "fields": [{"path": "p"}]}}`, true},
		{"field missing path", `{"changes": {"current_path": "a", "proposed_path": "b", "fields": [{"label": "L"}]}}`, true},
		{"field bad format", `{"changes": {"current_path": "a", "proposed_path": "b", "fields": [{"label": "L", "path": "p", "format": "nope"}]}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTemplate(json.RawMessage(tc.tmpl))
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestLabelFromPath(t *testing.T) {
	cases := map[string]string{
		"name":               "Name",
		"account_id":         "Account Id",
		"billing.account_id": "Billing / Account Id",
		"kebab-case":         "Kebab Case",
	}
	for in, want := range cases {
		if got := labelFromPath(in); got != want {
			t.Errorf("labelFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}
