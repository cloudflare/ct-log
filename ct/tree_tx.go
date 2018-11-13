package ct

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/cloudflare/ct-log/ct/cache"
	"github.com/cloudflare/ct-log/custom"

	"github.com/google/trillian"
	"github.com/google/trillian/storage"
	"github.com/google/trillian/storage/storagepb"
)

// leafCache stores *trillian.LogLeaf's that have been queued but may not have
// been integrated into an STH yet. This reduces the number of duplicate leaves
// that can be added.
var leafCache = cache.New(1*time.Hour, 1*time.Minute, 75000)

func dupSlice(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

func dupLeaf(leaf *trillian.LogLeaf) *trillian.LogLeaf {
	return &trillian.LogLeaf{
		MerkleLeafHash:   dupSlice(leaf.MerkleLeafHash),
		LeafValue:        dupSlice(leaf.LeafValue),
		ExtraData:        dupSlice(leaf.ExtraData),
		LeafIndex:        leaf.LeafIndex,
		LeafIdentityHash: dupSlice(leaf.LeafIdentityHash),
	}
}

func addLeaf(treeID int64, leaf *trillian.LogLeaf) {
	leafCache.Set(
		fmt.Sprintf("treeID=%v,id=%x", treeID, leaf.LeafIdentityHash),
		dupLeaf(leaf), cache.DefaultExpiration,
	)
}

func getLeafByIdentityHash(treeID int64, id []byte) *trillian.LogLeaf {
	leaf, ok := leafCache.Get(fmt.Sprintf("treeID=%v,id=%x", treeID, id))
	if !ok {
		return nil
	}
	return dupLeaf(leaf.(*trillian.LogLeaf))
}

// logTreeTX is the transactional interface for reading/updating a Log. A
// logTreeTX can only modify the tree specified in its creation.
type logTreeTX struct {
	readOnlyLogTreeTX
	fsm

	localTx *custom.LocalTx

	queuedLeaves     bool
	dequeuedChecksum []byte
}

// WriteRevision returns the tree revision that any writes through this
// logTreeTX will be stored at.
func (lt *logTreeTX) WriteRevision(ctx context.Context) (int64, error) {
	return lt.root.TreeRevision + 1, nil
}

// QueueLeaves enqueues leaves for later integration into the tree.
//
// If error is nil, the returned slice of leaves will be the same size as the
// input, and each entry will hold:
//  - the existing leaf entry if a duplicate has been submitted
//  - nil otherwise.
//
// Duplicates are only reported if the underlying tree does not permit
// duplicates, and are considered duplicate if their leaf.LeafIdentityHash
// matches.
func (lt *logTreeTX) QueueLeaves(ctx context.Context, leaves []*trillian.LogLeaf, queueTimestamp time.Time) ([]*trillian.LogLeaf, error) {
	if err := lt.emit(sQueueLeaves); err != nil {
		return nil, err
	}
	lt.queuedLeaves = true

	out := make([]*trillian.LogLeaf, 0, len(leaves))
	for _, leaf := range leaves {
		l, err := lt.queueLeaf(ctx, leaf, queueTimestamp)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

func (lt *logTreeTX) queueLeaf(ctx context.Context, leaf *trillian.LogLeaf, queueTimestamp time.Time) (*trillian.LogLeaf, error) {
	// NOTE(brendan): To improve performance of add-chain, lt is minimally
	// mocked in LogStorage.QueueLeaves.

	// Check if this leaf is already in our local cache.
	cachedLeaf := getLeafByIdentityHash(lt.treeID, leaf.LeafIdentityHash)
	if cachedLeaf != nil {
		return cachedLeaf, nil
	}
	// Since it's not in cache, check if this leaf is already in B2.
	seq, err := lt.getSequenceByIdentityHash(ctx, leaf.LeafIdentityHash)
	if err != nil {
		return nil, err
	} else if seq > 0 && seq < lt.root.TreeSize {
		// Read the old leaf from B2.
		dup, err := lt.remote.GetLeaves(ctx, lt.treeID, []int64{seq})
		if err != nil {
			return nil, err
		}
		return dup[0], nil
	}

	// Save the new leaves to disk and our local cache.
	err = lt.local.QueueLeaves(lt.treeID, queueTimestamp.UnixNano(), []*trillian.LogLeaf{leaf})
	if err != nil {
		return nil, err
	}
	addLeaf(lt.treeID, leaf)

	return nil, nil
}

func (lt *logTreeTX) getSequenceByIdentityHash(ctx context.Context, id []byte) (int64, error) {
	seqs, err := lt.local.GetSequenceByIdentityHash(lt.treeID, [][]byte{id})
	if err != nil {
		return 0, err
	}
	return seqs[0], nil
}

// DequeueLeaves will return between [0, limit] leaves from the queue.
//
// Leaves which have been dequeued within a rolled-back tx will become available
// for dequeuing again. Leaves queued more recently than the cutoff time will
// not be returned.
func (lt *logTreeTX) DequeueLeaves(ctx context.Context, limit int, cutoffTime time.Time) ([]*trillian.LogLeaf, error) {
	if err := lt.emit(sDequeueLeaves); err != nil {
		return nil, err
	}

	leaves, err := lt.localTx.DequeueLeaves(lt.treeID, lt.root.TreeSize, cutoffTime.UnixNano(), limit)
	if err != nil {
		return nil, err
	}

	// Checksum the leaves that we got, so we can verify they weren't modified
	// when we get them back in UpdateSequencedLeaves.
	if lt.dequeuedChecksum != nil {
		return nil, fmt.Errorf("refusing to overwrite previous dequeued checksum")
	}
	sum, err := checksumLeaves(leaves)
	if err != nil {
		return nil, err
	}
	lt.dequeuedChecksum = sum

	return leaves, nil
}

func (lt *logTreeTX) AddSequencedLeaves(ctx context.Context, leaves []*trillian.LogLeaf, ts time.Time) ([]*trillian.QueuedLogLeaf, error) {
	return nil, fmt.Errorf("adding sequenced leaves is not implemented")
}

func (lt *logTreeTX) UpdateSequencedLeaves(ctx context.Context, leaves []*trillian.LogLeaf) error {
	if err := lt.emit(sUpdateSequencedLeaves); err != nil {
		return err
	}

	// Verify that the leaves trillian wants me to store are what I actually
	// gave it in DequeueLeaves.
	for _, leaf := range leaves {
		leaf.IntegrateTimestamp = nil
	}
	sum, err := checksumLeaves(leaves)
	if err != nil {
		return err
	} else if !bytes.Equal(lt.dequeuedChecksum, sum) {
		return fmt.Errorf("leaf checksum does not match")
	}
	lt.dequeuedChecksum = nil

	// Save leaves to B2.
	err = lt.remote.PutLeaves(ctx, lt.treeID, leaves)
	if err != nil {
		return err
	}

	// Index leaves by Merkle hash and by identity hash.
	var (
		seqs         = make([]int64, 0, len(leaves))
		merkleHashes = make([][]byte, 0, len(leaves))
		idHashes     = make([][]byte, 0, len(leaves))
	)
	for _, leaf := range leaves {
		lt.front.Append(leaf.MerkleLeafHash)

		seqs = append(seqs, leaf.LeafIndex)
		merkleHashes = append(merkleHashes, leaf.MerkleLeafHash)
		idHashes = append(idHashes, leaf.LeafIdentityHash)
	}
	err = lt.localTx.PutLeaves(lt.treeID, seqs, merkleHashes, idHashes)
	if err != nil {
		return err
	}

	return nil
}

// SetMerkleNodes stores the provided nodes, at the transaction's WriteRevision.
func (lt *logTreeTX) SetMerkleNodes(ctx context.Context, nodes []storage.Node) error {
	if err := lt.emit(sSetMerkleNodes); err != nil {
		return err
	}

	getSubtree := func(nID storage.NodeID) (*storagepb.SubtreeProto, error) {
		rev, err := lt.WriteRevision(ctx)
		if err != nil {
			return nil, err
		}
		return lt.getSubtree(ctx, rev, nID)
	}

	for _, n := range nodes {
		err := lt.subtreeCache.SetNodeHash(n.NodeID, n.Hash, getSubtree)
		if err != nil {
			return err
		}
	}
	return nil
}

// StoreSignedLogRoot stores a freshly created SignedLogRoot.
func (lt *logTreeTX) StoreSignedLogRoot(ctx context.Context, root trillian.SignedLogRoot) error {
	if err := lt.emit(sStoreSignedLogRoot); err != nil {
		return err
	} else if !bytes.Equal(root.RootHash, lt.front.Head()) {
		return fmt.Errorf("root hash does not match what is expected")
	}

	if err := lt.localTx.StoreRoot(lt.treeID, root, lt.front); err != nil {
		return err
	}

	return nil
}

func (lt *logTreeTX) storeSubtrees(subtrees []*storagepb.SubtreeProto) error {
	ids := make([]storage.NodeID, 0)
	for _, subtree := range subtrees {
		if len(subtree.Prefix) > 8 {
			return fmt.Errorf("subtree prefix is too long: %v", len(subtree.Prefix))
		}
		ids = append(ids, storage.NodeID{subtree.Prefix, 8 * len(subtree.Prefix)})
	}
	rev, err := lt.WriteRevision(context.TODO())
	if err != nil {
		return err
	}
	return lt.localTx.PutSubtrees(lt.treeID, rev, ids, subtrees)
}

func (lt *logTreeTX) Commit() error {
	if err := lt.emit(sCommit); err != nil {
		return err
	} else if lt.queuedLeaves {
		lt.closed = true
		return nil
	}

	if err := lt.subtreeCache.Flush(lt.storeSubtrees); err != nil {
		return err
	} else if err := lt.localTx.Commit(); err != nil {
		return err
	}

	lt.closed = true
	return nil
}

func (lt *logTreeTX) Rollback() error {
	if err := lt.emit(sRollback); err != nil {
		return err
	}
	lt.closed = true
	return nil
}

func (lt *logTreeTX) Close() error {
	if err := lt.emit(sClose); err != nil {
		return err
	}
	lt.closed = true
	return nil
}

func (lt *logTreeTX) IsOpen() bool { return !lt.closed }

func checksumLeaves(in []*trillian.LogLeaf) ([]byte, error) {
	h := sha256.New()
	if err := gob.NewEncoder(h).Encode(in); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}
