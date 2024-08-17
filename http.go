package main

import (
	"context"
	"embed"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~cmcevoy/srchd/search"
	"github.com/go-chi/chi/v5"
)

type tmplData struct {
	Title   string
	Query   string
	Page    int
	Results []search.Result
	Errors  map[string]error
	Error   error
	BaseURL string
}

type confData struct {
	tmplData
	Engines  []string
	Selected []string
}

//go:embed views/*.html views/*.xml
var tmplFS embed.FS

//go:embed static/*
var staticFS embed.FS

var tmpl = template.Must(template.New("").Funcs(template.FuncMap{
	"inc": func(x int) int {
		return x + 1
	},
	"dec": func(x int) int {
		return x - 1
	},
	"strIn":         slices.Contains[[]string],
	"engineLatency": getEngineLatency,
	"version": func() string {
		return Version
	},
}).ParseFS(tmplFS, "views/*.html", "views/*.xml"))

func templateExecute(out io.Writer, name string, data any) {
	if err := tmpl.ExecuteTemplate(out, name, data); err != nil {
		log.Printf("executing template %q failed: %v", name, err)
	}
}

// Sets up a HTTP server from the current configuration.
//
// When the context that is passed to this function is canceled, the server
// will be shutdown and the error will be [context.Canceled].
//
// serveHTTP never returns a nil error.
func serveHTTP(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	h := chi.NewRouter()

	// search endpoint is the one most people will be hitting.
	h.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			// Unsupported method
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Grab and parse query parameters.
		page := r.FormValue("p")
		query := r.FormValue("q")

		var pageNo int
		var err error
		if page != "" {
			// Only parse the page value if it isn't empty.
			pageNo, err = strconv.Atoi(page)
			if err != nil {
				// TODO
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		// Perform the search.
		res, errors, err := doSearch(r, query, pageNo)
		if err != nil {
			// Set a failure response code.
			// Everything else is handled by the template.
			w.WriteHeader(500)
		} else if len(res) == 0 {
			// No results found, set the status code to a 404.
			w.WriteHeader(404)
		}

		// Return the results.
		templateExecute(w, "search.html", tmplData{
			Title:   query,
			Query:   query,
			Page:    pageNo,
			Results: res,
			Errors:  errors,
			Error:   err,
			BaseURL: cfg.BaseURL,
		})
	})

	// index
	h.Get("/", func(w http.ResponseWriter, r *http.Request) {
		templateExecute(w, "index.html", tmplData{
			BaseURL: cfg.BaseURL,
		})
	})

	// opensearch thing.
	// Allows you to add srchd as a search engine in your browser, provided
	// BaseURL is configured correctly.
	h.Get("/opensearch.xml", func(w http.ResponseWriter, r *http.Request) {
		templateExecute(w, "opensearch.xml", tmplData{
			BaseURL: cfg.BaseURL,
		})
	})

	// settings stuff.
	h.Get("/settings", func(w http.ResponseWriter, r *http.Request) {
		// Grab a list of currently enabled engines.
		// This is used to mark engines as checked.
		wanted := findWantedEngines(r)
		if len(wanted) == 0 {
			// Nothing enabled so use the default ones.
			wanted = search.DefaultEngines()
		}

		templateExecute(w, "settings.html", confData{
			tmplData: tmplData{
				Title:   "Settings",
				BaseURL: cfg.BaseURL,
			},
			Engines:  enabledEngines(),
			Selected: wanted,
		})
	})

	// write settings
	h.Post("/settings", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form submitted", 400)
			return
		}

		// Determine the engines that the user wants to search.
		wantedEngines, ok := r.Form["engine"]
		if !ok {
			http.Error(w, "invalid form submitted", 400)
			return
		}

		// The engines cookie determines what engines the user wants to
		// search on.
		http.SetCookie(w, &http.Cookie{
			Name:  "engines",
			Value: strings.Join(wantedEngines, ","),
		})

		http.Redirect(w, r, "/settings", http.StatusFound)
	})

	staticHandler := func(w http.ResponseWriter, r *http.Request) {
		// TODO: This should probably be replaced with something
		// infinitely better.

		fp := filepath.Join("static", r.URL.EscapedPath())
		h, err := staticFS.Open(fp)
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		defer h.Close()

		// Hack
		if strings.HasSuffix(fp, ".css") {
			w.Header().Set("Content-Type", "text/css")
		}

		io.Copy(w, h)
	}

	h.Get("/css/*", staticHandler)
	h.Get("/robots.txt", staticHandler)

	// With the HTTP stuff dealt with, let's setup the server
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: h,

		// TODO: Should we allow these values to be tweaked from the
		// config?
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,

		// We want to use our own context
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	// Special goroutine to close the server when the context is canceled.
	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	log.Printf("listening on %s", cfg.Addr)
	err := srv.ListenAndServe()

	if ctx.Err() != nil {
		// If this is not nil, then the server was closed because the
		// context was canceled.
		// We can safely ignore the error from the server.
		return ctx.Err()
	}

	// Return the error from the server, which is always non-nil.
	return err
}
