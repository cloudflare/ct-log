package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"time"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/trillian"
	"github.com/google/trillian/merkle"
	"github.com/google/trillian/merkle/hashers"

	_ "github.com/google/trillian/merkle/rfc6962"
)

var (
	hasher   hashers.LogHasher
	verifier merkle.LogVerifier
)

func init() {
	rand.Seed(time.Now().UnixNano())

	var err error
	hasher, err = hashers.NewLogHasher(trillian.HashStrategy_RFC6962_SHA256)
	if err != nil {
		panic(err)
	}
	verifier = merkle.NewLogVerifier(hasher)
}

func fatalf(format string, args ...interface{}) {
	depth := 3
	_, file, _, ok := runtime.Caller(depth - 1)
	if ok && !strings.HasSuffix(file, "main.go") {
		depth = 4
	}

	log.Output(depth, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func simpleCheck(out interface{}, err error) func(string) {
	got := fmt.Sprint(out, err)

	return func(wanted string) {
		if got != wanted {
			fatalf("output does not match expected\n\twanted: %v\n\tgot:    %v\n", wanted, got)
		}
	}
}

func consistencyCheck(proof [][]byte, err error) func(first, second int64, sh1, sh2 *ct.SignedTreeHead) {
	if err != nil {
		fatalf("failed to hit get-sth-consistency: %v", err)
	}

	return func(first, second int64, sh1, sh2 *ct.SignedTreeHead) {
		err := verifier.VerifyConsistencyProof(first, second,
			sh1.SHA256RootHash[:], sh2.SHA256RootHash[:], proof)
		if err != nil {
			fatalf("failed to verify consistency proof: %v", err)
		}
	}
}

func manyProofChecks(ctx context.Context, client *LogClient, treeSize int64, sh *ct.SignedTreeHead) {
	for attempt := 0; attempt < 10; attempt++ {
		index := rand.Int63n(treeSize)
		leaf, err := client.GetRawLeaf(ctx, index)
		if err != nil {
			log.Fatal(err)
		}
		hash := sha256.Sum256(append([]byte{ct.TreeLeafPrefix}, leaf...))

		proofCheck(client.GetProofByHash(ctx, hash[:], treeSize))(index, treeSize, sh, leaf)
	}
}

func proofCheck(resp *ct.GetProofByHashResponse, err error) func(leafIndex, treeSize int64, sh *ct.SignedTreeHead, leaf []byte) {
	if err != nil {
		fatalf("failed to hit get-proof-by-hash: %v", err)
	}

	return func(leafIndex, treeSize int64, sh *ct.SignedTreeHead, leaf []byte) {
		if resp.LeafIndex != leafIndex {
			fatalf("LeafIndex from server (%v) and LeafIndex from client (%v) do not match", resp.LeafIndex, leafIndex)
		}
		leafHash, err := hasher.HashLeaf(leaf)
		if err != nil {
			fatalf("failed to hash leaf: %v", err)
		}
		err = verifier.VerifyInclusionProof(resp.LeafIndex, treeSize,
			resp.AuditPath, sh.SHA256RootHash[:], leafHash)
		if err != nil {
			fatalf("failed to verify inclusion proof: %v", err)
		}
	}
}

func manyEntryChecks(ctx context.Context, client *LogClient, treeSize int64) {
	for attempt := 0; attempt < 10; attempt++ {
		start := rand.Int63n(treeSize)
		end := start + rand.Int63n(treeSize-start)

		entriesCheck(client.GetEntries(ctx, start, end))(start, end)
	}
}

func entriesCheck(entries []ct.LogEntry, err error) func(start, end int64) {
	if err != nil {
		fatalf("failed to hit get-entries: %v", err)
	}

	return func(start, end int64) {
		if len(entries) != int(end-start+1) {
			fatalf("got %v entries, but wanted %v", len(entries), end-start+1)
		}

		for i, entry := range entries {
			j, sans := int64(i), entry.X509Cert.DNSNames

			if len(sans) != 1 {
				fatalf("received weird certificate, should have 1 SAN, but has %v", len(sans))
			} else if sans[0] != fmt.Sprintf("cert%v.com", start+j) {
				fatalf("received certificate with SAN %v at position %v", sans[0], start+j)
			}
		}
	}
}

func manyEntryAndProofChecks(ctx context.Context, client *LogClient, treeSize int64, sh *ct.SignedTreeHead) {
	for attempt := 0; attempt < 10; attempt++ {
		leafIndex := rand.Int63n(treeSize)

		entryAndProofCheck(client.GetEntryAndProof(ctx, leafIndex, treeSize))(leafIndex, treeSize, sh)
	}
}

func entryAndProofCheck(resp *ct.GetEntryAndProofResponse, err error) func(leafIndex, treeSize int64, sh *ct.SignedTreeHead) {
	if err != nil {
		fatalf("failed to hit get-entry-and-proof: %v", err)
	}

	return func(leafIndex, treeSize int64, sh *ct.SignedTreeHead) {
		var leaf ct.MerkleTreeLeaf
		if rest, err := tls.Unmarshal(resp.LeafInput, &leaf); err != nil {
			fatalf("failed to unmarshal MerkleTreeLeaf: %v", err)
		} else if len(rest) > 0 {
			fatalf("trailing data (%d bytes) after MerkleTreeLeaf", len(rest))
		}
		cert, err := leaf.X509Certificate()
		if err != nil {
			fatalf("failed to parse certificate in MerkleTreeLeaf: %v", err)
		}
		sans := cert.DNSNames

		if len(sans) != 1 {
			fatalf("received weird certificate, should have 1 SAN, but has %v", len(sans))
		} else if sans[0] != fmt.Sprintf("cert%v.com", leafIndex) {
			fatalf("received certificate with SAN %v at position %v", sans[0], leafIndex)
		}

		leafHash, err := hasher.HashLeaf(resp.LeafInput)
		if err != nil {
			fatalf("failed to hash leaf: %v", err)
		}
		err = verifier.VerifyInclusionProof(leafIndex, treeSize,
			resp.AuditPath, sh.SHA256RootHash[:], leafHash)
		if err != nil {
			fatalf("failed to verify inclusion proof: %v", err)
		}
	}
}
