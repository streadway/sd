package main

import (
	"github.com/soundcloud/doozer"
	"testing"
	"time"
)

func mustSetup() *doozer.Conn {
	c, err := doozer.Dial("localhost:8046")
	if err != nil {
		panic(err)
	}
	mustClean(c)
	return c
}

func mustClean(c *doozer.Conn) {
	r, err := c.Rev()
	if err != nil {
		panic(err)
	}

	doozer.Walk(c, r, root,
		func(path string, fi *doozer.FileInfo, err error) error {
			if err == nil && !fi.IsDir && fi.IsSet {
				c.Del(path, fi.Rev)
			}
			return nil
		})
}

func TestDoozerDeclareReplaces(t *testing.T) {
	s := doozerStore{conn: mustSetup()}
	r, ok := ParseResource("/zz/pp/ee/jj/0:http")
	if !ok {
		t.Fatal("parse error")
	}

	a1 := address{"lolcathost", "6060"}
	a2 := address{"lolcathost", "5050"}

	old, err := s.Declare(binding{r, a1})
	if err != nil {
		t.Fatalf("expected to delcare path %q, got: %v", r, err)
	}
	if old != nil {
		t.Fatalf("expected initial declare to not have an old value, got: %v", old)
	}

	old, err = s.Declare(binding{r, a2})
	if err != nil {
		t.Fatalf("expected to delcare path %q, got: %v", r, err)
	}
	if old == nil || old.address != a1 {
		t.Fatalf("expected second declare to return previous value, got: %+v, want: %q", old, a1)
	}
}

type mockBinding struct {
	r string
	a address
}

func mustPrepare(s doozerStore, binds []mockBinding) {
	for _, b := range binds {
		r, ok := ParseResource(b.r)
		if !ok {
			panic("unparsable mock bind: " + b.r)
		}

		_, err := s.Declare(binding{r, b.a})
		if err != nil {
			panic(err)
		}
	}
}

func TestDoozerMatch(t *testing.T) {
	s := doozerStore{conn: mustSetup()}

	mustPrepare(s, []mockBinding{
		{"/zz/p1/prod/jj/0:http", address{"lolcats", "8000"}},
		{"/zz/p1/prod/jj/1:http", address{"lolcats", "8000"}},
		{"/zz/p1/prod/jj/0:mgmt-http", address{"lolcats", "8000"}},
		{"/zz/p2/prod/jj/0:http", address{"lolcats", "8000"}},
		{"/zz/p2/prod/jj/1:http", address{"lolcats", "8000"}},
		{"/zz/p2/test/jj/0:http", address{"lolcats", "8000"}},
		{"/z1/pp/test/jj/0:http", address{"lolcats", "8000"}},
		{"/z1/pp/prod/jj/0:http", address{"lolcats", "8000"}},
		{"/z1/pp/prod/jj/1:http", address{"lolcats", "8000"}},
		{"/z1/pp/prod/jj/2:http", address{"lolcats", "8000"}},
	})

	m, _ := ParseResource("/*/*/prod/*/*:http")
	bindings, err := s.Match(m)
	if err != nil {
		t.Fatalf("expected match to succeed, got: %v", err)
	}

	if len(bindings) != 7 {
		t.Fatalf("expected to match 7 serivces, only matched %d", len(bindings))
	}
}

func TestDoozerRemoveExisting(t *testing.T) {
	s := doozerStore{conn: mustSetup()}
	path := "/zz/p1/temp/jj/0:http"
	addr := address{"lolcats", "8000"}
	mustPrepare(s, []mockBinding{{path, addr}})

	r, _ := ParseResource(path)
	prev, err := s.Remove(r)
	if err != nil {
		t.Fatalf("expected no error to remove, got: %v", err)
	}
	if prev == nil || prev.address != addr {
		t.Fatalf("expected previous binding to be returned, got: %v", prev)
	}
}

func TestDoozerRemoveMissing(t *testing.T) {
	s := doozerStore{conn: mustSetup()}
	path := "/zz/p1/temp/jj/0:http"

	r, _ := ParseResource(path)
	prev, err := s.Remove(r)
	if err != nil {
		t.Fatalf("expected no error on Remove, got: %v", err)
	}
	if prev != nil {
		t.Fatalf("expected previous binding not to have existed, got: %v", prev)
	}
}

func TestDoozerRenewExisting(t *testing.T) {
	s := doozerStore{conn: mustSetup()}
	r, _ := ParseResource("/zz/pp/prod/jj:http")
	a := address{"lolcats", "8001"}

	mustPrepare(s, []mockBinding{
		{"/zz/pp/prod/jj/0:http", address{"lolcats", "8000"}},
		{"/zz/pp/prod/jj/0:http-mgmt", a},
		{"/zz/pp/prod/jj/1:http-mgmt", address{"lolcats", "8002"}},
	})

	instance, exp, created, err := s.Renew(binding{r, a})
	if err != nil {
		t.Fatalf("expected no error on Renew, got: %v", err)
	}
	if !created {
		t.Fatal("expected to reuse an existing instance")
	}
	if instance.resource.String() != "/zz/pp/prod/jj/1:http" {
		t.Fatalf("expected to claim first instance, claimed: %q", instance.resource.String())
	}
	if time.Now().Before(exp) {
		t.Fatalf("expected to expire after now, expires: %v", exp)
	}

}

func TestDoozerRenewMissing(t *testing.T) {
	s := doozerStore{conn: mustSetup()}

	r, _ := ParseResource("/zz/p1/temp/jj:http")
	a := address{"lolcat", "8080"}

	instance, exp, created, err := s.Renew(binding{r, a})
	if err != nil {
		t.Fatalf("expected no error on Renew, got: %v", err)
	}
	if !created {
		t.Fatal("expected to create first instance")
	}
	if instance.resource.String() != "/zz/p1/temp/jj/0:http" {
		t.Fatalf("expected to claim first instance, claimed: %q", instance.resource.String())
	}
	if time.Now().Before(exp) {
		t.Fatalf("expected to expire after now, expires: %v", exp)
	}
}

func doozerShouldBrowse(t *testing.T, s doozerStore, base string, children ...string) {
	p := maybePartialResourceFromPath(base)
	if p == nil {
		t.Fatalf("should parse browser base, testing error, got: %q", base)
	}

	res, exists, err := s.Browse(*p)
	if err != nil {
		t.Fatalf("expected not to error during Browse at %q, got: %v", base, err)
	}
	if !exists {
		t.Fatalf("expected to find children at %q but did not", base)
	}
	if len(res) != len(children) {
		t.Fatalf("expected to find %d children for %q, got: %d", len(children), base, len(res))
	}

	found := make(map[string]bool)
	for _, r := range res {
		found[r.String()] = true
	}
	for _, child := range children {
		if !found[child] {
			t.Fatalf("expected to find %q in the children, but did not", child)
		}
	}

}

func TestDoozerBrowse(t *testing.T) {
	s := doozerStore{conn: mustSetup()}
	mustPrepare(s, []mockBinding{
		{"/z1/pp/prod/jj/0:http", address{"lolcats", "8000"}},
		{"/z1/pp/prod/jj/1:http", address{"lolcats", "8000"}},
		{"/z1/pp/prod/jj/2:http", address{"lolcats", "8000"}},
		{"/z1/pp/test/jj/0:http", address{"lolcats", "8000"}},
		{"/zz/p1/prod/jj/0:http", address{"lolcats", "8000"}},
		{"/zz/p1/prod/jj/0:mgmt-http", address{"lolcats", "8000"}},
		{"/zz/p1/prod/jj/1:http", address{"lolcats", "8000"}},
		{"/zz/p1/test/jj/0:http", address{"lolcats", "8000"}},
		{"/zz/p2/prod/jj/0:http", address{"lolcats", "8000"}},
		{"/zz/p2/prod/jj/1:http", address{"lolcats", "8000"}},
		{"/zz/p2/test/jj/0:http", address{"lolcats", "8000"}},
	})

	doozerShouldBrowse(t, s,
		"/zz",
		"/zz/p1",
		"/zz/p2",
	)
	doozerShouldBrowse(t, s,
		"/zz/p1",
		"/zz/p1/prod",
		"/zz/p1/test",
	)
	doozerShouldBrowse(t, s,
		"/zz/p1/prod",
		"/zz/p1/prod/jj",
	)
	doozerShouldBrowse(t, s,
		"/zz/p1/prod/jj",
		"/zz/p1/prod/jj/0",
		"/zz/p1/prod/jj/1",
	)
}
