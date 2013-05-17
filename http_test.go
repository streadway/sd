package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeclare(t *testing.T) {
	h := directory{store: newMemStore()}

	r, _ := http.NewRequest("PUT", "/zone/product/env/job/1:http-api", bytes.NewBufferString("host1:port"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if 201 != w.Code {
		t.Fatalf("expected PUT for a creation return 201, returned %v %q", w.Code, w.Body.String())
	}

	if ex := "add: /zone/product/env/job/1:http-api host1:port\n"; ex != w.Body.String() {
		t.Fatalf("expected add operation: %q, got: %q", ex, w.Body.String())
	}
}

func TestReplace(t *testing.T) {
	s := newMemStore()
	h := directory{store: s}

	res, _ := ParseResource("/zone/product/env/job/1:http-api")
	s.Declare(binding{res, address{"host1", "port"}})

	r, _ := http.NewRequest("PUT", res.String(), bytes.NewBufferString("host2:port"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if 200 != w.Code {
		t.Fatalf("expected PUT for a relpacement to return 200, returned %v %q", w.Code, w.Body.String())
	}

	if ex := "del: /zone/product/env/job/1:http-api host1:port\nadd: /zone/product/env/job/1:http-api host2:port\n"; ex != w.Body.String() {
		t.Fatalf("expected del + add operation: %q, got: %q", ex, w.Body.String())
	}
}

func TestDelete(t *testing.T) {
	s := newMemStore()
	h := directory{store: s}

	res, _ := ParseResource("/zone/product/env/job/1:http-api")
	s.Declare(binding{res, address{"host", "port"}})

	r, _ := http.NewRequest("DELETE", res.String(), nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if 200 != w.Code {
		t.Fatalf("expected DELETE to return 200, returned %v %q", w.Code, w.Body.String())
	}

	if ex := "del: /zone/product/env/job/1:http-api host:port\n"; ex != w.Body.String() {
		t.Fatalf("expected del operation, want: %q, got: %q", ex, w.Body.String())
	}
}

func TestRenew(t *testing.T) {
	s := newMemStore()
	h := directory{store: s}

	r, _ := http.NewRequest("PUT", "/zone/product/env/job:http-api", bytes.NewBufferString("host:port"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if 201 != w.Code {
		t.Fatalf("expected PUT to return 201 when renewing for the first time, returned %v %q", w.Code, w.Body.String())
	}

	if ex := "add: /zone/product/env/job/0:http-api host:port\n"; ex != w.Body.String() {
		t.Fatalf("expected add operation of a FQSN, want: %q, got: %q", ex, w.Body.String())
	}
}

func TestRenewExisting(t *testing.T) {
	s := newMemStore()
	h := directory{store: s}

	res, _ := ParseResource("/zone/product/env/job:http-api")
	s.Renew(binding{res, address{"host", "port"}})

	r, _ := http.NewRequest("PUT", res.String(), bytes.NewBufferString("host:port"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if 200 != w.Code {
		t.Fatalf("expected PUT for a renewal to return 200, returned %v %q", w.Code, w.Body.String())
	}

	if ex := "add: /zone/product/env/job/0:http-api host:port\n"; ex != w.Body.String() {
		t.Fatalf("expected del + add operation: %q, got: %q", ex, w.Body.String())
	}
}

func fill(s *memStore, bindings map[string]address) {
	for path, addr := range bindings {
		res, ok := ParseResource(path)
		if !ok {
			panic("can't parse fixture resource")
		}
		s.Declare(binding{res, addr})
	}
}

func TestMatch(t *testing.T) {
	s := newMemStore()
	h := directory{store: s}

	fill(s, map[string]address{
		"/aa/iaaa/prod/job/0:http-api": address{"host", "port"},
		"/aa/iaaa/test/job/1:http-api": address{"host", "port"},
		"/bb/iaaa/prod/job/0:http-api": address{"host", "port"},
	})

	r, _ := http.NewRequest("GET", "/aa/iaaa/*/job/*:http-api", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if 200 != w.Code {
		t.Fatalf("expected GET for a match to return 200, returned %v %q", w.Code, w.Body.String())
	}

	if ex := "/aa/iaaa/prod/job/0:http-api host:port\n/aa/iaaa/test/job/1:http-api host:port\n"; ex != w.Body.String() {
		t.Fatalf("expected single matched result %q, got: %q", ex, w.Body.String())
	}
}

func testBrowse(t *testing.T, h http.Handler, headers map[string]string, path string, lines ...string) {
	r, _ := http.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()

	for k, v := range headers {
		r.Header.Set(k, v)
	}

	h.ServeHTTP(w, r)

	if 200 != w.Code {
		t.Fatalf("expected GET for a match to return 200, returned %v %q", w.Code, w.Body.String())
	}

	if ex := strings.Join(lines, ""); ex != w.Body.String() {
		t.Fatalf("expected lines to match body for %q, got %d %q, want %q", path, w.Code, w.Body.String(), ex)
	}
}

func TestBrowse(t *testing.T) {
	s := newMemStore()
	h := directory{store: s}
	a := map[string]string{"Accept": "text/plain"}

	fill(s, map[string]address{
		"/aa/iaaa/prod/job/0:http-api":  address{"host", "port"},
		"/aa/iaaa/prod/job/0:http-mgmt": address{"host", "port"},
		"/aa/iaaa/test/job/1:http-api":  address{"host", "port"},
		"/aa/catz/prod/job/0:http-api":  address{"host", "port"},
		"/bb/iaaa/prod/job/0:http-api":  address{"host", "port"},
	})

	testBrowse(t, h, a,
		"/",
		"/aa\n",
		"/bb\n",
	)
	testBrowse(t, h, a,
		"/bb",
		"/bb/iaaa\n",
	)
	testBrowse(t, h, a,
		"/aa/",
		"/aa/iaaa\n",
		"/aa/catz\n",
	)
	testBrowse(t, h, a,
		"/aa/iaaa/",
		"/aa/iaaa/prod\n",
		"/aa/iaaa/test\n",
	)
	testBrowse(t, h, a,
		"/aa/iaaa/prod/job",
		"/aa/iaaa/prod/job/0\n",
	)
	testBrowse(t, h, a,
		"/aa/iaaa/prod/job/0",
		"/aa/iaaa/prod/job/0:http-api\n",
		"/aa/iaaa/prod/job/0:http-mgmt\n",
	)
}
