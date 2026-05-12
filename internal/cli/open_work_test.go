package cli

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func openWorkStub(t *testing.T) string {
	t.Helper()
	srv := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/open_work" && r.Method == "GET":
			_, _ = w.Write([]byte(`{"items":[{"Entry":{"id":"X-1","title":"x","type":"design"},"Effort":"S"}]}`))
		case strings.HasSuffix(r.URL.Path, "/claim") && r.Method == "POST":
			_, _ = w.Write([]byte(`{"task_id":"task-1","entry_id":"X-1"}`))
		case strings.HasSuffix(r.URL.Path, "/release") && r.Method == "POST":
			w.WriteHeader(204)
		case strings.HasSuffix(r.URL.Path, "/mark_merged") && r.Method == "POST":
			w.WriteHeader(204)
		default:
			http.NotFound(w, r)
		}
	})
	return srv.URL
}

func TestCmdOpenList(t *testing.T) {
	cfg := testHarness(t)
	writeCfg(t, cfg, openWorkStub(t), "tok")
	out := &bytes.Buffer{}
	if err := CmdOpen([]string{"list", "--role", "detective"}, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "X-1") {
		t.Fatalf("out: %s", out.String())
	}
}

func TestCmdOpenListEmpty(t *testing.T) {
	cfg := testHarness(t)
	emptyStub := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	writeCfg(t, cfg, emptyStub.URL, "tok")
	out := &bytes.Buffer{}
	if err := CmdOpen([]string{"list"}, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no open work") {
		t.Fatalf("empty: %s", out.String())
	}
}

func TestCmdOpenClaim(t *testing.T) {
	cfg := testHarness(t)
	writeCfg(t, cfg, openWorkStub(t), "tok")
	out := &bytes.Buffer{}
	if err := CmdOpen([]string{"claim",
		"--entry", "X-1", "--role", "detective", "--instance", "det-01", "--effort", "S",
	}, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "task-1") {
		t.Fatalf("out: %s", out.String())
	}
}

func TestCmdOpenReleaseAndMerge(t *testing.T) {
	cfg := testHarness(t)
	writeCfg(t, cfg, openWorkStub(t), "tok")
	if err := CmdOpen([]string{"release", "--entry", "X-1", "--instance", "det-01"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := CmdOpen([]string{"merge", "--entry", "X-1", "--instance", "det-01",
		"--result", "shipped", "--impl", "L-IMPL"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestCmdOpenValidation(t *testing.T) {
	cfg := testHarness(t)
	writeCfg(t, cfg, "http://x", "t")
	if err := CmdOpen(nil, &bytes.Buffer{}); err == nil {
		t.Fatal("expected usage err")
	}
	if err := CmdOpen([]string{"weird"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected unknown subcommand")
	}
	for _, args := range [][]string{
		{"claim"},
		{"release"},
		{"merge"},
	} {
		if err := CmdOpen(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("expected validation err: %v", args)
		}
	}
}
