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

	"git.sr.ht/~cmcevoy/mwr"
	"git.sr.ht/~cmcevoy/srchd/search"
)

type tmplData struct {
	Title   string
	Query   string
	Page    int
	Results []search.Result
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

	h := &mwr.Handler{}

	h.Get("/search", func(c *mwr.Ctx) error {
		pageNo, _ := strconv.Atoi(c.Query("p", "0"))
		category, ok := getCategory(c.Query("c"))
		if !ok {
			return c.Status(400).SendString("invalid category") // TODO
		}

		res, err := doSearch(c, category, c.Query("q"), pageNo)
		if err != nil {
			return err
		}

		return tmpl.ExecuteTemplate(c, "search.html", tmplData{
			Title:   c.Query("q"),
			Query:   c.Query("q"),
			Page:    pageNo,
			Results: res,
			BaseURL: cfg.BaseURL,
		})
	})

	h.Get("/", func(c *mwr.Ctx) error {
		return tmpl.ExecuteTemplate(c, "index.html", tmplData{
			BaseURL: cfg.BaseURL,
		})
	})

	h.Get("/opensearch.xml", func(c *mwr.Ctx) error {
		return tmpl.ExecuteTemplate(c, "opensearch.xml", tmplData{
			BaseURL: cfg.BaseURL,
		})
	})

	h.Get("/settings", func(c *mwr.Ctx) error {
		return tmpl.ExecuteTemplate(c, "settings.html", confData{
			tmplData: tmplData{
				Title:   "Settings",
				BaseURL: cfg.BaseURL,
			},
			Engines:  search.Supported(),
			Selected: findWantedEngines(c),
		})
	})

	h.Post("/settings", func(c *mwr.Ctx) error {
		wantedEngines := c.FormValues("engine")
		c.SetCookie("engines", strings.Join(wantedEngines, ","))
		return c.Redirect("/settings")
	})

	h.Use(func(c *mwr.Ctx) error {
		fp := filepath.Join("static", c.URL().EscapedPath())
		h, err := staticFS.Open(fp)
		if err != nil {
			return c.Status(404).SendString("404 not found")
		}
		defer h.Close()

		// Hack
		if strings.HasSuffix(fp, ".css") {
			c.Set("Content-Type", "text/css")
		}

		_, err = io.Copy(c, h)
		return err
	})

	log.Printf("listening on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, h))
}
