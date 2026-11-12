// Package identity defines a node's ed25519 keypair and its PeerID, the
// SHA-256 of the public key.
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// PeerID is the logical identity of a node: sha256(ed25519 public key).
type PeerID [sha256.Size]byte

// PeerIDFromPublicKey derives the PeerID for a public key.
func PeerIDFromPublicKey(pub ed25519.PublicKey) PeerID {
	return sha256.Sum256(pub)
}

// String returns the 64-character hex encoding of the PeerID.
func (id PeerID) String() string { return hex.EncodeToString(id[:]) }

// Short returns the first 8 hex characters, for log lines.
func (id PeerID) Short() string { return id.String()[:8] }

// ParsePeerID parses the hex form produced by String.
func ParsePeerID(s string) (PeerID, error) {
	var id PeerID
	b, err := hex.DecodeString(s)
	if err != nil {
		return id, fmt.Errorf("identity: parse peer id: %w", err)
	}
	if len(b) != len(id) {
		return id, fmt.Errorf("identity: parse peer id: got %d bytes, want %d", len(b), len(id))
	}
	copy(id[:], b)
	return id, nil
}

// Identity is a node's keypair. The private key leaves this package only
// through Save.
type Identity struct {
	priv ed25519.PrivateKey
}

// Generate creates a new random identity.
func Generate() (*Identity, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("identity: generate: %w", err)
	}
	return &Identity{priv: priv}, nil
}

// FromSeed reconstructs an identity from its 32-byte seed.
func FromSeed(seed []byte) (*Identity, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("identity: seed is %d bytes, want %d", len(seed), ed25519.SeedSize)
	}
	return &Identity{priv: ed25519.NewKeyFromSeed(seed)}, nil
}

// PublicKey returns the public half of the keypair.
func (id *Identity) PublicKey() ed25519.PublicKey {
	return id.priv.Public().(ed25519.PublicKey)
}

// PeerID returns the node's PeerID.
func (id *Identity) PeerID() PeerID { return PeerIDFromPublicKey(id.PublicKey()) }

// Sign signs msg with the private key.
func (id *Identity) Sign(msg []byte) []byte { return ed25519.Sign(id.priv, msg) }

// Verify reports whether sig is a valid signature of msg under pub.
func Verify(pub ed25519.PublicKey, msg, sig []byte) bool {
	// ed25519.Verify panics on a key of the wrong length.
	return len(pub) == ed25519.PublicKeySize && ed25519.Verify(pub, msg, sig)
}

// Save writes the seed to path as hex, readable only by the owner.
func (id *Identity) Save(path string) error {
	data := []byte(hex.EncodeToString(id.priv.Seed()) + "\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("identity: save: %w", err)
	}
	return nil
}

// Load reads an identity written by Save.
func Load(path string) (*Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("identity: load: %w", err)
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("identity: load %s: %w", path, err)
	}
	id, err := FromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("identity: load %s: %w", path, err)
	}
	return id, nil
}

// LoadOrGenerate loads the identity at path, generating and saving a new one
// only if the file does not exist. A corrupt or unreadable file is an error,
// never replaced.
func LoadOrGenerate(path string) (*Identity, error) {
	id, err := Load(path)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	id, err = Generate()
	if err != nil {
		return nil, err
	}
	if err := id.Save(path); err != nil {
		return nil, err
	}
	return id, nil
}
