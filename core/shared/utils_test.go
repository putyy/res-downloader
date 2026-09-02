package shared

import "testing"

func TestGetTopLevelDomainStripsURLPort(t *testing.T) {
	for _, rawURL := range []string{
		"https://findera4.video.qq.com:443/251/20302/stodownload",
		"https://finder.video.qq.com/251/20302/stodownload",
		"findera4.video.qq.com:443",
	} {
		if got := GetTopLevelDomain(rawURL); got != "qq.com" {
			t.Fatalf("GetTopLevelDomain(%q) = %q, want qq.com", rawURL, got)
		}
	}
}
