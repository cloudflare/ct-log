package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
)

// LogClient overloads the proper CT log client to remove validation, so I can
// send weird stuff to logs.
type LogClient struct {
	client.LogClient
}

// WaitForSTH repeatedly polls the get-sth endpoint and stops blocking with the
// STH's tree size is `treeSize`.
func (c *LogClient) WaitForSTH(ctx context.Context, treeSize uint64) *ct.SignedTreeHead {
	for i := 0; i < 30; i++ {
		sh, err := c.GetSTH(ctx)
		if err != nil {
			log.Fatal(err)
		} else if sh.TreeSize == treeSize {
			return sh
		}

		time.Sleep(5*time.Second)
	}

	log.Fatalf("timed out waiting to read an STH with tree size %v", treeSize)
	return nil
}

// GetSTHConsistency retrieves the consistency proof between two snapshots.
func (c *LogClient) GetSTHConsistency(ctx context.Context, first, second int64) ([][]byte, error) {
	params := map[string]string{
		"first":  fmt.Sprint(first),
		"second": fmt.Sprint(second),
	}
	var resp ct.GetSTHConsistencyResponse
	if _, _, err := c.GetAndParse(ctx, ct.GetSTHConsistencyPath, params, &resp); err != nil {
		return nil, err
	}
	return resp.Consistency, nil
}

// GetRawEntries exposes the /ct/v1/get-entries result with only the JSON
// parsing done.
func (c *LogClient) GetRawEntries(ctx context.Context, start, end int64) (*ct.GetEntriesResponse, error) {
	params := map[string]string{
		"start": fmt.Sprint(start),
		"end":   fmt.Sprint(end),
	}
	var resp ct.GetEntriesResponse
	_, _, err := c.GetAndParse(ctx, ct.GetEntriesPath, params, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRawLeaf returns the LeafInput of the entry at `index`.
func (c *LogClient) GetRawLeaf(ctx context.Context, index int64) ([]byte, error) {
	resp, err := c.GetRawEntries(ctx, index, index)
	if err != nil {
		return nil, err
	} else if len(resp.Entries) != 1 {
		return nil, fmt.Errorf("wrong number of entries returned; got %v, wanted 1", len(resp.Entries))
	}

	return resp.Entries[0].LeafInput, nil
}

// GetProofByHash returns an audit path for the hash of an SCT.
func (c *LogClient) GetProofByHash(ctx context.Context, hash []byte, treeSize int64) (*ct.GetProofByHashResponse, error) {
	params := map[string]string{
		"tree_size": fmt.Sprint(treeSize),
		"hash":      base64.StdEncoding.EncodeToString(hash),
	}
	var resp ct.GetProofByHashResponse
	if _, _, err := c.GetAndParse(ctx, ct.GetProofByHashPath, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetEntryAndProof returns an entry and the audit path for the specified leaf.
func (c *LogClient) GetEntryAndProof(ctx context.Context, leafIndex, treeSize int64) (*ct.GetEntryAndProofResponse, error) {
	params := map[string]string{
		"leaf_index": fmt.Sprint(leafIndex),
		"tree_size":  fmt.Sprint(treeSize),
	}
	var resp ct.GetEntryAndProofResponse
	if _, _, err := c.GetAndParse(ctx, ct.GetEntryAndProofPath, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
