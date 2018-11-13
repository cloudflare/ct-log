package ct

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudflare/ct-log/custom"

	"github.com/google/trillian"
	"github.com/google/trillian/merkle/hashers"
	"github.com/google/trillian/storage"
	"github.com/google/trillian/storage/cache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	defaultLogStrata = []int{8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8}
)

// LogStorage implements storage.LogStorage over a Backblaze B2 bucket and a
// local queue of unsequenced certificates.
type LogStorage struct {
	Local  *custom.Local
	Remote *custom.Remote

	AdminStorage storage.AdminStorage
}

var _ storage.LogStorage = &LogStorage{}

// CheckDatabaseAccessible returns nil if the database is accessible, error
// otherwise.
func (ls *LogStorage) CheckDatabaseAccessible(ctx context.Context) error {
	return nil
}

// Snapshot starts a read-only transaction not tied to any particular tree.
func (ls *LogStorage) Snapshot(ctx context.Context) (storage.ReadOnlyLogTX, error) {
	tx, err := ls.AdminStorage.Snapshot(ctx)
	if err != nil {
		return nil, err
	}

	return &readOnlyLogTX{
		adminTx: tx,
		closed:  false,
	}, nil
}

// SnapshotForTree starts a read-only transaction for the specified treeID.
func (ls *LogStorage) SnapshotForTree(ctx context.Context, tree *trillian.Tree) (storage.ReadOnlyLogTreeTX, error) {
	hasher, err := hashers.NewLogHasher(trillian.HashStrategy_RFC6962_SHA256)
	if err != nil {
		return nil, err
	}
	stCache := cache.NewLogSubtreeCache(defaultLogStrata, hasher)

	root, front, err := ls.Local.MostRecentRoot(tree.TreeId)
	if err != nil {
		return nil, err
	}

	return &readOnlyLogTreeTX{
		local:        ls.Local,
		remote:       ls.Remote,
		subtreeCache: stCache,

		treeID: tree.TreeId,
		root:   root,
		front:  front,
		closed: false,
	}, nil
}

// beginForTree starts a transaction for the specified treeID.
func (ls *LogStorage) beginForTree(ctx context.Context, treeID int64) (storage.LogTreeTX, error) {
	hasher, err := hashers.NewLogHasher(trillian.HashStrategy_RFC6962_SHA256)
	if err != nil {
		return nil, err
	}
	stCache := cache.NewLogSubtreeCache(defaultLogStrata, hasher)

	root, front, err := ls.Local.MostRecentRoot(treeID)
	if err != nil && err != storage.ErrTreeNeedsInit {
		return nil, err
	}

	return &logTreeTX{
		readOnlyLogTreeTX: readOnlyLogTreeTX{
			local:        ls.Local,
			remote:       ls.Remote,
			subtreeCache: stCache,

			treeID: treeID,
			root:   root,
			front:  front,
			closed: false,
		},
		fsm: fsm{state: sBegin},

		localTx: ls.Local.Begin(),
	}, nil
}

// ReadWriteTransaction starts a RW transaction on the underlying storage, and
// calls f with it.
func (ls *LogStorage) ReadWriteTransaction(ctx context.Context, tree *trillian.Tree, f storage.LogTXFunc) error {
	ltx, err := ls.beginForTree(ctx, tree.TreeId)
	if err != nil {
		return err
	}
	if err := f(ctx, ltx); err != nil {
		ltx.Rollback()
		return err
	} else if err := ltx.Commit(); err != nil {
		return err
	}
	return nil
}

// QueueLeaves enqueues leaves for later integration into the tree.
func (ls *LogStorage) QueueLeaves(ctx context.Context, tree *trillian.Tree, leaves []*trillian.LogLeaf, queueTimestamp time.Time) ([]*trillian.QueuedLogLeaf, error) {
	root, _, err := ls.Local.MostRecentRoot(tree.TreeId)
	if err != nil {
		return nil, err
	}
	lt := &logTreeTX{
		readOnlyLogTreeTX: readOnlyLogTreeTX{
			local:  ls.Local,
			remote: ls.Remote,

			treeID: tree.TreeId,
			root:   root,
			closed: false,
		},
	}

	out := make([]*trillian.QueuedLogLeaf, 0, len(leaves))
	for i, leaf := range leaves {
		dup, err := lt.queueLeaf(ctx, leaf, queueTimestamp)
		if err != nil {
			return nil, err
		}

		if dup == nil {
			out = append(out, &trillian.QueuedLogLeaf{Leaf: leaves[i]})
		} else {
			out = append(out, &trillian.QueuedLogLeaf{
				Leaf:   dup,
				Status: status.Newf(codes.AlreadyExists, "leaf already exists: %v", dup.LeafIdentityHash).Proto(),
			})
		}
	}
	return out, nil
}

// AddSequencedLeaves stores the `leaves` and associates them with the log
// positions according to their `LeafIndex` field. The indices must be
// contiguous.
func (ls *LogStorage) AddSequencedLeaves(ctx context.Context, tree *trillian.Tree, leaves []*trillian.LogLeaf, ts time.Time) ([]*trillian.QueuedLogLeaf, error) {
	return nil, fmt.Errorf("adding sequenced leaves is not implemented")
}
