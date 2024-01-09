package main

import (
	"embed"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"git.int21h.xyz/mwr"
	"git.int21h.xyz/srchd/search"

	_ "git.int21h.xyz/srchd/search/ddg"
	_ "git.int21h.xyz/srchd/search/google"
	_ "git.int21h.xyz/srchd/search/wiby"
)

type tmplData struct {
	Title   string
	Query   string
	Page    int
	Results []search.Result
}

//go:embed views/*.html
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
}).ParseFS(tmplFS, "views/*.html"))

func main() {
	for _, v := range []string{"ddg", "wiby", "google"} {
		eng, err := search.New(v, v)
		if err != nil {
			panic(err)
		}
		engines[v] = eng
	}

	h := &mwr.Handler{}

	h.Get("/search", func(c *mwr.Ctx) error {
		pageNo, _ := strconv.Atoi(c.Query("p", "0"))

		res, err := doSearch(c, c.Query("q"), pageNo)
		if err != nil {
			return err
		}

		log.Printf("found %d results for %q", len(res), c.Query("q"))

		return tmpl.ExecuteTemplate(c, "search.html", tmplData{
			Title:   c.Query("q"),
			Query:   c.Query("q"),
			Page:    pageNo,
			Results: res,
		})
	})

	h.Get("/", func(c *mwr.Ctx) error {
		return tmpl.ExecuteTemplate(c, "index.html", tmplData{})
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

	http.ListenAndServe(":8080", h)
}
