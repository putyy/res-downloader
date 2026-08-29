package resource

import (
	shared "res-downloader/internal/model"
	"time"
)

func (r *Resource) getResType(key string) (bool, bool) {
	r.resTypeMux.RLock()
	value, ok := r.resType[key]
	r.resTypeMux.RUnlock()
	return value, ok
}

func (r *Resource) setResType(types []string) {
	r.resTypeMux.Lock()
	for key := range r.resType {
		r.resType[key] = false
	}
	for _, value := range types {
		if _, ok := r.resType[value]; ok {
			r.resType[value] = true
		}
	}
	r.resTypeMux.Unlock()
}

func (r *Resource) registerTypes(types []string) {
	r.resTypeMux.Lock()
	defer r.resTypeMux.Unlock()
	allEnabled := r.resType["all"]
	for _, value := range types {
		if value == "" || value == "all" {
			continue
		}
		if _, exists := r.resType[value]; !exists {
			r.resType[value] = allEnabled
		}
	}
}

func (r *Resource) RegisterTypes(types []string) {
	r.registerTypes(types)
}

func (r *Resource) SetTypes(types []string) { r.setResType(types) }

// filterSelectedCandidates applies the capture-kind selection to a complete
// plugin result. Collections are evaluated together with their children so a
// selected child never appears without its parent and selecting a collection
// keeps its complete subtree.
func (r *Resource) filterSelectedCandidates(candidates []shared.ResourceCandidate) []shared.ResourceCandidate {
	if len(candidates) == 0 {
		return nil
	}

	kinds := make([]string, 0, len(candidates))
	for index := range candidates {
		normalizeResourceModel(&candidates[index], time.Now())
		kinds = append(kinds, candidates[index].PrimaryType)
	}
	r.registerTypes(kinds)

	r.resTypeMux.RLock()
	allEnabled := r.resType["all"]
	selectedKinds := make(map[string]bool, len(r.resType))
	for kind, selected := range r.resType {
		selectedKinds[kind] = selected
	}
	r.resTypeMux.RUnlock()
	if allEnabled {
		return candidates
	}

	collectionByGroup := make(map[string]int)
	for index, candidate := range candidates {
		if candidate.Kind == shared.ResourceKindCollection && candidate.GroupKey != "" {
			collectionByGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey)] = index
		}
	}

	keep := make([]bool, len(candidates))
	includeSubtree := make([]bool, len(candidates))
	for index, candidate := range candidates {
		keep[index] = selectedKinds[candidate.PrimaryType] || selectedKinds[candidate.Kind]
		includeSubtree[index] = keep[index] && candidate.Kind == shared.ResourceKindCollection
	}

	for changed := true; changed; {
		changed = false
		for index, candidate := range candidates {
			if candidate.ParentGroupKey == "" {
				continue
			}
			parentIndex, exists := collectionByGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.ParentGroupKey)]
			if !exists || !includeSubtree[parentIndex] || includeSubtree[index] {
				continue
			}
			keep[index] = true
			includeSubtree[index] = true
			changed = true
		}
	}

	for changed := true; changed; {
		changed = false
		for index, candidate := range candidates {
			if !keep[index] || candidate.ParentGroupKey == "" {
				continue
			}
			parentIndex, exists := collectionByGroup[resourceGroupIndexKey(candidate.Source.PluginID, candidate.ParentGroupKey)]
			if exists && !keep[parentIndex] {
				keep[parentIndex] = true
				changed = true
			}
		}
	}

	filtered := make([]shared.ResourceCandidate, 0, len(candidates))
	for index, candidate := range candidates {
		if keep[index] {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func (r *Resource) FilterSelectedCandidates(candidates []shared.ResourceCandidate) []shared.ResourceCandidate {
	return r.filterSelectedCandidates(candidates)
}
