package main

import (
	"regexp"
	"strings"
)

type part struct {
	name string
	set  bool
	any  bool
}

type resource struct {
	zone     part
	product  part
	env      part
	job      part
	instance part
	service  part
}

var resourceRegexp = regexp.MustCompile(`^` +
	`(?:/([a-z*][a-z0-9-]{0,62}))` + //  zone
	`(?:/([a-z*][a-z0-9-]{0,62}))` + //  product
	`(?:/([a-z*][a-z0-9-]{0,62}))` + //  env
	`(?:/([a-z*][a-z0-9-]{0,62}))` + //  job
	`(?:/([0-9*]{0,5}))?` +
	`(?::([a-z*][a-z0-9-]{0,62}))?` + // /instance:service
	`$`,
)

func maybePartialResourceFromPath(path string) *resource {
	res := new(resource)
	parts := res.parts()

	names := strings.Split(strings.Trim(path, "/"), "/")
	if len(names) > len(parts) {
		return nil
	}
	for i, name := range names {
		if name != "" {
			parts[i].name = name
			parts[i].set = true
		}
	}
	return res
}

func ParseResource(path string) (resource, bool) {
	r := resource{}
	c := resourceRegexp.FindStringSubmatch(path)
	if len(c) == 0 {
		return r, false
	}

	for i, p := range r.parts() {
		if i > len(c)-1 {
			break
		}

		part := c[i+1]
		switch part {
		case "":
			// zero value, skip
		case "*":
			p.any = true
			p.name = part
		default:
			p.set = true
			p.name = part
		}
	}

	return r, true
}

func (r *resource) parts() []*part {
	return []*part{
		&(r.zone),
		&(r.product),
		&(r.env),
		&(r.job),
		&(r.instance),
		&(r.service),
	}
}

func (r resource) FullyQualified() bool {
	for _, p := range r.parts() {
		if !p.set || p.any {
			return false
		}
	}
	return true
}

func (r resource) Any() bool {
	for _, p := range r.parts() {
		if p.any {
			return true
		}
	}
	return false
}

func (r resource) Match(other resource) bool {
	theirs := other.parts()
	for i, mine := range r.parts() {
		if i > len(theirs) {
			return true
		}
		if mine.any {
			continue
		}
		if mine.set && mine.name != theirs[i].name {
			return false
		}
	}
	return true
}

func (r resource) String() (s string) {
	if r.zone.set || r.zone.any {
		s += "/" + r.zone.name
	} else {
		goto service
	}

	if r.product.set || r.product.any {
		s += "/" + r.product.name
	} else {
		goto service
	}

	if r.env.set || r.env.any {
		s += "/" + r.env.name
	} else {
		goto service
	}

	if r.job.set || r.job.any {
		s += "/" + r.job.name
	} else {
		goto service
	}

	if r.instance.set || r.instance.any {
		s += "/" + r.instance.name
	}

service:
	if r.service.set || r.service.any {
		s += ":" + r.service.name
	}

	return s
}
