package common

import "testing"

func TestParseCPARequestID(t *testing.T) {
	tests := []struct {
		name  string
		trace string
		want  string
	}{
		{name: "normal", trace: "20260717215849-auth-index-1234abcd", want: "1234abcd"},
		{name: "uuidv7 request id", trace: "20260717215849-auth-index-018f0c3e-7a1b-7c2d-9e3f-123456789abc", want: "018f0c3e-7a1b-7c2d-9e3f-123456789abc"},
		{name: "hyphenated auth index", trace: "20260717215849-auth-index-1234abcd", want: "1234abcd"},
		{name: "empty", trace: "", want: ""},
		{name: "no separator", trace: "1234abcd", want: ""},
		{name: "trailing separator", trace: "20260717215849-auth-index-", want: ""},
		{name: "invalid token", trace: "20260717215849-auth-index-1234/abcd", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseCPARequestID(test.trace); got != test.want {
				t.Fatalf("ParseCPARequestID(%q) = %q, want %q", test.trace, got, test.want)
			}
		})
	}
}
