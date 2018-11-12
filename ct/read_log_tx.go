package ct

import (
	"context"
	"fmt"

	"github.com/google/trillian"
	"github.com/google/trillian/storage"
)

// readOnlyLogTX provides a read-only view into log data. A readOnlyLogTX,
// unlike readOnlyLogTreeTX, is not tied to a particular tree.
type readOnlyLogTX struct {
	adminTx storage.ReadOnlyAdminTX
	closed  bool
}

// GetActiveLogs returns a list of the IDs of all the logs that are configured
// in storage.
func (rol *readOnlyLogTX) GetActiveLogIDs(ctx context.Context) ([]int64, error) {
	if rol.closed {
		return nil, fmt.Errorf("read-only log tx is closed")
	}

	trees, err := rol.adminTx.ListTrees(ctx, false)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(trees))
	for _, tree := range trees {
		if tree.TreeState != trillian.TreeState_ACTIVE {
			continue
		}
		ids = append(ids, tree.TreeId)
	}
	return ids, nil
}

func (rol *readOnlyLogTX) GetUnsequencedCounts(ctx context.Context) (storage.CountByLogID, error) {
	return nil, fmt.Errorf("getting unsequenced counts is not implemented")
}

func (rol *readOnlyLogTX) Commit() error {
	if rol.closed {
		return fmt.Errorf("read-only log tx is closed")
	}
	return rol.Close()
}

func (rol *readOnlyLogTX) Rollback() error { return rol.Commit() }

func (rol *readOnlyLogTX) Close() error {
	rol.closed = true
	return nil
}
