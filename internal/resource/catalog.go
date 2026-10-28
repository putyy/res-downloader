package resource

import (
	"encoding/json"
	"fmt"
	shared "res-downloader/internal/model"
	"sort"
	"strconv"
	"time"
)

func (r *Resource) clear() {
	r.catalogMux.Lock()
	defer r.catalogMux.Unlock()
	clear(r.catalog)
	clear(r.dedupeIndex)
	clear(r.groupIndex)
	if r.store != nil {
		if err := r.store.Clear(); err != nil {
			r.logger.Esg(err, "clear resource database")
		}
	}
}

func (r *Resource) deleteMany(ids []string) {
	r.catalogMux.Lock()
	defer r.catalogMux.Unlock()
	selectedIDs := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selectedIDs[id] = struct{}{}
	}
	candidates := r.catalogCandidatesLocked()
	// Removing a collection removes all descendants, while removing a child
	// leaves the rest of the collection intact.
	for changed := true; changed; {
		changed = false
		for _, candidate := range candidates {
			if _, exists := selectedIDs[candidate.ID]; exists {
				continue
			}
			if r.candidateHasSelectedParent(candidate, candidates, selectedIDs) {
				selectedIDs[candidate.ID] = struct{}{}
				changed = true
			}
		}
	}
	deletedIDs := make([]string, 0, len(selectedIDs))
	for id, candidate := range r.catalog {
		if _, exists := selectedIDs[id]; exists {
			r.removeCandidateIndexesLocked(candidate)
			delete(r.catalog, id)
			deletedIDs = append(deletedIDs, id)
		}
	}
	if r.store != nil {
		if err := r.store.Delete(deletedIDs); err != nil {
			r.logger.Esg(err, "delete resources from database")
		}
	}
}

func (r *Resource) candidateHasSelectedParent(candidate shared.ResourceCandidate, all []shared.ResourceCandidate, selected map[string]struct{}) bool {
	if candidate.ParentID != "" {
		for _, parent := range all {
			if parent.ID == candidate.ParentID && parent.Source.PluginID == candidate.Source.PluginID {
				_, exists := selected[parent.ID]
				return exists
			}
		}
		return false
	}
	if candidate.ParentGroupKey == "" {
		return false
	}
	for _, parent := range all {
		if parent.Source.PluginID == candidate.Source.PluginID && parent.GroupKey == candidate.ParentGroupKey {
			_, exists := selected[parent.ID]
			return exists
		}
	}
	return false
}

func (r *Resource) restore() error {
	r.catalogMux.Lock()
	defer r.catalogMux.Unlock()
	records, err := r.store.List()
	if err != nil {
		return err
	}
	for _, record := range records {
		candidate := record.Candidate
		if candidate.ID == "" || candidate.DedupeKey == "" {
			continue
		}
		persistedUpdatedAt := candidate.Lifecycle.UpdatedAt
		normalizeResourceModel(&candidate, time.Now())
		candidate.Lifecycle.UpdatedAt = persistedUpdatedAt
		r.saveCandidateLocked(candidate)
		r.registerTypes([]string{candidate.PrimaryType})
	}
	return nil
}

func (r *Resource) ReconcilePluginAvailability(manager PluginStatusProvider) {
	if manager == nil {
		return
	}
	r.catalogMux.Lock()
	defer r.catalogMux.Unlock()
	now := time.Now().UnixMilli()
	for id, candidate := range r.catalog {
		previous := candidate.Lifecycle.Availability
		status, exists := manager.Status(candidate.Source.PluginID)
		switch {
		case candidate.Source.PluginID != "" && (!exists || !status.Loaded):
			candidate.Lifecycle.Availability = shared.ResourceAvailabilityPluginMissing
			candidate.Lifecycle.UnavailableReason = "resource plugin is not loaded"
		case candidate.Source.PluginDigest != "" && status.Digest != "" && candidate.Source.PluginDigest != status.Digest:
			candidate.Lifecycle.Availability = shared.ResourceAvailabilityPluginIncompatible
			candidate.Lifecycle.UnavailableReason = "resource was captured by a different plugin version"
		case resourceNeedsRecaptureForHeaders(candidate):
			candidate.Lifecycle.Availability = shared.ResourceAvailabilityNeedsRefresh
			candidate.Lifecycle.UnavailableReason = "non-persistent request headers require recapture"
		case candidate.Lifecycle.ExpiresAt > 0 && candidate.Lifecycle.ExpiresAt <= now:
			candidate.Lifecycle.Availability = shared.ResourceAvailabilityNeedsRefresh
			candidate.Lifecycle.UnavailableReason = "resource URL or signature may have expired"
		default:
			candidate.Lifecycle.Availability = shared.ResourceAvailabilityAvailable
			candidate.Lifecycle.UnavailableReason = ""
		}
		if candidate.Lifecycle.Availability != previous {
			candidate.Lifecycle.UpdatedAt = now
			r.catalog[id] = candidate
			if r.store != nil {
				_ = r.store.Upsert(candidate)
			}
		}
	}
}

func (r *Resource) list() []shared.ResourceView {
	items, _ := r.listWithRecordCount()
	return items
}

func (r *Resource) listWithRecordCount() ([]shared.ResourceView, int) {
	candidates := r.catalogCandidates()
	return r.viewsFromCandidates(candidates), len(candidates)
}

func (r *Resource) viewsFromCandidates(candidates []shared.ResourceCandidate) []shared.ResourceView {
	items := resourceViewTree(candidates)
	if r.config != nil && r.config.Snapshot().InsertTail {
		for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
			items[left], items[right] = items[right], items[left]
		}
	}
	return items
}

func resourceViewTree(candidates []shared.ResourceCandidate) []shared.ResourceView {
	byID := make(map[string]shared.ResourceCandidate, len(candidates))
	byGroup := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
		if candidate.GroupKey != "" {
			byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey)] = candidate.ID
		}
	}
	children := make(map[string][]shared.ResourceView)
	roots := make([]shared.ResourceView, 0, len(candidates))
	for _, candidate := range candidates {
		parentID := candidate.ParentID
		if parentID == "" && candidate.ParentGroupKey != "" {
			parentID = byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.ParentGroupKey)]
		}
		candidate.ParentID = parentID
		view := resourceViewFromCandidate(candidate)
		if parentID != "" {
			if parent, exists := byID[parentID]; exists && parent.Source.PluginID == candidate.Source.PluginID {
				children[parentID] = append(children[parentID], view)
				continue
			}
		}
		roots = append(roots, view)
	}
	for index := range roots {
		attachResourceChildren(&roots[index], children)
	}
	sort.SliceStable(roots, func(i, j int) bool {
		leftTime := resourceInsertionTime(roots[i].ResourceCandidate)
		rightTime := resourceInsertionTime(roots[j].ResourceCandidate)
		if leftTime == rightTime {
			return roots[i].ID < roots[j].ID
		}
		return leftTime > rightTime
	})
	return roots
}

func resourceInsertionTime(candidate shared.ResourceCandidate) int64 {
	if candidate.Lifecycle.DiscoveredAt > 0 {
		return candidate.Lifecycle.DiscoveredAt
	}
	return candidate.Lifecycle.UpdatedAt
}

func attachResourceChildren(parent *shared.ResourceView, children map[string][]shared.ResourceView) {
	parent.Children = children[parent.ID]
	if len(parent.Children) == 0 {
		return
	}
	sort.SliceStable(parent.Children, func(i, j int) bool {
		return candidateCollectionIndex(parent.Children[i].ResourceCandidate) < candidateCollectionIndex(parent.Children[j].ResourceCandidate)
	})
	for index := range parent.Children {
		attachResourceChildren(&parent.Children[index], children)
	}
}

func (r *Resource) catalogCandidates() []shared.ResourceCandidate {
	r.catalogMux.RLock()
	defer r.catalogMux.RUnlock()
	return r.catalogCandidatesLocked()
}

// catalogCandidatesLocked requires catalogMux to be held for reading or writing.
func (r *Resource) catalogCandidatesLocked() []shared.ResourceCandidate {
	candidates := make([]shared.ResourceCandidate, 0, len(r.catalog))
	for _, candidate := range r.catalog {
		candidates = append(candidates, candidate)
	}
	return candidates
}

// saveCandidateLocked updates the catalog and both indexes under catalogMux.
func (r *Resource) saveCandidateLocked(candidate shared.ResourceCandidate) {
	if r.catalog == nil {
		r.catalog = make(map[string]shared.ResourceCandidate)
		r.dedupeIndex = make(map[string]string)
		r.groupIndex = make(map[string]string)
	}
	if previous, exists := r.catalog[candidate.ID]; exists {
		r.removeCandidateIndexesLocked(previous)
	}
	r.catalog[candidate.ID] = candidate
	if candidate.DedupeKey != "" {
		r.dedupeIndex[candidate.DedupeKey] = candidate.ID
	}
	if candidate.GroupKey != "" {
		r.groupIndex[resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey)] = candidate.ID
	}
}

func (r *Resource) removeCandidateIndexesLocked(candidate shared.ResourceCandidate) {
	if r.dedupeIndex[candidate.DedupeKey] == candidate.ID {
		delete(r.dedupeIndex, candidate.DedupeKey)
	}
	groupKey := resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey)
	if r.groupIndex[groupKey] == candidate.ID {
		delete(r.groupIndex, groupKey)
	}
}

func (r *Resource) childrenOf(parent shared.ResourceCandidate) []shared.ResourceCandidate {
	children := make([]shared.ResourceCandidate, 0)
	for _, candidate := range r.catalogCandidates() {
		samePlugin := candidate.Source.PluginID == parent.Source.PluginID
		byID := candidate.ParentID != "" && candidate.ParentID == parent.ID
		byGroup := candidate.ParentID == "" && candidate.ParentGroupKey != "" && candidate.ParentGroupKey == parent.GroupKey
		if samePlugin && (byID || byGroup) {
			children = append(children, candidate)
		}
	}
	sort.SliceStable(children, func(i, j int) bool {
		return candidateCollectionIndex(children[i]) < candidateCollectionIndex(children[j])
	})
	return children
}

func candidateCollectionIndex(candidate shared.ResourceCandidate) int {
	value, exists := candidate.Metadata["collectionIndex"]
	if !exists {
		return int(^uint(0) >> 1)
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		parsed, _ := strconv.Atoi(typed.String())
		return parsed
	default:
		return int(^uint(0) >> 1)
	}
}

func (r *Resource) resourceViewWithChildren(parent shared.ResourceCandidate) shared.ResourceView {
	view := resourceViewFromCandidate(parent)
	view.Children = resourceViewTree(r.childrenOf(parent))
	return view
}

func (r *Resource) importResources(items []shared.ResourceView) ([]shared.ResourceView, error) {
	r.catalogMux.Lock()
	defer r.catalogMux.Unlock()
	for _, item := range flattenResourceViews(items, "") {
		candidate := item.ResourceCandidate
		if err := validateCandidate(&candidate); err != nil {
			return nil, fmt.Errorf("import resource %q: %w", item.ID, err)
		}
		if candidate.ID == "" {
			candidate.ID = candidate.DedupeKey
		}
		r.saveCandidateLocked(candidate)
		r.registerTypes([]string{candidate.PrimaryType})
		if r.store != nil {
			if err := r.store.Upsert(candidate); err != nil {
				return nil, err
			}
		}
	}
	return r.viewsFromCandidates(r.catalogCandidatesLocked()), nil
}

func flattenResourceViews(items []shared.ResourceView, inheritedParentID string) []shared.ResourceView {
	flattened := make([]shared.ResourceView, 0)
	for _, item := range items {
		if item.ParentID == "" {
			item.ParentID = inheritedParentID
		}
		children := item.Children
		item.Children = nil
		flattened = append(flattened, item)
		flattened = append(flattened, flattenResourceViews(children, item.ID)...)
	}
	return flattened
}

func (r *Resource) Close() {
	if r.store != nil {
		if err := r.store.Close(); err != nil {
			r.logger.Esg(err, "close resource database")
		}
	}
}

func resourceGroupIndexKey(pluginID, groupKey string) string {
	return pluginID + "\x00" + groupKey
}
