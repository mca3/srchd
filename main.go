package main

import (
	"context"
	"embed"
	"flag"
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

	_ "git.sr.ht/~cmcevoy/srchd/search/engines"
	_ "net/http/pprof"
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
	configPath = flag.String("conf", "", "configuration file; ./config.yaml will be used if it exists")
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

func templateExecute(out io.Writer, name string, data any) {
	if err := tmpl.ExecuteTemplate(out, name, data); err != nil {
		log.Printf("executing template %q failed: %v", name, err)
	}
}

func main() {
	if *configPath == "" {
		// Try config.yaml
		if _, err := os.Stat("./config.yaml"); err == nil {
			*configPath = "./config.yaml"
		}
	}

	if *configPath != "" {
		if err := loadConfig(*configPath); err != nil {
			log.Fatalf("failed to load config file: %v", err)
		}
	}

	for _, v := range enabledEngines() {
		log.Printf("initializing engine %q", v)

		eng, err := initializeEngine(v)
		if err != nil {
			panic(err)
		}
		engines[v] = eng
	}

	go pinger(context.TODO())

	if cfg.Pprof != "" {
		go func() {
			// TODO: VERY TEMPORARY
			log.Println(http.ListenAndServe(cfg.Pprof, nil))
		}()
	}

	h := chi.NewRouter()

	h.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		var res []search.Result
		var errors map[string]error
		var err error

		var query, page string

		if r.Method == http.MethodGet {
			page = r.URL.Query().Get("p")
			query = r.URL.Query().Get("q")
		} else if r.Method == http.MethodPost {
			page = r.FormValue("p")
			query = r.FormValue("q")
		} else {
			// Unsupported method
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		pageNo, _ := strconv.Atoi(page)

		res, errors, err = doSearch(r, query, pageNo)
		if err != nil {
			// Set a failure response code.
			// Everything else is handled by the template.
			w.WriteHeader(500)
		}

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
		wanted := findWantedEngines(r)
		if len(wanted) == 0 {
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
