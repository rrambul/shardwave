package peer_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rrambul/shardwave/internal/peer"
)

func ExampleLoadOrGenerate() {
	dir, err := os.MkdirTemp("", "peer")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "node.key")

	first, err := peer.LoadOrGenerate(path) // no file yet: generates and saves one
	if err != nil {
		fmt.Println(err)
		return
	}
	second, err := peer.LoadOrGenerate(path) // file present: loads it
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(first.ID() == second.ID())
	// Output: true
}

func ExampleVerify() {
	k, err := peer.Generate()
	if err != nil {
		log.Fatal(err)
	}
	msg := []byte("hello")
	sig := k.Sign(msg)

	fmt.Println(peer.Verify(k.PublicKey(), msg, sig))
	fmt.Println(peer.Verify(k.PublicKey(), []byte("tampered"), sig))
	// Output:
	// true
	// false
}
