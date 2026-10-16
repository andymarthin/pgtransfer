// Package output provides machine-readable output for pgtransfer.
//
// By default pgtransfer prints human-friendly, coloured text. When the global
// --json flag is set (or PGTRANSFER_JSON=1 is present in the environment), the
// commands emit a single structured JSON object on stdout instead, so that
// scripts, CI pipelines and AI agents can parse results reliably without
// scraping prose.
//
// The JSON contract is intentionally small and stable:
//
//	{
//	  "ok": true,
//	  "command": "export csv",
//	  "data": { ... command-specific fields ... },
//	  "error": null
//	}
//
// On failure:
//
//	{
//	  "ok": false,
//	  "command": "export csv",
//	  "data": null,
//	  "error": {"code": "connection_failed", "message": "..."}
//	}
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Mode controls how command results are rendered.
type Mode int

const (
	// ModeText is the default, human-friendly output.
	ModeText Mode = iota
	// ModeJSON emits a single machine-readable JSON object.
	ModeJSON
)

var (
	mu            sync.RWMutex
	mode                    = ModeText
	stdout        io.Writer = os.Stdout
	stderr        io.Writer = os.Stderr
	initialised   bool
	resultEmitted bool
	mcpMode       bool
)

// SetMode forces the output mode. Primarily used by tests and by the root
// command once flags have been parsed.
func SetMode(m Mode) {
	mu.Lock()
	defer mu.Unlock()
	mode = m
	initialised = true
	resultEmitted = false
}

// SetJSON is a convenience wrapper around SetMode.
func SetJSON(enabled bool) {
	if enabled {
		SetMode(ModeJSON)
	} else {
		SetMode(ModeText)
	}
}

// IsJSON reports whether JSON output is currently enabled. It also honours the
// PGTRANSFER_JSON environment variable when the mode has not been set
// explicitly, so agents can simply export the variable.
func IsJSON() bool {
	mu.RLock()
	defer mu.RUnlock()
	if initialised {
		return mode == ModeJSON
	}
	return envWantsJSON()
}

func envWantsJSON() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("PGTRANSFER_JSON")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// IsInteractive reports whether writes to stdout are attached to a terminal.
// Agents and pipelines generally are not, which lets commands suppress spinners
// and colour automatically.
func IsInteractive() bool {
	f, ok := stdout.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Configure applies explicit settings, typically called once from the root
// command. When both forceJSON and the environment agree, json wins.
func Configure(jsonFlag bool) {
	if jsonFlag || envWantsJSON() {
		SetMode(ModeJSON)
	} else {
		SetMode(ModeText)
	}
}

// SetWriters overrides the output destinations. Intended for tests.
func SetWriters(out, errW io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	if out != nil {
		stdout = out
	}
	if errW != nil {
		stderr = errW
	}
}

// Reset restores the default state. Intended for tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	mode = ModeText
	initialised = false
	resultEmitted = false
	mcpMode = false
	stdout = os.Stdout
	stderr = os.Stderr
}

// EnableMCPMode switches the process into MCP server mode.
//
// When pgtransfer speaks the Model Context Protocol over stdio, stdout is
// reserved for protocol frames: a single stray byte (a progress message, an
// ANSI escape) corrupts the stream and the client fails to parse it. Third-party
// libraries, however, print progress messages straight to os.Stdout.
//
// EnableMCPMode therefore redirects the process-level os.Stdout to stderr for
// the lifetime of the MCP server, so any such output becomes harmless
// diagnostics. The caller must keep a reference to the real stdout (returned
// here) and hand it to the MCP transport.
func EnableMCPMode() *os.File {
	mu.Lock()
	defer mu.Unlock()

	realStdout := os.Stdout
	mcpMode = true
	// Route stray writes to stderr.
	os.Stdout = os.Stderr
	stdout = os.Stderr

	return realStdout
}

// IsMCP reports whether the process is running as an MCP server.
func IsMCP() bool {
	mu.RLock()
	defer mu.RUnlock()
	return mcpMode
}

// Result is the envelope emitted by every command in JSON mode.
type Result struct {
	OK      bool        `json:"ok"`
	Command string      `json:"command"`
	Data    interface{} `json:"data,omitempty"`
	Error   *Error      `json:"error"`
}

// Error is a structured, machine-readable error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

// NewError builds a structured error.
func NewError(code, format string, args ...interface{}) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Success emits a successful result. In text mode, data is ignored because the
// caller has already printed human-friendly output; in JSON mode it is
// serialised to stdout.
func Success(command string, data interface{}) {
	if !IsJSON() {
		return
	}
	if markEmitted() {
		emit(Result{OK: true, Command: command, Data: data})
	}
}

// Fail emits a failed result in JSON mode and returns the structured error so
// the caller can propagate it. Emission happens at most once per process: a
// command body that reports a failure prevents the top-level handler from
// emitting a second, less specific error.
func Fail(command, code, format string, args ...interface{}) *Error {
	err := NewError(code, format, args...)
	if IsJSON() {
		if markEmitted() {
			emit(Result{OK: false, Command: command, Error: err})
		}
	}
	return err
}

// markEmitted records that a JSON result was written and reports whether this
// call is the first. It keeps the JSON output to a single object.
func markEmitted() bool {
	mu.Lock()
	defer mu.Unlock()
	if resultEmitted {
		return false
	}
	resultEmitted = true
	return true
}

// WriteJSON serialises v to stdout. Used for commands whose entire output is
// inherently structured (e.g. profile list).
func WriteJSON(v interface{}) error {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func emit(r Result) {
	mu.RLock()
	w := stdout
	mu.RUnlock()

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	// Encoding a Result can only fail on a broken writer; there is no useful
	// recovery, so the error is intentionally dropped.
	_ = enc.Encode(r)
}

// PrintErrorLine writes a human-readable error to stderr. It is a no-op in JSON
// mode, where the error has already been emitted as part of the result object.
func PrintErrorLine(err error) {
	if err == nil || IsJSON() {
		return
	}
	mu.RLock()
	w := stderr
	mu.RUnlock()
	fmt.Fprintf(w, "Error: %v\n", err)
}
