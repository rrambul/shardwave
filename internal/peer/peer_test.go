package peer

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func mustGenerate(t *testing.T) *Identity {
	t.Helper()
	k, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return k
}

func TestGenerateProducesDistinctIdentities(t *testing.T) {
	a, b := mustGenerate(t), mustGenerate(t)
	if a.ID() == b.ID() {
		t.Fatalf("two generated identities share ID %s", a.ID())
	}
}

func TestIDIsDerivedFromPublicKey(t *testing.T) {
	k := mustGenerate(t)
	if got, want := k.ID(), IDFromPublicKey(k.PublicKey()); got != want {
		t.Fatalf("ID() = %s, want %s", got, want)
	}
}

func TestFromSeedIsDeterministic(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	a, err := FromSeed(seed)
	if err != nil {
		t.Fatalf("FromSeed: %v", err)
	}
	b, err := FromSeed(seed)
	if err != nil {
		t.Fatalf("FromSeed: %v", err)
	}
	if a.ID() != b.ID() {
		t.Fatalf("FromSeed(seed) twice gave IDs %s and %s, want equal", a.ID(), b.ID())
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
	k, other := mustGenerate(t), mustGenerate(t)
	msg := []byte("shardwave")
	sig := k.Sign(msg)

	tests := []struct {
		name string
		pub  ed25519.PublicKey
		msg  []byte
		sig  []byte
		want bool
	}{
		{"valid", k.PublicKey(), msg, sig, true},
		{"tampered message", k.PublicKey(), []byte("shardwavE"), sig, false},
		{"wrong key", other.PublicKey(), msg, sig, false},
		{"truncated signature", k.PublicKey(), msg, sig[:10], false},
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

func TestIDStringRoundTrip(t *testing.T) {
	id := mustGenerate(t).ID()
	s := id.String()

	if len(s) != 64 {
		t.Fatalf("String() has length %d, want 64", len(s))
	}
	if short := id.Short(); short != s[:8] {
		t.Errorf("Short() = %q, want %q", short, s[:8])
	}

	parsed, err := ParseID(s)
	if err != nil {
		t.Fatalf("ParseID(%q): %v", s, err)
	}
	if parsed != id {
		t.Fatalf("ParseID round-trip = %s, want %s", parsed, id)
	}
}

func TestParseIDRejectsBadInput(t *testing.T) {
	tests := []struct{ name, in string }{
		{"empty", ""},
		{"not hex", strings.Repeat("zz", 32)},
		{"too short", "abcd"},
		{"too long", strings.Repeat("ab", 33)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseID(tt.in); err == nil {
				t.Errorf("ParseID(%q) = nil error, want error", tt.in)
			}
		})
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	k := mustGenerate(t)
	path := filepath.Join(t.TempDir(), "node.key")

	if err := k.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID() != k.ID() {
		t.Fatalf("Load gave ID %s, want %s", loaded.ID(), k.ID())
	}

	if runtime.GOOS != "windows" { // Windows has no POSIX permission bits
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("key file mode = %o, want 600", perm)
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
			_, err := Load(path)
			if err == nil {
				t.Fatalf("Load(%q) = nil error, want error", tt.content)
			}
			if n := strings.Count(err.Error(), "peer: "); n != 1 {
				t.Errorf("Load(%q) error = %q, want the package prefix once, got %d", tt.content, err, n)
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
	if first.ID() != second.ID() {
		t.Fatalf("second call gave ID %s, want %s", second.ID(), first.ID())
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
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "corrupt\n" {
		t.Fatalf("corrupt key file was modified to %q", data)
	}
}
