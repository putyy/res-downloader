//go:build darwin

package system

import (
	"errors"
	"strings"
	"testing"
)

func TestCertificateListContainsFingerprint(t *testing.T) {
	output := []byte("SHA-256 hash: ignored\nSHA-1 hash: F0:A4:3A:E8:E1:88:4C:E1:34:A9:95:27:1F:5C:D7:B0:BA:F8:70:80\n")
	if !certificateListContainsFingerprint(output, legacyCertificateSHA1) {
		t.Fatal("fingerprint with separators was not matched")
	}
	if certificateListContainsFingerprint(output, "0000000000000000000000000000000000000000") {
		t.Fatal("unrelated fingerprint was matched")
	}
}

func TestCommandOutputErrorIncludesSecurityDetail(t *testing.T) {
	err := commandOutputError("delete legacy certificate", []byte("security: write permissions error\n"), errors.New("exit status 195"))
	message := err.Error()
	if !strings.Contains(message, "exit status 195") || !strings.Contains(message, "write permissions error") {
		t.Fatalf("missing command detail in %q", message)
	}
}
