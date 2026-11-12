package identity

import (
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func mustGenerate(t *testing.T) *Identity {
	t.Helper()
	id, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return id
}

func TestGenerateProducesDistinctIdentities(t *testing.T) {
	a, b := mustGenerate(t), mustGenerate(t)
	if a.PeerID() == b.PeerID() {
		t.Fatalf("two generated identities share PeerID %s", a.PeerID())
	}
}

func TestPeerIDIsDerivedFromPublicKey(t *testing.T) {
	id := mustGenerate(t)

	if got, want := id.PeerID(), PeerIDFromPublicKey(id.PublicKey()); got != want {
		t.Fatalf("PeerID() = %s, want %s", got, want)
	}

	again, err := FromSeed(id.priv.Seed())
	if err != nil {
		t.Fatalf("FromSeed: %v", err)
	}
	if again.PeerID() != id.PeerID() {
		t.Fatalf("FromSeed gave PeerID %s, want %s", again.PeerID(), id.PeerID())
	}
}

func TestFromSeedRejectsWrongLength(t *testing.T) {
	for _, n := range []int{0, 31, 33, 64} {
		if _, err := FromSeed(make([]byte, n)); err == nil {
			t.Errorf("FromSeed(%d bytes) = nil error, want error", n)
		}
	}
}

func TestSignVerify(t *testing.T) {
	id, other := mustGenerate(t), mustGenerate(t)
	msg := []byte("shardwave")
	sig := id.Sign(msg)

	tests := []struct {
		name string
		pub  ed25519.PublicKey
		msg  []byte
		sig  []byte
		want bool
	}{
		{"valid", id.PublicKey(), msg, sig, true},
		{"tampered message", id.PublicKey(), []byte("shardwavE"), sig, false},
		{"wrong key", other.PublicKey(), msg, sig, false},
		{"truncated signature", id.PublicKey(), msg, sig[:10], false},
		{"malformed public key must not panic", ed25519.PublicKey("short"), msg, sig, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Verify(tt.pub, tt.msg, tt.sig); got != tt.want {
				t.Errorf("Verify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPeerIDStringRoundTrip(t *testing.T) {
	id := mustGenerate(t).PeerID()
	s := id.String()

	if len(s) != 64 {
		t.Fatalf("String() has length %d, want 64", len(s))
	}
	if short := id.Short(); short != s[:8] {
		t.Errorf("Short() = %q, want %q", short, s[:8])
	}

	parsed, err := ParsePeerID(s)
	if err != nil {
		t.Fatalf("ParsePeerID(%q): %v", s, err)
	}
	if parsed != id {
		t.Fatalf("ParsePeerID round-trip = %s, want %s", parsed, id)
	}
}

func TestParsePeerIDRejectsBadInput(t *testing.T) {
	tests := []struct{ name, in string }{
		{"empty", ""},
		{"not hex", strings.Repeat("zz", 32)},
		{"too short", "abcd"},
		{"too long", strings.Repeat("ab", 33)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParsePeerID(tt.in); err == nil {
				t.Errorf("ParsePeerID(%q) = nil error, want error", tt.in)
			}
		})
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	id := mustGenerate(t)
	path := filepath.Join(t.TempDir(), "node.key")

	if err := id.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.PeerID() != id.PeerID() {
		t.Fatalf("Load gave PeerID %s, want %s", loaded.PeerID(), id.PeerID())
	}

	if runtime.GOOS != "windows" { // Windows has no POSIX permission bits
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("key file mode = %o, want 600 — private keys are owner-only", perm)
		}
	}
}

func TestLoadRejectsGarbage(t *testing.T) {
	tests := []struct{ name, content string }{
		{"not hex", "hello world\n"},
		{"wrong length", "abcd\n"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "node.key")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Errorf("Load(%q) = nil error, want error", tt.content)
			}
		})
	}
}

func TestLoadMissingFileWrapsNotExist(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.key"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load(missing) = %v, want an error wrapping os.ErrNotExist", err)
	}
}

func TestLoadOrGenerate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.key")

	first, err := LoadOrGenerate(path) // file absent: generates and saves
	if err != nil {
		t.Fatalf("LoadOrGenerate (first): %v", err)
	}
	second, err := LoadOrGenerate(path) // file present: loads
	if err != nil {
		t.Fatalf("LoadOrGenerate (second): %v", err)
	}
	if first.PeerID() != second.PeerID() {
		t.Fatalf("second call gave PeerID %s, want %s", second.PeerID(), first.PeerID())
	}
}

func TestLoadOrGenerateDoesNotReplaceCorruptKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.key")
	if err := os.WriteFile(path, []byte("corrupt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrGenerate(path); err == nil {
		t.Fatal("LoadOrGenerate on a corrupt key file = nil error; it must not silently overwrite it")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "corrupt\n" {
		t.Fatalf("corrupt key file was modified to %q", data)
	}
}
