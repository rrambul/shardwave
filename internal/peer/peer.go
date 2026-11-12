// Package peer defines a node's ed25519 keypair and its ID, the SHA-256 of
// the public key.
package peer

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

// ID is the logical identity of a node: sha256(ed25519 public key).
type ID [sha256.Size]byte

// IDFromPublicKey derives the ID for a public key.
func IDFromPublicKey(pub ed25519.PublicKey) ID {
	return sha256.Sum256(pub)
}

// String returns the 64-character hex encoding of the ID.
func (id ID) String() string { return hex.EncodeToString(id[:]) }

// Short returns the first 8 hex characters, for log lines.
func (id ID) Short() string { return id.String()[:8] }

// ParseID parses the hex form produced by String.
func ParseID(s string) (ID, error) {
	var id ID
	b, err := hex.DecodeString(s)
	if err != nil {
		return id, fmt.Errorf("peer: parse id: %w", err)
	}
	if len(b) != len(id) {
		return id, fmt.Errorf("peer: parse id: got %d bytes, want %d", len(b), len(id))
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
		return nil, fmt.Errorf("peer: generate: %w", err)
	}
	return &Identity{priv: priv}, nil
}

// FromSeed reconstructs an identity from its 32-byte seed.
func FromSeed(seed []byte) (*Identity, error) {
	k, err := fromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("peer: %w", err)
	}
	return k, nil
}

func fromSeed(seed []byte) (*Identity, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("seed is %d bytes, want %d", len(seed), ed25519.SeedSize)
	}
	return &Identity{priv: ed25519.NewKeyFromSeed(seed)}, nil
}

// PublicKey returns the public half of the keypair.
func (k *Identity) PublicKey() ed25519.PublicKey {
	return k.priv.Public().(ed25519.PublicKey)
}

// ID returns the node's ID.
func (k *Identity) ID() ID { return IDFromPublicKey(k.PublicKey()) }

// Sign signs msg with the private key.
func (k *Identity) Sign(msg []byte) []byte { return ed25519.Sign(k.priv, msg) }

// Verify reports whether sig is a valid signature of msg under pub.
func Verify(pub ed25519.PublicKey, msg, sig []byte) bool {
	// ed25519.Verify panics on a key of the wrong length.
	return len(pub) == ed25519.PublicKeySize && ed25519.Verify(pub, msg, sig)
}

// Save writes the seed to path as hex, readable only by the owner.
func (k *Identity) Save(path string) error {
	data := []byte(hex.EncodeToString(k.priv.Seed()) + "\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("peer: save: %w", err)
	}
	return nil
}

// Load reads an identity written by Save.
func Load(path string) (*Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("peer: load: %w", err)
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("peer: load %s: %w", path, err)
	}
	k, err := fromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("peer: load %s: %w", path, err)
	}
	return k, nil
}

// LoadOrGenerate loads the identity at path, generating and saving a new one
// only if the file does not exist. A corrupt or unreadable file is an error,
// never replaced.
func LoadOrGenerate(path string) (*Identity, error) {
	k, err := Load(path)
	if err == nil {
		return k, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	k, err = Generate()
	if err != nil {
		return nil, err
	}
	if err := k.Save(path); err != nil {
		return nil, err
	}
	return k, nil
}
