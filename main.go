package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"git.int21h.xyz/mwr"
	"git.int21h.xyz/srchd/search"

	_ "git.int21h.xyz/srchd/search/ddg"
)

type tmplData struct {
	Title   string
	Query   string
	Results []search.Result
}

//go:embed views/*.html
var tmplFS embed.FS

var tmpl = template.Must(template.New("").ParseFS(tmplFS, "views/*.html"))

func main() {
	engines := []search.Engine{}

	eng, err := search.New("ddg", "duckduckgo")
	if err != nil {
		panic(err)
	}
	engines = append(engines, eng)

	h := &mwr.Handler{}

	h.Get("/search", func(c *mwr.Ctx) error {
		res := []search.Result{}
		pageNo, _ := strconv.Atoi(c.Query("p", "0"))

		for _, v := range engines {
			r, err := v.Search(c.Context(), c.Query("q"), pageNo)
			if err != nil {
				log.Printf("search failed: %v", err)
			}
			res = append(res, r...)
		}

		log.Printf("found %d results for %q", len(res), c.Query("q"))

		return tmpl.ExecuteTemplate(c, "search.html", tmplData{
			Title:   c.Query("q"),
			Query:   c.Query("q"),
			Results: res,
		})
	})

	h.Get("/", func(c *mwr.Ctx) error {
		return tmpl.ExecuteTemplate(c, "index.html", tmplData{})
	})

	http.Handle("/", h)
	http.ListenAndServe(":8080", nil)
}
