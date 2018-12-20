package custom

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/gob"
	"fmt"

	"github.com/cloudflare/ct-log/custom/frontier"

	"github.com/golang/protobuf/proto"
	"github.com/google/trillian"
	"github.com/google/trillian/storage"
	"github.com/google/trillian/storage/storagepb"
	"github.com/google/trillian/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func dupSlice(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

func keyS(typ byte, treeID int64, val string) []byte {
	return []byte(fmt.Sprintf("%s%16.16x:%v", string(typ), treeID, val))
}

func keyB(typ byte, treeID int64, val []byte) []byte {
	return append([]byte(fmt.Sprintf("%s%16.16x:", string(typ), treeID)), val...)
}

// Local implements convenience methods over a local database connection. The
// local database is for metadata and indices, because they're small and
// frequently accessed.
type Local struct {
	db *leveldb.DB
}

// NewLocal returns a new local database, with data stored at `path`.
func NewLocal(path string) (*Local, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &Local{db}, nil
}

// MostRecentRoot returns most-recently committed root for the tree with the
// given treeID.
func (l *Local) MostRecentRoot(treeID int64) (trillian.SignedLogRoot, frontier.Frontier, error) {
	snap, err := l.db.GetSnapshot()
	if err != nil {
		return trillian.SignedLogRoot{}, frontier.Frontier{}, err
	}
	defer snap.Release()

	rootRaw, err := snap.Get(keyS('r', treeID, "root"), nil)
	if err == leveldb.ErrNotFound {
		return trillian.SignedLogRoot{}, frontier.Frontier{}, storage.ErrTreeNeedsInit
	} else if err != nil {
		return trillian.SignedLogRoot{}, frontier.Frontier{}, err
	}
	sig, err := snap.Get(keyS('r', treeID, "sig"), nil)
	if err != nil {
		return trillian.SignedLogRoot{}, frontier.Frontier{}, err
	}
	frontRaw, err := snap.Get(keyS('r', treeID, "frontier"), nil)
	if err != nil {
		return trillian.SignedLogRoot{}, frontier.Frontier{}, err
	}
	rootRaw, sig, frontRaw = dupSlice(rootRaw), dupSlice(sig), dupSlice(frontRaw)

	root := types.LogRootV1{}
	if err = root.UnmarshalBinary(rootRaw); err != nil {
		return trillian.SignedLogRoot{}, frontier.Frontier{}, err
	}
	sth := trillian.SignedLogRoot{
		TimestampNanos: int64(root.TimestampNanos),
		RootHash:       root.RootHash,
		TreeSize:       int64(root.TreeSize),
		TreeRevision:   int64(root.Revision),

		KeyHint:          types.SerializeKeyHint(treeID),
		LogRoot:          rootRaw,
		LogRootSignature: sig,
	}

	front := frontier.Frontier{}
	if err = gob.NewDecoder(bytes.NewBuffer(frontRaw)).Decode(&front); err != nil {
		return trillian.SignedLogRoot{}, frontier.Frontier{}, err
	}

	return sth, front, nil
}

func (l *Local) QueueLeaves(treeID, queueTimestamp int64, leaves []*trillian.LogLeaf) error {
	batch := new(leveldb.Batch)
	for _, leaf := range leaves {
		v, err := proto.Marshal(leaf)
		if err != nil {
			return err
		}
		batch.Put(keyB('l', treeID, rowkeyLeaf(queueTimestamp, true)), v)
	}
	return l.db.Write(batch, nil)
}

// Unsequenced returns the number of unsequenced leaves that a log has on disk.
func (l *Local) Unsequenced(treeID int64) (int, error) {
	snap, err := l.db.GetSnapshot()
	if err != nil {
		return 0, err
	}
	defer snap.Release()

	keys := 0

	iter := snap.NewIterator(&util.Range{
		Start: keyB('l', treeID, rowkeyLeaf(0, false)),
		Limit: keyB('l', treeID+1, rowkeyLeaf(0, false)),
	}, nil)
	for iter.Next() {
		keys++
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		return 0, nil
	}

	return keys, nil
}

// GetSequenceByMerkleHash returns the sequence numbers for the leaves with the
// given Merkle hashes, in the tree with the given tree id. Missing sequence
// numbers are returned as -1.
func (l *Local) GetSequenceByMerkleHash(treeID int64, hashes [][]byte) ([]int64, error) {
	leaves, err := l.getSequenceBy('m', treeID, hashes)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup by merkle hash: %v", err)
	}
	return leaves, nil
}

// GetSequenceByIdentityHash returns the sequence numbers for the leaves with
// the given identity hashes, in the tree with the given tree id. Missing
// sequence numbers are returned as -1.
func (l *Local) GetSequenceByIdentityHash(treeID int64, hashes [][]byte) ([]int64, error) {
	leaves, err := l.getSequenceBy('i', treeID, hashes)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup by identity hash: %v", err)
	}
	return leaves, nil
}

func (l *Local) getSequenceBy(typ byte, treeID int64, hashes [][]byte) ([]int64, error) {
	out := make([]int64, 0, len(hashes))

	snap, err := l.db.GetSnapshot()
	if err != nil {
		return nil, err
	}
	defer snap.Release()

	for _, hash := range hashes {
		raw, err := snap.Get(keyB(typ, treeID, hash), nil)
		if err == leveldb.ErrNotFound {
			out = append(out, -1)
			continue
		} else if err != nil {
			return nil, err
		}
		idx, n := binary.Varint(raw)
		if n != len(raw) {
			return nil, fmt.Errorf("malformed entry in index")
		}
		out = append(out, idx)
	}

	return out, nil
}

// GetSubtrees returns the most recent revision ( <= treeRevision ) of each
// subtree with a given id. Missing subtrees are silently elided.
func (l *Local) GetSubtrees(treeID, treeRevision int64, ids []storage.NodeID) ([]*storagepb.SubtreeProto, error) {
	out := make([]*storagepb.SubtreeProto, 0, len(ids))

	for i, id := range ids {
		subtree, err := l.getSubtree(treeID, treeRevision, id)
		if err != nil {
			return nil, fmt.Errorf("node id #%v: %v", i+1, err)
		} else if subtree != nil {
			out = append(out, subtree)
		}
	}

	return out, nil
}

func (l *Local) getSubtree(treeID, treeRevision int64, id storage.NodeID) (*storagepb.SubtreeProto, error) {
	start, stop, err := rangeNodeID(treeRevision, id)
	if err != nil {
		return nil, err
	}
	start, stop = keyB('s', treeID, start), keyB('s', treeID, stop)

	// Get the row with the equivalent rowkey or its immediate predecessor.
	snap, err := l.db.GetSnapshot()
	if err != nil {
		return nil, err
	}
	defer snap.Release()

	iter := snap.NewIterator(nil, nil)

	var k, v []byte
	if ok := iter.Seek(start); ok {
		if bytes.Equal(iter.Key(), start) {
			k, v = iter.Key(), iter.Value()
		} else if ok := iter.Prev(); ok {
			k, v = iter.Key(), iter.Value()
		}
	} else if ok := iter.Last(); ok {
		k, v = iter.Key(), iter.Value()
	}
	if k != nil && bytes.Compare(k, start) != 1 && bytes.Compare(k, stop) == 1 {
		k, v = nil, dupSlice(v)
	} else {
		k, v = nil, nil
	}

	iter.Release()
	if err := iter.Error(); err != nil {
		return nil, err
	}

	// Parse the subtree, should we have found one.
	if v == nil {
		return nil, nil
	}

	subtree := &storagepb.SubtreeProto{}
	if err := proto.Unmarshal(v, subtree); err != nil {
		return nil, err
	}
	if subtree.Prefix == nil {
		subtree.Prefix = []byte{}
	}
	return subtree, nil
}

func (l *Local) Begin() *LocalTx {
	return &LocalTx{
		db:    l.db,
		batch: &leveldb.Batch{},
	}
}

// LocalTx implements convenience methods over a transaction with the local
// storage.
type LocalTx struct {
	db    *leveldb.DB
	batch *leveldb.Batch
}

func (ltx *LocalTx) DequeueLeaves(treeID, seq, cutoffTime int64, limit int) ([]*trillian.LogLeaf, error) {
	leaves := make([]*trillian.LogLeaf, 0)

	snap, err := ltx.db.GetSnapshot()
	if err != nil {
		return nil, err
	}
	defer snap.Release()

	iter := snap.NewIterator(&util.Range{
		Start: keyB('l', treeID, rowkeyLeaf(0, false)),
		Limit: keyB('l', treeID, rowkeyLeaf(cutoffTime+1, false)),
	}, nil)
	for iter.Next() {
		if len(leaves) >= limit {
			break
		}
		leaf := &trillian.LogLeaf{}
		if err := proto.Unmarshal(iter.Value(), leaf); err != nil {
			return nil, err
		}

		ltx.batch.Delete(dupSlice(iter.Key()))
		leaves = append(leaves, leaf)
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		return nil, err
	}

	return leaves, nil
}

// PutLeaves creates the index entries that let leaves be looked up by
// merkle/identity hash. It is assumed that seqs[i] corresponds to
// merkleHashes[i], corresponds to idHashes[i].
func (ltx *LocalTx) PutLeaves(treeID int64, seqs []int64, merkleHashes, idHashes [][]byte) error {
	if len(seqs) != len(merkleHashes) {
		return fmt.Errorf("different number of sequence numbers than merkle hashes")
	} else if len(seqs) != len(idHashes) {
		return fmt.Errorf("different number of sequence numbers than identity hashes")
	}

	for i, seq := range seqs {
		raw := make([]byte, 8)
		n := binary.PutVarint(raw, seq)

		ltx.batch.Put(keyB('m', treeID, merkleHashes[i]), raw[:n])
		ltx.batch.Put(keyB('i', treeID, idHashes[i]), raw[:n])
	}

	return nil
}

// PutSubtrees serializes the subtrees and writes them to disk, indexed by their
// id. It is assumed that ids[i] corresponds to subtrees[i].
func (ltx *LocalTx) PutSubtrees(treeID, treeRevision int64, ids []storage.NodeID, subtrees []*storagepb.SubtreeProto) error {
	if len(ids) != len(subtrees) {
		return fmt.Errorf("different number of ids than subtrees")
	}

	for i, subtree := range subtrees {
		rowkey, err := rowkeyNodeID(treeRevision, ids[i])
		if err != nil {
			return fmt.Errorf("node id #%v: %v", i+1, err)
		}
		raw, err := proto.Marshal(subtree)
		if err != nil {
			return err
		}
		ltx.batch.Put(keyB('s', treeID, rowkey), raw)
	}

	return nil
}

func (ltx *LocalTx) StoreRoot(treeID int64, root trillian.SignedLogRoot, front frontier.Frontier) error {
	frontRaw := &bytes.Buffer{}
	if err := gob.NewEncoder(frontRaw).Encode(front); err != nil {
		return err
	}

	ltx.batch.Put(keyS('r', treeID, "root"), dupSlice(root.LogRoot))
	ltx.batch.Put(keyS('r', treeID, "sig"), dupSlice(root.LogRootSignature))
	ltx.batch.Put(keyS('r', treeID, "frontier"), frontRaw.Bytes())

	return nil
}

func (ltx *LocalTx) Commit() error {
	tx, err := ltx.db.OpenTransaction()
	if err != nil {
		return err
	} else if err := tx.Write(ltx.batch, nil); err != nil {
		tx.Discard()
		return err
	} else if err := tx.Commit(); err != nil {
		return err
	}

	// Clear the transaction to prevent it from being used again.
	ltx.batch = nil
	return nil
}

func rowkeyLeaf(queueTimestamp int64, noise bool) []byte {
	key := make([]byte, 12)
	for i := uint(0); i < 8; i++ {
		key[7-i] = byte(queueTimestamp >> (8 * i))
	}
	if noise {
		if _, err := rand.Read(key[8:]); err != nil {
			panic(err)
		}
	}
	return key
}

func rowkeyNodeID(treeRevision int64, id storage.NodeID) ([]byte, error) {
	if id.PrefixLenBits%8 != 0 {
		return nil, fmt.Errorf("invalid subtree ID - not multiple of 8: %v", id.PrefixLenBits)
	} else if id.PrefixLenBits < 0 || id.PrefixLenBits > 64 {
		return nil, fmt.Errorf("invalid subtree ID - bad prefix length: %v", id.PrefixLenBits)
	} else if len(id.Path) > 8 {
		return nil, fmt.Errorf("invalid subtree ID - path is too long: %v", len(id.Path))
	}

	// The rowkey should be:
	//   node ID (zero padding) || ID length || big-endian treeRevision.
	rowkey := make([]byte, 17)
	copy(rowkey[:8], id.Path[:id.PrefixLenBits/8])
	rowkey[8] = byte(id.PrefixLenBits)
	for i := uint(0); i < 8; i++ {
		rowkey[i+9] = byte(treeRevision >> (56 - 8*i))
	}

	return rowkey, nil
}

func rangeNodeID(treeRevision int64, id storage.NodeID) (start, stop []byte, err error) {
	start, err = rowkeyNodeID(treeRevision, id)
	if err != nil {
		return
	}

	// The stopping rowkey is immediately less than:
	//   node ID (zero padding) || ID length || big-endian zero.
	stop = make([]byte, 17)
	copy(stop, start)
	for i := 0; i < 8; i++ {
		stop[i+9] = 0xff
	}
	if stop[8] == 0x00 {
		stop = make([]byte, 0) // Need to be smaller than all-zero rowkey.
	} else {
		stop[8]--
	}

	return
}
