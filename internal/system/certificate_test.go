package system

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedCertificateAuthoritiesAreDeviceUnique(t *testing.T) {
	certA, keyA, err := generateCertificateAuthority()
	if err != nil {
		t.Fatal(err)
	}
	certB, keyB, err := generateCertificateAuthority()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(certA, certB) || bytes.Equal(keyA, keyB) {
		t.Fatal("generated device CAs are not unique")
	}
	if _, err := tls.X509KeyPair(certA, keyA); err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certA)
	if block == nil {
		t.Fatal("generated certificate is not PEM")
	}
}

func TestCertificateMigrationResultPreservesActionableStatus(t *testing.T) {
	status, message := certificateMigrationResult("needsManualCleanup", "", assertError("write permissions error"))
	if status != "needsManualCleanup" {
		t.Fatalf("status = %q, want needsManualCleanup", status)
	}
	if message != "write permissions error" {
		t.Fatalf("message = %q", message)
	}
}

func TestCertificateFileMatchesSHA1(t *testing.T) {
	certificatePEM, _, err := generateCertificateAuthority()
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certificatePEM)
	digest := sha1.Sum(block.Bytes)
	path := filepath.Join(t.TempDir(), "device.crt")
	if err := os.WriteFile(path, certificatePEM, 0600); err != nil {
		t.Fatal(err)
	}
	matches, err := certificateFileMatchesSHA1(path, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	if !matches {
		t.Fatal("certificate fingerprint did not match")
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
