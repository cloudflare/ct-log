package custom

import (
	"testing"

	"fmt"

	"github.com/google/trillian"
	"github.com/google/trillian/storage"
	"github.com/google/trillian/storage/storagepb"
)

func TestStructRoot(t *testing.T) {
	want := "trillian.SignedLogRoot{TimestampNanos:0, RootHash:[]uint8(nil), TreeSize:0, TreeRevision:0, KeyHint:[]uint8(nil), LogRoot:[]uint8(nil), LogRootSignature:[]uint8(nil), XXX_NoUnkeyedLiteral:struct {}{}, XXX_unrecognized:[]uint8(nil), XXX_sizecache:0}"
	cand := fmt.Sprintf("%#v", trillian.SignedLogRoot{})

	if want != cand {
		t.Log(cand)
		t.Fatal("SignedLogRoot struct has changed")
	}
}

func TestStructLeaf(t *testing.T) {
	want := "trillian.LogLeaf{MerkleLeafHash:[]uint8(nil), LeafValue:[]uint8(nil), ExtraData:[]uint8(nil), LeafIndex:0, LeafIdentityHash:[]uint8(nil), QueueTimestamp:(*timestamp.Timestamp)(nil), IntegrateTimestamp:(*timestamp.Timestamp)(nil), XXX_NoUnkeyedLiteral:struct {}{}, XXX_unrecognized:[]uint8(nil), XXX_sizecache:0}"
	cand := fmt.Sprintf("%#v", trillian.LogLeaf{})

	if want != cand {
		t.Log(cand)
		t.Fatal("LogLeaf struct has changed")
	}
}

func TestStructNodeID(t *testing.T) {
	want := "storage.NodeID{Path:[]uint8(nil), PrefixLenBits:0}"
	cand := fmt.Sprintf("%#v", storage.NodeID{})

	if want != cand {
		t.Fatal("NodeID struct has changed")
	}
}

func TestStructSubtreeProto(t *testing.T) {
	want := "storagepb.SubtreeProto{Prefix:[]uint8(nil), Depth:0, RootHash:[]uint8(nil), Leaves:map[string][]uint8(nil), InternalNodes:map[string][]uint8(nil), InternalNodeCount:0x0, XXX_NoUnkeyedLiteral:struct {}{}, XXX_unrecognized:[]uint8(nil), XXX_sizecache:0}"
	cand := fmt.Sprintf("%#v", storagepb.SubtreeProto{})

	if want != cand {
		t.Log(cand)
		t.Fatal("SubtreeProto struct has changed")
	}
}
