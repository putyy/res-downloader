//go:build linux

package system

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const certificateMigrationVersion = 2 // Recheck old shared CAs in NSS, too.
const nssCertificateNickname = "res-downloader"

type nssCertificate struct {
	nickname, fingerprint, trust string
}

func (certificate nssCertificate) matchesRemoval(fingerprint string, includeReservedName bool) bool {
	return strings.EqualFold(certificate.fingerprint, fingerprint) ||
		(includeReservedName && certificate.nickname == nssCertificateNickname)
}

var nssListRow = regexp.MustCompile(`^(.+?)\s+([A-Za-z]*,[A-Za-z]*,[A-Za-z]*)\s*$`)

// Setup.WithPassword returns a copy, so serialize certificate operations across
// all setup copies rather than storing the lock in a value copied by callers.
var linuxCertificateMu sync.Mutex

// Only touch existing shared databases belonging to the desktop user. Never
// create a legacy path that would change Chromium's database selection.
func nssDatabases() ([]string, error) {
	if os.Geteuid() == 0 {
		return nil, errors.New("NSS certificate management requires running the application as the desktop user, without sudo")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	var databases []string
	var errs []error
	seen := map[string]bool{}
	for _, path := range []string{filepath.Join(home, ".pki/nssdb"), filepath.Join(home, ".local/share/pki/nssdb")} {
		info, err := os.Stat(filepath.Join(path, "cert9.db"))
		if errors.Is(err, os.ErrNotExist) {
			if _, legacyErr := os.Stat(filepath.Join(path, "cert8.db")); legacyErr == nil {
				errs = append(errs, fmt.Errorf("legacy NSS database at %s: migrate it to SQL using the browser before installing certificates", path))
			} else if !errors.Is(legacyErr, os.ErrNotExist) {
				errs = append(errs, legacyErr)
			}
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("inspect NSS database %s: %w", path, err))
			continue
		}
		if !info.Mode().IsRegular() {
			errs = append(errs, fmt.Errorf("NSS certificate database is not a regular file: %s", path))
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !seen[resolved] {
			databases = append(databases, resolved)
			seen[resolved] = true
		}
	}
	return databases, errors.Join(errs...)
}

func runNSS(database string, args ...string) ([]byte, error) {
	tool, err := exec.LookPath("certutil")
	if err != nil {
		return nil, errors.New("NSS certificate tool certutil is missing; install libnss3-tools on Debian/Ubuntu, nss on Arch Linux, or nss-tools on Fedora")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, append([]string{"-d", "sql:" + database}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	// No sudo or administrator password: this is the current user's database.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("NSS operation %s in %s failed: %w: %s", args[0], database, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func listNSS(database string) ([]nssCertificate, error) {
	out, err := runNSS(database, "-L")
	if err != nil {
		return nil, err
	}
	var certificates []nssCertificate
	for _, line := range strings.Split(string(out), "\n") {
		row := nssListRow.FindStringSubmatch(line)
		if row == nil {
			continue
		}
		nickname := strings.TrimSpace(row[1])
		raw, err := runNSS(database, "-L", "-n", nickname, "-a")
		if err != nil {
			return nil, err
		}
		block, _ := pem.Decode(raw)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("invalid NSS certificate %q in %s", nickname, database)
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return nil, fmt.Errorf("parse NSS certificate %q: %w", nickname, err)
		}
		certificates = append(certificates, nssCertificate{nickname, CurrentCertificateSHA1(raw), row[2]})
	}
	return certificates, nil
}

// Presence is deliberately separate from complete installation: uninstall and
// reset must also remove an untrusted certificate or a partially installed CA.
func nssCertificateState(fingerprint string, includeReservedName bool) (anyPresent, allTrusted bool, err error) {
	databases, err := nssDatabases()
	if err != nil {
		return false, false, err
	}
	allTrusted = true
	for _, database := range databases {
		certificates, err := listNSS(database)
		if err != nil {
			return false, false, err
		}
		trusted := false
		for _, certificate := range certificates {
			anyPresent = anyPresent || certificate.matchesRemoval(fingerprint, includeReservedName)
			if strings.EqualFold(certificate.fingerprint, fingerprint) {
				trusted = trusted || strings.Contains(strings.Split(certificate.trust, ",")[0], "C")
			}
		}
		allTrusted = allTrusted && trusted
	}
	return anyPresent, allTrusted, nil
}

func removeNSSCertificate(fingerprint string, includeReservedName bool) (bool, error) {
	databases, discoveryErr := nssDatabases()
	errs := []error{discoveryErr}
	removed := false
	for _, database := range databases {
		certificates, err := listNSS(database)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, certificate := range certificates {
			if !certificate.matchesRemoval(fingerprint, includeReservedName) {
				continue
			}
			if _, err := runNSS(database, "-D", "-n", certificate.nickname); err != nil {
				errs = append(errs, err)
				continue
			}
			removed = true
		}
		remaining, err := listNSS(database)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, certificate := range remaining {
			if certificate.matchesRemoval(fingerprint, includeReservedName) {
				errs = append(errs, fmt.Errorf("certificate %q remains in NSS database %s after removal", certificate.nickname, database))
			}
		}
	}
	return removed, errors.Join(errs...)
}

func (s *Setup) installNSSCertificate() error {
	raw, err := os.ReadFile(s.CertFile)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return errors.New("invalid public CA for NSS installation")
	}
	fingerprint := CurrentCertificateSHA1(raw)
	// Old shared CA is identified by its known fingerprint, never by its name.
	if _, err := removeNSSCertificate(legacyCertificateSHA1, false); err != nil {
		return err
	}
	databases, err := nssDatabases()
	if err != nil {
		return err
	}
	for _, database := range databases {
		certificates, err := listNSS(database)
		if err != nil {
			return err
		}
		found := false
		for _, certificate := range certificates {
			if strings.EqualFold(certificate.fingerprint, fingerprint) {
				// Preserve existing non-TLS trust settings for this same certificate.
				trust := strings.Split(certificate.trust, ",")
				trust[0] = "C"
				if _, err := runNSS(database, "-M", "-n", certificate.nickname, "-t", strings.Join(trust, ",")); err != nil {
					return err
				}
				found = true
			}
		}
		if !found {
			// Only replace the exact app-reserved name when the current CA was
			// not found under any nickname. Custom names and prefixes are kept.
			for _, certificate := range certificates {
				if certificate.nickname == nssCertificateNickname {
					if _, err := runNSS(database, "-D", "-n", certificate.nickname); err != nil {
						return err
					}
				}
			}
			if _, err := runNSS(database, "-A", "-n", nssCertificateNickname, "-t", "C,,", "-i", s.CertFile); err != nil {
				return err
			}
		}
	}
	_, trusted, err := nssCertificateState(fingerprint, false)
	if err != nil {
		return err
	}
	if !trusted {
		return errors.New("current CA is not trusted in every detected NSS database after installation")
	}
	return nil
}

func (s *Setup) certificateInstallationComplete(fingerprint string) (bool, error) {
	linuxCertificateMu.Lock()
	defer linuxCertificateMu.Unlock()
	installed, err := s.isSystemCertificateInstalled(fingerprint)
	if err != nil || !installed {
		return installed, err
	}
	_, trusted, err := nssCertificateState(fingerprint, false)
	return trusted, err
}
