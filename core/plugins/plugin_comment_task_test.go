package plugins

import (
	"encoding/json"
	"testing"
)

func TestCommentIdentityRequiresAtomicTriple(t *testing.T) {
	p := &QqPlugin{}
	p.learnSignMapping("sign", commentIdentity{ObjectId: "oid", NonceId: "nonce", Source: "legacy"})
	if _, err := p.AddCommentTask("sign", "res"); err != errCommentIdentityUnavailable {
		t.Fatalf("incomplete identity must be rejected, got %v", err)
	}

	p.learnSignMapping("sign", commentIdentity{ObjectId: "oid", NonceId: "nonce", SessionBuffer: "sb", Source: "post_recommend"})
	p.learnSignMapping("sign", commentIdentity{NonceId: "stale", Source: "legacy_type6"})
	id := p.ResolveIdentity("sign")
	if !id.isComplete() || id.NonceId != "nonce" || id.SessionBuffer != "sb" {
		t.Fatalf("partial update corrupted atomic identity: %+v", id)
	}
}

func TestValidateCommentRequestIdentity(t *testing.T) {
	task := commentTask{ObjectId: "oid", NonceId: "nonce", SessionBuffer: "sb"}
	good, _ := json.Marshal(map[string]interface{}{
		"objectid":      "oid",
		"objectNonceId": "nonce",
		"sessionBuffer": "sb",
		"finderBasereq": map[string]interface{}{
			"objectBaseInfos": []interface{}{map[string]interface{}{"sessionBuffer": "sb"}},
		},
	})
	if code := validateCommentRequest(task, string(good)); code != "" {
		t.Fatalf("valid request rejected: %s", code)
	}

	missing, _ := json.Marshal(map[string]interface{}{"objectid": "oid", "objectNonceId": "nonce"})
	if code := validateCommentRequest(task, string(missing)); code != "identity_fields_missing" {
		t.Fatalf("missing sessionBuffer should fail preflight, got %s", code)
	}

	mismatch, _ := json.Marshal(map[string]interface{}{
		"objectid": "oid", "objectNonceId": "nonce", "sessionBuffer": "other",
		"finderBasereq": map[string]interface{}{"objectBaseInfos": []interface{}{map[string]interface{}{"sessionBuffer": "other"}}},
	})
	if code := validateCommentRequest(task, string(mismatch)); code != "target_mismatch" {
		t.Fatalf("mismatched sessionBuffer should be blocked, got %s", code)
	}
}

func TestCommentTaskNoCommentsLifecycle(t *testing.T) {
	bridge := newMockBridge()
	p := &QqPlugin{}
	p.SetBridge(&bridge.Bridge)
	p.learnSignMapping("sign", commentIdentity{ObjectId: "oid", NonceId: "nonce", SessionBuffer: "sb", Source: "post_recommend"})
	task, err := p.AddCommentTask("sign", "res")
	if err != nil {
		t.Fatal(err)
	}
	p.popCommentTasks()
	arg, _ := json.Marshal(map[string]interface{}{
		"objectid": "oid", "objectNonceId": "nonce", "sessionBuffer": "sb",
		"finderBasereq": map[string]interface{}{"objectBaseInfos": []interface{}{map[string]interface{}{"sessionBuffer": "sb"}}},
	})
	payload, _ := json.Marshal(map[string]interface{}{
		"name": "FinderGetCommentList", "requestId": task.RequestId, "resId": task.ResId,
		"expectedUrlSign": "sign", "arg": string(arg),
		"data": map[string]interface{}{"commentInfo": []interface{}{}},
	})
	p.handleComments(payload)

	var result map[string]interface{}
	for _, event := range bridge.sent {
		if event.Type == "newComments" {
			result = event.Data.(map[string]interface{})
		}
	}
	if result == nil || result["status"] != "no_comments" || result["targetVerified"] != true {
		t.Fatalf("no-comments result not emitted correctly: %+v", result)
	}
	if _, ok := p.activeTask(task.RequestId, task.ResId); ok {
		t.Fatal("completed task should be removed from active set")
	}
}

func TestCommentTaskBlocksTargetMismatch(t *testing.T) {
	bridge := newMockBridge()
	p := &QqPlugin{}
	p.SetBridge(&bridge.Bridge)
	p.learnSignMapping("sign", commentIdentity{ObjectId: "oid", NonceId: "nonce", SessionBuffer: "sb", Source: "post_recommend"})
	task, err := p.AddCommentTask("sign", "res")
	if err != nil {
		t.Fatal(err)
	}
	p.popCommentTasks()
	arg, _ := json.Marshal(map[string]interface{}{
		"objectid": "oid", "objectNonceId": "nonce", "sessionBuffer": "wrong",
		"finderBasereq": map[string]interface{}{"objectBaseInfos": []interface{}{map[string]interface{}{"sessionBuffer": "wrong"}}},
	})
	payload, _ := json.Marshal(map[string]interface{}{
		"requestId": task.RequestId, "resId": task.ResId, "expectedUrlSign": "sign",
		"arg": string(arg), "data": map[string]interface{}{"commentInfo": []interface{}{}},
	})
	p.handleComments(payload)

	for _, event := range bridge.sent {
		if event.Type == "newComments" {
			t.Fatalf("mismatched result must not reach UI: %+v", event)
		}
	}
	foundFailure := false
	for _, event := range bridge.sent {
		if event.Type != "commentTaskStatus" {
			continue
		}
		status := event.Data.(commentTaskStatus)
		if status.Code == "target_mismatch" {
			foundFailure = true
		}
	}
	if !foundFailure {
		t.Fatal("target mismatch status was not emitted")
	}
}
