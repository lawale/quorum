package mssql

import (
	"net/url"
	"testing"
)

func TestEnsureGUIDConversionDSN_URL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "adds guid conversion when missing",
			input: "sqlserver://sa:Secret1!@localhost:1433?database=quorum&encrypt=disable",
		},
		{
			name:  "overrides existing false value",
			input: "sqlserver://sa:Secret1!@localhost:1433?database=quorum&guid+conversion=false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ensureGUIDConversionDSN(tt.input)
			u, err := url.Parse(got)
			if err != nil {
				t.Fatalf("parse result: %v", err)
			}

			if v := u.Query().Get("guid conversion"); v != "true" {
				t.Fatalf("guid conversion = %q, want true", v)
			}
		})
	}
}

func TestEnsureGUIDConversionDSN_ADO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "appends when missing",
			input: "server=localhost;user id=sa;password=Secret1!;database=quorum",
			want:  "server=localhost;user id=sa;password=Secret1!;database=quorum;guid conversion=true",
		},
		{
			name:  "overrides existing false value",
			input: "server=localhost;guid conversion=false;database=quorum",
			want:  "server=localhost;guid conversion=true;database=quorum",
		},
		{
			name:  "appends after trailing semicolon",
			input: "server=localhost;database=quorum;",
			want:  "server=localhost;database=quorum;guid conversion=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ensureGUIDConversionDSN(tt.input); got != tt.want {
				t.Fatalf("ensureGUIDConversionDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}
