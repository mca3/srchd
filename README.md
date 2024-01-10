# srchd

srchd is a privacy-respecting metasearch engine written in Go.

It was written to compete against the likes of [SearxNG](https://github.com/searxng/searxng), which is written in Python, is heavy, and somewhat complicated to setup.
I have personally used SearxNG and do think it's a great piece of software, however I had trouble running it on constrained hardware alongside a couple other services.
So I set out to replace it, and this is the result.

## Running

Simply use `go build .` to build a srchd binary, or `go run .` to run it out of this repository.
srchd holds all of the resources it needs to run within itself, so all you would need to move is the binary.

There is no configuration as of now.

## Search engine support

srchd is a young project and as such doesn't have a lot of support.

- Bing
- DuckDuckGo
- Google
- wiby.me (native API)
