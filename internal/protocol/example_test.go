package protocol_test

import (
	"bytes"
	"fmt"
	"log"

	"github.com/rrambul/shardwave/internal/protocol"
)

func ExampleWriteFrame() {
	ping := protocol.Ping{Seq: 42}
	payload, err := ping.MarshalBinary()
	if err != nil {
		log.Fatal(err)
	}

	var conn bytes.Buffer // stands in for a net.Conn
	if err := protocol.WriteFrame(&conn, protocol.Frame{Type: protocol.MsgPing, Payload: payload}); err != nil {
		log.Fatal(err)
	}

	frame, err := protocol.ReadFrame(&conn)
	if err != nil {
		log.Fatal(err)
	}
	var got protocol.Ping
	if err := got.UnmarshalBinary(frame.Payload); err != nil {
		log.Fatal(err)
	}
	fmt.Println(frame.Type, got.Seq)
	// Output: Ping 42
}
