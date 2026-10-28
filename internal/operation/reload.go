package operation

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"slices"
	"time"

	m "res-downloader/internal/model"
)

const reloadTicketTTL = 60 * time.Second

// ErrReloadPageConnected leaves the ticket untouched. The bridge can wait for
// the old document's asynchronous SSE close within the same resume request.
var ErrReloadPageConnected = errors.New("previous page is still connected")

// Reload state is deliberately memory-only. Restart recovery still interrupts
// every active execution; neither a ticket nor its binding can revive it.
type reloadState struct {
	TokenHash     [sha256.Size]byte
	Identity      m.OperationReloadIdentity
	ExpiresAt     int64
	Pending       bool
	ClaimDeadline int64
}

type reloadIdentityProvider interface {
	ReloadIdentity(string) (m.OperationReloadIdentity, bool)
}

func (r *record) reloadPending() bool { return r != nil && r.Reload != nil && r.Reload.Pending }
func (r *record) reloadClaimPending() bool {
	return r != nil && r.Reload != nil && r.Reload.ClaimDeadline > 0
}

func reloadRuntimeMatches(r *record, info m.OperationInfo) bool {
	return r.Reload == nil || (info.RuntimeID != "" && info.RuntimeID == r.Reload.Identity.RuntimeID)
}

func executionTimedOut(r *record, now int64) bool {
	v := r.View
	if now > v.Deadline {
		return true
	}
	if r.reloadPending() {
		return now >= r.Reload.ExpiresAt
	}
	if r.reloadClaimPending() {
		return now >= r.Reload.ClaimDeadline
	}
	return (v.AcceptedAt == 0 && now-v.StartedAt > AcceptTimeout.Milliseconds()) ||
		(v.AcceptedAt != 0 && now-v.UpdatedAt > HeartbeatTimeout.Milliseconds())
}

func copyReloadRecord(r *record) *record {
	next := *r
	next.View = clone(r.View)
	if r.Reload != nil {
		reload := *r.Reload
		next.Reload = &reload
	}
	return &next
}

func (s *Service) replaceReloadRecord(next *record) error {
	if err := s.save([]*record{next}); err != nil {
		return err
	}
	s.records[next.View.ExecutionID] = next
	return nil
}

func (s *Service) reloadPage(r *record, sessionID, revision string, info m.OperationInfo) (m.OperationReloadIdentity, bool) {
	provider, ok := s.backend.(reloadIdentityProvider)
	if !ok || info.RuntimeID == "" {
		return m.OperationReloadIdentity{}, false
	}
	for _, page := range s.backend.Sessions() {
		if page.PageSessionID != sessionID || page.Revision != revision || !matches(info, page) {
			continue
		}
		identity, exists := provider.ReloadIdentity(sessionID)
		if exists && identity.PluginID == r.View.PluginID && identity.ScriptID == r.Definition.PageScript &&
			identity.RuntimeID == info.RuntimeID && identity.PageURL != "" &&
			identity.PageURL == page.PageURL && identity.Login == page.Login {
			return identity, true
		}
	}
	return m.OperationReloadIdentity{}, false
}

// PrepareReload relinquishes normal page reporting for one bounded reload. The
// caller must already have stopped its handler and drained unpublished effects.
func (s *Service) PrepareReload(executionID, sessionID, revision string) (m.OperationReloadTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return m.OperationReloadTicket{}, err
	}
	r := s.records[executionID]
	if s.closing || r == nil || r.View.State != "running" || r.View.PageSessionID != sessionID || r.View.Revision != revision ||
		r.View.AcceptedAt == 0 || !r.LeaseHeld || r.View.CancelRequested || r.Finalizing || r.View.ReloadCount != 0 ||
		!r.Definition.AllowReload || !slices.Contains(r.Definition.Effects, "page") ||
		len(r.View.ResourceIDs)+len(r.View.DownloadTaskIDs)+len(r.View.ArtifactIDs)+len(r.View.Result) != 0 {
		return m.OperationReloadTicket{}, problem("execution_unavailable", "execution cannot reload")
	}
	info, err := s.definition(r.View.PluginID, r.View.OperationID, r.View.Source)
	if err != nil || info.PluginVersion != r.View.PluginVersion || !info.Definition.AllowReload || !slices.Contains(info.Definition.Effects, "page") {
		return m.OperationReloadTicket{}, problem("execution_unavailable", "execution cannot reload")
	}
	if !s.validOwner(r) {
		if err := s.available(); err != nil {
			return m.OperationReloadTicket{}, err
		}
		return m.OperationReloadTicket{}, problem("execution_unavailable", "execution cannot reload")
	}
	identity, ok := s.reloadPage(r, sessionID, revision, info)
	if !ok {
		return m.OperationReloadTicket{}, problem("execution_unavailable", "page cannot reload")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return m.OperationReloadTicket{}, err
	}
	token := hex.EncodeToString(secret)
	expires := min(time.Now().Add(reloadTicketTTL).UnixMilli(), r.View.Deadline)
	next := copyReloadRecord(r)
	next.View.ReloadCount = 1
	next.View.UpdatedAt = time.Now().UnixMilli()
	next.Reload = &reloadState{TokenHash: sha256.Sum256([]byte(token)), Identity: identity, ExpiresAt: expires, Pending: true}
	if err := s.replaceReloadRecord(next); err != nil {
		return m.OperationReloadTicket{}, err
	}
	return m.OperationReloadTicket{Token: token, ExpiresAt: expires}, nil
}

func reloadTokenMatches(r *record, token string) bool {
	if !r.reloadPending() || len(token) != 64 {
		return false
	}
	digest := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(digest[:], r.Reload.TokenHash[:]) == 1
}

// ResumeReload transfers the existing execution, never its deadline or input,
// to a fresh connected page. A separately bounded ordinary Claim is mandatory.
func (s *Service) ResumeReload(executionID, token, sessionID, revision string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return err
	}
	r := s.records[executionID]
	if s.closing || !reloadTokenMatches(r, token) || r.View.State != "running" || r.Finalizing || r.View.CancelRequested ||
		r.View.PageSessionID == sessionID {
		return problem("execution_unavailable", "reload is unavailable")
	}
	now := time.Now().UnixMilli()
	if executionTimedOut(r, now) {
		if err := s.endReloadLocked(r, "timed_out", "execution_timeout"); err != nil {
			return err
		}
		return problem("execution_unavailable", "reload expired")
	}
	info, err := s.definition(r.View.PluginID, r.View.OperationID, r.View.Source)
	if err != nil || info.PluginVersion != r.View.PluginVersion || !reloadRuntimeMatches(r, info) || !info.Definition.AllowReload {
		if err := s.endReloadLocked(r, "interrupted", "page_or_plugin_changed"); err != nil {
			return err
		}
		return problem("execution_unavailable", "plugin changed during reload")
	}
	identity, ok := s.reloadPage(r, sessionID, revision, info)
	if !ok || identity != r.Reload.Identity {
		return problem("execution_unavailable", "reload page does not match")
	}
	for _, page := range s.backend.Sessions() {
		if page.PageSessionID == r.View.PageSessionID && page.Connected {
			return ErrReloadPageConnected
		}
	}
	for _, other := range s.records {
		if other != r && other.View.PageSessionID == sessionID &&
			(other.View.State == "running" || other.LeaseHeld || other.Finalizing) {
			return problem("execution_unavailable", "reload page is busy")
		}
	}
	next := copyReloadRecord(r)
	next.View.PageSessionID = sessionID
	next.View.Revision = revision
	next.View.UpdatedAt = now
	next.Request.PageSessionID = sessionID
	next.LeaseHeld = false
	next.Reload.Pending = false
	next.Reload.TokenHash = [sha256.Size]byte{}
	next.Reload.ClaimDeadline = min(now+AcceptTimeout.Milliseconds(), next.View.Deadline)
	if err := s.replaceReloadRecord(next); err != nil {
		return err
	}
	if err := s.backend.Send(next.View, next.Request, "invoke"); err != nil {
		if err := s.endReloadLocked(next, "failed", "page_delivery_failed"); err != nil {
			return err
		}
		return problem("execution_unavailable", "reload could not be delivered")
	}
	return nil
}

// endReloadLocked keeps an old live executor's lease until it disappears or
// acknowledges cancellation. The ticket itself is invalidated immediately.
func (s *Service) endReloadLocked(r *record, state, code string) error {
	next := copyReloadRecord(r)
	if next.LeaseHeld {
		connected := false
		for _, page := range s.backend.Sessions() {
			if page.PageSessionID == next.View.PageSessionID && page.Connected {
				connected = true
				break
			}
		}
		if !connected {
			next.LeaseHeld = false
		}
	}
	if state == "cancelled" {
		next.View.CancelRequested = true
	}
	s.finish(next, state, code, false)
	if err := s.available(); err != nil {
		return err
	}
	s.records[next.View.ExecutionID] = next
	return nil
}

// AbandonReload acknowledges that the bridge-authenticated owner has stopped.
// An empty token also covers an unknown/lost PrepareReload response, including
// an abort reaching the host before preparation. It never resumes execution.
func (s *Service) AbandonReload(executionID, token, sessionID, revision string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return err
	}
	r := s.records[executionID]
	if s.closing || r == nil || r.View.PageSessionID != sessionID || r.View.Revision != revision {
		return problem("execution_unavailable", "reload is unavailable")
	}
	if Terminal(r.View.State) {
		return nil
	}
	if (token != "" && !reloadTokenMatches(r, token)) || r.Finalizing || r.View.State != "running" ||
		r.View.AcceptedAt == 0 || !r.LeaseHeld || r.reloadClaimPending() ||
		len(r.View.ResourceIDs)+len(r.View.DownloadTaskIDs)+len(r.View.ArtifactIDs)+len(r.View.Result) != 0 {
		return problem("execution_unavailable", "reload is unavailable")
	}
	next := copyReloadRecord(r)
	next.LeaseHeld = false
	if next.View.CancelRequested {
		return s.endReloadLocked(next, "cancelled", "cancel_unconfirmed")
	}
	return s.endReloadLocked(next, "failed", "page_execution_failed")
}
