// Package operation owns execution state, admission, retention and durable history.
// Transport and plugin adapters never maintain a second execution state machine.
package operation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	m "res-downloader/internal/model"
)

type Backend interface {
	Operations() []m.OperationInfo
	Sessions() []m.OperationSession
	Send(m.OperationExecution, m.OperationRequest, string) error
	Finalize(context.Context, m.OperationExecution, m.OperationDefinition, json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error)
}
type record struct {
	View           m.OperationExecution  `json:"view"`
	Hash           string                `json:"hash"`
	Key            string                `json:"key,omitempty"`
	KeyExpires     int64                 `json:"keyExpires,omitempty"`
	Definition     m.OperationDefinition `json:"definition"`
	Request        m.OperationRequest    `json:"-"`
	BatchKey       string                `json:"batchKey,omitempty"`
	BatchHash      string                `json:"batchHash,omitempty"`
	LeaseHeld      bool                  `json:"leaseHeld,omitempty"`
	Finalizing     bool                  `json:"-"`
	FinalizeCancel context.CancelFunc    `json:"-"`
	Reused         bool                  `json:"-"`
	StoredResult   json.RawMessage       `json:"storedResult,omitempty"`
	Reload         *reloadState          `json:"-"`
}
type Settings struct {
	HistoryDays int             `json:"historyDays"`
	ResultHours int             `json:"resultHours"`
	Automation  map[string]bool `json:"automation"`
}
type Service struct {
	mu          sync.Mutex
	backend     Backend
	db          *bolt.DB
	keys        map[string]keyRecord
	records     map[string]*record
	cursors     map[string]cursor
	artifacts   map[string]m.OperationArtifact
	settings    Settings
	closing     bool
	fault       error
	stop        chan struct{}
	done        chan struct{}
	finalizers  sync.WaitGroup
	closeOnce   sync.Once
	nextCleanup int64
}

func New(path string, backend Backend) (*Service, error) {
	s := &Service{backend: backend, records: map[string]*record{}, keys: map[string]keyRecord{}, artifacts: map[string]m.OperationArtifact{}, settings: Settings{7, 2, map[string]bool{}}, stop: make(chan struct{}), done: make(chan struct{})}
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	s.db = db
	err = db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{"executions", "settings", "artifacts", "idempotency"} {
			if _, e := tx.CreateBucketIfNotExists([]byte(name)); e != nil {
				return e
			}
		}
		return nil
	})
	if err == nil {
		err = s.load()
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	s.mu.Lock()
	err = s.cleanLocked(false, false)
	s.mu.Unlock()
	if err != nil {
		db.Close()
		return nil, err
	}
	go s.loop()
	return s, nil
}
func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		close(s.stop)
		<-s.done
		s.mu.Lock()
		s.closing = true
		for _, r := range s.records {
			if !Terminal(r.View.State) {
				s.finish(r, "interrupted", "host_stopped", false)
			}
		}
		s.mu.Unlock()
		s.finalizers.Wait()
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.db.Close(); s.fault == nil {
			s.fault = err
		}
		s.db = nil

	})
	return s.fault
}
func (s *Service) loop() {
	defer close(s.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	n := 0
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			if s.fault == nil {
				s.tickLocked()
				n++
				if n%60 == 0 {
					_ = s.cleanLocked(false, false)
				}
			}
			s.mu.Unlock()
		}
	}
}
func id() (string, error) {
	b := make([]byte, 16)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
func hash(v interface{}) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }
func (s *Service) available() error {
	if s == nil || s.db == nil || s.fault != nil {
		return problem("storage_unavailable", "operation storage is unavailable")
	}
	return nil
}
func (s *Service) enabled(info m.OperationInfo) bool {
	enabled := info.Definition.Automation
	if v, ok := s.settings.Automation[info.PluginID]; ok {
		if !v {
			return false
		}
		enabled = v
	}
	if v, ok := s.settings.Automation[info.PluginID+"/"+info.OperationID]; ok {
		enabled = v
	}
	return enabled
}
func (s *Service) Discover(source string) []m.OperationInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	infos := s.backend.Operations()
	sessions := s.backend.Sessions()
	out := []m.OperationInfo{}
	for _, info := range infos {
		info.AutomationEnabled = s.enabled(info)
		if source == "automation" && !info.AutomationEnabled {
			continue
		}
		for _, session := range sessions {
			if matches(info, session) {
				info.Available = true
			}
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PluginID+out[i].OperationID < out[j].PluginID+out[j].OperationID })
	return clone(out)
}
func (s *Service) Sessions(source string) []m.OperationSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.backend.Sessions()
	if source == "automation" {
		allowed := map[string]bool{}
		for _, info := range s.backend.Operations() {
			if s.enabled(info) {
				allowed[info.PluginID+"/"+info.OperationID] = true
			}
		}
		filtered := []m.OperationSession{}
		for _, p := range out {
			ops := []string{}
			for _, op := range p.Operations {
				if allowed[p.PluginID+"/"+op] {
					ops = append(ops, op)
				}
			}
			if len(ops) > 0 {
				p.Operations = ops
				filtered = append(filtered, p)
			}
		}
		out = filtered
	}
	return clone(out)
}
func matches(info m.OperationInfo, p m.OperationSession) bool {
	if p.PluginID != info.PluginID || p.ScriptID != info.Definition.PageScript || !p.Connected || !p.Ready || (info.Definition.RequiresLogin && p.Login != "authenticated") {
		return false
	}
	for _, op := range p.Operations {
		if op == info.OperationID {
			return true
		}
	}
	return false
}
func (s *Service) definition(pluginID, op string, source string) (m.OperationInfo, error) {
	for _, info := range s.backend.Operations() {
		if info.PluginID == pluginID && info.OperationID == op {
			if source == "automation" && !s.enabled(info) {
				return info, problem("automation_disabled", "automation is disabled for this operation")
			}
			return info, nil
		}
	}
	return m.OperationInfo{}, problem("operation_unavailable", "plugin operation is unavailable")
}
func (s *Service) prepare(req m.OperationRequest, source, batch string, index int) (*record, error) {
	if s.closing {
		return nil, problem("service_stopping", "operation service is stopping")
	}
	if !oneOf(source, "desktop", "automation") {
		return nil, problem("invalid_source", "invalid invocation source")
	}
	if resolver, ok := s.backend.(interface {
		ResolveOperationRequest(m.OperationRequest) (m.OperationRequest, error)
	}); ok {
		var err error
		req, err = resolver.ResolveOperationRequest(req)
		if err != nil {
			return nil, problem("invalid_resource", err.Error())
		}
	}
	info, err := s.definition(req.PluginID, req.OperationID, source)
	if err != nil {
		return nil, err
	}
	if req.Input == nil {
		req.Input = map[string]interface{}{}
	}
	raw, err := json.Marshal(req.Input)
	if err != nil || len(raw) > MaxInputBytes {
		return nil, problem("input_too_large", "operation input exceeds limit")
	}
	req.Input = clone(req.Input)
	if err := ValidateValue(info.Definition.InputSchema, req.Input); err != nil {
		return nil, problem("invalid_input", err.Error())
	}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit < 1 || req.Limit > MaxItems || len(req.IdempotencyKey) > 128 || len(req.RetryOf) > 64 || len(req.Cursor) > 128 {
		return nil, problem("invalid_input", "invalid limit, cursor or key")
	}
	// Idempotency is scoped to caller/plugin/operation and checked before page availability.
	h := hash(req)
	key := ""
	if req.IdempotencyKey != "" {
		scope := "single"
		if batch != "" {
			scope = "batch-item"
		}
		key = hash([]string{scope, source, req.PluginID, req.OperationID, req.IdempotencyKey})
		if prior, ok := s.keys[key]; ok && prior.Expires > time.Now().UnixMilli() {
			if prior.Hash != h {
				return nil, problem("idempotency_conflict", "idempotency key was used with different parameters")
			}
			if r := s.records[prior.Execution.ExecutionID]; r != nil {
				copy := *r
				copy.Reused = true
				return &copy, nil
			}
			return &record{View: prior.Execution, Reused: true}, nil
		}
	}
	var selected *m.OperationSession
	candidates := []string{}
	for _, p := range s.backend.Sessions() {
		if !matches(info, p) {
			continue
		}
		candidates = append(candidates, p.PageSessionID)
		if req.PageSessionID == p.PageSessionID {
			cp := p
			selected = &cp
		}
	}
	if req.PageSessionID == "" {
		if len(candidates) > 1 {
			return nil, problem("page_ambiguous", "select a pageSessionId from list_page_sessions")
		}
		if len(candidates) == 1 {
			for _, p := range s.backend.Sessions() {
				if p.PageSessionID == candidates[0] {
					cp := p
					selected = &cp
				}
			}
		}
	}
	if selected == nil {
		return nil, problem("page_unavailable", "no connected, ready page satisfies the operation requirements")
	}
	for _, pending := range s.records {
		if pending.View.PageSessionID == selected.PageSessionID && Terminal(pending.View.State) && (pending.LeaseHeld || pending.Finalizing) {
			return nil, problem("page_busy_unknown", "previous execution has not stopped; wait for acknowledgement or refresh the page")
		}
	}
	if req.RetryOf != "" {
		old := s.records[req.RetryOf]
		if old == nil || old.View.PluginID != req.PluginID || old.View.OperationID != req.OperationID || !oneOf(old.View.State, "failed", "timed_out", "interrupted") || !info.Definition.SafeRetry || old.View.Certainty == "unknown" {
			return nil, problem("retry_unsafe", "execution cannot be safely retried")
		}
		attempts := 0
		for old != nil {
			attempts++
			old = s.records[old.View.RetryOf]
			if attempts > 2 {
				return nil, problem("retry_limit", "at most two retries are allowed")
			}
		}
	}
	if req.Cursor != "" {
		cursor, err := s.resolveCursor(req, *selected)
		if err != nil {
			return nil, err
		}
		req.Cursor = cursor
	}
	executionID, err := id()
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	r := &record{View: m.OperationExecution{ExecutionID: executionID, BatchID: batch, Index: index, PluginID: req.PluginID, PluginVersion: info.PluginVersion, OperationID: req.OperationID, PageSessionID: selected.PageSessionID, Revision: selected.Revision, Source: source, State: "queued", Certainty: "not_started", ResultStatus: "never_persisted", CreatedAt: now, UpdatedAt: now, RetryOf: req.RetryOf, ResourceID: req.ResourceID, InputSummary: Persisted(info.Definition.InputSchema, req.Input), ResourceIDs: []string{}, DownloadTaskIDs: []string{}, ArtifactIDs: []string{}}, Hash: h, Key: key, KeyExpires: now + IdempotencyTTL.Milliseconds(), Definition: clone(info.Definition), Request: req}
	return r, nil
}
func (s *Service) Submit(req m.OperationRequest, source string) (m.OperationExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return m.OperationExecution{}, err
	}
	if err := s.cleanLocked(false, false); err != nil {
		return m.OperationExecution{}, err
	}
	r, err := s.prepare(req, source, "", 0)
	if err != nil {
		return m.OperationExecution{}, err
	}
	if r.Reused {
		return clone(r.View), nil
	}
	if err = s.admit([]*record{r}); err != nil {
		return m.OperationExecution{}, err
	}
	return clone(r.View), nil
}
func (s *Service) SubmitBatch(requests []m.OperationRequest, key, source string) (m.OperationBatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return m.OperationBatch{}, err
	}
	if err := s.cleanLocked(false, false); err != nil {
		return m.OperationBatch{}, err
	}
	if len(requests) < 1 || len(requests) > MaxBatch || len(key) > 96 {
		return m.OperationBatch{}, problem("invalid_batch", "batch must contain 1–32 requests and a key of at most 96 bytes")
	}
	batchKey, batchHash := "", hash(requests)
	if key != "" {
		batchKey = hash([]string{"batch", source, key})
		if prior, ok := s.keys[batchKey]; ok && prior.Expires > time.Now().UnixMilli() {
			if prior.Hash != batchHash {
				return m.OperationBatch{}, problem("idempotency_conflict", "batch key was used with different parameters")
			}
			for _, req := range requests {
				if _, err := s.definition(req.PluginID, req.OperationID, source); err != nil {
					return m.OperationBatch{}, err
				}
			}
			return s.batchLocked(prior.BatchID, 0, MaxBatch), nil
		}
	}
	batch, err := id()
	if err != nil {
		return m.OperationBatch{}, err
	}
	prepared := []*record{}
	existingBatch := ""
	for i, req := range requests {
		if req.IdempotencyKey != "" {
			return m.OperationBatch{}, problem("invalid_batch", "use the batch idempotency key")
		}
		if key != "" {
			req.IdempotencyKey = "batch:" + key + ":" + fmt.Sprint(i)
		}
		r, e := s.prepare(req, source, batch, i)
		if e != nil {
			return m.OperationBatch{}, e
		}
		if r.Reused {
			if r.View.BatchID == "" {
				return m.OperationBatch{}, problem("idempotency_conflict", "key belongs to a single execution")
			}
			existingBatch = r.View.BatchID
		}
		r.BatchKey = batchKey
		r.BatchHash = batchHash
		prepared = append(prepared, r)
	}
	if existingBatch != "" {
		old := s.batchLocked(existingBatch, 0, MaxBatch)
		if len(old.Items) != len(prepared) {
			return m.OperationBatch{}, problem("idempotency_conflict", "batch parameters differ")
		}
		for _, r := range prepared {
			if r.View.BatchID != existingBatch {
				return m.OperationBatch{}, problem("idempotency_conflict", "batch parameters differ")
			}
		}
		return old, nil
	}
	if err = s.admit(prepared); err != nil {
		return m.OperationBatch{}, err
	}
	return s.batchLocked(batch, 0, MaxBatch), nil
}
func (s *Service) admit(records []*record) error {
	global := 0
	plugins := map[string]int{}
	pages := map[string]int{}
	for _, r := range s.records {
		if !Terminal(r.View.State) || r.Finalizing || r.LeaseHeld {
			global++
			plugins[r.View.PluginID]++
			pages[r.View.PageSessionID]++
		}
	}
	for _, r := range records {
		global++
		plugins[r.View.PluginID]++
		pages[r.View.PageSessionID]++
		if global > MaxActive || plugins[r.View.PluginID] > MaxPluginActive || pages[r.View.PageSessionID] > MaxPageActive {
			return problem("queue_full", "operation queue capacity exceeded; nothing was submitted")
		}
	}
	if err := s.cleanLocked(false, false); err != nil {
		return err
	}
	totalBytes := 0
	for _, r := range s.records {
		raw, _ := json.Marshal(persistent(r))
		totalBytes += len(raw)
	}
	for _, r := range records {
		raw, _ := json.Marshal(persistent(r))
		totalBytes += len(raw)
	}
	if len(s.keys)+len(records)+1 > MaxHistory*2 || totalBytes > MaxHistoryBytes || len(s.records)+len(records) > MaxHistory {
		return problem("history_full", "clear old history before submitting")
	}
	if err := s.save(records); err != nil {
		return err
	}
	for _, r := range records {
		s.records[r.View.ExecutionID] = r
	}
	return nil
}
func (s *Service) tickLocked() {
	now := time.Now()
	pages := map[string]m.OperationSession{}
	for _, p := range s.backend.Sessions() {
		pages[p.PageSessionID] = p
	}
	running := 0
	plugins := map[string]int{}
	busy := map[string]bool{}
	queue := []*record{}
	for _, r := range s.records {
		v := &r.View
		if Terminal(v.State) {
			if r.LeaseHeld {
				if _, exists := pages[v.PageSessionID]; !exists {
					r.LeaseHeld = false
					_ = s.save([]*record{r})
				}
			}
			if r.Finalizing || r.LeaseHeld {
				running++
				plugins[v.PluginID]++
				busy[v.PageSessionID] = true
			}
			continue
		}
		info, err := s.definition(v.PluginID, v.OperationID, v.Source)
		if r.reloadPending() {
			if err != nil || info.PluginVersion != v.PluginVersion || info.RuntimeID != r.Reload.Identity.RuntimeID {
				_ = s.endReloadLocked(r, "interrupted", "page_or_plugin_changed")
			} else if now.UnixMilli() >= r.Reload.ExpiresAt || executionTimedOut(r, now.UnixMilli()) {
				_ = s.endReloadLocked(r, "timed_out", "execution_timeout")
			}
			continue
		}
		p, ok := pages[v.PageSessionID]
		if err != nil || info.PluginVersion != v.PluginVersion || !reloadRuntimeMatches(r, info) || !ok || p.Revision != v.Revision || !matches(info, p) {
			s.finish(r, "interrupted", "page_or_plugin_changed", false)
			continue
		}
		if v.State == "queued" {
			if now.UnixMilli()-v.CreatedAt > QueueTimeout.Milliseconds() {
				s.finish(r, "timed_out", "queue_timeout", false)
			} else {
				queue = append(queue, r)
			}
			continue
		}
		if executionTimedOut(r, now.UnixMilli()) {
			s.finish(r, "timed_out", "execution_timeout", false)
			_ = s.backend.Send(*v, r.Request, "cancel")
			continue
		}
		running++
		plugins[v.PluginID]++
		busy[v.PageSessionID] = true
	}
	// Terminal is a public outcome; a still-running executor retains its lease.
	running = 0
	plugins = map[string]int{}
	busy = map[string]bool{}
	for _, r := range s.records {
		if r.View.State == "running" || r.Finalizing || r.LeaseHeld {
			running++
			plugins[r.View.PluginID]++
			busy[r.View.PageSessionID] = true
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		if queue[i].View.CreatedAt == queue[j].View.CreatedAt {
			if queue[i].View.BatchID == queue[j].View.BatchID {
				return queue[i].View.Index < queue[j].View.Index
			}
			return queue[i].View.ExecutionID < queue[j].View.ExecutionID
		}
		return queue[i].View.CreatedAt < queue[j].View.CreatedAt
	})
	for _, r := range queue {
		v := &r.View
		if running >= MaxConcurrent || plugins[v.PluginID] >= MaxPluginConcurrent || busy[v.PageSessionID] {
			continue
		}
		timeout := DefaultTimeout
		if r.Definition.TimeoutSeconds > 0 {
			timeout = time.Duration(r.Definition.TimeoutSeconds) * time.Second
		}
		v.State = "running"
		v.Certainty = "not_started"
		v.StartedAt = now.UnixMilli()
		v.UpdatedAt = v.StartedAt
		v.Deadline = now.Add(timeout).UnixMilli()
		if s.save([]*record{r}) != nil {
			return
		}
		if err := s.backend.Send(*v, r.Request, "invoke"); err != nil {
			s.finish(r, "failed", "page_delivery_failed", false)
			continue
		}
		running++
		plugins[v.PluginID]++
		busy[v.PageSessionID] = true
	}
}
func (s *Service) finish(r *record, state, code string, confirmed bool) {
	v := &r.View
	if Terminal(v.State) {
		return
	}
	if r.FinalizeCancel != nil {
		r.FinalizeCancel()
	}
	r.Reload = nil
	v.State = state
	v.ErrorCode = code
	v.UpdatedAt = time.Now().UnixMilli()
	if state == "succeeded" {
		v.Certainty = "confirmed"
	} else if v.AcceptedAt > 0 {
		v.Certainty = "unknown"
		if state == "failed" && r.Definition.SafeRetry {
			v.Certainty = "confirmed"
		}
	}
	v.CancelConfirmed = confirmed

	if v.ResultExpiresAt == 0 && len(v.ArtifactIDs) > 0 {
		v.ResultExpiresAt = time.Now().Add(s.resultTTL(r.Definition)).UnixMilli()
	}
	r.Request.Input = nil
	_ = s.save([]*record{r})
}
func (s *Service) validOwner(r *record) bool {
	if s.closing || r == nil || Terminal(r.View.State) || r.reloadPending() {
		return false
	}
	v := r.View
	now := time.Now().UnixMilli()
	if v.Deadline > 0 && executionTimedOut(r, now) {
		s.finish(r, "timed_out", "execution_timeout", false)
		return false
	}
	info, err := s.definition(v.PluginID, v.OperationID, v.Source)
	if err == nil && info.PluginVersion == v.PluginVersion && reloadRuntimeMatches(r, info) {
		for _, p := range s.backend.Sessions() {
			if p.PageSessionID == v.PageSessionID && p.Revision == v.Revision && matches(info, p) {
				return true
			}
		}
	}
	s.finish(r, "interrupted", "page_or_plugin_changed", false)
	return false
}
func (s *Service) Claim(executionID, sessionID, revision string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return err
	}
	r := s.records[executionID]
	if r == nil || r.View.PageSessionID != sessionID || r.View.Revision != revision || r.reloadPending() || !s.validOwner(r) || r.View.State != "running" || (r.View.AcceptedAt != 0 && !r.reloadClaimPending()) || r.View.CancelRequested {
		return problem("execution_unavailable", "execution is not claimable")
	}
	next := copyReloadRecord(r)
	next.LeaseHeld = true
	next.View.AcceptedAt = time.Now().UnixMilli()
	next.View.UpdatedAt = next.View.AcceptedAt
	next.View.Certainty = "unknown"
	if next.Reload != nil {
		next.Reload.ClaimDeadline = 0
	}
	return s.replaceReloadRecord(next)
}
func (s *Service) Report(ctx context.Context, sessionID, revision string, report m.OperationReport) (reportErr error) {
	s.mu.Lock()
	if err := s.available(); err != nil {
		s.mu.Unlock()
		return err
	}
	r := s.records[report.ExecutionID]
	if r == nil || r.View.PageSessionID != sessionID || r.View.Revision != revision || r.reloadPending() || r.reloadClaimPending() || !s.validOwner(r) || r.View.State != "running" || r.View.AcceptedAt == 0 {
		// A terminal acknowledgement may release a page lease, never replace a result/state.
		if r != nil && Terminal(r.View.State) && r.View.PageSessionID == sessionID && r.View.Revision == revision && r.LeaseHeld && oneOf(report.State, "succeeded", "failed", "cancelled") {
			r.LeaseHeld = false
			if report.State == "cancelled" {
				r.View.CancelConfirmed = true
			}
			err := s.save([]*record{r})
			s.mu.Unlock()
			return err
		}
		s.mu.Unlock()
		return problem("execution_unavailable", "execution is no longer owned by this page")
	}
	if report.Progress != nil && (math.IsNaN(*report.Progress) || math.IsInf(*report.Progress, 0) || *report.Progress < 0 || *report.Progress > 100) {
		s.mu.Unlock()
		return problem("invalid_report", "progress must be 0–100")
	}
	if !oneOf(report.State, "running", "succeeded", "failed", "cancelled") {
		s.mu.Unlock()
		return problem("invalid_report", "invalid execution state")
	}
	if r.Finalizing && report.State != "running" {
		s.mu.Unlock()
		return problem("execution_unavailable", "result is already being finalized")
	}
	r.View.Progress = report.Progress
	r.View.UpdatedAt = time.Now().UnixMilli()
	if report.State == "running" {
		err := s.save([]*record{r})
		s.mu.Unlock()
		return err
	}
	if r.View.CancelRequested {
		r.LeaseHeld = false
		code := "cancel_unconfirmed"
		if report.State == "cancelled" {
			code = ""
		}
		s.finish(r, "cancelled", code, report.State == "cancelled")
		err := s.available()
		s.mu.Unlock()
		return err
	}
	if report.State != "succeeded" {
		r.LeaseHeld = false
		s.finish(r, report.State, "page_execution_failed", report.State == "cancelled")
		err := s.available()
		s.mu.Unlock()
		return err
	}
	r.LeaseHeld = false
	failResult := func(code, message string) error {
		s.finish(r, "failed", code, false)
		s.mu.Unlock()
		return problem(code, message)
	}
	var value interface{}
	if len(report.Data) > MaxResultBytes || json.Unmarshal(report.Data, &value) != nil {
		return failResult("invalid_output", "invalid or oversized result")
	}
	if err := ValidateValue(r.Definition.OutputSchema, value); err != nil {
		return failResult("invalid_output", err.Error())
	}
	if err := validateItemBudget(value, r.Request.Limit); err != nil {
		return failResult("invalid_output", err.Error())
	}
	if report.Count < 0 || report.Count > r.Request.Limit || len(report.NextCursor) > 4096 || (report.HasMore && report.NextCursor == "") {
		return failResult("invalid_pagination", "invalid pagination")
	}
	// Reserve this result while releasing the global lock for bounded plugin work.
	// Cancellation, heartbeats, history and unrelated pages remain responsive.
	deadline := time.UnixMilli(r.View.Deadline)
	if bound := time.Now().Add(10 * time.Second); bound.Before(deadline) {
		deadline = bound
	}
	finalCtx, cancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
	r.Finalizing = true
	r.FinalizeCancel = cancel
	s.finalizers.Add(1)
	snapshot := r.View
	definition := r.Definition
	s.mu.Unlock()
	data, resources, downloads, artifacts, finalErr := s.backend.Finalize(finalCtx, snapshot, definition, report.Data)
	if finalErr == nil {
		finalErr = finalCtx.Err()
	}
	cancel()
	s.mu.Lock()
	defer func() {
		s.mu.Unlock()
		defer s.finalizers.Done()
		if err := s.reconcileDownloads(snapshot.PluginID, resources); err != nil && reportErr == nil {
			reportErr = err
		}
	}()
	// Cleanup skips records with in-flight finalization, including terminal records.
	r.Finalizing = false
	r.FinalizeCancel = nil
	r.View.ResourceIDs = resources
	r.View.DownloadTaskIDs = downloads
	for _, a := range artifacts {
		a.ExecutionID = r.View.ExecutionID
		a.PluginID = r.View.PluginID
		s.artifacts[a.ArtifactID] = a
		r.View.ArtifactIDs = append(r.View.ArtifactIDs, a.ArtifactID)
	}
	if len(artifacts) > 0 && r.View.ResultExpiresAt == 0 {
		r.View.ResultExpiresAt = time.Now().Add(s.resultTTL(r.Definition)).UnixMilli()
	}
	if !s.validOwner(r) {
		if err := s.save([]*record{r}); err != nil {
			return err
		}
		return problem("execution_unavailable", "execution ended while finalizing; associated effects remain queryable")
	}
	if r.View.CancelRequested {
		s.finish(r, "cancelled", "cancel_unconfirmed", false)
		return s.available()
	}
	if finalErr != nil {
		s.finish(r, "failed", "result_processing_failed", false)
		return problem("result_processing_failed", "result processing failed; inspect associated resources and downloads")
	}
	if len(data) > MaxResultBytes || json.Unmarshal(data, &value) != nil || ValidateValue(r.Definition.OutputSchema, value) != nil {
		s.finish(r, "failed", "invalid_output", false)
		return problem("invalid_output", "plugin returned an invalid result")
	}
	if err := validateItemBudget(value, r.Request.Limit); err != nil {
		s.finish(r, "failed", "invalid_output", false)
		return err
	}
	next := ""
	if report.HasMore {
		var err error
		next, err = s.createCursor(r, report.NextCursor)
		if err != nil {
			s.finish(r, "failed", "cursor_limit", false)
			return err
		}
	}
	envelope := map[string]interface{}{"data": value, "pagination": map[string]interface{}{"cursor": next, "hasMore": report.HasMore, "count": report.Count, "truncated": report.Truncated}}
	result, _ := json.Marshal(envelope)
	if len(result) > MaxResultBytes {
		s.finish(r, "failed", "output_too_large", false)
		return problem("output_too_large", "result exceeds limit")
	}
	r.View.Result = result
	r.View.ResultStatus = "available"
	r.View.ResultExpiresAt = time.Now().Add(s.resultTTL(r.Definition)).UnixMilli()
	if r.Definition.PersistResult {
		if projected := Persisted(r.Definition.OutputSchema, value); projected != nil {
			r.StoredResult, _ = json.Marshal(map[string]interface{}{"data": projected})
		}
	}
	s.finish(r, "succeeded", "", false)
	if err := s.available(); err != nil {
		return err
	}
	return s.cleanLocked(false, false)
}

func (s *Service) Get(executionID string) (m.OperationExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return m.OperationExecution{}, err
	}
	if err := s.cleanLocked(false, false); err != nil {
		return m.OperationExecution{}, err
	}
	r := s.records[executionID]
	if r == nil {
		for _, prior := range s.keys {
			if prior.Execution.ExecutionID == executionID {
				return clone(prior.Execution), nil
			}
		}
		return m.OperationExecution{}, problem("not_found", "execution not found")
	}
	return clone(r.View), nil
}
func (s *Service) Cancel(executionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancelLocked(executionID)
}
func (s *Service) cancelLocked(executionID string) error {
	if err := s.available(); err != nil {
		return err
	}
	r := s.records[executionID]
	if r == nil {
		return problem("not_found", "execution not found")
	}
	if Terminal(r.View.State) {
		return nil
	}
	if r.View.State == "queued" || r.View.AcceptedAt == 0 {
		s.finish(r, "cancelled", "", true)
		return s.available()
	}
	if !r.Definition.Cancellable {
		return problem("not_cancellable", "operation cannot be cancelled")
	}
	if r.reloadPending() || r.reloadClaimPending() {
		if err := s.endReloadLocked(r, "cancelled", "cancel_unconfirmed"); err != nil {
			return err
		}
		// The preparing page can still be awaiting its ticket response. Tell its
		// controller to stop before it stores a ticket or navigates away.
		_ = s.backend.Send(r.View, r.Request, "cancel")
		return nil
	}
	r.View.CancelRequested = true
	if r.FinalizeCancel != nil {
		r.FinalizeCancel()
	}
	if err := s.save([]*record{r}); err != nil {
		return err
	}
	_ = s.backend.Send(r.View, r.Request, "cancel")
	return nil
}
func (s *Service) CancelBatch(batch string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for _, r := range s.records {
		if r.View.BatchID == batch {
			found = true
			if !Terminal(r.View.State) && r.View.AcceptedAt > 0 && !r.Definition.Cancellable {
				return problem("not_cancellable", "batch contains a non-cancellable running item")
			}
		}
	}
	if !found {
		return problem("not_found", "batch not found")
	}
	for _, r := range s.records {
		if r.View.BatchID == batch {
			if err := s.cancelLocked(r.View.ExecutionID); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Service) batchLocked(batch string, offset, limit int) m.OperationBatch {
	items := []m.OperationExecution{}
	counts := map[string]int{}
	for _, r := range s.records {
		if r.View.BatchID == batch {
			items = append(items, clone(r.View))
			counts[r.View.State]++
		}
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.ExecutionID] = true
	}
	for _, prior := range s.keys {
		v := prior.Execution
		if v.ExecutionID != "" && v.BatchID == batch && !seen[v.ExecutionID] {
			items = append(items, clone(v))
			counts[v.State]++
			seen[v.ExecutionID] = true
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Index < items[j].Index })
	total := len(items)
	offset = min(max(offset, 0), total)
	limit = min(max(limit, 1), MaxBatch)
	return m.OperationBatch{BatchID: batch, Items: items[offset:min(offset+limit, total)], Counts: counts, Total: total, Offset: offset, HasMore: offset+limit < total}
}
func (s *Service) Batch(batch string, offset, limit int) (m.OperationBatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return m.OperationBatch{}, err
	}
	if err := s.cleanLocked(false, false); err != nil {
		return m.OperationBatch{}, err
	}
	out := s.batchLocked(batch, offset, limit)
	if out.Total == 0 {
		return out, problem("not_found", "batch not found")
	}
	return out, nil
}
func (s *Service) List(pluginID, state string, offset, limit int) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return nil, err
	}
	if err := s.cleanLocked(false, false); err != nil {
		return nil, err
	}
	items := []m.OperationExecution{}
	for _, r := range s.records {
		if (pluginID == "" || r.View.PluginID == pluginID) && (state == "" || r.View.State == state) {
			v := r.View
			v.Result = nil
			items = append(items, v)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].ExecutionID > items[j].ExecutionID
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	total := len(items)
	offset = min(max(offset, 0), total)
	limit = min(max(limit, 1), 100)
	return map[string]interface{}{"items": clone(items[offset:min(total, offset+limit)]), "total": total, "offset": offset, "hasMore": offset+limit < total}, nil
}
func (s *Service) Status() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{"available": s.available() == nil, "settings": clone(s.settings), "limits": map[string]int{"batch": MaxBatch, "pageItems": MaxItems, "inputBytes": MaxInputBytes, "resultBytes": MaxResultBytes, "active": MaxActive, "history": MaxHistory}}
}
func (s *Service) SetSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return err
	}
	if settings.HistoryDays < 1 || settings.HistoryDays > 30 || settings.ResultHours < 1 || settings.ResultHours > 24 || len(settings.Automation) > 1024 {
		return problem("invalid_settings", "retention exceeds host bounds")
	}
	for key := range settings.Automation {
		if key == "" || len(key) > 160 {
			return problem("invalid_settings", "invalid automation key")
		}
	}
	raw, _ := json.Marshal(settings)
	if err := s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket([]byte("settings")).Put([]byte("policy"), raw) }); err != nil {
		s.fault = err
		return s.available()
	}
	s.settings = clone(settings)
	s.nextCleanup = 0
	return s.cleanLocked(false, false)
}
func (s *Service) Clean(history, results bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return err
	}
	return s.cleanLocked(history, results)
}

func (s *Service) resultTTL(def m.OperationDefinition) time.Duration {
	ttl := time.Duration(s.settings.ResultHours) * time.Hour
	if def.ResultTTLSeconds > 0 && time.Duration(def.ResultTTLSeconds)*time.Second < ttl {
		ttl = time.Duration(def.ResultTTLSeconds) * time.Second
	}
	return ttl
}
