package mcp

import (
	"net/http"
	"strings"
	"testing"
)

func TestToolsListIncludesOpenWork(t *testing.T) {
	s, _ := fixture(t, func(http.ResponseWriter, *http.Request) {})
	resp := runRPC(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	for _, want := range []string{"kb_open_list", "kb_open_claim", "kb_open_release", "kb_open_merge"} {
		if !strings.Contains(resp, want) {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestKBOpenList(t *testing.T) {
	var path, qs string
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		qs = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	runRPC(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kb_open_list","arguments":{"role":"detective","effort":"S"}}}`)
	if path != "/v1/open_work" {
		t.Fatalf("path=%s", path)
	}
	if !strings.Contains(qs, "role=detective") || !strings.Contains(qs, "effort=S") {
		t.Fatalf("qs=%s", qs)
	}
}

func TestKBOpenListNoFilter(t *testing.T) {
	var qs string
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		qs = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	runRPC(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kb_open_list","arguments":{}}}`)
	if qs != "" {
		t.Fatalf("qs should be empty: %s", qs)
	}
}

func TestKBOpenClaimAndMerge(t *testing.T) {
	var lastPath, lastMethod string
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		lastMethod = r.Method
		_, _ = w.Write([]byte(`{}`))
	})
	runRPC(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kb_open_claim","arguments":{"entry_id":"X-1","role":"detective","instance_id":"det-01","effort":"S"}}}`)
	if lastPath != "/v1/entries/X-1/claim" || lastMethod != "POST" {
		t.Fatalf("claim: %s %s", lastMethod, lastPath)
	}
	runRPC(t, s,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"kb_open_release","arguments":{"entry_id":"X-1","instance_id":"det-01"}}}`)
	if lastPath != "/v1/entries/X-1/release" {
		t.Fatalf("release: %s", lastPath)
	}
	runRPC(t, s,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"kb_open_merge","arguments":{"entry_id":"X-1","instance_id":"det-01","result":"done","impl_entry_id":"L-1"}}}`)
	if lastPath != "/v1/entries/X-1/mark_merged" {
		t.Fatalf("merge: %s", lastPath)
	}
}

func TestKBOpenMissingEntryID(t *testing.T) {
	s, _ := fixture(t, func(http.ResponseWriter, *http.Request) {})
	for _, name := range []string{"kb_open_claim", "kb_open_release", "kb_open_merge"} {
		resp := runRPC(t, s,
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+name+`","arguments":{}}}`)
		if !strings.Contains(resp, "entry_id required") {
			t.Fatalf("%s: %s", name, resp)
		}
	}
}
