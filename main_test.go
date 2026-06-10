package main

import (
	"database/sql"
	"testing"
)

// The emacsql/prin1 encoding is the single biggest correctness gotcha (see
// AGENTS.md): stored values are double-quote-wrapped, so query params must be
// enc()'d and results dec()'d. These lock that contract down.

func TestEnc(t *testing.T) {
	cases := map[string]string{
		"contact":             `"contact"`,
		"Buy Changes - Denim": `"Buy Changes - Denim"`,
		"a@b.com":             `"a@b.com"`,
		"":                    `""`,
	}
	for in, want := range cases {
		if got := enc(in); got != want {
			t.Errorf("enc(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDec(t *testing.T) {
	cases := map[string]string{
		`"contact"`:             "contact",
		`"Buy Changes - Denim"`: "Buy Changes - Denim",
		`"a@b.com"`:             "a@b.com",
		`""`:                    "",
	}
	for in, want := range cases {
		if got := dec(in); got != want {
			t.Errorf("dec(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEncDecRoundTrip(t *testing.T) {
	for _, s := range []string{
		"contact", "a@b.com", "Buy Changes - Denim", "",
		`quote"inside`, "tab\there", "newline\nhere", "café", "emoji😀",
	} {
		if got := dec(enc(s)); got != s {
			t.Errorf("dec(enc(%q)) = %q, want %q", s, got, s)
		}
	}
}

func TestDecFallback(t *testing.T) {
	// Not a valid Go-quoted literal -> fall back to trimming wrapping quotes.
	cases := map[string]string{
		`"unterminated`: "unterminated",
		`bare`:          "bare",
		`""`:            "",
	}
	for in, want := range cases {
		if got := dec(in); got != want {
			t.Errorf("dec(%q) fallback = %q, want %q", in, got, want)
		}
	}
}

func TestDecN(t *testing.T) {
	if got := decN(sql.NullString{String: `"x"`, Valid: true}); got != "x" {
		t.Errorf("decN(valid) = %q, want %q", got, "x")
	}
	if got := decN(sql.NullString{Valid: false}); got != "" {
		t.Errorf("decN(NULL) = %q, want empty", got)
	}
}
