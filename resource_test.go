package main

import (
	"testing"
)

var testResources = []struct {
	path     string
	match    bool
	resource *resource
}{
	{"/aa/iaaa/prod/api/1:http", true, &resource{
		zone:     part{"aa", true, false},
		product:  part{"iaaa", true, false},
		env:      part{"prod", true, false},
		job:      part{"api", true, false},
		instance: part{"1", true, false},
		service:  part{"http", true, false},
	}},
	{"/aa/iaaa/prod/api/1", true, &resource{
		zone:     part{"aa", true, false},
		product:  part{"iaaa", true, false},
		env:      part{"prod", true, false},
		job:      part{"api", true, false},
		instance: part{"1", true, false},
		service:  part{"", false, false},
	}},
	{"/aa/iaaa/prod/api", true, &resource{
		zone:     part{"aa", true, false},
		product:  part{"iaaa", true, false},
		env:      part{"prod", true, false},
		job:      part{"api", true, false},
		instance: part{"", false, false},
		service:  part{"", false, false},
	}},
	{"/*/iaaa/*/api", true, &resource{
		zone:     part{"*", false, true},
		product:  part{"iaaa", true, false},
		env:      part{"*", false, true},
		job:      part{"api", true, false},
		instance: part{"", false, false},
		service:  part{"", false, false},
	}},
	{"/rute", false, nil},
	{"/browsing/products", false, nil},
	{"/aa/silly_underscore/*/api", false, nil},
	{"/yo/too/many/job/parts", false, nil},
	{"/yo/dd/pp/jj:double:service", false, nil},
	{"/yo/dd/pp/jj/0:double:service", false, nil},
	{"/yo/dd/inst/0:service", false, nil},
}

func TestRoundTripResources(t *testing.T) {
	for i, test := range testResources {
		r, ok := ParseResource(test.path)
		if !test.match {
			if ok {
				t.Fatalf("expected resource %q not to match, but it did", test.path)
			}
			continue
		}

		if !ok {
			t.Fatalf("expected resource %q to match, but it didn't", test.path)
		}

		if r != *test.resource {
			t.Errorf("expected resource(%d): %q to match %+v, got: %+v", i, test.path, test.resource, r)
		}

		s := test.resource.String()
		if s != test.path {
			t.Errorf("expected resource(%d) string: %q to match %q for: %+v", i, test.path, s, test.resource)
		}
	}
}
