package rules

import "testing"

func TestInterceptionPoliciesAreOrderedAndStructured(t *testing.T) {
	rules := &Set{}
	err := rules.Load([]Policy{
		{ID: "all", Name: "All", Enabled: true, Domains: []string{"*"}, Exclude: []string{"static.example.com"}, Action: ActionMITM},
		{ID: "private-pass", Name: "Pass", Enabled: true, Domains: []string{"*.private.example.com"}, Action: ActionPass},
		{ID: "one-private", Name: "One", Enabled: true, Domains: []string{"video.private.example.com"}, Action: ActionMITM},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rules.ShouldMITM("api.example.com:443") {
		t.Fatal("broad MITM policy did not match")
	}
	if rules.ShouldMITM("static.example.com:443") {
		t.Fatal("policy exclusion did not pass through")
	}
	if rules.ShouldMITM("audio.private.example.com:443") {
		t.Fatal("later pass policy did not override broad MITM")
	}
	if !rules.ShouldMITM("video.private.example.com:443") {
		t.Fatal("last matching MITM policy did not win")
	}
}

func TestInterceptionPolicyValidation(t *testing.T) {
	tests := [][]Policy{
		{{ID: "bad id", Domains: []string{"*"}, Action: ActionMITM}},
		{{ID: "missing-domains", Action: ActionMITM}},
		{{ID: "bad-action", Domains: []string{"*"}, Action: "shell"}},
		{{ID: "bad-domain", Domains: []string{"https://example.com"}, Action: ActionMITM}},
	}
	for index, policies := range tests {
		if err := Validate(policies); err == nil {
			t.Fatalf("case %d should be invalid", index)
		}
	}
}
