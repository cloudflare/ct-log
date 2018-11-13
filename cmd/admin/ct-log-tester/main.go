// Command ct-log-tester audits the functionality of all of a CT log's
// endpoints. It is used as part of our testing framework.
package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"io/ioutil"
	"log"

	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
)

var (
	uri        = flag.String("uri", "http://server:1944/", "The CT log server's URI.")
	pubkeyPath = flag.String("pubkey", "devdata/pubkey.dev.pem", "Path to a file with the log's public key in PEM.")
	caCertPath = flag.String("ca-cert", "devdata/certs.dev/ca.pem", "Path to a file with the CA's certificate in PEM.")
	caKeyPath  = flag.String("ca-key", "devdata/certs.dev/ca-key.pem", "Path to a file with the CA's private key in PEM.")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()
	ctx := context.Background()

	// Load the CA's public and private key from disk and parse them.
	caCertRaw, err := ioutil.ReadFile(*caCertPath)
	if err != nil {
		log.Fatalf("failed to read the CA certificate: %v", err)
	}
	block, _ := pem.Decode(caCertRaw)
	if block == nil {
		log.Fatal("failed to parse the CA certificate as PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Fatalf("failed to parse the CA certificate after decoding its PEM: %v", err)
	}

	caKeyRaw, err := ioutil.ReadFile(*caKeyPath)
	if err != nil {
		log.Fatalf("failed to read the CA private key: %v", err)
	}
	block, _ = pem.Decode(caKeyRaw)
	if block == nil {
		log.Fatal("failed to parse the CA private key as PEM")
	}
	caKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("failed to parse the CA private key after decoding its PEM: %v", err)
	}

	// Load the log's public key from disk and create a CT client.
	pubkey, err := ioutil.ReadFile(*pubkeyPath)
	if err != nil {
		log.Fatalf("failed to read the log public key: %v", err)
	}
	temp, err := client.New(*uri, nil, jsonclient.Options{
		PublicKey: string(pubkey),
	})
	if err != nil {
		log.Fatalf("failed to create a CT log client: %v", err)
	}
	client := &LogClient{*temp}

	// Get the log's STH and verify this is a fresh setup.
	sh, err := client.GetSTH(ctx)
	if err != nil {
		log.Fatal(err)
	} else if sh.TreeSize != 0 {
		log.Fatalf("cowardly refusing to continue, tree size is %v, not 0", sh.TreeSize)
	}
	log.Println("Starting test.")

	// Hit all of the major endpoints of the empty log, and verify they behave
	// as expected. Take ths opportunity to check edge cases.
	//
	// get-sth-consistency:
	simpleCheck(client.GetSTHConsistency(ctx, -1, 0))(`[] got HTTP Status "400 Bad Request"`)
	simpleCheck(client.GetSTHConsistency(ctx, 0, 0))(`[] <nil>`)
	simpleCheck(client.GetSTHConsistency(ctx, 0, 1))(`[] <nil>`)
	// get-proof-by-hash:
	simpleCheck(client.GetProofByHash(ctx, make([]byte, 32), -10))(`<nil> got HTTP Status "400 Bad Request"`)
	simpleCheck(client.GetProofByHash(ctx, make([]byte, 70), 1))(`<nil> got HTTP Status "404 Not Found"`)
	simpleCheck(client.GetProofByHash(ctx, make([]byte, 2), 1))(`<nil> got HTTP Status "404 Not Found"`)
	simpleCheck(client.GetProofByHash(ctx, make([]byte, 32), 1))(`<nil> got HTTP Status "404 Not Found"`)
	// get-entries:
	simpleCheck(client.GetRawEntries(ctx, -100, -10))(`<nil> got HTTP Status "400 Bad Request"`)
	simpleCheck(client.GetRawEntries(ctx, 10, 1))(`<nil> got HTTP Status "400 Bad Request"`)
	simpleCheck(client.GetRawEntries(ctx, 100000, 100000))(`<nil> got HTTP Status "500 Internal Server Error"`)
	// get-entry-and-proof:
	simpleCheck(client.GetEntryAndProof(ctx, 0, 0))(`<nil> got HTTP Status "400 Bad Request"`)
	simpleCheck(client.GetEntryAndProof(ctx, 3, 1))(`<nil> got HTTP Status "400 Bad Request"`)
	simpleCheck(client.GetEntryAndProof(ctx, 0, 1))(`<nil> got HTTP Status "400 Bad Request"`)

	timestamps := make([]uint64, 45)

	// Add the first batch of leaves and wait for them to be integrated.
	log.Println("Growing tree to 30 entries.")
	for i := 0; i < 30; i++ {
		sct, err := client.AddChain(ctx, generateNthCert(caCert, caKey, i))
		if err != nil {
			log.Fatal(err)
		}
		timestamps[i] = sct.Timestamp
	}
	sh30 := client.WaitForSTH(ctx, 30)

	// Hit same endpoints, and verify that the new leaves seem incorporated.
	//
	// get-sth-consistency:
	//   No consistency queries; we only have one STH.
	// get-proof-by-hash:
	manyProofChecks(ctx, client, 30, sh30)
	// get-entries:
	manyEntryChecks(ctx, client, 30)
	// get-entry-and-proof:
	manyEntryAndProofChecks(ctx, client, 30, sh30)

	// Add a second batch of leaves, but not so many that we start using a
	// different storage node.
	//
	// Entries [20, 30) and [40, 45) are duplicated and should be ignored. We
	// want to verify that we de-dup things from previous STHs and the
	// in-progress STH.
	log.Println("Growing tree to 45 entries.")
	for i := 20; i < 45; i++ {
		sct, err := client.AddChain(ctx, generateNthCert(caCert, caKey, i))
		if err != nil {
			log.Fatal(err)
		} else if i < 30 && sct.Timestamp != timestamps[i] {
			log.Fatal("AddChain submission was not properly de-duplicated.")
		} else if i >= 30 {
			timestamps[i] = sct.Timestamp
		}
	}
	for i := 40; i < 45; i++ {
		sct, err := client.AddChain(ctx, generateNthCert(caCert, caKey, i))
		if err != nil {
			log.Fatal(err)
		} else if sct.Timestamp != timestamps[i] {
			log.Fatal("AddChain submission was not properly de-duplicated.")
		}
	}
	sh45 := client.WaitForSTH(ctx, 45)

	// Verify the new leaves were incorporated alongside the old ones.
	//
	// get-sth-consistency:
	consistencyCheck(client.GetSTHConsistency(ctx, 30, 45))(30, 45, sh30, sh45)
	// get-proof-by-hash:
	manyProofChecks(ctx, client, 45, sh45)
	// get-entries:
	manyEntryChecks(ctx, client, 45)
	// get-entry-and-proof:
	manyEntryAndProofChecks(ctx, client, 45, sh45)

	// Add a third batch of leaves, this time enough to create a storage node.
	log.Println("Growing tree to 300 entries.")
	for i := 45; i < 300; i++ {
		_, err := client.AddChain(ctx, generateNthCert(caCert, caKey, i))
		if err != nil {
			log.Fatal(err)
		}
	}
	sh300 := client.WaitForSTH(ctx, 300)

	// Try to break things by making requests across both nodes.
	//
	// get-sth-consistency:
	consistencyCheck(client.GetSTHConsistency(ctx, 30, 45))(30, 45, sh30, sh45)
	consistencyCheck(client.GetSTHConsistency(ctx, 30, 300))(30, 300, sh30, sh300)
	consistencyCheck(client.GetSTHConsistency(ctx, 45, 300))(45, 300, sh45, sh300)
	// get-proof-by-hash:
	manyProofChecks(ctx, client, 300, sh300)
	// get-entries:
	manyEntryChecks(ctx, client, 300)
	// get-entry-and-proof:
	manyEntryAndProofChecks(ctx, client, 300, sh300)

	// Add a fourth batch of leaves, which should leave us with 3 nodes.
	log.Println("Growing tree to 530 entries.")
	for i := 300; i < 530; i++ {
		_, err := client.AddChain(ctx, generateNthCert(caCert, caKey, i))
		if err != nil {
			log.Fatal(err)
		}
	}
	sh530 := client.WaitForSTH(ctx, 530)

	// Try to break things by making requests across both nodes.
	//
	// get-sth-consistency:
	consistencyCheck(client.GetSTHConsistency(ctx, 30, 45))(30, 45, sh30, sh45)
	consistencyCheck(client.GetSTHConsistency(ctx, 30, 300))(30, 300, sh30, sh300)
	consistencyCheck(client.GetSTHConsistency(ctx, 30, 530))(30, 530, sh30, sh530)
	consistencyCheck(client.GetSTHConsistency(ctx, 45, 300))(45, 300, sh45, sh300)
	consistencyCheck(client.GetSTHConsistency(ctx, 45, 530))(45, 530, sh45, sh530)
	// get-proof-by-hash:
	manyProofChecks(ctx, client, 530, sh530)
	// get-entries:
	manyEntryChecks(ctx, client, 530)
	// get-entry-and-proof:
	manyEntryAndProofChecks(ctx, client, 530, sh530)

	// Spawn several goroutines that all submit certs as quickly as possible. We
	// expect the requests to eventually fail and stop. We assume that
	// max_unsequenced_leaves is 600.
	log.Println("Trying to trigger rate-limiting.")

	added := make(chan int)
	for i := 0; i < 5; i++ {
		i := i
		go func() {
			count := 0

			for j := 0; j < 800; j++ {
				n := 530 + 800*i + j
				_, err := client.AddChain(ctx, generateNthCert(caCert, caKey, n))
				if err != nil {
					if err.Error() == "got HTTP Status \"403 Forbidden\"" {
						added <- count
						return
					}
					log.Fatalf("Unexpected error while flooding log: %v", err)
				}
				count++
			}

			log.Fatal("Failed to trigger rate-limiting.")
		}()
	}
	sum := 0
	for i := 0; i < 5; i++ {
		sum += <-added
	}
	log.Printf("Rate-limiting was triggered after submitting %v certs.", sum)
}
