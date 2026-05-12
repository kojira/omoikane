package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestLibrarianInstanceAPI(t *testing.T) {
	base, tok, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)

	// Register
	s, raw := doJSON(t, http.MethodPost, base+"/v1/librarian/instances", tok,
		map[string]any{"role": "coordinator", "agent_runtime": "stub"}, nil)
	if s != 201 {
		t.Fatalf("register: %d %s", s, raw)
	}
	var out struct {
		InstanceID string `json:"instance_id"`
	}
	_ = json.Unmarshal(raw, &out)

	// Heartbeat
	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/instances/"+out.InstanceID+"/heartbeat", tok, nil, nil)
	if s != 204 {
		t.Fatalf("heartbeat: %d", s)
	}
	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/instances/missing/heartbeat", tok, nil, nil)
	if s != 404 {
		t.Fatalf("heartbeat-missing: %d", s)
	}

	// Patch status
	s, _ = doJSON(t, http.MethodPatch,
		base+"/v1/librarian/instances/"+out.InstanceID, tok,
		map[string]any{"status": "ACTIVE"}, nil)
	if s != 204 {
		t.Fatalf("patch: %d", s)
	}
	s, _ = doJSON(t, http.MethodPatch,
		base+"/v1/librarian/instances/"+out.InstanceID, tok,
		map[string]any{"status": "JUNK"}, nil)
	if s != 400 {
		t.Fatalf("bad-status: %d", s)
	}

	// List
	s, _ = doJSON(t, http.MethodGet, base+"/v1/librarian/instances?role=coordinator", tok, nil, nil)
	if s != 200 {
		t.Fatalf("list: %d", s)
	}
}

func TestLibrarianRegisterValidation(t *testing.T) {
	base, tok, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)

	s, _ := doJSON(t, http.MethodPost, base+"/v1/librarian/instances", tok,
		map[string]any{"role": "wizard"}, nil)
	if s != 400 {
		t.Fatalf("bad role: %d", s)
	}
	if got := postRaw(t, http.MethodPost, base+"/v1/librarian/instances", tok, "{"); got != 400 {
		t.Fatalf("bad json: %d", got)
	}
}

func TestLibrarianChat(t *testing.T) {
	base, tok, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)

	// Open thread
	s, raw := doJSON(t, http.MethodPost, base+"/v1/librarian/threads", tok,
		map[string]any{"title": "t", "intent": "observation"}, nil)
	if s != 201 {
		t.Fatalf("open: %d %s", s, raw)
	}
	var thr struct {
		ThreadID string `json:"thread_id"`
	}
	_ = json.Unmarshal(raw, &thr)

	// Post message
	s, _ = doJSON(t, http.MethodPost, base+"/v1/librarian/chat", tok,
		map[string]any{
			"thread_id": thr.ThreadID, "author_role": "coordinator",
			"intent": "observation", "content": "hello",
		}, nil)
	if s != 201 {
		t.Fatalf("post: %d", s)
	}

	// List messages
	s, _ = doJSON(t, http.MethodGet,
		base+"/v1/librarian/threads/"+thr.ThreadID+"/messages", tok, nil, nil)
	if s != 200 {
		t.Fatalf("list-msgs: %d", s)
	}

	// List threads (OPEN)
	s, _ = doJSON(t, http.MethodGet, base+"/v1/librarian/threads?status=OPEN", tok, nil, nil)
	if s != 200 {
		t.Fatalf("list-threads: %d", s)
	}

	// Close
	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/threads/"+thr.ThreadID+"/close", tok,
		map[string]any{"summary": "wrap"}, nil)
	if s != 204 {
		t.Fatalf("close: %d", s)
	}
	// Close with empty body
	s2, raw2 := doJSON(t, http.MethodPost, base+"/v1/librarian/threads", tok,
		map[string]any{"title": "t2"}, nil)
	if s2 != 201 {
		t.Fatalf("open2: %d %s", s2, raw2)
	}
	var thr2 struct {
		ThreadID string `json:"thread_id"`
	}
	_ = json.Unmarshal(raw2, &thr2)
	req, _ := http.NewRequest(http.MethodPost,
		base+"/v1/librarian/threads/"+thr2.ThreadID+"/close", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("close-empty: %d", resp.StatusCode)
	}
}

// chatPost must derive author_user_id from the auth token, never from
// the request body. Migration 012 adds this column and the handler
// fills it server-side; this test locks both halves:
//   1) the field is populated for new messages
//   2) a client-supplied author_user_id is ignored (no impersonation)
func TestLibrarianChatAuthorUserIDFromAuthContext(t *testing.T) {
	base, tok, st := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)

	// Open thread.
	s, raw := doJSON(t, http.MethodPost, base+"/v1/librarian/threads", tok,
		map[string]any{"title": "authorship test"}, nil)
	if s != 201 {
		t.Fatalf("open: %d %s", s, raw)
	}
	var thr struct {
		ThreadID string `json:"thread_id"`
	}
	_ = json.Unmarshal(raw, &thr)

	// Post a message AND try to spoof the author by sending a different
	// author_user_id in the body. The server must ignore that field.
	s, _ = doJSON(t, http.MethodPost, base+"/v1/librarian/chat", tok,
		map[string]any{
			"thread_id":      thr.ThreadID,
			"author_role":    "coordinator",
			"content":        "this should be attributed to me",
			"author_user_id": "u-impersonated-id", // attempted spoof — ignored
		}, nil)
	if s != 201 {
		t.Fatalf("post: %d", s)
	}

	// Verify directly via the store (cleanest source of truth — no
	// dependency on the messages API surfacing the field yet).
	msgs, err := st.ListChatMessages(context.Background(), thr.ThreadID, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("want 1 message, got %d", len(msgs))
	}
	if msgs[0].AuthorUserID != "admin" {
		t.Errorf("author_user_id not bound to bearer-token user: got %q, want %q",
			msgs[0].AuthorUserID, "admin")
	}
	if msgs[0].AuthorUserID == "u-impersonated-id" {
		t.Fatal("CRITICAL: client spoof was honoured — impersonation possible")
	}
}

func TestLibrarianChatValidation(t *testing.T) {
	base, tok, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)

	for _, p := range []string{
		"/v1/librarian/instances",
		"/v1/librarian/threads",
		"/v1/librarian/chat",
		"/v1/librarian/tasks",
		"/v1/librarian/quartet",
		"/v1/librarian/findings",
		"/v1/librarian/emergency_stop",
	} {
		if got := postRaw(t, http.MethodPost, base+p, tok, "{"); got != 400 {
			t.Fatalf("bad-json %s: %d", p, got)
		}
	}
}

func TestLibrarianTasks(t *testing.T) {
	base, tok, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)

	// Pre-register the instance (FK requirement).
	_, raw := doJSON(t, http.MethodPost, base+"/v1/librarian/instances", tok,
		map[string]any{"role": "curator", "instance_id": "curator-01"}, nil)
	_ = raw

	s, raw := doJSON(t, http.MethodPost, base+"/v1/librarian/tasks", tok,
		map[string]any{"role": "curator", "title": "audit"}, nil)
	if s != 201 {
		t.Fatalf("enqueue: %d %s", s, raw)
	}
	var task struct {
		TaskID string `json:"task_id"`
	}
	_ = json.Unmarshal(raw, &task)

	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/tasks/"+task.TaskID+"/claim", tok,
		map[string]any{"instance_id": "curator-01"}, nil)
	if s != 204 {
		t.Fatalf("claim: %d", s)
	}
	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/tasks/"+task.TaskID+"/claim", tok,
		map[string]any{}, nil)
	if s != 400 {
		t.Fatalf("claim-missing-instance: %d", s)
	}

	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/tasks/"+task.TaskID+"/complete", tok,
		map[string]any{"success": true, "result": "ok"}, nil)
	if s != 204 {
		t.Fatalf("complete: %d", s)
	}

	s, _ = doJSON(t, http.MethodGet,
		base+"/v1/librarian/tasks?role=curator&status=DONE", tok, nil, nil)
	if s != 200 {
		t.Fatalf("list: %d", s)
	}
}

func TestLibrarianQuartetAndFindings(t *testing.T) {
	base, tok, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)

	// Quartet
	s, raw := doJSON(t, http.MethodPost, base+"/v1/librarian/quartet", tok,
		map[string]any{
			"topic":         "x",
			"participant_1": "a", "participant_2": "b", "participant_3": "c",
			"judge": "j",
		}, nil)
	if s != 201 {
		t.Fatalf("quartet: %d %s", s, raw)
	}
	var q struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &q)
	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/quartet/"+q.ID+"/decide", tok,
		map[string]any{"decision": "approve"}, nil)
	if s != 204 {
		t.Fatalf("decide: %d", s)
	}
	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/quartet/"+q.ID+"/decide", tok,
		map[string]any{}, nil)
	if s != 400 {
		t.Fatalf("decide-missing: %d", s)
	}
	s, _ = doJSON(t, http.MethodGet, base+"/v1/librarian/quartet", tok, nil, nil)
	if s != 200 {
		t.Fatalf("list: %d", s)
	}

	// Findings
	_, _ = doJSON(t, http.MethodPost, base+"/v1/projects", tok,
		map[string]any{"id": "p", "name": "P"}, nil)
	_, raw = doJSON(t, http.MethodPost, base+"/v1/entries", tok,
		map[string]any{"project_id": "p", "type": "trap", "title": "x", "body": "y"}, nil)
	var e struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &e)

	s, raw = doJSON(t, http.MethodPost, base+"/v1/librarian/findings", tok,
		map[string]any{"agent_lens": "scout", "source_url": "https://x", "excerpt": "y"}, nil)
	if s != 201 {
		t.Fatalf("finding: %d %s", s, raw)
	}
	var f struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &f)

	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/findings/"+f.ID+"/correlate", tok,
		map[string]any{"entry_id": e.ID, "correlation": 0.7}, nil)
	if s != 204 {
		t.Fatalf("correlate: %d", s)
	}
	s, _ = doJSON(t, http.MethodPost,
		base+"/v1/librarian/findings/"+f.ID+"/correlate", tok,
		map[string]any{}, nil)
	if s != 400 {
		t.Fatalf("correlate-missing: %d", s)
	}
	s, _ = doJSON(t, http.MethodGet, base+"/v1/librarian/findings?agent_lens=scout", tok, nil, nil)
	if s != 200 {
		t.Fatalf("list-findings: %d", s)
	}
}

func TestLibrarianEmergencyStop(t *testing.T) {
	base, tok, _ := testServer(t)
	t.Cleanup(ResetEmergencyStopForTest)

	// Engage
	s, raw := doJSON(t, http.MethodPost, base+"/v1/librarian/emergency_stop", tok,
		map[string]any{"engage": true}, nil)
	if s != 200 {
		t.Fatalf("engage: %d %s", s, raw)
	}

	// Now writes should 503
	s, _ = doJSON(t, http.MethodPost, base+"/v1/librarian/instances", tok,
		map[string]any{"role": "coordinator"}, nil)
	if s != 503 {
		t.Fatalf("expected 503 during emergency: %d", s)
	}

	// But reads still work
	s, _ = doJSON(t, http.MethodGet, base+"/v1/librarian/instances", tok, nil, nil)
	if s != 200 {
		t.Fatalf("read during emergency: %d", s)
	}

	// Release
	s, _ = doJSON(t, http.MethodPost, base+"/v1/librarian/emergency_stop", tok,
		map[string]any{"engage": false}, nil)
	if s != 200 {
		t.Fatalf("release: %d", s)
	}

	// Writes work again
	s, _ = doJSON(t, http.MethodPost, base+"/v1/librarian/instances", tok,
		map[string]any{"role": "coordinator"}, nil)
	if s != 201 {
		t.Fatalf("post-release write: %d", s)
	}
}
