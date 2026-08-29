package resource

import (
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

// PublishCandidate is the narrow ingestion boundary used by plugin runtimes.
// Keeping catalog mutation here prevents the plugin manager from depending on
// Resource's storage and indexing internals.
func (r *Resource) PublishCandidate(candidate shared.ResourceCandidate) {
	r.PublishCandidates([]shared.ResourceCandidate{candidate})
}

func (r *Resource) PublishCandidates(candidates []shared.ResourceCandidate) {
	if len(candidates) == 0 {
		return
	}
	now := time.Now()
	types := make([]string, 0, len(candidates))
	for index := range candidates {
		normalizeResourceModel(&candidates[index], now)
		types = append(types, candidates[index].PrimaryType)
	}
	r.registerTypes(types)
	r.catalogMux.Lock()
	persisted := make([]shared.ResourceCandidate, 0, len(candidates))
	changedIDs := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		isUpdate := false
		if candidate.GroupKey != "" {
			groupIndexKey := resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey)
			if existingID, exists := r.groupIndex.Load(groupIndexKey); exists {
				if stored, ok := r.catalog.Load(existingID); ok {
					candidate = plugin.MergeResourceCandidate(stored.(shared.ResourceCandidate), candidate)
					candidate.ID = existingID.(string)
					isUpdate = true
				}
			}
		}
		if !isUpdate && r.mediaIsMarked(candidate.DedupeKey) {
			continue
		}
		id := candidate.ID
		if id == "" {
			id, _ = gonanoid.New()
			if id == "" {
				id = candidate.DedupeKey
			}
		}
		candidate.ID = id
		if candidate.ParentID == "" && candidate.ParentGroupKey != "" {
			if parentID, exists := r.groupIndex.Load(resourceGroupIndexKey(candidate.Source.PluginID, candidate.ParentGroupKey)); exists {
				candidate.ParentID, _ = parentID.(string)
			}
		}
		r.mediaMark.Store(candidate.DedupeKey, true)
		r.catalog.Store(id, candidate)
		if candidate.GroupKey != "" {
			r.groupIndex.Store(resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey), id)
		}
		persisted = append(persisted, candidate)
		changedIDs[id] = struct{}{}
	}
	if r.store != nil && len(persisted) > 0 {
		if err := r.store.UpsertMany(persisted); err != nil {
			r.logger.Esg(err, "persist resource batch")
		}
	}
	all := r.catalogCandidates()
	rootIDs := resourceRootIDs(all, changedIDs)
	tree := resourceViewTree(all)
	updates := make([]shared.ResourceView, 0, len(rootIDs))
	for _, root := range tree {
		if _, exists := rootIDs[root.ID]; exists {
			updates = append(updates, root)
		}
	}
	r.catalogMux.Unlock()
	if len(updates) > 0 {
		r.emitEvent("resourcesBatch", map[string]interface{}{"items": updates, "total": len(tree)})
	}
}

func resourceRootIDs(candidates []shared.ResourceCandidate, changed map[string]struct{}) map[string]struct{} {
	byID := make(map[string]shared.ResourceCandidate, len(candidates))
	byGroup := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
		if candidate.GroupKey != "" {
			byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey)] = candidate.ID
		}
	}
	roots := make(map[string]struct{}, len(changed))
	for id := range changed {
		current := id
		seen := make(map[string]struct{})
		for {
			if _, exists := seen[current]; exists {
				break
			}
			seen[current] = struct{}{}
			candidate, exists := byID[current]
			if !exists {
				break
			}
			parentID := candidate.ParentID
			if parentID == "" && candidate.ParentGroupKey != "" {
				parentID = byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.ParentGroupKey)]
			}
			parent, hasParent := byID[parentID]
			if parentID == "" || !hasParent || parent.Source.PluginID != candidate.Source.PluginID {
				break
			}
			current = parentID
		}
		roots[current] = struct{}{}
	}
	return roots
}
