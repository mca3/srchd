package engtest

import (
	"crypto/md5"
	"encoding/base32"
	"encoding/gob"
	"os"
	"path/filepath"
)

// HTTP header k/v pair.
type header struct {
	Key, Value string
}

// request holds all information from a request.
type request struct {
	URL     string
	Method  string
	Headers []header
	Body    []byte
}

// response holds all information returned from the server.
type response struct {
	Status  int
	Headers []header
	Body    []byte
}

// Request/response pair for testdata.
type reqres struct {
	Req request
	Res response
}

// It is often problematic to save files with "/" in the name.
// Send it through MD5 and base32 the result; it's overkill, but it works.
func hashuri(uri string) string {
	md := md5.New()
	s := md.Sum([]byte(uri))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(s[:])
}

// Saves a request/response pair to disk.
func save(base string, req request, res response) {
	rr := reqres{req, res}

	// Create the contianing directory if we need to.
	if err := os.MkdirAll(base, 0755); err != nil {
		panic(err)
	}

	// Save stuff to disk.
	h, err := os.Create(filepath.Join(base, hashuri(req.URL)))
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Gob is convenient because it works quite well for our use cases.
	if err := gob.NewEncoder(h).Encode(rr); err != nil {
		panic(err)
	}
}
