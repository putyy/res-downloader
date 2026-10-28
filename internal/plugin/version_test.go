package plugin

import "testing"

func TestSemanticVersionComparisonAndPermissionIncrease(t *testing.T) {
	comparison, err := compareSemanticVersions("1.2.0", "1.1.9")
	if err != nil || comparison <= 0 {
		t.Fatalf("comparison=%d err=%v", comparison, err)
	}
	comparison, err = compareSemanticVersions("2.0.0-beta.1", "2.0.0")
	if err != nil || comparison >= 0 {
		t.Fatalf("prerelease comparison=%d err=%v", comparison, err)
	}
	if _, err := parseSemanticVersion("v1.0"); err == nil {
		t.Fatal("invalid SemVer was accepted")
	}
	added := addedPluginPermissions([]string{"emit-resource"}, []string{"emit-resource", "media.basic"})
	if len(added) != 1 || added[0] != "media.basic" {
		t.Fatalf("added=%#v", added)
	}
}
