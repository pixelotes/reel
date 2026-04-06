package indexers

import (
	"fmt"
	"time"

	"reel/internal/clients/metadata"
)

// MangaDexIndexer implements indexers.Client for MangaDex.
// Returns results with mangadex:// URLs that DirectDownloadClient handles.
type MangaDexIndexer struct {
	client    *metadata.MangaDexClient
	languages []string
}

func NewMangaDexIndexer(timeout time.Duration, languages []string, defaultLang string) *MangaDexIndexer {
	if len(languages) == 0 {
		languages = []string{defaultLang}
	}
	return &MangaDexIndexer{
		client:    metadata.NewMangaDexClient(timeout, languages[0]),
		languages: languages,
	}
}

func (m *MangaDexIndexer) SearchMovies(query string, _ string, _ string) ([]IndexerResult, error) {
	return m.searchChapter(query, 0, 0)
}

func (m *MangaDexIndexer) SearchTVShows(query string, season, episode int, _ string) ([]IndexerResult, error) {
	return m.searchChapter(query, season, episode)
}

func (m *MangaDexIndexer) searchChapter(query string, volume, chapter int) ([]IndexerResult, error) {
	// Search for manga
	mangas, err := m.client.SearchManga(query)
	if err != nil {
		return nil, err
	}
	if len(mangas) == 0 {
		return nil, nil
	}

	// Use first match
	manga := mangas[0]

	// Get chapters with language fallback
	chapters, err := m.client.GetMangaChapters(manga.ID, m.languages)
	if err != nil {
		return nil, err
	}

	var results []IndexerResult
	for _, ch := range chapters {
		// If specific chapter requested, filter
		if chapter > 0 {
			var chNum float64
			fmt.Sscanf(ch.Chapter, "%f", &chNum)
			if int(chNum) != chapter {
				continue
			}
		}

		title := fmt.Sprintf("%s - Chapter %s", manga.Title, ch.Chapter)
		if ch.Title != "" {
			title += " - " + ch.Title
		}
		title += fmt.Sprintf(" [%s]", ch.Language)

		results = append(results, IndexerResult{
			Title:       title,
			DownloadURL: "mangadex://chapter/" + ch.ID,
			Seeders:     ch.Pages, // Use page count as proxy for "quality"
			Size:        int64(ch.Pages) * 500 * 1024, // ~500KB per page estimate
			Indexer:     "MangaDex",
			PublishDate: parseTime(ch.CreatedAt),
		})
	}

	return results, nil
}

func (m *MangaDexIndexer) HealthCheck() (bool, error) {
	_, err := m.client.SearchManga("test")
	return err == nil, err
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
