package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/cloudflare/ct-log/cmd/admin/ct-log-tester/internal/drand"

	ct "github.com/google/certificate-transparency-go"
)

var (
	notBefore = time.Now() // TODO(brendan): Look into submitting certs not yet valid?
	notAfter  = notBefore.Add(72 * time.Hour)
)

// generateNthCert returns the n^th certificate that should be added to the log.
// It has "cert${n}.com" as a SAN, which allows tests to verify the correctness
// of get-entries.
func generateNthCert(caCert *x509.Certificate, caKey interface{}, n int) []ct.ASN1Cert {
	r, err := drand.NewReader(fmt.Sprintf("cert %v", n))
	if err != nil {
		log.Fatalf("failed to create random stream: %v", err)
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), r)
	if err != nil {
		log.Fatalf("failed to generate private key: %v", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(r, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,

		DNSNames: []string{fmt.Sprintf("cert%v.com", n)},
	}
	derBytes, err := x509.CreateCertificate(r, template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		log.Fatalf("failed to create certificate: %v", err)
	}

	return []ct.ASN1Cert{ct.ASN1Cert{Data: derBytes}}
}
