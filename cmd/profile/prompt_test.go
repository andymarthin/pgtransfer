package profile

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestPromptPort pins that a bad answer is reported rather than stored as a
// zero port. Silently writing 0 produced a profile that connected to the
// default port with no indication that the answer had been ignored.
func TestPromptPort(t *testing.T) {
	cases := []struct {
		name      string
		answer    string
		def       string
		want      int
		wantError bool
	}{
		{name: "explicit number", answer: "6543", def: "5432", want: 6543},
		{name: "default when empty", answer: "", def: "5432", want: 5432},
		{name: "not a number", answer: "fifty", def: "5432", wantError: true},
		{name: "empty default too", answer: "", def: "", wantError: true},
		{name: "zero", answer: "0", def: "5432", wantError: true},
		{name: "negative", answer: "-1", def: "5432", wantError: true},
		{name: "too large", answer: "70000", def: "5432", wantError: true},
		{name: "upper bound", answer: "65535", def: "5432", want: 65535},
		{name: "lower bound", answer: "1", def: "5432", want: 1},
		{name: "whitespace around", answer: " 2222 ", def: "22", wantError: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prompt := func(label, defaultValue string) string { return c.answer }

			got, err := promptPort(prompt, "Port", c.def)

			if c.wantError {
				if err == nil {
					t.Fatalf("promptPort(%q) = %d, want an error", c.answer, got)
				}
				if !strings.Contains(err.Error(), "Port") {
					t.Errorf("error does not name the prompt: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("promptPort(%q) error = %v", c.answer, err)
			}
			if got != c.want {
				t.Errorf("promptPort(%q) = %d, want %d", c.answer, got, c.want)
			}
		})
	}
}

// TestSecurePromptRefusesWithoutTerminal pins that a password prompt with no
// terminal reports an error instead of returning an empty password. Stdin under
// `go test` is not a terminal, which is exactly the situation a CI job or a
// piped invocation hits.
func TestSecurePromptRefusesWithoutTerminal(t *testing.T) {
	got, err := securePrompt("Database password")
	if err == nil {
		t.Fatalf("securePrompt() = %q, nil; want an error without a terminal", got)
	}
	if got != "" {
		t.Errorf("securePrompt() returned %q alongside an error", got)
	}
	if !errors.Is(err, errNoTerminal) {
		t.Errorf("securePrompt() error = %v, want errNoTerminal", err)
	}
	if !strings.Contains(err.Error(), "Database password") {
		t.Errorf("error does not name the prompt: %v", err)
	}
}

// TestPromptProfileInputPropagatesSecurePromptError pins that the interactive
// path aborts instead of assembling a profile with an empty password.
func TestPromptProfileInputPropagatesSecurePromptError(t *testing.T) {
	// Reaching the password prompt requires the user prompt to answer, which
	// reads os.Stdin. Under the test runner stdin is empty, so the prompt
	// returns the default for every question and securePrompt fails on the
	// password. That is the path under test.
	p, err := promptProfileInput(&cobra.Command{}, "test", nil)
	if err == nil {
		t.Fatalf("promptProfileInput() = %+v, nil; want an error", p)
	}
	if !errors.Is(err, errNoTerminal) {
		t.Errorf("promptProfileInput() error = %v, want errNoTerminal", err)
	}
	if p.Name != "" || p.Host != "" {
		t.Errorf("promptProfileInput() returned a partially built profile: %+v", p)
	}
}
