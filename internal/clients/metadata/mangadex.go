package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"reel/internal/version"
)

const mangaDexBaseURL = "https://api.mangadex.org"

type MangaDexClient struct {
	baseURL    string
	httpClient *http.Client
	language   string
}

// MangaDex API response structures

type mdResponse struct {
	Result string          `json:"result"`
	Data   json.RawMessage `json:"data"`
	Total  int             `json:"total"`
}

type mdManga struct {
	ID         string `json:"id"`
	Attributes struct {
		Title       map[string]string `json:"title"`
		Description map[string]string `json:"description"`
		Status      string            `json:"status"`
		Year        *int              `json:"year"`
	} `json:"attributes"`
	Relationships []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes *struct {
			Name     string `json:"name"`
			FileName string `json:"fileName"`
		} `json:"attributes,omitempty"`
	} `json:"relationships"`
}

type mdChapter struct {
	ID         string `json:"id"`
	Attributes struct {
		Chapter            string `json:"chapter"`
		Volume             string `json:"volume"`
		Title              string `json:"title"`
		TranslatedLanguage string `json:"translatedLanguage"`
		Pages              int    `json:"pages"`
		CreatedAt          string `json:"createdAt"`
	} `json:"attributes"`
}

type mdAtHomeResponse struct {
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		Data      []string `json:"data"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
}

func NewMangaDexClient(timeout time.Duration, language string) *MangaDexClient {
	if language == "" {
		language = "en"
	}
	return &MangaDexClient{
		baseURL:    mangaDexBaseURL,
		httpClient: &http.Client{Timeout: timeout},
		language:   language,
	}
}

func (c *MangaDexClient) doRequest(reqURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.GetUserAgent())
	return c.httpClient.Do(req)
}

func (c *MangaDexClient) SearchManga(title string) ([]*MangaResult, error) {
	params := url.Values{}
	params.Set("title", title)
	params.Set("limit", "5")
	params.Add("includes[]", "cover_art")
	params.Add("includes[]", "author")

	reqURL := fmt.Sprintf("%s/manga?%s", c.baseURL, params.Encode())
	resp, err := c.doRequest(reqURL)
	if err != nil {
		return nil, fmt.Errorf("MangaDex search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaDex API error: %s", resp.Status)
	}

	var apiResp struct {
		Data []mdManga `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode MangaDex response: %w", err)
	}

	var results []*MangaResult
	for _, m := range apiResp.Data {
		// Get title (prefer English, fallback to first available)
		title := ""
		if t, ok := m.Attributes.Title["en"]; ok {
			title = t
		} else if t, ok := m.Attributes.Title["ja-ro"]; ok {
			title = t
		} else {
			for _, t := range m.Attributes.Title {
				title = t
				break
			}
		}

		// Get description
		desc := ""
		if d, ok := m.Attributes.Description["en"]; ok {
			desc = d
		}

		// Get author from relationships
		author := ""
		for _, rel := range m.Relationships {
			if rel.Type == "author" && rel.Attributes != nil {
				author = rel.Attributes.Name
				break
			}
		}

		// Get cover from relationships
		coverURL := ""
		for _, rel := range m.Relationships {
			if rel.Type == "cover_art" && rel.Attributes != nil && rel.Attributes.FileName != "" {
				coverURL = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s.256.jpg", m.ID, rel.Attributes.FileName)
				break
			}
		}

		year := 0
		if m.Attributes.Year != nil {
			year = *m.Attributes.Year
		}

		results = append(results, &MangaResult{
			ID:          m.ID,
			Title:       title,
			Author:      author,
			Year:        year,
			Description: desc,
			CoverURL:    coverURL,
			Status:      m.Attributes.Status,
		})
	}

	return results, nil
}

func (c *MangaDexClient) GetMangaChapters(mangaID string, languages []string) ([]MangaChapter, error) {
	if len(languages) == 0 {
		languages = []string{c.language}
	}

	// Fetch all chapters for all requested languages
	var allChapters []mdChapter
	for _, lang := range languages {
		chapters, err := c.fetchChaptersForLanguage(mangaID, lang)
		if err != nil {
			continue // Try next language
		}
		allChapters = append(allChapters, chapters...)
	}

	// Deduplicate: for each chapter number, keep the best version
	// Priority: first language in the list > later languages, then newest upload
	best := make(map[string]mdChapter)      // chapter number -> best chapter
	bestLangPri := make(map[string]int)      // chapter number -> language priority (lower = better)

	for _, ch := range allChapters {
		num := ch.Attributes.Chapter
		if num == "" {
			continue
		}

		langPri := len(languages) // Worst priority
		for i, lang := range languages {
			if ch.Attributes.TranslatedLanguage == lang {
				langPri = i
				break
			}
		}

		existing, exists := best[num]
		if !exists {
			best[num] = ch
			bestLangPri[num] = langPri
		} else if langPri < bestLangPri[num] {
			// Better language priority
			best[num] = ch
			bestLangPri[num] = langPri
		} else if langPri == bestLangPri[num] && ch.Attributes.CreatedAt > existing.Attributes.CreatedAt {
			// Same language, newer upload
			best[num] = ch
		}
	}

	// Convert to sorted result
	var results []MangaChapter
	for _, ch := range best {
		results = append(results, MangaChapter{
			ID:        ch.ID,
			Chapter:   ch.Attributes.Chapter,
			Volume:    ch.Attributes.Volume,
			Title:     ch.Attributes.Title,
			Language:  ch.Attributes.TranslatedLanguage,
			Pages:     ch.Attributes.Pages,
			CreatedAt: ch.Attributes.CreatedAt,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		var ni, nj float64
		fmt.Sscanf(results[i].Chapter, "%f", &ni)
		fmt.Sscanf(results[j].Chapter, "%f", &nj)
		return ni < nj
	})

	return results, nil
}

func (c *MangaDexClient) fetchChaptersForLanguage(mangaID, lang string) ([]mdChapter, error) {
	var all []mdChapter
	offset := 0
	limit := 500

	for {
		params := url.Values{}
		params.Add("translatedLanguage[]", lang)
		params.Set("order[chapter]", "asc")
		params.Set("limit", fmt.Sprintf("%d", limit))
		params.Set("offset", fmt.Sprintf("%d", offset))

		reqURL := fmt.Sprintf("%s/manga/%s/feed?%s", c.baseURL, mangaID, params.Encode())
		resp, err := c.doRequest(reqURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("MangaDex feed error: %s", resp.Status)
		}

		var apiResp struct {
			Data  []mdChapter `json:"data"`
			Total int         `json:"total"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return nil, err
		}

		all = append(all, apiResp.Data...)

		if offset+limit >= apiResp.Total {
			break
		}
		offset += limit
	}

	return all, nil
}

func (c *MangaDexClient) GetChapterPages(chapterID string) (string, []string, error) {
	reqURL := fmt.Sprintf("%s/at-home/server/%s", c.baseURL, chapterID)
	resp, err := c.doRequest(reqURL)
	if err != nil {
		return "", nil, fmt.Errorf("MangaDex at-home failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("MangaDex at-home error: %s", resp.Status)
	}

	var atHome mdAtHomeResponse
	if err := json.NewDecoder(resp.Body).Decode(&atHome); err != nil {
		return "", nil, err
	}

	// Build full image URLs using data-saver for lower bandwidth
	var urls []string
	files := atHome.Chapter.DataSaver
	if len(files) == 0 {
		files = atHome.Chapter.Data // Fallback to full quality
	}
	for _, f := range files {
		quality := "data-saver"
		if len(atHome.Chapter.DataSaver) == 0 {
			quality = "data"
		}
		urls = append(urls, fmt.Sprintf("%s/%s/%s/%s", atHome.BaseURL, quality, atHome.Chapter.Hash, f))
	}

	return atHome.BaseURL, urls, nil
}

// Implement metadata.Client interface (not supported for manga provider)
func (c *MangaDexClient) SearchMovie(title string, year int) ([]*MovieResult, error) {
	return nil, fmt.Errorf("MangaDex does not support movie search")
}

func (c *MangaDexClient) SearchTVShow(title string) ([]*TVShowResult, error) {
	return nil, fmt.Errorf("MangaDex does not support TV show search")
}

func (c *MangaDexClient) GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error) {
	return nil, fmt.Errorf("MangaDex does not support TV show details")
}
