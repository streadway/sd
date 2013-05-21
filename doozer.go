package main

import (
	"fmt"
	"github.com/soundcloud/doozer"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// TODO flag
var root = "/srv"

type doozerStore struct {
	conn *doozer.Conn
}

func toPath(r resource) string {
	return root + escape(r.String())
}

func escape(path string) string {
	return strings.Replace(path, ":", "/.", -1)
}

func unescape(path string) string {
	return strings.Replace(path, "/.", ":", -1)
}

func fromPath(path string) (resource, bool) {
	return ParseResource(unescape(path[len(root):]))
}

func fromPartialPath(path string) (resource, bool) {
	res := maybePartialResourceFromPath(unescape(path))
	if res == nil {
		return resource{}, false 
	}
	return *res, true
}

func (s *doozerStore) get(r resource, revAt *int64) (*address, int64, error) {
	body, rev, err := s.conn.Get(toPath(r), revAt)
	if err != nil {
		return nil, rev, err
	}

	if len(body) > 0 {
		host, port, err := net.SplitHostPort(strings.TrimSpace(string(body)))
		if err != nil {
			return nil, rev, err
		}
		return &address{host, port}, rev, nil
	}

	return nil, rev, doozer.ErrNoEnt
}

func (s *doozerStore) set(r resource, a address, rev int64) (int64, error) {
	return s.conn.Set(toPath(r), rev, []byte(a.String()))
}

func (s *doozerStore) Browse(partial resource) ([]resource, bool, error) {
	rev, err := s.conn.Rev()
	if err != nil {
		return nil, false, err
	}

	base := partial.String()
	children, err := s.conn.Getdir(toPath(partial), rev, 0, -1)
	if err == doozer.ErrNoEnt {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	res := make([]resource, len(children))
	for i, child := range children {
		path := base + "/" + child
		partial, ok := fromPartialPath(unescape(path))
		if !ok {
			return nil, false, fmt.Errorf("invalid path in service tree: %q", path)
		}
		res[i] = partial
	}
	
	return res, true, nil
}

func (s *doozerStore) Renew(b binding) (*binding, time.Time, bool, error) {
	now := time.Now()
	pat := b.resource
	pat.instance.any = true
	pat.instance.set = false

Retry:
	bindings, rev, err := s.match(pat)
	if err != nil {
		return nil, now, false, err
	}

	for _, have := range bindings {
		if have.address == b.address {
			// TODO update lease
			return have, now.Add(time.Hour), false, nil
		}
	}

	i := 0
	for ; i < len(bindings); i++ {
		// found gap
		if strconv.Itoa(i) != bindings[i].instance.name {
			break
		}
	}

	next := b.resource
	next.instance.name = strconv.Itoa(i)
	next.instance.set = true
	next.instance.any = false

	_, err = s.set(next, b.address, rev)
	if err == doozer.ErrOldRev || err == doozer.ErrTooLate {
		goto Retry
	}
	if err != nil {
		return nil, now, false, err
	}
	// TODO update lease
	return &binding{next, b.address}, now, true, nil
}

func (s *doozerStore) Remove(r resource) (*binding, error) {
	addr, rev, err := s.get(r, nil)
	if err == doozer.ErrNoEnt {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	err = s.conn.Del(toPath(r), rev)
	if err != nil {
		return nil, err
	}

	return &binding{r, *addr}, nil
}

func (s *doozerStore) match(pat resource) ([]*binding, int64, error) {
	rev, err := s.conn.Rev()
	if err != nil {
		return nil, rev, err
	}

	var bindings []*binding

	visit := func(path string, fi *doozer.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if len(path)-len(root) > 0 {
			if !fi.IsDir && fi.IsSet {
				if leaf, ok := fromPath(path); ok {
					if pat.Match(leaf) {
						addr, _, err := s.get(leaf, &fi.Rev)
						if err != nil {
							return err
						}
						bindings = append(bindings, &binding{leaf, *addr})
					}
				} else if fi.IsDir {
					return filepath.SkipDir
				}
			}
		}
		return nil
	}

	err = doozer.Walk(s.conn, rev, root, visit)
	if err == doozer.ErrNoEnt {
		err = nil
	}
	return bindings, rev, err
}

func (s *doozerStore) Match(pat resource) ([]*binding, error) {
	bindings, _, err := s.match(pat)
	return bindings, err
}

func (s *doozerStore) Declare(b binding) (old *binding, err error) {
	var rev int64
	var addr *address

Retry:
	addr, rev, err = s.get(b.resource, nil)
	if err == nil {
		old = &binding{b.resource, *addr}
	}

	rev, err = s.set(b.resource, b.address, rev)
	if err == doozer.ErrTooLate || err == doozer.ErrOldRev {
		goto Retry
	}

	return old, err
}

