package resource

import (
	"errors"
	"fmt"
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
	_, err := r.publishCandidates(candidates, false)
	if err != nil && r.logger != nil {
		r.logger.Esg(err, "publish resource batch")
	}
}

// PublishOperationCandidates publishes an explicitly requested result without
// applying passive capture filters. Returned records have their final catalog
// IDs, in first-occurrence order, with duplicate groups merged into one record.
// Success requires durable storage; no catalog or event changes escape a failed
// transaction. Plugin permissions and candidate ownership are checked upstream.
func (r *Resource) PublishOperationCandidates(candidates []shared.ResourceCandidate) ([]shared.ResourceCandidate, error) {
	return r.publishCandidates(candidates, true)
}

func (r *Resource) publishCandidates(candidates []shared.ResourceCandidate, requireDurable bool) ([]shared.ResourceCandidate, error) {
	if len(candidates) == 0 {
		return []shared.ResourceCandidate{}, nil
	}
	if requireDurable {
		// Validation and normalization modify nested tracks and metadata. Keep
		// caller objects and existing catalog snapshots untouched until commit.
		var err error
		candidates, err = shared.CloneJSON(candidates)
		if err != nil {
			return nil, fmt.Errorf("copy operation resources: %w", err)
		}
		for i := range candidates {
			if candidates[i].Source.PluginID == "" {
				return nil, fmt.Errorf("operation resource %d has no plugin owner", i)
			}
			if err := validateCandidate(&candidates[i]); err != nil {
				return nil, fmt.Errorf("operation resource %d: %w", i, err)
			}
		}
	}
	now := time.Now()
	for i := range candidates {
		normalizeResourceModel(&candidates[i], now)
	}
	r.catalogMux.Lock()
	if requireDurable && r.store == nil {
		r.catalogMux.Unlock()
		return nil, errors.New("resource database is not available")
	}
	plan, err := r.planPublicationLocked(candidates, requireDurable)
	if err != nil {
		r.catalogMux.Unlock()
		return nil, err
	}
	if r.store != nil && len(plan) > 0 {
		if err := r.store.UpsertMany(plan); err != nil {
			if requireDurable {
				r.catalogMux.Unlock()
				return nil, fmt.Errorf("persist operation resources: %w", err)
			}
			if r.logger != nil {
				r.logger.Esg(err, "persist resource batch")
			}
		}
	}
	changedIDs := make(map[string]struct{}, len(plan))
	types := make([]string, 0, len(plan))
	for _, candidate := range plan {
		r.mediaMark.Store(candidate.DedupeKey, true)
		r.catalog.Store(candidate.ID, candidate)
		if candidate.GroupKey != "" {
			r.groupIndex.Store(resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey), candidate.ID)
		}
		changedIDs[candidate.ID] = struct{}{}
		types = append(types, candidate.PrimaryType)
	}
	r.registerTypes(types)
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
		r.emitEvent("resourcesBatch", map[string]interface{}{"items": updates, "total": len(tree), "recordCount": len(all)})
	}
	return plan, nil
}

// planPublicationLocked builds a private overlay: every candidate in this call
// sees earlier group merges, but no shared map is mutated before persistence.
func (r *Resource) planPublicationLocked(candidates []shared.ResourceCandidate, strict bool) ([]shared.ResourceCandidate, error) {
	byID := map[string]shared.ResourceCandidate{}
	byGroup := map[string]string{}
	byDedupe := map[string]string{}
	for _, candidate := range r.catalogCandidates() {
		byID[candidate.ID] = candidate
		byDedupe[candidate.DedupeKey] = candidate.ID
		if candidate.GroupKey != "" {
			byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey)] = candidate.ID
		}
	}
	ids := make([]string, 0, len(candidates))
	selected := map[string]bool{}
	for _, candidate := range candidates {
		isUpdate := false
		if candidate.GroupKey != "" {
			if id := byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey)]; id != "" {
				candidate = plugin.MergeResourceCandidate(byID[id], candidate)
				candidate.ID = id
				isUpdate = true
			}
		}
		if !isUpdate {
			if id, exists := byDedupe[candidate.DedupeKey]; exists || r.mediaIsMarked(candidate.DedupeKey) {
				if !strict {
					continue
				}
				stored, ok := byID[id]
				if !ok || stored.Source.PluginID != candidate.Source.PluginID {
					return nil, errors.New("resource deduplication references an unavailable or differently owned resource")
				}
				candidate = stored
				isUpdate = true
			}
		}
		if candidate.ID == "" || (strict && !isUpdate) {
			id, err := gonanoid.New()
			if err != nil && strict {
				return nil, fmt.Errorf("create resource ID: %w", err)
			}
			if id == "" {
				id = candidate.DedupeKey
			}
			candidate.ID = id
		}
		if candidate.ParentID == "" && candidate.ParentGroupKey != "" {
			candidate.ParentID = byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.ParentGroupKey)]
		}
		byID[candidate.ID] = candidate
		byDedupe[candidate.DedupeKey] = candidate.ID
		if candidate.GroupKey != "" {
			byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey)] = candidate.ID
		}
		if !selected[candidate.ID] {
			selected[candidate.ID] = true
			ids = append(ids, candidate.ID)
		}
	}
	// Resolve forward parent references after every ID is known. This makes
	// publication independent of parent/child input order.
	for _, id := range ids {
		candidate := byID[id]
		if candidate.ParentGroupKey != "" && (strict || candidate.ParentID == "") {
			candidate.ParentID = byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.ParentGroupKey)]
		}
		if strict {
			cloned, err := shared.CloneJSON(candidate)
			if err != nil {
				return nil, fmt.Errorf("copy merged operation resource: %w", err)
			}
			candidate = cloned
			if err := validateCandidate(&candidate); err != nil {
				return nil, fmt.Errorf("merged operation resource: %w", err)
			}
		}
		byID[id] = candidate
	}
	if strict {
		if err := validatePublicationParents(ids, byID, byGroup); err != nil {
			return nil, err
		}
	}
	plan := make([]shared.ResourceCandidate, 0, len(ids))
	for _, id := range ids {
		plan = append(plan, byID[id])
	}
	return plan, nil
}

func validatePublicationParents(ids []string, byID map[string]shared.ResourceCandidate, byGroup map[string]string) error {
	for _, id := range ids {
		seen := map[string]bool{}
		for current := id; current != ""; {
			if seen[current] {
				return errors.New("operation resources contain a parent cycle")
			}
			seen[current] = true
			candidate := byID[current]
			parentID := candidate.ParentID
			if parentID == "" && candidate.ParentGroupKey != "" {
				parentID = byGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.ParentGroupKey)]
			}
			if parentID == "" && candidate.ParentGroupKey == "" {
				break
			}
			parent, exists := byID[parentID]
			if !exists || parent.Source.PluginID != candidate.Source.PluginID {
				return errors.New("operation resource parent is missing or owned by another plugin")
			}
			current = parentID
		}
	}
	return nil
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
