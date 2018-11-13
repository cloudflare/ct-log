package ct

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cloudflare/ct-log/custom"

	"github.com/google/trillian/quota"
	"github.com/google/trillian/storage"
	"github.com/prometheus/client_golang/prometheus"
)

// QuotaManager is the mechanism which provides backpressure from the signer to
// the servers, when leaves are being queued faster than they're being
// sequenced.
type QuotaManager struct {
	maxUnsequencedLeaves int64

	unsequenced map[int64]int64
	mu          sync.Mutex

	TreeSize, UnsequencedLeaves *prometheus.GaugeVec
}

var _ quota.Manager = &QuotaManager{}

func NewQuotaManager(maxUnsequencedLeaves int64) *QuotaManager {
	treeSizeGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tree_size",
		Help: "The number of sequenced leaves in a log.",
	}, []string{"tree"})
	unsequencedLeavesGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "unsequenced_leaves",
		Help: "The number of unsequenced leaves in a log.",
	}, []string{"tree"})

	return &QuotaManager{
		maxUnsequencedLeaves: maxUnsequencedLeaves,

		unsequenced: make(map[int64]int64),

		TreeSize:          treeSizeGauge,
		UnsequencedLeaves: unsequencedLeavesGauge,
	}
}

// WatchLog spawns a goroutine to keep track of the number of unsequenced leaves
// in the log with the given treeID.
func (qm *QuotaManager) WatchLog(local *custom.Local, treeID int64) {
	qm.mu.Lock()
	qm.unsequenced[treeID] = 0
	qm.mu.Unlock()

	// Spawn a goroutine to update the unsequenced count every so often.
	go func() {
		for {
			time.Sleep(10 * time.Second)

			sth, _, err := local.MostRecentRoot(treeID)
			if err == storage.ErrTreeNeedsInit {
				continue
			} else if err != nil {
				log.Printf("error getting the most recent STH: treeID=%v: %v", treeID, err)
				continue
			}
			count, err := local.Unsequenced(treeID)
			if err != nil {
				log.Printf("error getting the unsequenced count: treeID=%v: %v", treeID, err)
				continue
			}

			qm.TreeSize.WithLabelValues(fmt.Sprint(treeID)).Set(float64(sth.TreeSize))
			qm.UnsequencedLeaves.WithLabelValues(fmt.Sprint(treeID)).Set(float64(count))

			qm.mu.Lock()
			qm.unsequenced[treeID] = int64(count)
			qm.mu.Unlock()
		}
	}()
}

// GetUser returns the quota user, as defined by the manager implementation. req
// is the RPC request message.
func (qm *QuotaManager) GetUser(ctx context.Context, req interface{}) string {
	return "" // Not used.
}

// GetTokens acquires numTokens from all specs. Tokens are taken in the order
// specified by specs. Returns error if numTokens could not be acquired for all
// specs.
func (qm *QuotaManager) GetTokens(ctx context.Context, numTokens int, specs []quota.Spec) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for _, spec := range specs {
		if err := qm.getTokens(ctx, numTokens, spec); err != nil {
			return err
		}
	}
	return nil
}

func (qm *QuotaManager) getTokens(ctx context.Context, numTokens int, spec quota.Spec) error {
	if spec.Group != quota.Tree {
		return nil
	} else if spec.Kind != quota.Write {
		return nil
	}
	tokens := int64(numTokens)

	count, ok := qm.unsequenced[spec.TreeID]
	if !ok {
		return fmt.Errorf("unknown tree id: %v", spec.TreeID)
	} else if count+tokens > qm.maxUnsequencedLeaves {
		return fmt.Errorf("too many unsequenced leaves")
	}
	qm.unsequenced[spec.TreeID] += tokens

	return nil
}

// PeekTokens returns how many tokens are available for each spec, without
// acquiring any. Infinite quotas should return MaxTokens.
func (qm *QuotaManager) PeekTokens(ctx context.Context, specs []quota.Spec) (map[quota.Spec]int, error) {
	return nil, fmt.Errorf("peeking into the quota is not implemented")
}

// PutTokens adds numTokens for all specs.
func (qm *QuotaManager) PutTokens(ctx context.Context, numTokens int, specs []quota.Spec) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for _, spec := range specs {
		if err := qm.putTokens(ctx, numTokens, spec); err != nil {
			return err
		}
	}
	return nil
}

func (qm *QuotaManager) putTokens(ctx context.Context, numTokens int, spec quota.Spec) error {
	if spec.Group != quota.Tree {
		return nil
	} else if spec.Kind != quota.Write {
		return nil
	}

	count, ok := qm.unsequenced[spec.TreeID]
	if !ok {
		return fmt.Errorf("unknown tree id: %v", spec.TreeID)
	}
	count -= int64(numTokens)
	if count < 0 {
		count = 0
	}
	qm.unsequenced[spec.TreeID] = count

	return nil
}

// ResetQuota resets the quota for all specs.
func (qm *QuotaManager) ResetQuota(ctx context.Context, specs []quota.Spec) error {
	return fmt.Errorf("resetting the quota is not implemented")
}
