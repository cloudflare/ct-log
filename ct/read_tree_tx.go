package ct

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cloudflare/ct-log/custom"
	"github.com/cloudflare/ct-log/custom/frontier"

	"github.com/google/trillian"
	"github.com/google/trillian/storage"
	"github.com/google/trillian/storage/cache"
	"github.com/google/trillian/storage/storagepb"
)

// readOnlyLogTreeTX provides a read-only view into the log data. A
// readOnlyLogTreeTX can only read from the tree specified in its creation.
type readOnlyLogTreeTX struct {
	local        *custom.Local
	remote       *custom.Remote
	subtreeCache cache.SubtreeCache

	treeID int64
	root   trillian.SignedLogRoot
	front  frontier.Frontier
	closed bool
}

// LatestSignedLogRoot returns the most recent SignedLogRoot, if any.
func (rolt *readOnlyLogTreeTX) LatestSignedLogRoot(ctx context.Context) (trillian.SignedLogRoot, error) {
	return rolt.root, nil
}

// ReadRevision returns the tree revision that was current at the time this
// transaction was started.
func (rolt *readOnlyLogTreeTX) ReadRevision() int64 {
	return rolt.root.TreeRevision
}

// GetSequencedLeafCount returns the total number of leaves that have been
// integrated into the tree via sequencing.
func (rolt *readOnlyLogTreeTX) GetSequencedLeafCount(ctx context.Context) (int64, error) {
	return rolt.root.TreeSize, nil
}

// GetLeavesByIndex returns leaf metadata and data for a set of specified
// sequenced leaf indexes.
func (rolt *readOnlyLogTreeTX) GetLeavesByIndex(ctx context.Context, indexes []int64) ([]*trillian.LogLeaf, error) {
	// Verify the indexes are in an acceptable range.
	for _, idx := range indexes {
		if idx < 0 {
			return nil, fmt.Errorf("%v is a bad leaf index", idx)
		}
		if idx >= rolt.root.TreeSize {
			return nil, fmt.Errorf("there is no leaf with index %v yet", idx)
		}
	}

	// Read leaves from B2.
	leaves, err := rolt.remote.GetLeaves(ctx, rolt.treeID, indexes)
	if err != nil {
		return nil, err
	}

	// Verify that the Merkle hash of the leaf we read equals the Merkle hash
	// that the signer put in B2.
	nodeIds := make([]storage.NodeID, 0, len(leaves))
	for _, idx := range indexes {
		n, err := storage.NewNodeIDForTreeCoords(0, idx, 64)
		if err != nil {
			return nil, err
		}
		nodeIds = append(nodeIds, n)
	}
	nodes, err := rolt.GetMerkleNodes(ctx, rolt.ReadRevision(), nodeIds)
	if err != nil {
		return nil, err
	} else if len(nodes) != len(leaves) {
		return nil, fmt.Errorf("error verifying leaves read from B2")
	}
	for i, node := range nodes {
		if !bytes.Equal(node.Hash, leaves[i].MerkleLeafHash) {
			return nil, fmt.Errorf("leaf at index %v doesn't have the expected merkle hash", indexes[i])
		}
	}

	return leaves, nil
}

// GetLeavesByRange returns leaf data for a range of indexes.
func (rolt *readOnlyLogTreeTX) GetLeavesByRange(ctx context.Context, start, count int64) ([]*trillian.LogLeaf, error) {
	indexes := make([]int64, 0, count)
	for idx := start; idx < start+count && idx < rolt.root.TreeSize; idx++ {
		indexes = append(indexes, idx)
	}
	return rolt.GetLeavesByIndex(ctx, indexes)
}

// GetLeavesByHash looks up sequenced leaf metadata and data by their Merkle
// leaf hash.
func (rolt *readOnlyLogTreeTX) GetLeavesByHash(ctx context.Context, leafHashes [][]byte, orderBySequence bool) ([]*trillian.LogLeaf, error) {
	temp, err := rolt.local.GetSequenceByMerkleHash(rolt.treeID, leafHashes)
	if err != nil {
		return nil, err
	}

	// Drop any sequence numbers in temp that are out-of-bounds to build the
	// real slice of leaves to read.
	indexes := make([]int64, 0, len(temp))
	for _, idx := range temp {
		if idx < 0 || idx >= rolt.root.TreeSize {
			continue
		}
		indexes = append(indexes, idx)
	}

	// Read leaves from B2.
	leaves, err := rolt.remote.GetLeaves(ctx, rolt.treeID, indexes)
	if err != nil {
		return nil, err
	}

	// Verify that the Merkle hash of the leaf we read equals the Merkle hash
	// that we asked for.
	j := 0
	for i, idx := range temp {
		if idx < 0 || idx >= rolt.root.TreeSize {
			continue
		}

		if !bytes.Equal(leafHashes[i], leaves[j].MerkleLeafHash) {
			return nil, fmt.Errorf("leaf at index %v doesn't have the expected merkle hash", idx)
		}
		j++
	}
	if j != len(leaves) {
		return nil, fmt.Errorf("error verifying leaves read from B2")
	}

	return leaves, nil
}

// GetMerkleNodes looks up the set of nodes identified by ids, at treeRevision,
// and returns them.
func (rolt *readOnlyLogTreeTX) GetMerkleNodes(ctx context.Context, treeRevision int64, ids []storage.NodeID) ([]storage.Node, error) {
	return rolt.subtreeCache.GetNodes(ids, rolt.getSubtreesAtRev(ctx, treeRevision))
}

func (rolt *readOnlyLogTreeTX) getSubtreesAtRev(ctx context.Context, treeRevision int64) cache.GetSubtreesFunc {
	return func(ids []storage.NodeID) ([]*storagepb.SubtreeProto, error) {
		return rolt.getSubtrees(ctx, treeRevision, ids)
	}
}

func (rolt *readOnlyLogTreeTX) getSubtrees(ctx context.Context, treeRevision int64, ids []storage.NodeID) ([]*storagepb.SubtreeProto, error) {
	return rolt.local.GetSubtrees(rolt.treeID, treeRevision, ids)
}

func (rolt *readOnlyLogTreeTX) getSubtree(ctx context.Context, treeRevision int64, nodeID storage.NodeID) (*storagepb.SubtreeProto, error) {
	s, err := rolt.getSubtrees(ctx, treeRevision, []storage.NodeID{nodeID})
	if err != nil {
		return nil, err
	} else if len(s) == 0 {
		return nil, nil
	} else if len(s) == 1 {
		return s[0], nil
	}
	return nil, fmt.Errorf("got %d subtrees, but expected 0 or 1", len(s))
}

func (rolt *readOnlyLogTreeTX) Close() error {
	rolt.closed = true
	return nil
}

func (rolt *readOnlyLogTreeTX) Commit() error   { return rolt.Close() }
func (rolt *readOnlyLogTreeTX) Rollback() error { return rolt.Close() }
func (rolt *readOnlyLogTreeTX) IsOpen() bool    { return !rolt.closed }
