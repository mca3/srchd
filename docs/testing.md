# Testing search engines

Since srchd is a web scraper by design, it is a good idea to be able to test search engines quickly and be able to quickly fix things when they break.
As such most engines in srchd have some form of test suite, though generally it just sends off queries and hopes it gets the correct answer back.

The test suite of srchd can be run using `go test .` but the engines test suite can be run with `go test ./search/engines`.

## engtest

srchd contains a small test harness called engtest which will cache all requests and responses sent to a search engine.
The test files are not distributed with srchd because they are big and may contain somewhat personal information, but provided the engines work on your machine running the tests will create ones for you if they aren't there already.

If you would like to update your test files, use `-update`.
