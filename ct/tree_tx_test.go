package ct

import (
	"testing"

	"fmt"
	"reflect"

	"github.com/google/trillian"
)

func TestStructLeaf(t *testing.T) {
	want := "trillian.LogLeaf{MerkleLeafHash:[]uint8(nil), LeafValue:[]uint8(nil), ExtraData:[]uint8(nil), LeafIndex:0, LeafIdentityHash:[]uint8(nil), QueueTimestamp:(*timestamp.Timestamp)(nil), IntegrateTimestamp:(*timestamp.Timestamp)(nil), XXX_NoUnkeyedLiteral:struct {}{}, XXX_unrecognized:[]uint8(nil), XXX_sizecache:0}"
	cand := fmt.Sprintf("%#v", trillian.LogLeaf{})

	if want != cand {
		t.Fatal("LogLeaf struct has changed")
	}
}

func TestStructQueuedLeaf(t *testing.T) {
	want := "trillian.QueuedLogLeaf{Leaf:(*trillian.LogLeaf)(nil), Status:(*status.Status)(nil), XXX_NoUnkeyedLiteral:struct {}{}, XXX_unrecognized:[]uint8(nil), XXX_sizecache:0}"
	cand := fmt.Sprintf("%#v", trillian.QueuedLogLeaf{})

	if want != cand {
		t.Fatal("QueuedLogLeaf struct has changed")
	}
}

func TestDupLeaf(t *testing.T) {
	in := &trillian.LogLeaf{
		MerkleLeafHash:   []byte("asdf"),
		LeafValue:        []byte("fdsa"),
		ExtraData:        []byte("qwerty"),
		LeafIndex:        700,
		LeafIdentityHash: []byte("yuiop"),
	}
	out := dupLeaf(in)

	if !reflect.DeepEqual(in, out) {
		t.Fatal("duplicated leaf is not equal to original")
	}
}
