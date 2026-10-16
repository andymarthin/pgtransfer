package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func setup(t *testing.T) *bytes.Buffer {
	t.Helper()
	t.Cleanup(Reset)
	Reset()
	buf := &bytes.Buffer{}
	SetWriters(buf, buf)
	return buf
}

func TestDefaultModeIsText(t *testing.T) {
	t.Setenv("PGTRANSFER_JSON", "")
	setup(t)

	if IsJSON() {
		t.Fatalf("expected text mode by default")
	}
}

func TestSetJSON(t *testing.T) {
	setup(t)

	SetJSON(true)
	if !IsJSON() {
		t.Fatalf("expected JSON mode after SetJSON(true)")
	}
	SetJSON(false)
	if IsJSON() {
		t.Fatalf("expected text mode after SetJSON(false)")
	}
}

func TestEnvWantsJSON(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "on", "On"} {
		t.Run(v, func(t *testing.T) {
			Reset()
			t.Setenv("PGTRANSFER_JSON", v)
			if !IsJSON() {
				t.Fatalf("expected PGTRANSFER_JSON=%q to enable JSON", v)
			}
		})
	}
	for _, v := range []string{"0", "false", "no", "off", "", "  "} {
		t.Run("off:"+v, func(t *testing.T) {
			Reset()
			t.Setenv("PGTRANSFER_JSON", v)
			if IsJSON() {
				t.Fatalf("expected PGTRANSFER_JSON=%q to keep text mode", v)
			}
		})
	}
}

func TestConfigure_FlagBeatsEnv(t *testing.T) {
	setup(t)
	t.Setenv("PGTRANSFER_JSON", "1")
	Configure(false)
	if !IsJSON() {
		t.Fatalf("expected env to enable JSON even without the flag")
	}

	Reset()
	SetWriters(&bytes.Buffer{}, &bytes.Buffer{})
	t.Setenv("PGTRANSFER_JSON", "")
	Configure(true)
	if !IsJSON() {
		t.Fatalf("expected --json flag to enable JSON")
	}
}

func TestSuccess_EmitsEnvelope(t *testing.T) {
	buf := setup(t)
	SetJSON(true)

	Success("version", map[string]string{"version": "1.2.3"})

	var got Result
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output was not valid JSON: %v\n%s", err, buf.String())
	}
	if !got.OK || got.Command != "version" || got.Error != nil {
		t.Fatalf("unexpected envelope: %+v", got)
	}
}

func TestSuccess_SilentInTextMode(t *testing.T) {
	buf := setup(t)
	Success("version", map[string]string{"version": "1.2.3"})
	if buf.Len() != 0 {
		t.Fatalf("expected no output in text mode, got %q", buf.String())
	}
}

func TestFail_EmitsStructuredError(t *testing.T) {
	buf := setup(t)
	SetJSON(true)

	err := Fail("profile use", "profile_not_found", "no profile %q", "ghost")
	if err == nil {
		t.Fatalf("expected a non-nil error")
	}
	if err.Code != "profile_not_found" {
		t.Fatalf("expected code profile_not_found, got %q", err.Code)
	}

	var got Result
	if e := json.Unmarshal(buf.Bytes(), &got); e != nil {
		t.Fatalf("output was not valid JSON: %v\n%s", e, buf.String())
	}
	if got.OK {
		t.Fatalf("expected ok=false")
	}
	if got.Error == nil || got.Error.Code != "profile_not_found" {
		t.Fatalf("unexpected error payload: %+v", got.Error)
	}
	if !strings.Contains(got.Error.Message, "ghost") {
		t.Fatalf("expected message to mention the profile, got %q", got.Error.Message)
	}
}

func TestFail_OnlyEmitsOnce(t *testing.T) {
	buf := setup(t)
	SetJSON(true)

	Fail("cmd", "first", "first failure")
	Fail("cmd", "second", "second failure")

	// Exactly one JSON object should have been written.
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	var n int
	for {
		var r Result
		if err := dec.Decode(&r); err != nil {
			break
		}
		n++
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 emitted result, got %d:\n%s", n, buf.String())
	}
	if !strings.Contains(buf.String(), "first failure") {
		t.Fatalf("expected the first failure to win, got %q", buf.String())
	}
}

func TestSuccessThenFail_EmitsOnce(t *testing.T) {
	buf := setup(t)
	SetJSON(true)

	Success("cmd", map[string]string{"a": "b"})
	Fail("cmd", "late", "too late")

	// Exactly one JSON object should be emitted, and it should be the success.
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	var n int
	var first Result
	for {
		var r Result
		if err := dec.Decode(&r); err != nil {
			break
		}
		if n == 0 {
			first = r
		}
		n++
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 emitted result, got %d:\n%s", n, buf.String())
	}
	if !first.OK {
		t.Fatalf("expected the emitted result to be the success, got %+v", first)
	}
}

func TestPrintErrorLine(t *testing.T) {
	buf := setup(t)

	PrintErrorLine(errors.New("boom"))
	if !strings.Contains(buf.String(), "boom") {
		t.Fatalf("expected the error text, got %q", buf.String())
	}

	// In JSON mode it must stay silent.
	buf.Reset()
	SetJSON(true)
	PrintErrorLine(errors.New("boom"))
	if buf.Len() != 0 {
		t.Fatalf("expected no output in JSON mode, got %q", buf.String())
	}
}

func TestPrintErrorLine_Nil(t *testing.T) {
	buf := setup(t)
	PrintErrorLine(nil)
	if buf.Len() != 0 {
		t.Fatalf("expected no output for nil error, got %q", buf.String())
	}
}

func TestWriteJSON(t *testing.T) {
	buf := setup(t)
	if err := WriteJSON(map[string]int{"count": 3}); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}
	var got map[string]int
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got["count"] != 3 {
		t.Fatalf("expected count 3, got %d", got["count"])
	}
}

func TestError_Error(t *testing.T) {
	e := NewError("code", "message %d", 7)
	if e.Error() != "message 7" {
		t.Fatalf("unexpected Error() string: %q", e.Error())
	}
}
