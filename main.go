package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

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

var (
	configPath = flag.String("conf", "", "configuration file; ./config.json will be used if it exists")
)

var tmpl = template.Must(template.New("").Funcs(template.FuncMap{
	"inc": func(x int) int {
		return x + 1
	},
	"dec": func(x int) int {
		return x - 1
	},
	"strIn":  slices.Contains[[]string],
	"timing": getTiming,
}).ParseFS(tmplFS, "views/*.html", "views/*.xml"))

func getCategory(q string) (category, bool) {
	switch q {
	case "", "g":
		return General, true
	case "v":
		return Videos, true
	case "i":
		return Images, true
	case "n":
		return News, true
	}

	return -1, false
}

func templateExecute(out io.Writer, name string, data any) {
	if err := tmpl.ExecuteTemplate(out, name, data); err != nil {
		log.Printf("executing template %q failed: %v", name, err)
	}
}

func main() {
	if *configPath == "" {
		// Try config.json
		if _, err := os.Stat("./config.json"); err == nil {
			*configPath = "./config.json"
		}
	}

	if *configPath != "" {
		if err := loadConfig(*configPath); err != nil {
			log.Fatalf("failed to load config file: %v", err)
		}
	}

	for _, v := range cfg.Engines {
		log.Printf("initializing engine %q", v)

		eng, err := initializeEngine(v, v)
		if err != nil {
			panic(err)
		}
		engines[v] = eng
	}

	go pinger(context.TODO())

	h := chi.NewRouter()

	h.Get("/search", func(w http.ResponseWriter, r *http.Request) {
		var res []search.Result
		var errors map[string]error
		var err error

		pageNo, _ := strconv.Atoi(r.URL.Query().Get("p"))
		category, ok := getCategory(r.URL.Query().Get("c"))
		if !ok {
			w.WriteHeader(500)
			err = fmt.Errorf("invalid category")
			goto render
		}

		res, errors, err = doSearch(r, category, r.URL.Query().Get("q"), pageNo)
		if err != nil {
			// Set a failure response code.
			// Everything else is handled by the template.
			w.WriteHeader(500)
		}

	render:
		templateExecute(w, "search.html", tmplData{
			Title:   r.URL.Query().Get("q"),
			Query:   r.URL.Query().Get("q"),
			Page:    pageNo,
			Results: res,
			Errors:  errors,
			Error:   err,
			BaseURL: cfg.BaseURL,
		})
	})

	h.Get("/", func(w http.ResponseWriter, r *http.Request) {
		templateExecute(w, "index.html", tmplData{
			BaseURL: cfg.BaseURL,
		})
	})

	h.Get("/opensearch.xml", func(w http.ResponseWriter, r *http.Request) {
		templateExecute(w, "opensearch.xml", tmplData{
			BaseURL: cfg.BaseURL,
		})
	})

	h.Get("/settings", func(w http.ResponseWriter, r *http.Request) {
		templateExecute(w, "settings.html", confData{
			tmplData: tmplData{
				Title:   "Settings",
				BaseURL: cfg.BaseURL,
			},
			Engines:  search.Supported(),
			Selected: findWantedEngines(r),
		})
	})

	h.Post("/settings", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form submitted", 400)
			return
		}

		wantedEngines, ok := r.Form["engine"]
		if !ok {
			http.Error(w, "invalid form submitted", 400)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "engines",
			Value: strings.Join(wantedEngines, ","),
		})
		http.Redirect(w, r, "/settings", http.StatusFound)
	})

	staticHandler := func(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("listening on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, h))
}
