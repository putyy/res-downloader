package resource

import (
	shared "res-downloader/internal/model"
	"testing"
)

func TestResourceRecordCountIncludesChildrenOutsidePage(t *testing.T) {
	resources := &Resource{}
	for _, candidate := range []shared.ResourceCandidate{
		{ID: "collection", GroupKey: "group", Kind: shared.ResourceKindCollection},
		{ID: "first", ParentID: "collection", Kind: "media.image"},
		{ID: "second", ParentGroupKey: "group", Kind: "media.image"},
		{ID: "other", Kind: "media.video"},
	} {
		resources.catalog.Store(candidate.ID, candidate)
	}
	page := resources.ListPage(0, 1)
	if len(page.Items) != 1 || page.Total != 2 || page.RecordCount != 4 || page.NextOffset != 1 {
		t.Fatalf("paginated counts = %#v", page)
	}
	resources.DeleteMany([]string{"collection"})
	if count := resources.RecordCount(); count != 1 {
		t.Fatalf("count after deleting collection = %d, want 1", count)
	}
	resources.Clear()
	if count := resources.ListPage(0, 1).RecordCount; count != 0 {
		t.Fatalf("count after clearing = %d, want 0", count)
	}
}
