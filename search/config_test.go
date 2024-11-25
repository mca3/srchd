package search

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

type dummyEngine struct {
	name string
	http *HttpClient
}

var (
	_ Engine = &dummyEngine{}
)

var errProxyFail = errors.New("proxy test not OK")

func init() {
	Add("dummy", false, func(config Config) (Engine, error) {
		return &dummyEngine{
			name: config.Name,
			http: config.NewHttpClient(),
		}, nil
	})
}

func (d *dummyEngine) Search(ctx context.Context, query string, page int) ([]Result, error) {
	// Just send a request to somewhere to make sure it works
	res, err := d.http.Get(ctx, "http://example.com")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	buf := [11]byte{}
	res.Body.Read(buf[:])

	// Dummy proxy should return "hello world".
	// example.com will return something entirely different
	if !bytes.Equal([]byte("hello world"), buf[:]) {
		return nil, errProxyFail
	}

	return nil, nil
}

func (d *dummyEngine) Ping(ctx context.Context) error {
	panic("unimplemented")
}

// This test is to make sure that the proxy configuration as specified in
// [Config] works properly.
func TestConfigHttpProxy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Setup a dummy HTTP handler which doesn't actually do anything, but
	// does enough to tell us that the proxy stuff works.
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer l.Close()

	srv := &http.Server{
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Write([]byte("hello world"))
		}),
	}
	go srv.Serve(l)

	// Initialize a dummy engine and see if we get the expected result.
	eng, _ := (Config{
		Name:      "dummy",
		Type:      "dummy",
		HttpProxy: "http://" + l.Addr().String(),
	}).New()
	_, err = eng.Search(ctx, "", -1)
	if err != nil {
		t.Errorf("expected err = nil, got %v", err)
	}
}
