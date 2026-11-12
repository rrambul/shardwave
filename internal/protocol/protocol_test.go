package protocol

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestMsgTypeString(t *testing.T) {
	tests := []struct {
		typ  MsgType
		want string
	}{
		{MsgHello, "Hello"},
		{MsgHelloAck, "HelloAck"},
		{MsgPing, "Ping"},
		{MsgPong, "Pong"},
		{MsgInvalid, "MsgType(0)"},
		{MsgType(200), "MsgType(200)"},
	}
	for _, tt := range tests {
		if got := tt.typ.String(); got != tt.want {
			t.Errorf("MsgType(%d).String() = %q, want %q", uint8(tt.typ), got, tt.want)
		}
	}
}

func TestWriteFrameWireLayout(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, Frame{Type: MsgPing, Payload: []byte{0xAA, 0xBB, 0xCC}}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	want := []byte{
		0x00, 0x00, 0x00, 0x03, // length = 3, big-endian
		byte(MsgPing),    // type
		0xAA, 0xBB, 0xCC, // payload
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("wire bytes = % x\n             want = % x", buf.Bytes(), want)
	}
}

func TestFrameRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		frame Frame
	}{
		{"empty payload", Frame{Type: MsgPing}},
		{"small payload", Frame{Type: MsgHello, Payload: []byte("hello")}},
		{"binary payload", Frame{Type: MsgPong, Payload: []byte{0, 1, 2, 255, 0}}},
		{"1 MiB payload", Frame{Type: MsgHelloAck, Payload: bytes.Repeat([]byte{7}, 1<<20)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteFrame(&buf, tt.frame); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}
			got, err := ReadFrame(&buf)
			if err != nil {
				t.Fatalf("ReadFrame: %v", err)
			}
			if got.Type != tt.frame.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.frame.Type)
			}
			if !bytes.Equal(got.Payload, tt.frame.Payload) { // nil and empty compare equal
				t.Errorf("Payload length %d, want %d", len(got.Payload), len(tt.frame.Payload))
			}
			if buf.Len() != 0 {
				t.Errorf("ReadFrame left %d unread bytes; it must consume exactly one frame", buf.Len())
			}
		})
	}
}

func TestReadFrameSequential(t *testing.T) {
	frames := []Frame{
		{Type: MsgHello, Payload: []byte("a")},
		{Type: MsgPing},
		{Type: MsgPong, Payload: []byte("ccc")},
	}
	var buf bytes.Buffer
	for _, f := range frames {
		if err := WriteFrame(&buf, f); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}
	for i, want := range frames {
		got, err := ReadFrame(&buf)
		if err != nil {
			t.Fatalf("ReadFrame #%d: %v", i, err)
		}
		if got.Type != want.Type || !bytes.Equal(got.Payload, want.Payload) {
			t.Fatalf("frame #%d = %+v, want %+v", i, got, want)
		}
	}
	if _, err := ReadFrame(&buf); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadFrame on drained stream = %v, want io.EOF", err)
	}
}

func TestReadFrameDistinguishesCleanEOFFromTruncation(t *testing.T) {
	var full bytes.Buffer
	if err := WriteFrame(&full, Frame{Type: MsgPing, Payload: []byte("12345678")}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	wire := full.Bytes()

	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"empty stream", nil, io.EOF},
		{"partial header", wire[:3], io.ErrUnexpectedEOF},
		{"header only, payload missing", wire[:headerSize], io.ErrUnexpectedEOF},
		{"partial payload", wire[:headerSize+4], io.ErrUnexpectedEOF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadFrame(bytes.NewReader(tt.in))
			if !errors.Is(err, tt.want) {
				t.Errorf("ReadFrame = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestWriteFrameRejectsOversizedPayload(t *testing.T) {
	var buf bytes.Buffer
	err := WriteFrame(&buf, Frame{Type: MsgPing, Payload: make([]byte, MaxPayloadSize+1)})
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("WriteFrame(oversized) = %v, want ErrPayloadTooLarge", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("WriteFrame wrote %d bytes before failing; it must write nothing on error", buf.Len())
	}
}

func TestWriteFrameRejectsUnknownType(t *testing.T) {
	for _, typ := range []MsgType{MsgInvalid, MsgType(200)} {
		var buf bytes.Buffer
		err := WriteFrame(&buf, Frame{Type: typ})
		if !errors.Is(err, ErrUnknownType) {
			t.Errorf("WriteFrame(type %d) = %v, want ErrUnknownType", uint8(typ), err)
		}
		if buf.Len() != 0 {
			t.Errorf("WriteFrame(type %d) wrote %d bytes before failing", uint8(typ), buf.Len())
		}
	}
}

func TestReadFrameRejectsOversizedLengthFromHeaderAlone(t *testing.T) {
	header := []byte{0xFF, 0xFF, 0xFF, 0xFF, byte(MsgPing)}
	_, err := ReadFrame(bytes.NewReader(header))
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("ReadFrame(length=0xFFFFFFFF) = %v, want ErrPayloadTooLarge", err)
	}

	boundary := []byte{0x01, 0x00, 0x00, 0x01, byte(MsgPing)} // MaxPayloadSize + 1
	if _, err := ReadFrame(bytes.NewReader(boundary)); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("ReadFrame(length=MaxPayloadSize+1) = %v, want ErrPayloadTooLarge", err)
	}
}

func TestReadFrameRejectsUnknownType(t *testing.T) {
	for _, typ := range []byte{0, 200} {
		in := []byte{0, 0, 0, 0, typ}
		_, err := ReadFrame(bytes.NewReader(in))
		if !errors.Is(err, ErrUnknownType) {
			t.Errorf("ReadFrame(type %d) = %v, want ErrUnknownType", typ, err)
		}
	}
}

func TestHelloRoundTrip(t *testing.T) {
	h := Hello{Version: Version}
	for i := range h.PublicKey {
		h.PublicKey[i] = byte(i)
	}
	for i := range h.Nonce {
		h.Nonce[i] = byte(255 - i)
	}

	b, err := h.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if len(b) != helloSize {
		t.Fatalf("encoded length = %d, want %d", len(b), helloSize)
	}
	if b[0] != Version {
		t.Errorf("first byte = %d, want Version %d", b[0], Version)
	}

	var got Hello
	if err := got.UnmarshalBinary(b); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if got != h {
		t.Fatalf("round-trip = %+v, want %+v", got, h)
	}
}

func TestHelloUnmarshalRejectsWrongLength(t *testing.T) {
	for _, n := range []int{0, helloSize - 1, helloSize + 1} {
		var h Hello
		if err := h.UnmarshalBinary(make([]byte, n)); !errors.Is(err, ErrBadLength) {
			t.Errorf("UnmarshalBinary(%d bytes) = %v, want ErrBadLength", n, err)
		}
	}
}

func TestHelloAckRoundTrip(t *testing.T) {
	var a HelloAck
	for i := range a.Signature {
		a.Signature[i] = byte(i * 3)
	}

	b, err := a.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if len(b) != helloAckSize {
		t.Fatalf("encoded length = %d, want %d", len(b), helloAckSize)
	}

	var got HelloAck
	if err := got.UnmarshalBinary(b); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if got != a {
		t.Fatal("round-trip changed the signature")
	}
}

func TestHelloAckUnmarshalRejectsWrongLength(t *testing.T) {
	for _, n := range []int{0, helloAckSize - 1, helloAckSize + 1} {
		var a HelloAck
		if err := a.UnmarshalBinary(make([]byte, n)); !errors.Is(err, ErrBadLength) {
			t.Errorf("UnmarshalBinary(%d bytes) = %v, want ErrBadLength", n, err)
		}
	}
}

func TestPingPongWireLayoutAndRoundTrip(t *testing.T) {
	const seq = uint64(0x0102030405060708)
	wantBytes := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08} // big-endian

	ping := Ping{Seq: seq}
	pb, err := ping.MarshalBinary()
	if err != nil {
		t.Fatalf("Ping.MarshalBinary: %v", err)
	}
	if !bytes.Equal(pb, wantBytes) {
		t.Errorf("Ping bytes = % x, want % x", pb, wantBytes)
	}
	var gotPing Ping
	if err := gotPing.UnmarshalBinary(pb); err != nil {
		t.Fatalf("Ping.UnmarshalBinary: %v", err)
	}
	if gotPing.Seq != seq {
		t.Errorf("Ping.Seq = %#x, want %#x", gotPing.Seq, seq)
	}

	pong := Pong{Seq: seq}
	qb, err := pong.MarshalBinary()
	if err != nil {
		t.Fatalf("Pong.MarshalBinary: %v", err)
	}
	if !bytes.Equal(qb, wantBytes) {
		t.Errorf("Pong bytes = % x, want % x", qb, wantBytes)
	}
	var gotPong Pong
	if err := gotPong.UnmarshalBinary(qb); err != nil {
		t.Fatalf("Pong.UnmarshalBinary: %v", err)
	}
	if gotPong.Seq != seq {
		t.Errorf("Pong.Seq = %#x, want %#x", gotPong.Seq, seq)
	}
}

func TestPingPongUnmarshalRejectWrongLength(t *testing.T) {
	for _, n := range []int{0, seqSize - 1, seqSize + 1} {
		var ping Ping
		if err := ping.UnmarshalBinary(make([]byte, n)); !errors.Is(err, ErrBadLength) {
			t.Errorf("Ping.UnmarshalBinary(%d bytes) = %v, want ErrBadLength", n, err)
		}
		var pong Pong
		if err := pong.UnmarshalBinary(make([]byte, n)); !errors.Is(err, ErrBadLength) {
			t.Errorf("Pong.UnmarshalBinary(%d bytes) = %v, want ErrBadLength", n, err)
		}
	}
}

func TestHandshakeDigest(t *testing.T) {
	var n1, n2 [32]byte
	n2[0] = 1

	d1 := HandshakeDigest(n1)
	if !bytes.Equal(d1, HandshakeDigest(n1)) {
		t.Fatal("HandshakeDigest is not deterministic")
	}
	if bytes.Equal(d1, HandshakeDigest(n2)) {
		t.Fatal("HandshakeDigest gave the same bytes for different nonces")
	}
	if !bytes.HasPrefix(d1, []byte(handshakeDomain)) {
		t.Errorf("digest does not start with the domain string %q", handshakeDomain)
	}
	if !bytes.HasSuffix(d1, n1[:]) {
		t.Error("digest does not end with the nonce")
	}
	if len(d1) != len(handshakeDomain)+32 {
		t.Errorf("digest length = %d, want %d", len(d1), len(handshakeDomain)+32)
	}
}
