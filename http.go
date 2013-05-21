package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// TODO JSONify
type address struct {
	host string
	port string
}

func (a address) String() string { return a.host + ":" + a.port }

// TODO JSONify
type binding struct {
	resource
	address
}

type Store interface {
	Renew(binding) (*binding, time.Time, bool, error)
	Declare(binding) (*binding, error)
	Remove(resource) (*binding, error)
	Match(resource) ([]*binding, error)
	Browse(resource) ([]resource, bool, error)
}

type directory struct {
	store Store
}

func readAddress(r io.Reader) (*address, error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	addr := new(address)
	addr.host, addr.port, err = net.SplitHostPort(strings.TrimSpace(string(body)))
	if err != nil {
		return nil, err
	}

	return addr, nil
}

func writeDel(w io.Writer, b *binding) error {
	_, err := fmt.Fprintf(w, "del: %s %s\n", b.resource, b.address)
	return err
}

func writeAdd(w io.Writer, b *binding) error {
	_, err := fmt.Fprintf(w, "add: %s %s\n", b.resource, b.address)
	return err
}

func canHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		return true
	}
	if strings.Contains(accept, "*/*") {
		return true
	}
	return false
}

func writeLink(w io.Writer, r *http.Request, res resource) error {
	if canHTML(r) {
		_, err := fmt.Fprintf(w, "<a href=%q>%s</a>\n", res.String(), res.String())
		return err
	}
	_, err := fmt.Fprintf(w, "%s\n", res.String())
	return err
}

func (h directory) declare(w http.ResponseWriter, b *binding) {
	old, err := h.store.Declare(*b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if old == nil {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
		writeDel(w, old)
	}
	writeAdd(w, b)
}

func (h directory) renew(w http.ResponseWriter, b *binding) {
	resolved, expires, created, err := h.store.Renew(*b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Expires", expires.UTC().Format(time.RFC1123))
	writeAdd(w, resolved)
}

func (h directory) servePUT(w http.ResponseWriter, r *http.Request) {
	res, ok := ParseResource(r.URL.Path)
	if !ok {
		http.Error(w, "URL must match /zone/product/env/job(/instance)?:service", http.StatusBadRequest)
		return
	}

	addr, err := readAddress(r.Body)
	if err != nil {
		http.Error(w, "Body must contain host:port", http.StatusBadRequest)
		return
	}

	switch {
	case res.service.set && res.instance.set:
		h.declare(w, &binding{res, *addr})
	case res.service.set:
		h.renew(w, &binding{res, *addr})
	default:
		http.Error(w, "the PUT resource must include a service and optionally an instance", http.StatusBadRequest)
	}
}

func (h directory) serveDELETE(w http.ResponseWriter, r *http.Request) {
	res, ok := ParseResource(r.URL.Path)
	if !ok {
		http.Error(w, "URL must match /zone/product/env/job(/instance)?:service", http.StatusBadRequest)
		return
	}

	b, err := h.store.Remove(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if b != nil {
		writeDel(w, b)
	}
}

func (h directory) match(w http.ResponseWriter, r *http.Request, res resource) {
	bindings, err := h.store.Match(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, b := range bindings {
		fmt.Fprintf(w, "%s %s\n", b.resource, b.address)
	}
}

func (h directory) browse(w http.ResponseWriter, r *http.Request) {
	res := maybePartialResourceFromPath(r.URL.Path)
	if res == nil {
		http.Error(w, "Path too long", http.StatusBadRequest)
		return
	}

	resources, found, err := h.store.Browse(*res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if canHTML(r) {
		w.Header().Set("Content-Type", "text/html")

		fmt.Fprintln(w, "<pre>")
		for _, res := range resources {
			fmt.Fprintf(w, "<a href=%q>%s</a>\n", res.String(), res.String())
		}
		fmt.Fprintln(w, "</pre>")
	} else {
		w.Header().Set("Content-Type", "text/plain")
		for _, res := range resources {
			fmt.Fprintf(w, "%s\n", res.String())
		}
	}
}

func (h directory) serveGET(w http.ResponseWriter, r *http.Request) {
	res, ok := ParseResource(r.URL.Path)

	if ok && (res.Any() || res.FullyQualified()) {
		h.match(w, r, res)
	} else {
		h.browse(w, r)
	}
}

func (h directory) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		h.servePUT(w, r)
	case "DELETE":
		h.serveDELETE(w, r)
	case "GET":
		h.serveGET(w, r)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func main() {
	log.Fatalln(http.ListenAndServe(":8080", directory{store: newMemStore()}))
}
