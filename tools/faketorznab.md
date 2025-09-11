This is a simple, fake Torznab server written in Go.

It's designed to help test applications that use Torznab indexers (like the Reel app).

## What It Does
- Starts a web server on port 8080.
- Responds to Torznab caps requests with a detailed, compliant XML, so your app can detect it as a valid indexer.
- Responds to any Torznab search request with a list of ~50 fake TV show episodes for a series you specify.
- Intentionally includes "bad" results with keywords like "HDCAM", "SCREENER", "VOSTFR", etc., to help you test your application's rejection filters. Also includes some mismatched episode numbers.
- Episode numbers are parsed from the search parameters or search term.
- It accepts any indexer name in the URL (e.g., `/torznab/anyindexer/), any search term and any API key.

## How to Use It
- Open your terminal and navigate to the directory where this script is saved.
- Run the server using go run, no need to provide any extra flags.

```bash
go run faketorznab.go -search "My Favorite Show"
```

The server will now be running and ready to accept requests from your Reel application at `http://localhost:8080/torznab/`.

You can send a query with a curl command similar to this one:

```bash
curl http://localhost:8080/torznab/any_indexer_name?t=search?q=Series+Name+S01E05
```
