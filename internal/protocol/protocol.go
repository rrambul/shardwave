// Package protocol defines the wire format: frames of
// [u32 length][u8 type][payload], big-endian, and the encoding of each
// message. It is independent of the transport that carries the bytes.
package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// Version is carried in Hello; peers with a different version are rejected.
	Version uint8 = 1

	// MaxPayloadSize bounds a frame's payload. ReadFrame rejects a larger
	// length from the header alone, before allocating.
	MaxPayloadSize = 16 << 20 // 16 MiB

	headerSize   = 4 + 1
	helloSize    = 1 + 32 + 32 // version + public key + nonce
	helloAckSize = 64          // ed25519 signature
	seqSize      = 8

	// handshakeDomain is prefixed to the nonce before signing, so the
	// signature cannot be replayed into another protocol.
	handshakeDomain = "shardwave/hello/v1:"
)

// MsgType identifies what a frame's payload is.
type MsgType uint8

// Message types carried in Frame.Type.
const (
	MsgInvalid MsgType = iota
	MsgHello
	MsgHelloAck
	MsgPing
	MsgPong

	msgTypeEnd
)

// String returns the message name, or "MsgType(N)" for unknown values.
func (t MsgType) String() string {
	switch t {
	case MsgHello:
		return "Hello"
	case MsgHelloAck:
		return "HelloAck"
	case MsgPing:
		return "Ping"
	case MsgPong:
		return "Pong"
	default:
		return fmt.Sprintf("MsgType(%d)", uint8(t))
	}
}

func (t MsgType) valid() bool { return t > MsgInvalid && t < msgTypeEnd }

// Frame is one unit on the wire: a type and an opaque payload.
type Frame struct {
	Type    MsgType
	Payload []byte
}

// Errors returned by WriteFrame, ReadFrame, and the UnmarshalBinary methods.
var (
	ErrPayloadTooLarge = errors.New("protocol: payload exceeds MaxPayloadSize")
	ErrUnknownType     = errors.New("protocol: unknown message type")
	ErrBadLength       = errors.New("protocol: payload has wrong length for message")
)

// WriteFrame writes f to w. On error nothing has been written, so the stream
// stays in sync.
func WriteFrame(w io.Writer, f Frame) error {
	if !f.Type.valid() {
		return fmt.Errorf("%w: %d", ErrUnknownType, uint8(f.Type))
	}
	if len(f.Payload) > MaxPayloadSize {
		return fmt.Errorf("%w: %d bytes", ErrPayloadTooLarge, len(f.Payload))
	}

	buf := make([]byte, headerSize+len(f.Payload))
	binary.BigEndian.PutUint32(buf[0:4], uint32(len(f.Payload))) //nolint:gosec // bounded by the MaxPayloadSize check above
	buf[4] = byte(f.Type)
	copy(buf[headerSize:], f.Payload)

	_, err := w.Write(buf)
	return err
}

// ReadFrame reads one frame from r. It returns io.EOF only if the stream
// ended before the first byte of a frame, and io.ErrUnexpectedEOF if it ended
// inside one.
func ReadFrame(r io.Reader) (Frame, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Frame{}, err
	}

	length := binary.BigEndian.Uint32(header[0:4])
	typ := MsgType(header[4])

	if !typ.valid() {
		return Frame{}, fmt.Errorf("%w: %d", ErrUnknownType, uint8(typ))
	}
	if length > MaxPayloadSize {
		return Frame{}, fmt.Errorf("%w: header announces %d bytes", ErrPayloadTooLarge, length)
	}

	f := Frame{Type: typ}
	if length == 0 {
		return f, nil
	}

	f.Payload = make([]byte, length)
	if _, err := io.ReadFull(r, f.Payload); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF // header with no payload is a truncated frame
		}
		return Frame{}, err
	}
	return f, nil
}

func badLength(msg string, got, want int) error {
	return fmt.Errorf("%w: %s is %d bytes, want %d", ErrBadLength, msg, got, want)
}

// Hello opens the handshake. Both sides send one; the exchange is symmetric.
type Hello struct {
	Version   uint8
	PublicKey [32]byte
	Nonce     [32]byte
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (h *Hello) MarshalBinary() ([]byte, error) {
	b := make([]byte, 0, helloSize)
	b = append(b, h.Version)
	b = append(b, h.PublicKey[:]...)
	b = append(b, h.Nonce[:]...)
	return b, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (h *Hello) UnmarshalBinary(b []byte) error {
	if len(b) != helloSize {
		return badLength("Hello", len(b), helloSize)
	}
	h.Version = b[0]
	copy(h.PublicKey[:], b[1:1+32])
	copy(h.Nonce[:], b[1+32:])
	return nil
}

// HelloAck completes the handshake: a signature over HandshakeDigest of the
// peer's nonce.
type HelloAck struct {
	Signature [64]byte
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (a *HelloAck) MarshalBinary() ([]byte, error) {
	return bytes.Clone(a.Signature[:]), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (a *HelloAck) UnmarshalBinary(b []byte) error {
	if len(b) != helloAckSize {
		return badLength("HelloAck", len(b), helloAckSize)
	}
	copy(a.Signature[:], b)
	return nil
}

// Ping is a liveness probe; Pong echoes its Seq.
type Ping struct {
	Seq uint64
}

// Pong answers a Ping.
type Pong struct {
	Seq uint64
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (p *Ping) MarshalBinary() ([]byte, error) { return marshalSeq(p.Seq), nil }

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (p *Ping) UnmarshalBinary(b []byte) error { return unmarshalSeq("Ping", b, &p.Seq) }

// MarshalBinary implements encoding.BinaryMarshaler.
func (p *Pong) MarshalBinary() ([]byte, error) { return marshalSeq(p.Seq), nil }

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (p *Pong) UnmarshalBinary(b []byte) error { return unmarshalSeq("Pong", b, &p.Seq) }

func marshalSeq(seq uint64) []byte {
	return binary.BigEndian.AppendUint64(make([]byte, 0, seqSize), seq)
}

func unmarshalSeq(msg string, b []byte, dst *uint64) error {
	if len(b) != seqSize {
		return badLength(msg, len(b), seqSize)
	}
	*dst = binary.BigEndian.Uint64(b)
	return nil
}

// HandshakeDigest returns the bytes a node signs in HelloAck:
// handshakeDomain ‖ peerNonce.
func HandshakeDigest(peerNonce [32]byte) []byte {
	b := make([]byte, 0, len(handshakeDomain)+len(peerNonce))
	b = append(b, handshakeDomain...)
	return append(b, peerNonce[:]...)
}
