package rules

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"unicode"
)

const (
	ActionMITM  = "mitm"
	ActionPass  = "pass"
	maxPolicies = 500
)

// Policy is evaluated at TLS CONNECT time. Only host information
// exists at this stage, so URL, MIME and resource filters intentionally belong
// to later capture policies instead of this type.
type Policy struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Enabled bool     `json:"enabled"`
	Domains []string `json:"domains"`
	Exclude []string `json:"exclude,omitempty"`
	Action  string   `json:"action"`
}

type Set struct {
	mu       sync.RWMutex
	policies []Policy
}

func New(policies []Policy) (*Set, error) {
	rules := &Set{}
	if err := rules.Load(policies); err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *Set) Load(policies []Policy) error {
	if err := Validate(policies); err != nil {
		return err
	}
	cloned := Clone(policies)
	r.mu.Lock()
	r.policies = cloned
	r.mu.Unlock()
	return nil
}

func Validate(policies []Policy) error {
	if len(policies) > maxPolicies {
		return fmt.Errorf("interception policies exceed %d entries", maxPolicies)
	}
	ids := make(map[string]struct{}, len(policies))
	for index, policy := range policies {
		if !validIdentifier(policy.ID) {
			return fmt.Errorf("interception policy %d has an invalid id", index)
		}
		if _, exists := ids[policy.ID]; exists {
			return fmt.Errorf("duplicate interception policy id %q", policy.ID)
		}
		ids[policy.ID] = struct{}{}
		if policy.Action != ActionMITM && policy.Action != ActionPass {
			return fmt.Errorf("interception policy %q has invalid action %q", policy.ID, policy.Action)
		}
		if len(policy.Domains) == 0 {
			return fmt.Errorf("interception policy %q requires at least one domain", policy.ID)
		}
		for _, pattern := range append(append([]string(nil), policy.Domains...), policy.Exclude...) {
			if err := validateDomainPattern(pattern); err != nil {
				return fmt.Errorf("interception policy %q: %w", policy.ID, err)
			}
		}
	}
	return nil
}

func validateDomainPattern(pattern string) error {
	pattern = strings.TrimSpace(strings.ToLower(pattern))
	if pattern == "*" {
		return nil
	}
	if strings.HasPrefix(pattern, "*.") {
		pattern = strings.TrimPrefix(pattern, "*.")
	}
	if pattern == "" || strings.ContainsAny(pattern, " /\\:@") || strings.HasPrefix(pattern, ".") || strings.HasSuffix(pattern, ".") {
		return fmt.Errorf("invalid domain pattern %q", pattern)
	}
	for _, char := range pattern {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '.' && char != '-' {
			return fmt.Errorf("invalid domain pattern %q", pattern)
		}
	}
	return nil
}

func Clone(policies []Policy) []Policy {
	out := make([]Policy, len(policies))
	for index, policy := range policies {
		out[index] = policy
		out[index].Domains = append([]string(nil), policy.Domains...)
		out[index].Exclude = append([]string(nil), policy.Exclude...)
	}
	return out
}

// ShouldMITM evaluates enabled policies in display order. Later matching
// policies override earlier ones, allowing a broad default followed by narrow
// pass-through or MITM exceptions.
func (r *Set) ShouldMITM(host string) bool {
	host = interceptionHostname(host)
	r.mu.RLock()
	defer r.mu.RUnlock()
	action := false
	for _, policy := range r.policies {
		if !policy.Enabled || !matchesDomainPatterns(policy.Domains, host) {
			continue
		}
		if matchesDomainPatterns(policy.Exclude, host) {
			action = false
			continue
		}
		action = policy.Action == ActionMITM
	}
	return action
}

func interceptionHostname(host string) string {
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return strings.ToLower(strings.Trim(host, "[]"))
}

func matchesDomainPatterns(patterns []string, host string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(strings.ToLower(pattern))
		switch {
		case pattern == "*":
			return true
		case strings.HasPrefix(pattern, "*."):
			domain := strings.TrimPrefix(pattern, "*.")
			if host == domain || strings.HasSuffix(host, "."+domain) {
				return true
			}
		case host == pattern:
			return true
		}
	}
	return false
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return false
		}
	}
	return true
}
