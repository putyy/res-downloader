package operation

import m "res-downloader/internal/model"

const MaxResourceStatusIDs = 1000

// ResourceExecutions supplies resource-row status independently of paginated
// history. Active work takes precedence over a newer completed invocation.
// A result payload is never needed for a resource row.
func (s *Service) ResourceExecutions(ids []string) ([]m.OperationExecution, error) {
	if len(ids) > MaxResourceStatusIDs {
		return nil, problem("invalid_input", "resource status accepts at most 1000 IDs")
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || len(id) > 512 {
			return nil, problem("invalid_input", "invalid resource ID")
		}
		wanted[id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return nil, err
	}
	if err := s.cleanLocked(false, false); err != nil {
		return nil, err
	}
	latest := make(map[string]m.OperationExecution, len(wanted))
	for _, r := range s.records {
		v := r.View
		if !wanted[v.ResourceID] {
			continue
		}
		previous, exists := latest[v.ResourceID]
		active, previousActive := !Terminal(v.State), !Terminal(previous.State)
		newer := v.CreatedAt > previous.CreatedAt || (v.CreatedAt == previous.CreatedAt && v.ExecutionID > previous.ExecutionID)
		if !exists || (active && !previousActive) || (active == previousActive && newer) {
			v.Result = nil
			latest[v.ResourceID] = v
		}
	}
	out := make([]m.OperationExecution, 0, len(latest))
	for _, id := range ids {
		if v, ok := latest[id]; ok {
			out = append(out, v)
			delete(latest, id)
		}
	}
	return clone(out), nil
}
