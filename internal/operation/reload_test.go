package operation

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
	m "res-downloader/internal/model"
)

type reloadTestBackend struct {
	*testBackend
	runtime    string
	identities map[string]m.OperationReloadIdentity
	sends      []m.OperationExecution
	cancels    []m.OperationExecution
	sendErr    error
}

func (b *reloadTestBackend) Operations() []m.OperationInfo {
	definition := testDefinition()
	definition.AllowReload = true
	definition.Effects = []string{"read", "page"}
	return []m.OperationInfo{{PluginID: "test.plugin", PluginVersion: "1", OperationID: "read", RuntimeID: b.runtime, Definition: definition}}
}
func (b *reloadTestBackend) ReloadIdentity(id string) (m.OperationReloadIdentity, bool) {
	identity, ok := b.identities[id]
	return identity, ok
}
func (b *reloadTestBackend) Send(v m.OperationExecution, _ m.OperationRequest, kind string) error {
	if b.sendErr != nil {
		return b.sendErr
	}
	if kind == "invoke" {
		b.sends = append(b.sends, clone(v))
	} else if kind == "cancel" {
		b.cancels = append(b.cancels, clone(v))
	}
	return nil
}

func reloadService(t *testing.T) (*Service, *reloadTestBackend, m.OperationExecution) {
	t.Helper()
	s, base := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	base.pages[0].PageURL = "https://example.test/watch?v=private-context"
	identity := m.OperationReloadIdentity{PluginID: "test.plugin", ScriptID: "page", PageURL: base.pages[0].PageURL, Login: "unknown", Context: "private-context", RuntimeID: "runtime1"}
	b := &reloadTestBackend{testBackend: base, runtime: identity.RuntimeID, identities: map[string]m.OperationReloadIdentity{"page1": identity}}
	s.backend = b
	v := startExecution(t, s, "reload-once")
	return s, b, v
}

func prepareReload(t *testing.T, s *Service, id string) m.OperationReloadTicket {
	t.Helper()
	ticket, err := s.PrepareReload(id, "page1", "revision1")
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}

func (b *reloadTestBackend) newPage() {
	p := b.pages[0]
	p.PageSessionID, p.Revision = "page2", "revision2"
	b.pages = []m.OperationSession{p}
	b.identities["page2"] = b.identities["page1"]
}

func TestReloadTransfersSameExecutionAndRequiresFreshClaim(t *testing.T) {
	s, b, v := reloadService(t)
	r := s.records[v.ExecutionID]
	// A long-running operation must not reuse its original start for the new
	// page's acceptance deadline.
	r.View.StartedAt = time.Now().Add(-2 * AcceptTimeout).UnixMilli()
	started, deadline := r.View.StartedAt, r.View.Deadline
	ticket := prepareReload(t, s, v.ExecutionID)
	if len(ticket.Token) != 64 || ticket.ExpiresAt > deadline || s.records[v.ExecutionID].View.ReloadCount != 1 {
		t.Fatal("invalid reload ticket or allowance")
	}
	if _, err := s.PrepareReload(v.ExecutionID, "page1", "revision1"); err == nil {
		t.Fatal("second reload ticket issued")
	}
	b.newPage()
	s.tickLocked()
	if s.records[v.ExecutionID].View.State != "running" {
		t.Fatal("reload gap interrupted the execution")
	}
	if err := s.Report(context.Background(), "page1", "revision1", m.OperationReport{ExecutionID: v.ExecutionID, State: "failed"}); err == nil {
		t.Fatal("old page report accepted during reload")
	}
	if err := s.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2"); err != nil {
		t.Fatal(err)
	}
	next := s.records[v.ExecutionID]
	if next.View.StartedAt != started || next.View.Deadline != deadline || next.Request.Input["query"] != "private search" || len(b.sends) != 2 || b.sends[1].ExecutionID != v.ExecutionID || b.sends[1].ReloadCount != 1 {
		t.Fatal("reload replaced the execution, deadline, or input")
	}
	if err := s.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2"); err == nil {
		t.Fatal("consumed ticket reused")
	}
	if err := s.Report(context.Background(), "page2", "revision2", m.OperationReport{ExecutionID: v.ExecutionID, State: "running"}); err == nil {
		t.Fatal("new page reported before claiming")
	}
	// A stale owner's report must not call validOwner and terminate a new owner
	// even if the new page is temporarily absent from discovery.
	pages := b.pages
	b.pages = nil
	if err := s.Report(context.Background(), "page1", "revision1", m.OperationReport{ExecutionID: v.ExecutionID, State: "failed"}); err == nil || next.View.State != "running" {
		t.Fatal("old page disturbed the transferred execution")
	}
	b.pages = pages
	s.tickLocked()
	if err := s.Claim(v.ExecutionID, "page2", "revision2"); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(v.ExecutionID, "page2", "revision2"); err == nil {
		t.Fatal("new page claimed twice")
	}
	if _, err := s.PrepareReload(v.ExecutionID, "page2", "revision2"); err == nil {
		t.Fatal("reload allowance reset after transfer")
	}
	if err := s.Report(context.Background(), "page2", "revision2", m.OperationReport{ExecutionID: v.ExecutionID, State: "succeeded", Data: json.RawMessage(`{"title":"done"}`)}); err != nil {
		t.Fatal(err)
	}
	if s.records[v.ExecutionID].View.State != "succeeded" || len(s.records) != 1 {
		t.Fatal("resumed execution did not finish exactly once")
	}
}

func TestReloadRejectsWrongBindingAndLiveOldPage(t *testing.T) {
	for _, field := range []string{"plugin", "script", "url", "login", "context", "runtime", "disconnected", "not-ready", "old-connected", "busy", "revision", "token"} {
		t.Run(field, func(t *testing.T) {
			s, b, v := reloadService(t)
			ticket := prepareReload(t, s, v.ExecutionID)
			oldPage := b.pages[0]
			b.newPage()
			identity := b.identities["page2"]
			revision, token := "revision2", ticket.Token
			switch field {
			case "plugin":
				identity.PluginID = "other.plugin"
			case "script":
				identity.ScriptID = "other-page"
			case "url":
				identity.PageURL = "https://other.test/watch"
				b.pages[0].PageURL = identity.PageURL
			case "login":
				identity.Login = "authenticated"
				b.pages[0].Login = identity.Login
			case "context":
				identity.Context = "other-content"
			case "runtime":
				identity.RuntimeID = "runtime2"
			case "disconnected":
				b.pages[0].Connected = false
			case "not-ready":
				b.pages[0].Ready = false
			case "old-connected":
				b.pages = append(b.pages, oldPage)
			case "busy":
				other := copyReloadRecord(s.records[v.ExecutionID])
				other.View.ExecutionID = "busy"
				other.View.PageSessionID = "page2"
				s.records["busy"] = other
			case "revision":
				revision = "wrong"
			case "token":
				token = strings.Repeat("0", 64)
			}
			b.identities["page2"] = identity
			if err := s.ResumeReload(v.ExecutionID, token, "page2", revision); err == nil {
				t.Fatal("invalid reload accepted")
			}
			if !s.records[v.ExecutionID].reloadPending() || len(b.sends) != 1 {
				t.Fatal("rejection consumed ticket or dispatched work")
			}
		})
	}
}

func TestReloadPreparationRejectsUnsafeStages(t *testing.T) {
	for _, stage := range []string{"unclaimed", "finalizing", "cancelled", "not-enabled", "not-page", "resource", "download", "artifact", "result"} {
		t.Run(stage, func(t *testing.T) {
			s, _, v := reloadService(t)
			r := s.records[v.ExecutionID]
			switch stage {
			case "unclaimed":
				r.View.AcceptedAt = 0
			case "finalizing":
				r.Finalizing = true
			case "cancelled":
				r.View.CancelRequested = true
			case "not-enabled":
				r.Definition.AllowReload = false
			case "not-page":
				r.Definition.Effects = []string{"read"}
			case "resource":
				r.View.ResourceIDs = []string{"resource"}
			case "download":
				r.View.DownloadTaskIDs = []string{"download"}
			case "artifact":
				r.View.ArtifactIDs = []string{"artifact"}
			case "result":
				r.View.Result = json.RawMessage(`{}`)
			}
			if _, err := s.PrepareReload(v.ExecutionID, "page1", "revision1"); err == nil || r.Reload != nil || r.View.ReloadCount != 0 {
				t.Fatal("unsafe stage changed reload state")
			}
		})
	}
}

func TestReloadCancellationExpiryAndRuntimeChange(t *testing.T) {
	for _, reason := range []string{"cancel", "ticket-expired", "execution-expired", "runtime-changed", "claim-expired"} {
		t.Run(reason, func(t *testing.T) {
			s, b, v := reloadService(t)
			ticket := prepareReload(t, s, v.ExecutionID)
			if reason == "cancel" {
				if err := s.Cancel(v.ExecutionID); err != nil {
					t.Fatal(err)
				}
				if !s.records[v.ExecutionID].LeaseHeld || s.records[v.ExecutionID].View.State != "cancelled" {
					t.Fatal("cancellation failed to terminate or retain old live lease")
				}
				if len(b.cancels) != 1 || b.cancels[0].PageSessionID != "page1" || b.cancels[0].ExecutionID != v.ExecutionID {
					t.Fatal("cancellation did not notify the preparing page")
				}
			} else {
				b.newPage()
				r := s.records[v.ExecutionID]
				switch reason {
				case "ticket-expired":
					r.Reload.ExpiresAt = time.Now().Add(-time.Second).UnixMilli()
				case "execution-expired":
					r.View.Deadline = time.Now().Add(-time.Second).UnixMilli()
				case "runtime-changed":
					b.runtime = "runtime2"
				case "claim-expired":
					if err := s.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2"); err != nil {
						t.Fatal(err)
					}
					s.records[v.ExecutionID].Reload.ClaimDeadline = time.Now().Add(-time.Second).UnixMilli()
				}
				s.tickLocked()
			}
			if r := s.records[v.ExecutionID]; !Terminal(r.View.State) || r.Reload != nil {
				t.Fatal("terminal execution retained a reload ticket")
			}
			if err := s.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2"); err == nil {
				t.Fatal("terminal execution revived")
			}
		})
	}
}

func TestReloadTicketIsMemoryOnlyAndRestartCannotReplay(t *testing.T) {
	s, b, v := reloadService(t)
	ticket := prepareReload(t, s, v.ExecutionID)
	if err := s.db.View(func(tx *bolt.Tx) error {
		raw := string(tx.Bucket([]byte("executions")).Get([]byte(v.ExecutionID)))
		if strings.Contains(raw, ticket.Token) || strings.Contains(raw, "TokenHash") || strings.Contains(raw, "private-context") || !strings.Contains(raw, `"reloadCount":1`) {
			t.Fatal("ticket or page binding persisted, or reload count missing")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	restarted := &Service{db: s.db, backend: b, records: map[string]*record{}, keys: map[string]keyRecord{}, artifacts: map[string]m.OperationArtifact{}}
	if err := restarted.load(); err != nil {
		t.Fatal(err)
	}
	b.newPage()
	if err := restarted.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2"); err == nil {
		t.Fatal("restart restored a reload ticket")
	}
	if r := restarted.records[v.ExecutionID]; r.View.State != "interrupted" || r.View.ErrorCode != "host_restarted" || r.View.ReloadCount != 1 {
		t.Fatal("restart changed reload recovery semantics")
	}
}

func TestReloadPersistenceFailureDoesNotPublishTransition(t *testing.T) {
	for _, step := range []string{"prepare", "resume", "claim", "abandon", "abandon-before-prepare", "cancel"} {
		t.Run(step, func(t *testing.T) {
			s, b, v := reloadService(t)
			var ticket m.OperationReloadTicket
			if step != "prepare" && step != "abandon-before-prepare" {
				ticket = prepareReload(t, s, v.ExecutionID)
			}
			if step == "resume" || step == "claim" {
				b.newPage()
			}
			if step == "claim" {
				if err := s.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2"); err != nil {
					t.Fatal(err)
				}
			}
			before := s.records[v.ExecutionID]
			beforeRaw, _ := json.Marshal(before)
			beforeReload := before.Reload
			sends := len(b.sends)
			if err := s.db.Close(); err != nil {
				t.Fatal(err)
			}
			var err error
			switch step {
			case "prepare":
				_, err = s.PrepareReload(v.ExecutionID, "page1", "revision1")
			case "resume":
				err = s.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2")
			case "claim":
				err = s.Claim(v.ExecutionID, "page2", "revision2")
			case "abandon":
				err = s.AbandonReload(v.ExecutionID, ticket.Token, "page1", "revision1")
			case "abandon-before-prepare":
				err = s.AbandonReload(v.ExecutionID, "", "page1", "revision1")
			case "cancel":
				err = s.Cancel(v.ExecutionID)
			}
			afterRaw, _ := json.Marshal(s.records[v.ExecutionID])
			if err == nil || s.fault == nil || s.records[v.ExecutionID] != before || string(beforeRaw) != string(afterRaw) || before.Reload != beforeReload || len(b.sends) != sends || len(b.cancels) != 0 {
				t.Fatal("failed persistence exposed a partial reload transition")
			}
		})
	}
}

func TestReloadAbandonAndDeliveryFailureCannotRetry(t *testing.T) {
	for _, abandon := range []bool{false, true} {
		t.Run(map[bool]string{true: "abandon", false: "delivery-failure"}[abandon], func(t *testing.T) {
			s, b, v := reloadService(t)
			ticket := prepareReload(t, s, v.ExecutionID)
			if abandon {
				if err := s.AbandonReload(v.ExecutionID, strings.Repeat("0", 64), "page1", "revision1"); err == nil {
					t.Fatal("unauthenticated abandon accepted")
				}
				if err := s.AbandonReload(v.ExecutionID, ticket.Token, "page1", "revision1"); err != nil {
					t.Fatal(err)
				}
			} else {
				b.newPage()
				b.sendErr = errors.New("queue full")
				if err := s.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2"); err == nil {
					t.Fatal("delivery failure hidden")
				}
			}
			if r := s.records[v.ExecutionID]; r.View.State != "failed" || r.Reload != nil || r.LeaseHeld {
				t.Fatal("failed reload retained ticket or lease")
			}
			if err := s.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2"); err == nil {
				t.Fatal("failed reload retried")
			}
		})
	}
}

func TestReloadUnknownTicketAbortClosesBothPreparationOrderings(t *testing.T) {
	for _, preparedFirst := range []bool{false, true} {
		t.Run(map[bool]string{true: "response-lost", false: "abort-before-prepare"}[preparedFirst], func(t *testing.T) {
			s, _, v := reloadService(t)
			if preparedFirst {
				prepareReload(t, s, v.ExecutionID)
			}
			if err := s.AbandonReload(v.ExecutionID, "", "other-page", "revision1"); err == nil {
				t.Fatal("another page abandoned the execution")
			}
			if err := s.AbandonReload(v.ExecutionID, "", "page1", "wrong-revision"); err == nil {
				t.Fatal("stale revision abandoned the execution")
			}
			if err := s.AbandonReload(v.ExecutionID, "", "page1", "revision1"); err != nil {
				t.Fatal(err)
			}
			r := s.records[v.ExecutionID]
			if r.View.State != "failed" || r.LeaseHeld || r.Reload != nil {
				t.Fatal("stopped owner retained a ticket or busy lease")
			}
			if _, err := s.PrepareReload(v.ExecutionID, "page1", "revision1"); err == nil {
				t.Fatal("late preparation revived a stopped owner")
			}
			if err := s.AbandonReload(v.ExecutionID, "", "page1", "revision1"); err != nil || r.View.State != "failed" {
				t.Fatal("duplicate stop acknowledgement changed the result")
			}
		})
	}
}

func TestReloadCancellationBeforeFreshClaimNotifiesNewPage(t *testing.T) {
	s, b, v := reloadService(t)
	ticket := prepareReload(t, s, v.ExecutionID)
	b.newPage()
	if err := s.ResumeReload(v.ExecutionID, ticket.Token, "page2", "revision2"); err != nil {
		t.Fatal(err)
	}
	if err := s.AbandonReload(v.ExecutionID, "", "page2", "revision2"); err == nil {
		t.Fatal("unclaimed new page abandoned the execution")
	}
	if err := s.Cancel(v.ExecutionID); err != nil {
		t.Fatal(err)
	}
	if len(b.cancels) != 1 || b.cancels[0].PageSessionID != "page2" {
		t.Fatal("new page did not receive cancellation")
	}
	if err := s.Claim(v.ExecutionID, "page2", "revision2"); err == nil {
		t.Fatal("cancelled transfer was claimed")
	}
}

func TestReloadAbandonPreservesCancellationBeforePreparation(t *testing.T) {
	s, _, v := reloadService(t)
	if err := s.Cancel(v.ExecutionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareReload(v.ExecutionID, "page1", "revision1"); err == nil {
		t.Fatal("cancelled owner prepared a reload")
	}
	if err := s.AbandonReload(v.ExecutionID, "", "page1", "revision1"); err != nil {
		t.Fatal(err)
	}
	if r := s.records[v.ExecutionID]; r.View.State != "cancelled" || r.LeaseHeld || r.Reload != nil {
		t.Fatal("stop acknowledgement overwrote cancellation or retained a lease")
	}
}
