package plugin

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	shared "res-downloader/internal/model"
)

var semanticVersionPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

type semanticVersion struct {
	major, minor, patch int
	prerelease          string
}

func parseSemanticVersion(value string) (semanticVersion, error) {
	matches := semanticVersionPattern.FindStringSubmatch(value)
	if len(matches) == 0 {
		return semanticVersion{}, fmt.Errorf("version %q is not valid SemVer", value)
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	return semanticVersion{major: major, minor: minor, patch: patch, prerelease: matches[4]}, nil
}

func compareSemanticVersions(left, right string) (int, error) {
	a, err := parseSemanticVersion(left)
	if err != nil {
		return 0, err
	}
	b, err := parseSemanticVersion(right)
	if err != nil {
		return 0, err
	}
	for _, pair := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1, nil
		}
		if pair[0] > pair[1] {
			return 1, nil
		}
	}
	if a.prerelease == b.prerelease {
		return 0, nil
	}
	if a.prerelease == "" {
		return 1, nil
	}
	if b.prerelease == "" {
		return -1, nil
	}
	return comparePrerelease(a.prerelease, b.prerelease), nil
}

func comparePrerelease(left, right string) int {
	a, b := strings.Split(left, "."), strings.Split(right, ".")
	for index := 0; index < len(a) && index < len(b); index++ {
		if a[index] == b[index] {
			continue
		}
		aNumber, aErr := strconv.Atoi(a[index])
		bNumber, bErr := strconv.Atoi(b[index])
		switch {
		case aErr == nil && bErr == nil:
			if aNumber < bNumber {
				return -1
			}
			return 1
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		default:
			return strings.Compare(a[index], b[index])
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func addedPluginPermissions(previous, next []string) []string {
	existing := make(map[string]bool, len(previous))
	for _, value := range previous {
		existing[value] = true
	}
	added := make([]string, 0)
	for _, value := range next {
		if !existing[value] {
			added = append(added, value)
		}
	}
	return added
}

func pluginPermissionIncrease(previous, next shared.PluginPermissions) []string {
	added := addedPluginPermissions(previous.Capabilities, next.Capabilities)
	for _, domain := range addedPluginPermissions(previous.Domains, next.Domains) {
		added = append(added, "domain:"+domain)
	}
	if next.BodyLimit > previous.BodyLimit {
		added = append(added, "bodyLimit")
	}
	return added
}
