package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/google/trillian"
	"gopkg.in/kothar/go-backblaze.v0"
)

var (
	client = &http.Client{
		Transport: &http.Transport{ // copied from net/http.DefaultTransport
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          3,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 30 * time.Second,
	}

	errLeavesNotFound = fmt.Errorf("leaves not found in remote database")
)

// Remote implements convenience methods over a large-scale data host. The data
// is possibly hosted remotely, so may take a long time to fetch.
type Remote struct {
	b2     *backblaze.B2
	bucket string
	url    string
}

// NewRemote returns a new remote database, where `acctId` and `appKey` are the
// Account ID and Application Key of a B2 bucket. `bucket` is the name of the
// bucket. `url` is the URL to use to download data.
func NewRemote(acctId, appKey, bucket, url string) (*Remote, error) {
	b2, err := backblaze.NewB2(backblaze.Credentials{
		AccountID:      acctId,
		ApplicationKey: appKey,
	})
	if err != nil {
		return nil, err
	}
	return &Remote{
		b2:     b2,
		bucket: bucket,
		url:    url,
	}, nil
}

func (r *Remote) GetLeaves(ctx context.Context, treeID int64, seqs []int64) ([]*trillian.LogLeaf, error) {
	if len(seqs) == 0 {
		return nil, nil
	} else if !sort.IsSorted(int64Slice(seqs)) {
		sort.Sort(int64Slice(seqs))
	}

	out := make([]*trillian.LogLeaf, 0)

	for start := 0; start < len(seqs); {
		batch, off := seqs[start]/1024, int(seqs[start]%1024)

		data, err := r.getBatch(ctx, treeID, batch)
		if err != nil {
			return nil, err
		} else if off >= len(data) {
			return nil, fmt.Errorf("set of stored leaves is truncated")
		}

		for {
			out = append(out, data[off])
			start++
			off++

			if start < len(seqs) && off < len(data) && seqs[start-1]+1 == seqs[start] {
				continue
			}
			break
		}
	}

	return out, nil
}

func (r *Remote) getBatch(ctx context.Context, treeID, batch int64) ([]*trillian.LogLeaf, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(
		"%v/leaves-%v/%x", r.url, treeID, batch,
	), nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, errLeavesNotFound
	} else if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response status: %v", resp.Status)
	}

	parsed := make([]*trillian.LogLeaf, 0)
	if err = json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	return parsed, nil
}

func (r *Remote) PutLeaves(ctx context.Context, treeID int64, leaves []*trillian.LogLeaf) error {
	// Group leaves into batches.
	batches := make(map[int64][]*trillian.LogLeaf)
	for _, leaf := range leaves {
		b := leaf.LeafIndex / 1024
		batches[b] = append(batches[b], leaf)
	}

	for b, leaves := range batches {
		// Fetch this batch. Merge the leaves that are already stored (if any)
		// into the set of new leaves we want to store.
		existing, err := r.getBatch(ctx, treeID, b)
		if err != nil && err != errLeavesNotFound {
			return err
		}

		pos := make(map[int]*trillian.LogLeaf)
		for _, leaf := range leaves {
			off := int(leaf.LeafIndex % 1024)
			if _, ok := pos[off]; ok {
				return fmt.Errorf("multiple leaves in the same position")
			}
			pos[off] = leaf
		}
		for _, leaf := range existing {
			off := int(leaf.LeafIndex % 1024)
			if _, ok := pos[off]; !ok {
				pos[off] = leaf
			}
		}

		updated := make([]*trillian.LogLeaf, 0)
		for i := 0; i < 1024; i++ {
			leaf, ok := pos[i]
			if !ok {
				return fmt.Errorf("gap in set of leaves to store")
			}
			updated = append(updated, leaf)

			delete(pos, i)
			if len(pos) == 0 {
				break
			}
		}
		if len(pos) > 0 {
			return fmt.Errorf("too many leaves stored in batch")
		}

		// Serialize the merged batch and write to B2.
		buff := &bytes.Buffer{}
		if err := json.NewEncoder(buff).Encode(updated); err != nil {
			return err
		}
		name := fmt.Sprintf("leaves-%v/%x", treeID, b)
		meta := make(map[string]string)

		bucket, err := r.b2.Bucket(r.bucket)
		if err != nil {
			return err
		}
		if _, err := bucket.UploadFile(name, meta, buff); err != nil {
			return err
		}
	}

	return nil
}
