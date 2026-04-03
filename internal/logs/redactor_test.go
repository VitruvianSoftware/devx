package logs

import (
	"strings"
	"testing"
)

func TestRedactor_BasicReplacement(t *testing.T) {
	r := NewSecretRedactorFromPairs([]string{
		"API_KEY=sk-super-secret-key-12345",
		"DB_PASSWORD=hunter2",
	})

	input := "Connecting to DB with password hunter2 and key sk-super-secret-key-12345"
	got := r.Redact(input)

	if strings.Contains(got, "hunter2") {
		t.Errorf("expected 'hunter2' to be redacted, got: %s", got)
	}
	if strings.Contains(got, "sk-super-secret-key-12345") {
		t.Errorf("expected API key to be redacted, got: %s", got)
	}
	if !strings.Contains(got, redactedPlaceholder) {
		t.Errorf("expected redaction placeholder, got: %s", got)
	}
}

func TestRedactor_LongestFirstPreventsPartialMatch(t *testing.T) {
	r := NewSecretRedactorFromPairs([]string{
		"SHORT_KEY=abcd",
		"LONG_KEY=abcdefgh",
	})

	input := "value is abcdefgh here"
	got := r.Redact(input)

	// The long key should be replaced as a whole, not partially by the short key
	if strings.Contains(got, "abcd") {
		t.Errorf("expected full replacement, got partial match: %s", got)
	}
	// Should contain exactly one redaction placeholder
	count := strings.Count(got, redactedPlaceholder)
	if count != 1 {
		t.Errorf("expected 1 redaction, got %d: %s", count, got)
	}
}

func TestRedactor_SkipsShortValues(t *testing.T) {
	r := NewSecretRedactorFromPairs([]string{
		"FLAG=on",   // too short (<= 3)
		"NUM=42",    // too short
		"TOKEN=abcdef",
	})

	input := "flag is on, num is 42, token is abcdef"
	got := r.Redact(input)

	// Short values should be preserved
	if !strings.Contains(got, "on") {
		t.Errorf("'on' should not be redacted (too short)")
	}
	if !strings.Contains(got, "42") {
		t.Errorf("'42' should not be redacted (too short)")
	}
	// Real secret should be redacted
	if strings.Contains(got, "abcdef") {
		t.Errorf("'abcdef' should be redacted")
	}
}

func TestRedactor_SkipsNonSensitiveKeys(t *testing.T) {
	r := NewSecretRedactorFromPairs([]string{
		"HOME=/Users/james",
		"PATH=/usr/local/bin:/usr/bin",
		"SECRET_TOKEN=my-real-secret-value",
	})

	input := "home is /Users/james, secret is my-real-secret-value"
	got := r.Redact(input)

	// HOME value should not be redacted (non-sensitive key)
	if !strings.Contains(got, "/Users/james") {
		t.Errorf("HOME value should not be redacted")
	}
	// Real secret should be redacted
	if strings.Contains(got, "my-real-secret-value") {
		t.Errorf("SECRET_TOKEN value should be redacted")
	}
}

func TestRedactor_MultipleOccurrences(t *testing.T) {
	r := NewSecretRedactorFromPairs([]string{
		"API_KEY=secret123",
	})

	input := "key=secret123 and again secret123"
	got := r.Redact(input)

	if strings.Contains(got, "secret123") {
		t.Errorf("all occurrences should be redacted, got: %s", got)
	}
	count := strings.Count(got, redactedPlaceholder)
	if count != 2 {
		t.Errorf("expected 2 redactions, got %d: %s", count, got)
	}
}

func TestRedactor_EmptyInput(t *testing.T) {
	r := NewSecretRedactorFromPairs([]string{"KEY=value1234"})
	got := r.Redact("")
	if got != "" {
		t.Errorf("empty input should produce empty output, got: %q", got)
	}
}

func TestRedactor_NoSecrets(t *testing.T) {
	r := NewSecretRedactorFromPairs(nil)
	input := "nothing to redact here"
	got := r.Redact(input)
	if got != input {
		t.Errorf("no secrets should produce unchanged output, got: %q", got)
	}
}
