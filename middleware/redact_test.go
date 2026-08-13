package middleware

import (
	"strings"
	"testing"
)

func TestRedactSensitive(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "camelCase privateKey",
			input: `{"data":{"privateKey":"27fcd38a0675c7ad108fbcb8975e49f06e7a75cd3666f931c0a2872f3dd1f7a9","publicKey":"02684d9","address":"abc"},"code":0}`,
			want:  `{"data":{"privateKey":"***","publicKey":"02684d9","address":"abc"},"code":0}`,
		},
		{
			name:  "snake_case variants",
			input: `{"private_key":"a","secret":"b","password":"c"}`,
			want:  `{"private_key":"***","secret":"***","password":"***"}`,
		},
		{
			name:  "mnemonic",
			input: `{"mnemonic":"word1 word2"}`,
			want:  `{"mnemonic":"***"}`,
		},
		{
			name:  "unrelated keys untouched",
			input: `{"keyType":"secp256k1","signature":"abc","publicKey":"02684d9"}`,
			want:  `{"keyType":"secp256k1","signature":"abc","publicKey":"02684d9"}`,
		},
		{
			name:  "PascalCase",
			input: `{"PrivateKey":"abc","Password":"123"}`,
			want:  `{"PrivateKey":"***","Password":"***"}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := redactSensitive(c.input)
			if got != c.want {
				t.Fatalf("redactSensitive(%q)\n got: %q\nwant: %q", c.input, got, c.want)
			}
		})
	}
}

func TestShouldLogBody(t *testing.T) {
	cases := []struct {
		method, path string
		want         bool
	}{
		{"POST", "/api/read", true},
		{"POST", "/api/accounts/generate", true},
		{"GET", "/api/accounts/abc", false},
		{"POST", "/", false},
		{"POST", "/static/js/app.js", false},
		{"PUT", "/api/foo", true},
	}
	for _, c := range cases {
		if got := shouldLogBody(c.method, c.path); got != c.want {
			t.Fatalf("shouldLogBody(%s, %s) = %v, want %v", c.method, c.path, got, c.want)
		}
	}
}

func TestTruncateBody(t *testing.T) {
	s := strings.Repeat("a", bodyLogMaxChars+100)
	got := truncateBody(s)
	if len(got) != bodyLogMaxChars+len("...(truncated)") {
		t.Fatalf("truncateBody length = %d, want %d", len(got), bodyLogMaxChars+len("...(truncated)"))
	}
	if !strings.HasSuffix(got, "...(truncated)") {
		t.Fatalf("truncateBody missing marker: %q", got[len(got)-20:])
	}
	if got2 := truncateBody("short"); got2 != "short" {
		t.Fatalf("short body should stay unchanged, got %q", got2)
	}
}
