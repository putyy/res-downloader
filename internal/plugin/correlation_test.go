package plugin

import "testing"

func TestPluginCorrelationStoreSupportsOneToMany(t *testing.T) {
	store := &pluginCorrelationStore{entries: make(map[string]pluginCorrelationEntry)}
	alias := "HTTPS://CDN.EXAMPLE/audio.m4a#fragment"
	store.register("example.plugin", pluginCorrelationRegistration{GroupKey: "item-1", TrackID: "audio", Role: "audio", Aliases: []string{alias}})
	store.register("example.plugin", pluginCorrelationRegistration{GroupKey: "item-2", TrackID: "audio", Role: "audio", Aliases: []string{alias}})
	refs := store.find("example.plugin", "https://cdn.example/audio.m4a")
	if len(refs) != 2 || refs[0].GroupKey != "item-1" || refs[1].GroupKey != "item-2" {
		t.Fatalf("correlation refs = %#v", refs)
	}
	if refs := store.find("another.plugin", "https://cdn.example/audio.m4a"); len(refs) != 0 {
		t.Fatalf("correlations leaked across plugins: %#v", refs)
	}
}
