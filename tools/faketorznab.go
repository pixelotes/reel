package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// splitCamelCase splits a camelCase string into a slice of words.
func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}
	// This regex finds positions where a lowercase letter is followed by an uppercase one.
	re := regexp.MustCompile("([a-z0-9])([A-Z])")
	s = re.ReplaceAllString(s, "${1} ${2}")
	return strings.Fields(s)
}

func main() {
	// A single endpoint now handles all torznab-related requests.
	http.HandleFunc("/torznab/", torznabRouter)

	fmt.Println("Fake Torznab server starting on :8080")
	fmt.Println("Now accepting any indexer name (e.g., /torznab/anyindexer)")
	fmt.Println("Search term is now read dynamically from the 'q' parameter.")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// torznabRouter checks the request and routes to the correct handler.
func torznabRouter(w http.ResponseWriter, r *http.Request) {
	tParam := r.URL.Query().Get("t")

	// Respond with caps if the 't' param is 'caps' OR if the URL path ends with '/caps'
	if tParam == "caps" || strings.HasSuffix(r.URL.Path, "/caps") {
		capsHandler(w, r)
	} else {
		searchHandler(w, r)
	}
}

func capsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprint(w, `
<caps>
  <server version="1.1" title="Fake Acme Indexer" strapline="A fake indexer for testing" email="test@example.com" url="http://localhost:8080" image="http://localhost:8080/image.jpg"/>
  <limits max="100" default="50"/>
  <registration available="no" open="no"/>
  <searching>
    <search available="yes" supportedParams="q"/>
    <tv-search available="yes" supportedParams="q,season,ep"/>
    <movie-search available="yes" supportedParams="q,imdbid,tmdbid"/>
  </searching>
  <categories>
    <category id="2000" name="Movies">
      <subcat id="2010" name="Movies/Foreign"/>
      <subcat id="2020" name="Movies/Other"/>
      <subcat id="2030" name="Movies/SD"/>
      <subcat id="2040" name="Movies/HD"/>
      <subcat id="2050" name="Movies/BluRay"/>
      <subcat id="2060" name="Movies/3D"/>
    </category>
    <category id="5000" name="TV">
      <subcat id="5020" name="TV/Foreign"/>
      <subcat id="5030" name="TV/SD"/>
      <subcat id="5040" name="TV/HD"/>
      <subcat id="5070" name="TV/Anime"/>
      <subcat id="5080" name="TV/Documentary"/>
    </category>
  </categories>
</caps>
`)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request URL: %s", r.URL.String())

	query := r.URL.Query()
	seriesName := query.Get("q")
	if seriesName == "" {
		seriesName = "Default Series (No Query Provided)"
	}

	// Logic to correctly determine season and episode numbers.
	var requestedSeason, requestedEp int
	re := regexp.MustCompile(`(?i)S(\d+)E(\d+)`)
	matches := re.FindStringSubmatch(query.Get("q"))

	// Try to get season from 'season' param first
	seasonStr := query.Get("season")
	s, err := strconv.Atoi(seasonStr)
	if err == nil {
		requestedSeason = s
	} else if len(matches) > 2 { // Fallback to parsing from 'q'
		s, err = strconv.Atoi(matches[1])
		if err == nil {
			requestedSeason = s
		}
	}
	if requestedSeason == 0 { // Default if not found
		requestedSeason = 1
	}

	// Try to get episode from 'ep' param first
	epStr := query.Get("ep")
	ep, err := strconv.Atoi(epStr)
	if err == nil {
		requestedEp = ep
	} else if len(matches) > 2 { // Fallback to parsing from 'q'
		ep, err = strconv.Atoi(matches[2])
		if err == nil {
			requestedEp = ep
		}
	}
	if requestedEp == 0 { // Default if not found
		requestedEp = rand.Intn(12) + 1
	}

	// Clean the series name by removing episode info.
	seriesName = re.ReplaceAllString(seriesName, "")
	seriesName = strings.TrimSpace(seriesName)

	log.Printf("Interpreted search for: '%s', Season: %d, Episode: %d", seriesName, requestedSeason, requestedEp)

	w.Header().Set("Content-Type", "application/xml")

	rejectionTerms := []string{"VOSTFR", "HDCAM", "SCREENER", "FRENCH", "RUS", "MD", "TC", "DVDSCR"}
	goodQualities := []string{"360p", "480p", "1080p REMUX", "720p WEB-DL", "2160p BluRay", "1080p WEB-DL", "720p BluRay"}
	randomWords := []string{"x264", "x265", "HEVC", "DTS-HD", "AC3", "PROPER", "REPACK"}

	var items []string
	rand.Seed(time.Now().UnixNano())

	// NEW: CamelCase handling
	nameToUse := seriesName
	splitNameParts := splitCamelCase(seriesName)
	if len(splitNameParts) > 1 {
		// Randomly use the original or the split name for some results
		if rand.Intn(2) == 0 {
			nameToUse = strings.Join(splitNameParts, " ")
			log.Printf("Using split name variation for some results: '%s'", nameToUse)
		}
	}

	for i := 0; i < 5; i++ {
		wrongEp := requestedEp + rand.Intn(5) + 1
		quality := goodQualities[rand.Intn(len(goodQualities))]
		title := fmt.Sprintf("%s S%02dE%02d %s %s", nameToUse, requestedSeason, wrongEp, quality, randomWords[rand.Intn(len(randomWords))])
		items = append(items, generateItemXML(title, i))
	}

	for i := 5; i < 30; i++ {
		rejection := rejectionTerms[rand.Intn(len(rejectionTerms))]
		title := fmt.Sprintf("%s S%02dE%02d %s %s", seriesName, requestedSeason, requestedEp, goodQualities[rand.Intn(len(goodQualities))], rejection)
		items = append(items, generateItemXML(title, i))
	}

	for i := 30; i < 50; i++ {
		quality := goodQualities[rand.Intn(len(goodQualities))]
		extra := randomWords[rand.Intn(len(randomWords))]
		title := fmt.Sprintf("%s S%02dE%02d %s %s", nameToUse, requestedSeason, requestedEp, quality, extra)
		items = append(items, generateItemXML(title, i))
	}

	rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	response := fmt.Sprintf(`<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed"><channel><title>Fake Acme Indexer</title><description>Fake search results for %s</description><link>http://localhost:8080</link>%s</channel></rss>`, seriesName, strings.Join(items, "\n"))
	fmt.Fprint(w, response)
}

func generateItemXML(title string, id int) string {
	seeders := rand.Intn(100) + 1
	leechers := rand.Intn(20)
	size := rand.Int63n(2000000000) + 500000000
	pubDate := time.Now().Add(-time.Duration(rand.Intn(24*30)) * time.Hour).Format(time.RFC1123Z)
	encodedTitle := url.QueryEscape(title)

	fakeMagnet := fmt.Sprintf("magnet:?xt=urn:btih:fakesha1hash%d&dn=%s", id, encodedTitle)
	xmlEscapedMagnet := strings.ReplaceAll(fakeMagnet, "&", "&amp;")

	return fmt.Sprintf(`
    <item>
      <title>%s</title>
      <link>%s</link>
      <guid>%s</guid>
      <pubDate>%s</pubDate>
      <size>%d</size>
      <torznab:attr name="seeders" value="%d"/>
      <torznab:attr name="leechers" value="%d"/>
    </item>`, title, xmlEscapedMagnet, xmlEscapedMagnet, pubDate, size, seeders, leechers)
}
