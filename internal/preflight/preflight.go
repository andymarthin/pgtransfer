// Package preflight checks a command's requirements before it starts doing
// work.
//
// Two kinds matter. pgtransfer drives PostgreSQL's own client tools, so a
// machine without pg_dump or pg_restore cannot dump or restore at all. And a
// tunnelled profile needs an SSH identity the tunnel can present. Either one
// missing otherwise surfaces part-way through: a migration that has already
// written its archive, an import that fails on its first batch. The checks here
// are cheap — a PATH lookup, a stat — and run once, up front, so the message
// names what to install or which profile field to set.
package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Tool is an external program pgtransfer may need.
type Tool struct {
	Name string // the executable, as looked up on PATH
	Why  string // what needs it, for the error message
}

// The tools pgtransfer invokes.
var (
	PgDump    = Tool{"pg_dump", "export dump and migrate database"}
	PgRestore = Tool{"pg_restore", "restoring a custom, tar or directory archive"}
	Psql      = Tool{"psql", "restoring a plain SQL dump"}
)

// MissingError reports a tool that is not on PATH.
type MissingError struct {
	Tools []Tool
}

func (e *MissingError) Error() string {
	reasons := make([]string, len(e.Tools))
	for i, t := range e.Tools {
		reasons[i] = fmt.Sprintf("%s (needed by %s)", t.Name, t.Why)
	}
	return fmt.Sprintf("%s not found in PATH — install the PostgreSQL client tools",
		strings.Join(reasons, ", "))
}

// Lookup reports whether name is on PATH.
//
// It is a variable so tests can stand in for the real PATH.
var Lookup = func(name string) (string, error) { return exec.LookPath(name) }

// Check returns a *MissingError naming every tool in tools that is not on PATH.
// It reports all of them at once: being told about one missing binary, fixing
// it, and being told about the next is a worse experience than one list.
func Check(tools ...Tool) error {
	var missing []Tool
	for _, t := range tools {
		if _, err := Lookup(t.Name); err != nil {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		return &MissingError{Tools: missing}
	}
	return nil
}

// SSH identity checks.
//
// The tunnel is opened by golang.org/x/crypto/ssh inside this process, so there
// is no `ssh` binary to look for. What can be checked before connecting is the
// identity the tunnel will present: a key path that does not exist, or a key
// with no passphrase and no password and no agent to ask, cannot authenticate.
// Catching that here names the profile field to fix, instead of surfacing as
// "failed SSH connection" once the transfer has started.

// SSHIdentity describes the credentials a tunnelled profile will present.
type SSHIdentity struct {
	KeyPath    string
	Passphrase string
	Password   string
	// AgentAvailable reports whether SSH_AUTH_SOCK points at a usable agent.
	AgentAvailable bool
}

// CheckSSHIdentity reports why a tunnelled profile cannot authenticate.
//
// The order mirrors BuildAuthMethods: an agent first, then an explicit key,
// then a password. A profile needs at least one of them, and a key path that is
// set but unreadable is an error even when a password is also present, because
// the profile says the key is what it wants to use.
func CheckSSHIdentity(id SSHIdentity) error {
	if id.KeyPath != "" {
		expanded := os.ExpandEnv(id.KeyPath)
		info, err := os.Stat(expanded)
		switch {
		case err != nil:
			return fmt.Errorf("SSH key %s is not readable: %w", expanded, err)
		case info.IsDir():
			return fmt.Errorf("SSH key %s is a directory, not a key file", expanded)
		}
		return nil
	}
	if id.AgentAvailable || id.Password != "" {
		return nil
	}
	return fmt.Errorf("the profile has SSH enabled but no way to authenticate: " +
		"set ssh.key_path or ssh.password, or run an ssh-agent (SSH_AUTH_SOCK is not set)")
}

// AgentAvailable reports whether an SSH agent is reachable.
//
// It looks at the environment variable only: connecting to the socket is what
// BuildAuthMethods does, and that error is more informative from there.
func AgentAvailable() bool {
	return os.Getenv("SSH_AUTH_SOCK") != ""
}

// ProfileSSH describes one end of a transfer, for a tunnel check.
//
// It is defined here rather than taking config.Profile so that this package
// stays free of the config import, matching sshx.
type ProfileSSH struct {
	Name       string
	Enabled    bool
	KeyPath    string
	Passphrase string
	Password   string
}

// CheckProfiles checks the SSH identity of every profile in profiles that has
// SSH enabled. It returns the first problem, naming the profile it belongs to.
//
// A migration has two ends and either may be tunnelled, so both are checked
// before the archive is written.
func CheckProfiles(profiles ...ProfileSSH) error {
	for _, p := range profiles {
		if !p.Enabled {
			continue
		}
		err := CheckSSHIdentity(SSHIdentity{
			KeyPath:        p.KeyPath,
			Passphrase:     p.Passphrase,
			Password:       p.Password,
			AgentAvailable: AgentAvailable(),
		})
		if err != nil {
			return fmt.Errorf("profile %q: %w", p.Name, err)
		}
	}
	return nil
}
