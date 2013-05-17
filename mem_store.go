package main

import (
	"errors"
	"strconv"
	"time"
)

type renew struct {
	resource resource
	expires  time.Time
}

type memStore struct {
	binds  map[resource]*address
	renews map[resource]*renew
}

func newMemStore() *memStore {
	return &memStore{
		binds:  make(map[resource]*address),
		renews: make(map[resource]*renew),
	}
}

func (s *memStore) Declare(b binding) (*binding, error) {
	if !b.resource.FullyQualified() {
		return nil, errors.New("Declare expects a fully qualfied resource")
	}
	old := s.binds[b.resource]
	s.binds[b.resource] = &b.address

	if old != nil {
		return &binding{b.resource, *old}, nil
	}
	return nil, nil
}

func (s *memStore) Remove(r resource) (*binding, error) {
	old := s.binds[r]
	delete(s.binds, r)
	if old != nil {
		return &binding{r, *old}, nil
	}
	return nil, nil
}

func (s *memStore) makeInstance(service resource) resource {
	service.instance.set = true
	service.instance.any = false

	for i := 0; ; i++ {
		service.instance.name = strconv.Itoa(i)
		if s.binds[service] == nil {
			return service
		}
	}
}

func (s *memStore) Renew(b binding) (*binding, time.Time, bool, error) {
	now := time.Now()
	next := now.Add(time.Hour)

	if ptr := s.renews[b.resource]; ptr != nil {
		if addr, ok := s.binds[ptr.resource]; ok {
			if *addr == b.address {
				ptr.expires = next
				return &binding{ptr.resource, *addr}, next, false, nil
			}
		}
	}

	// addr doesn't match or binding is removed, allocate a new instance
	instance := s.makeInstance(b.resource)
	ptr := renew{resource: instance, expires: next}
	s.binds[instance] = &b.address
	s.renews[b.resource] = &ptr
	return &binding{instance, b.address}, ptr.expires, true, nil
}

func (s *memStore) Match(r resource) (binds []*binding, err error) {
	parts := r.parts()

miss:
	for res, addr := range s.binds {
		have := res.parts()
		for i, want := range parts {
			if want.any {
				continue
			}
			if want.name != have[i].name {
				continue miss
			}
		}
		binds = append(binds, &binding{res, *addr})
	}
	return
}

func (s *memStore) Browse(parent resource) (children []resource, found bool, err error) {
	group := make(map[resource]bool)

miss:
	for match, _ := range s.binds {
		matchParts := match.parts()

		for i, part := range parent.parts() {
			if !part.set {
				for i++; i < len(matchParts); i++ {
					// clear the template except for the child
					matchParts[i].any = false
					matchParts[i].set = false
					matchParts[i].name = ""
				}
				group[match] = true
				break
			}

			if part.name != matchParts[i].name {
				continue miss
			}
		}
	}

	for res, _ := range group {
		children = append(children, res)
	}

	return children, true, nil
}
