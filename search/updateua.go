//go:build ignore

package main

// "Good enough" quality code.
// It's fine, this is just to update the user agent and is simple enough to do
// manually anyway.

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"text/template"

	_ "embed"
)

const chromeTrainData = "https://chromiumdash.appspot.com/fetch_releases?channel=Stable&platform=Windows&num=1"
const chromePlatform = "Windows NT 10.0; Win64; x64"

//go:embed useragent.go.tmpl
var uaTemplateCode string
var uaTemplate = template.Must(template.New("").Parse(uaTemplateCode))

func main() {
	res, err := http.Get(chromeTrainData)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	data := []map[string]any{}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		panic(err)
	}
	if len(data) == 0 {
		panic("no data in response")
	}
	ver, _, _ := strings.Cut(data[0]["version"].(string), ".")

	tmpl := map[string]string{
		"platform": chromePlatform,
		"ver":      ver,
	}

	h, err := os.Create("useragent.go")
	if err != nil {
		panic(err)
	}
	defer h.Close()

	err = uaTemplate.Execute(h, tmpl)
	if err != nil {
		panic(err)
	}
}
