package frontier

import (
	"testing"

	"bytes"
	"crypto/rand"

	"github.com/google/trillian"
	"github.com/google/trillian/merkle"
	"github.com/google/trillian/merkle/hashers"
	_ "github.com/google/trillian/merkle/rfc6962"
)

func TestTreeHead(t *testing.T) {
	tree1 := &Frontier{}

	h, err := hashers.NewLogHasher(trillian.HashStrategy_RFC6962_SHA256)
	if err != nil {
		t.Fatal(err)
	}
	tree2 := merkle.NewInMemoryMerkleTree(h)

	if !bytes.Equal(tree1.Head(), tree2.CurrentRoot().Hash()) {
		t.Fatal("nil sth is interpreted incorrectly")
	}

	for leaves := 0; leaves < 128; leaves++ {
		cert := make([]byte, 1024)
		if _, err := rand.Read(cert); err != nil {
			t.Fatal(err)
		}

		tree1.Append(hashDomain(0x00, cert))
		tree2.AddLeaf(cert)
	}
	if !bytes.Equal(tree1.Head(), tree2.CurrentRoot().Hash()) {
		t.Fatal("trees with a power of 2 number of leaves are hashed incorrectly")
	}

	for leaves := 0; leaves < 7000-128; leaves++ {
		cert := make([]byte, 1024)
		if _, err := rand.Read(cert); err != nil {
			t.Fatal(err)
		}

		tree1.Append(hashDomain(0x00, cert))
		tree2.AddLeaf(cert)
	}
	if !bytes.Equal(tree1.Head(), tree2.CurrentRoot().Hash()) {
		t.Fatal("trees with a random number of leaves are hashed incorrectly")
	}
}
