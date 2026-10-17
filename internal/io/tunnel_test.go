package io

import (
	"net"
	"testing"
	"time"

	"github.com/andymarthin/pgtransfer/internal/config"
)

// TestTunnelForCommandIsNilWithoutSSH pins that a direct profile gets no
// tunnel, so a dump does not pay for an SSH dial it will not use.
func TestTunnelForCommandIsNilWithoutSSH(t *testing.T) {
	tunnel, err := tunnelForCommand(config.Profile{Host: "localhost", Port: 5432})
	if err != nil {
		t.Fatalf("tunnelForCommand() error = %v", err)
	}
	if tunnel != nil {
		t.Cleanup(func() { _ = tunnel.Close() })
		t.Fatalf("tunnelForCommand() returned a tunnel for a profile without SSH")
	}
}

// TestDBURLForDirect pins that without a tunnel the connection string is the
// profile unchanged.
func TestDBURLForDirect(t *testing.T) {
	profile := config.Profile{
		User: "u", Password: "p", Host: "db.internal", Port: 5432,
		Database: "d", SSLMode: "disable",
	}
	got := dbURLFor(profile, nil)
	want := config.BuildDSN(profile)
	if got != want {
		t.Errorf("dbURLFor() = %q, want %q", got, want)
	}
	if want == "" {
		t.Fatal("BuildDSN returned an empty string")
	}
}

// TestDBURLForTunnel points the connection at the tunnel port and disables
// SSH, so the child process connects directly instead of trying to dial SSH
// itself.
func TestDBURLForTunnel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	tunnel := &sshTunnel{listener: listener, port: port, done: make(chan struct{})}

	profile := config.Profile{
		User: "u", Password: "p", Host: "db.internal", Port: 5432,
		Database: "d", SSLMode: "disable",
		SSH: config.SSHConfig{Enabled: true, Host: "bastion", Port: 22, User: "u"},
	}

	got := dbURLFor(profile, tunnel)

	wantLocal := config.BuildDSN(config.Profile{
		User: "u", Password: "p", Host: "127.0.0.1", Port: port,
		Database: "d", SSLMode: "disable",
	})
	if got != wantLocal {
		t.Errorf("dbURLFor() = %q, want %q", got, wantLocal)
	}
	if got == config.BuildDSN(profile) {
		t.Error("dbURLFor() ignored the tunnel")
	}
}

// TestOpenTunnelRejectsProfileWithoutSSH covers the guard that keeps
// openTunnel from dialling an empty address.
func TestOpenTunnelRejectsProfileWithoutSSH(t *testing.T) {
	if _, err := openTunnel(config.Profile{}); err == nil {
		t.Fatal("openTunnel() accepted a profile without SSH enabled")
	}
}

// TestPipeConnectionsClosesBothSides pins the resource behaviour: when one end
// goes away the other is closed too, rather than being left half-open.
func TestPipeConnectionsClosesBothSides(t *testing.T) {
	a, aPeer := net.Pipe()
	b, bPeer := net.Pipe()

	go pipeConnections(a, b)

	if _, err := aPeer.Write([]byte("ping")); err != nil {
		t.Fatalf("write to a: %v", err)
	}
	buf := make([]byte, 8)
	_ = bPeer.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := bPeer.Read(buf)
	if err != nil {
		t.Fatalf("read from b: %v", err)
	}
	if string(buf[:n]) != "ping" {
		t.Errorf("forwarded %q, want %q", buf[:n], "ping")
	}

	// Closing one peer must propagate through to the other side.
	_ = aPeer.Close()

	_ = bPeer.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := bPeer.Read(buf); err == nil {
		t.Error("bPeer still readable after aPeer closed; pipe was left open")
	}
}

// TestCopyDataStopsAtEOF pins that a copy returns rather than spinning once the
// source is done.
func TestCopyDataStopsAtEOF(t *testing.T) {
	src, srcPeer := net.Pipe()
	dst, dstPeer := net.Pipe()

	go func() {
		_, _ = srcPeer.Write([]byte("hello"))
		_ = srcPeer.Close()
	}()

	go dstPeer.Read(make([]byte, 16))

	done := make(chan struct{})
	go func() {
		copyData(dst, src)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("copyData did not return after the source closed")
	}
}
