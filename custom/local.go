package custom

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/gob"
	"fmt"

	"github.com/cloudflare/ct-log/custom/frontier"

	"github.com/etcd-io/bbolt"
	"github.com/golang/protobuf/proto"
	"github.com/google/trillian"
	"github.com/google/trillian/storage"
	"github.com/google/trillian/storage/storagepb"
	"github.com/google/trillian/types"
)

func dupSlice(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

func name(table string, treeID int64) []byte {
	return []byte(fmt.Sprintf("%v-%v", table, treeID))
}

// Local implements convenience methods over a local database connection. The
// local database is for metadata and indices, because they're small and
// frequently accessed.
type Local struct {
	db *bbolt.DB
}

// NewLocal returns a new local database, with data stored at `path`.
func NewLocal(path string) (*Local, error) {
	db, err := bbolt.Open(path, 0777, &bbolt.Options{})
	if err != nil {
		return nil, err
	}
	return &Local{db}, nil
}

// MostRecentRoot returns most-recently committed root for the tree with the
// given treeID.
func (l *Local) MostRecentRoot(treeID int64) (trillian.SignedLogRoot, frontier.Frontier, error) {
	var rootRaw, sig, frontRaw []byte
	err := l.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(name("sth", treeID))
		if b == nil {
			return storage.ErrTreeNeedsInit
		}
		rootRaw = dupSlice(b.Get([]byte("root")))
		sig = dupSlice(b.Get([]byte("sig")))
		frontRaw = dupSlice(b.Get([]byte("frontier")))
		return nil
	})
	if err != nil {
		return trillian.SignedLogRoot{}, frontier.Frontier{}, err
	}

	root := types.LogRootV1{}
	if err = gob.NewDecoder(bytes.NewBuffer(rootRaw)).Decode(&root); err != nil {
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
	return l.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(name("leaves", treeID))
		if b == nil {
			return storage.ErrTreeNeedsInit
		}

		for _, leaf := range leaves {
			key, err := rowkeyLeaf(queueTimestamp, true)
			if err != nil {
				return err
			}
			val, err := proto.Marshal(leaf)
			if err != nil {
				return err
			}

			if err := b.Put(key, val); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetSequenceByMerkleHash returns the sequence numbers for the leaves with the
// given Merkle hashes, in the tree with the given tree id. Missing sequence
// numbers are returned as -1.
func (l *Local) GetSequenceByMerkleHash(treeID int64, hashes [][]byte) ([]int64, error) {
	leaves, err := l.getSequenceBy("merkle", treeID, hashes)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup by merkle hash: %v", err)
	}
	return leaves, nil
}

// GetSequenceByIdentityHash returns the sequence numbers for the leaves with
// the given identity hashes, in the tree with the given tree id. Missing
// sequence numbers are returned as -1.
func (l *Local) GetSequenceByIdentityHash(treeID int64, hashes [][]byte) ([]int64, error) {
	leaves, err := l.getSequenceBy("identity", treeID, hashes)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup by identity hash: %v", err)
	}
	return leaves, nil
}

func (l *Local) getSequenceBy(table string, treeID int64, hashes [][]byte) ([]int64, error) {
	out := make([]int64, 0, len(hashes))

	err := l.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(name(table, treeID))
		if b == nil {
			return storage.ErrTreeNeedsInit
		}

		for _, hash := range hashes {
			raw := b.Get(hash)
			if raw == nil {
				out = append(out, -1)
				continue
			}
			idx, n := binary.Varint(raw)
			if n != len(raw) {
				return fmt.Errorf("malformed entry in index")
			}
			out = append(out, idx)
		}

		return nil
	})
	if err != nil {
		return nil, err
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

	// Get the row with the equivalent rowkey or its immediate predecessor.
	var raw []byte
	err = l.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(name("subtrees", treeID))
		if b == nil {
			return storage.ErrTreeNeedsInit
		}
		c := b.Cursor()

		k, v := c.Seek(stop)
		if bytes.Equal(stop, k) {
			raw = dupSlice(v)
			return nil
		} else if k == nil {
			k, v = c.Last()
		} else {
			k, v = c.Prev()
		}
		if bytes.Compare(start, k) != 1 {
			raw = dupSlice(v)
		}

		return nil
	})
	if err != nil {
		return nil, err
	} else if raw == nil {
		return nil, nil
	}

	subtree := &storagepb.SubtreeProto{}
	if err := proto.Unmarshal(raw, subtree); err != nil {
		return nil, err
	}
	if subtree.Prefix == nil {
		subtree.Prefix = []byte{}
	}
	return subtree, nil
}

func (l *Local) Begin() *LocalTx {
	return &LocalTx{
		db:      l.db,
		treeIDs: make(map[int64]struct{}),
	}
}

// LocalTx implements convenience methods over a transaction with the local
// storage.
type LocalTx struct {
	db      *bbolt.DB
	treeIDs map[int64]struct{}
	acts    []func(*bbolt.Tx) error
}

func (ltx *LocalTx) DequeueLeaves(treeID, seq, cutoffTime int64, limit int) ([]*trillian.LogLeaf, error) {
	ltx.treeIDs[treeID] = struct{}{}

	keys := make([][]byte, 0)
	leaves := make([]*trillian.LogLeaf, 0)

	// Read off the new leaves.
	err := ltx.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(name("leaves", treeID))
		if b == nil {
			return storage.ErrTreeNeedsInit
		}
		c := b.Cursor()

		stop, err := rowkeyLeaf(cutoffTime+1, false)
		if err != nil {
			return err
		}
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.Compare(k, stop) != -1 {
				break
			} else if len(leaves) >= limit {
				break
			}
			leaf := &trillian.LogLeaf{}
			if err := proto.Unmarshal(v, leaf); err != nil {
				return err
			}

			keys = append(keys, dupSlice(k))
			leaves = append(leaves, leaf)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Note to delete these leaves when the transaction is committed.
	ltx.acts = append(ltx.acts, func(tx *bbolt.Tx) error {
		b := tx.Bucket(name("leaves", treeID))
		if b == nil {
			return storage.ErrTreeNeedsInit
		}

		for _, key := range keys {
			if err := b.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})

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
	ltx.treeIDs[treeID] = struct{}{}

	// Duplicate all these values so we don't have to worry about them changing
	// between now and when the transaction is committed.
	seqs2 := make([]int64, 0, len(seqs))
	for _, seq := range seqs {
		seqs2 = append(seqs2, seq)
	}
	merkles2 := make([][]byte, 0, len(merkleHashes))
	for _, hash := range merkleHashes {
		merkles2 = append(merkles2, dupSlice(hash))
	}
	ids2 := make([][]byte, 0, len(idHashes))
	for _, hash := range idHashes {
		ids2 = append(ids2, dupSlice(hash))
	}

	ltx.acts = append(ltx.acts, func(tx *bbolt.Tx) error {
		bMerkle := tx.Bucket(name("merkle", treeID))
		if bMerkle == nil {
			return storage.ErrTreeNeedsInit
		}
		bId := tx.Bucket(name("identity", treeID))
		if bId == nil {
			return storage.ErrTreeNeedsInit
		}

		for i, seq := range seqs2 {
			raw := make([]byte, 8)
			n := binary.PutVarint(raw, seq)

			if err := bMerkle.Put(merkles2[i], raw[:n]); err != nil {
				return err
			} else if err := bId.Put(ids2[i], raw[:n]); err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

// PutSubtrees serializes the subtrees and writes them to disk, indexed by their
// id. It is assumed that ids[i] corresponds to subtrees[i].
func (ltx *LocalTx) PutSubtrees(treeID, treeRevision int64, ids []storage.NodeID, subtrees []*storagepb.SubtreeProto) error {
	if len(ids) != len(subtrees) {
		return fmt.Errorf("different number of ids than subtrees")
	}
	ltx.treeIDs[treeID] = struct{}{}

	data := make([][2][]byte, 0, len(ids))
	for i, subtree := range subtrees {
		rowkey, err := rowkeyNodeID(treeRevision, ids[i])
		if err != nil {
			return fmt.Errorf("node id #%v: %v", i+1, err)
		}
		raw, err := proto.Marshal(subtree)
		if err != nil {
			return err
		}
		data = append(data, [2][]byte{rowkey, raw})
	}

	ltx.acts = append(ltx.acts, func(tx *bbolt.Tx) error {
		b := tx.Bucket(name("subtrees", treeID))
		if b == nil {
			return storage.ErrTreeNeedsInit
		}

		for _, row := range data {
			if err := b.Put(row[0], row[1]); err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

func (ltx *LocalTx) StoreRoot(treeID int64, root trillian.SignedLogRoot, front frontier.Frontier) error {
	ltx.treeIDs[treeID] = struct{}{}

	rootRaw := dupSlice(root.LogRoot)
	sig := dupSlice(root.LogRootSignature)
	frontRaw := &bytes.Buffer{}
	if err := gob.NewEncoder(frontRaw).Encode(front); err != nil {
		return err
	}

	ltx.acts = append(ltx.acts, func(tx *bbolt.Tx) error {
		b := tx.Bucket(name("sth", treeID))
		if b == nil {
			return storage.ErrTreeNeedsInit
		} else if err := b.Put([]byte("root"), rootRaw); err != nil {
			return err
		} else if err := b.Put([]byte("sig"), sig); err != nil {
			return err
		} else if err := b.Put([]byte("frontier"), frontRaw.Bytes()); err != nil {
			return err
		}
		return nil
	})

	return nil
}

func (ltx *LocalTx) Commit() error {
	return ltx.db.Update(func(tx *bbolt.Tx) error {
		// Create any buckets if necessary.
		for _, table := range []string{"sth", "leaves", "merkle", "identity", "subtrees"} {
			for treeID, _ := range ltx.treeIDs {
				_, err := tx.CreateBucketIfNotExists(name(table, treeID))
				if err != nil {
					return err
				}
			}
		}

		// Execute queued actions in the transaction.
		for _, act := range ltx.acts {
			if err := act(tx); err != nil {
				return nil
			}
		}

		// Clear the transaction to prevent it from being used again.
		ltx.treeIDs, ltx.acts = nil, nil
		return nil
	})
}

func rowkeyLeaf(queueTimestamp int64, noise bool) ([]byte, error) {
	key := make([]byte, 12)
	for i := uint(0); i < 8; i++ {
		key[7-i] = byte(queueTimestamp >> (8 * i))
	}
	if noise {
		if _, err := rand.Read(key[8:]); err != nil {
			return nil, err
		}
	}
	return key, nil
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
