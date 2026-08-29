package app

import internalrules "res-downloader/internal/rules"

const (
	InterceptionActionMITM = internalrules.ActionMITM
	InterceptionActionPass = internalrules.ActionPass
)

type InterceptionPolicy = internalrules.Policy
type RuleSet = internalrules.Set

func newRuleSet(policies []InterceptionPolicy) (*RuleSet, error) {
	return internalrules.New(policies)
}

func validateInterceptionPolicies(policies []InterceptionPolicy) error {
	return internalrules.Validate(policies)
}

func cloneInterceptionPolicies(policies []InterceptionPolicy) []InterceptionPolicy {
	return internalrules.Clone(policies)
}
