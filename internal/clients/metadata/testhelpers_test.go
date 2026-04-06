package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newFakeGoogleBooksServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/volumes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"totalItems": 2,
			"items": [
				{
					"id": "gb123",
					"volumeInfo": {
						"title": "Dune",
						"authors": ["Frank Herbert"],
						"publishedDate": "1965-08-01",
						"description": "A science fiction novel about the desert planet Arrakis.",
						"industryIdentifiers": [
							{"type": "ISBN_13", "identifier": "9780441013593"},
							{"type": "ISBN_10", "identifier": "0441013597"}
						],
						"pageCount": 412,
						"averageRating": 4.5,
						"imageLinks": {
							"thumbnail": "http://books.google.com/books/content?id=gb123"
						}
					}
				},
				{
					"id": "gb456",
					"volumeInfo": {
						"title": "Dune Messiah",
						"authors": ["Frank Herbert"],
						"publishedDate": "1969",
						"description": "The sequel to Dune.",
						"pageCount": 256,
						"averageRating": 4.0
					}
				}
			]
		}`)
	})
	return httptest.NewServer(mux)
}

func newFakeOpenLibraryServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/search.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := openLibraryResponse{
			NumFound: 2,
			Docs: []struct {
				Key              string   `json:"key"`
				Title            string   `json:"title"`
				AuthorName       []string `json:"author_name"`
				FirstPublishYear int      `json:"first_publish_year"`
				CoverI           int      `json:"cover_i"`
			}{
				{
					Key:              "/works/OL893415W",
					Title:            "Dune",
					AuthorName:       []string{"Frank Herbert"},
					FirstPublishYear: 1965,
					CoverI:           5428803,
				},
				{
					Key:              "/works/OL893416W",
					Title:            "Dune Messiah",
					AuthorName:       []string{"Frank Herbert"},
					FirstPublishYear: 1969,
					CoverI:           0,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}

func newEmptyServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"totalItems": 0, "items": []}`)
	})
	return httptest.NewServer(mux)
}

func newFakeGutendexServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"count": 1,
			"results": [
				{
					"id": 1661,
					"title": "The Adventures of Sherlock Holmes",
					"authors": [
						{"name": "Arthur Conan Doyle", "birth_year": 1859, "death_year": 1930}
					],
					"subjects": ["Detective and mystery stories", "Fiction"],
					"languages": ["en"],
					"formats": {
						"application/epub+zip": "https://www.gutenberg.org/ebooks/1661.epub.images"
					}
				}
			]
		}`)
	})
	return httptest.NewServer(mux)
}
