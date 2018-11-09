package config

import (
	"context"
	"fmt"

	"github.com/google/trillian"
	"github.com/google/trillian/storage"
)

type adminStorage struct {
	trees []*trillian.Tree
}

// Snapshot starts a read-only transaction.
func (as *adminStorage) Snapshot(ctx context.Context) (storage.ReadOnlyAdminTX, error) {
	return &adminTx{as.trees}, nil
}

// ReadWriteTransaction creates a transaction, and runs f with it.
func (as *adminStorage) ReadWriteTransaction(ctx context.Context, f storage.AdminTXFunc) error {
	return f(ctx, &adminTx{as.trees})
}

// CheckDatabaseAccessible checks whether we are able to connect to / open the
// underlying storage.
func (as *adminStorage) CheckDatabaseAccessible(ctx context.Context) error {
	return nil
}

type adminTx struct {
	trees []*trillian.Tree
}

func (at *adminTx) Commit() error   { return nil }
func (at *adminTx) Rollback() error { return nil }
func (at *adminTx) Close() error    { return nil }
func (at *adminTx) IsClosed() bool  { return false }

// GetTree returns the tree corresponding to treeID or an error.
func (at *adminTx) GetTree(ctx context.Context, treeID int64) (*trillian.Tree, error) {
	for _, tree := range at.trees {
		if tree.TreeId == treeID {
			return tree, nil
		}
	}

	return nil, fmt.Errorf("tree %v not found", treeID)
}

// ListTreeIDs returns the IDs of all trees in storage.
func (at *adminTx) ListTreeIDs(ctx context.Context, includeDeleted bool) ([]int64, error) {
	ids := make([]int64, 0, len(at.trees))

	for _, tree := range at.trees {
		ids = append(ids, tree.TreeId)
	}

	return ids, nil
}

// ListTrees returns all trees in storage.
func (at *adminTx) ListTrees(ctx context.Context, includeDeleted bool) ([]*trillian.Tree, error) {
	return at.trees, nil
}

// CreateTree inserts the specified tree in storage, returning a tree with all
// storage-generated fields set.
func (at *adminTx) CreateTree(ctx context.Context, tree *trillian.Tree) (*trillian.Tree, error) {
	return nil, fmt.Errorf("creating trees is not implemented")
}

// UpdateTree updates the specified tree in storage, returning a tree with all
// storage-generated fields set.
func (at *adminTx) UpdateTree(ctx context.Context, treeID int64, updateFunc func(*trillian.Tree)) (*trillian.Tree, error) {
	return nil, fmt.Errorf("updating trees is not implemented")
}

// SoftDeleteTree soft deletes the specified tree.
func (at *adminTx) SoftDeleteTree(ctx context.Context, treeID int64) (*trillian.Tree, error) {
	return nil, fmt.Errorf("soft-deleting trees is not implemented")
}

// HardDeleteTree hard deletes (i.e. completely removes from storage) the
// specified tree and all records related to it.
func (at *adminTx) HardDeleteTree(ctx context.Context, treeID int64) error {
	return fmt.Errorf("hard-deleting trees is not implemented")
}

// UndeleteTree undeletes a soft-deleted tree.
func (at *adminTx) UndeleteTree(ctx context.Context, treeID int64) (*trillian.Tree, error) {
	return nil, fmt.Errorf("undeleting trees is not implemented")
}
