// Package scoutclient connects Reel to scout, the shared metadata and
// subtitle library. It turns Reel's config into a scout backend and holds the
// subtitle download logic the pipeline, the post-processor and the library
// scan share.
package scoutclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pixelotes/scout"
	"github.com/pixelotes/scout/client"
	scoutcfg "github.com/pixelotes/scout/config"
	"github.com/pixelotes/scout/moviehash"

	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// scoutProviders are the provider names scout knows. Book providers stay in
// Reel and are filtered out here.
var scoutProviders = map[string]bool{"tmdb": true, "imdb": true, "tvmaze": true, "anilist": true, "kitsu": true, "trakt": true}

// New builds the scout backend from Reel's config.
func New(cfg *config.Config, logger *utils.Logger) (scout.Backend, error) {
	timeout := time.Duration(cfg.Metadata.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	sc := scoutcfg.Default()
	sc.Backends = cfg.Crawlers.Backends
	sc.Service.URL = cfg.Crawlers.Service.URL
	sc.Service.Token = cfg.Crawlers.Service.Token
	sc.Service.Timeout = time.Duration(cfg.Crawlers.Service.Timeout) * time.Second

	sc.Local.Language = cfg.Metadata.Language
	sc.Local.Timeout = timeout
	sc.Local.Providers = map[scout.Kind][]string{
		scout.Movie: Providers(cfg, models.MediaTypeMovie),
		scout.Show:  Providers(cfg, models.MediaTypeTVShow),
		scout.Anime: Providers(cfg, models.MediaTypeAnime),
	}
	if !KeyMissing(cfg.Metadata.TMDB.APIKey) {
		sc.Local.TMDB.APIKey = cfg.Metadata.TMDB.APIKey
	} else if hasProvider(cfg, "tmdb") && hasBackend(cfg, "local") {
		logger.Warn("TMDB metadata disabled: metadata.tmdb.api_key is not set (get a free key at themoviedb.org)")
	}
	if !KeyMissing(cfg.Metadata.Trakt.ClientID) {
		sc.Local.Trakt.ClientID = cfg.Metadata.Trakt.ClientID
	}
	if !KeyMissing(cfg.Subtitles.APIKey) {
		sc.Local.OpenSubtitles.APIKey = cfg.Subtitles.APIKey
	}
	if !KeyMissing(cfg.Subtitles.SubDLAPIKey) {
		sc.Local.SubDL.APIKey = cfg.Subtitles.SubDLAPIKey
	}
	sc.Local.Subdivx.FlareSolverrURL = cfg.Subtitles.FlareSolverrURL
	if len(cfg.Subtitles.Providers) > 0 {
		sc.Local.SubtitleProviders = cfg.Subtitles.Providers
	}
	sc.Local.SubtitleLanguages = cfg.Subtitles.Languages
	sc.Local.Cache.MetadataTTL = 6 * time.Hour
	if cfg.App.DataPath != "" {
		sc.Local.Cache.SubtitleDir = filepath.Join(cfg.App.DataPath, "subtitle-cache")
	}

	level := slog.LevelWarn
	if cfg.App.Debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})).With("component", "scout")
	b, err := client.New(sc, log)
	if err != nil {
		return nil, err
	}
	logger.Info("Metadata and subtitles via scout, backends:", strings.Join(sc.Backends, " → "))
	return b, nil
}

// KeyMissing reports whether a key is unset or still the REPLACE_WITH_...
// placeholder from the sample config.
func KeyMissing(key string) bool {
	key = strings.TrimSpace(key)
	return key == "" || strings.HasPrefix(key, "REPLACE_WITH_")
}

func hasBackend(cfg *config.Config, name string) bool {
	for _, b := range cfg.Crawlers.Backends {
		if b == name {
			return true
		}
	}
	return false
}

func hasProvider(cfg *config.Config, name string) bool {
	for _, t := range []models.MediaType{models.MediaTypeMovie, models.MediaTypeTVShow, models.MediaTypeAnime} {
		for _, p := range Providers(cfg, t) {
			if p == name {
				return true
			}
		}
	}
	return false
}

// Kind maps a Reel media type to a scout kind. Ebooks have none.
func Kind(t models.MediaType) (scout.Kind, bool) {
	switch t {
	case models.MediaTypeMovie:
		return scout.Movie, true
	case models.MediaTypeTVShow:
		return scout.Show, true
	case models.MediaTypeAnime:
		return scout.Anime, true
	}
	return "", false
}

// Providers is the configured provider order for a media type, limited to
// the ones scout handles.
func Providers(cfg *config.Config, t models.MediaType) []string {
	var list []string
	switch t {
	case models.MediaTypeMovie:
		list = cfg.Movies.Providers
	case models.MediaTypeTVShow:
		list = cfg.TVShows.Providers
	case models.MediaTypeAnime:
		list = cfg.Anime.Providers
	}
	var out []string
	for _, p := range list {
		if scoutProviders[p] {
			out = append(out, p)
		}
	}
	return out
}

// FetchSubtitles downloads the best subtitle for each language in langs that
// has no <video>.<lang>.srt yet, and returns how many it wrote. It hashes the
// file here, so a scout service never needs access to the media.
func FetchSubtitles(ctx context.Context, b scout.Backend, media *models.Media, videoPath string, season, episode int, langs []string, logger *utils.Logger) (int, error) {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	var missing []string
	for _, l := range langs {
		if _, err := os.Stat(base + "." + l + ".srt"); err != nil {
			missing = append(missing, l)
		}
	}
	if len(missing) == 0 {
		return 0, nil
	}

	q := scout.SubtitleQuery{Season: season, Episode: episode, Languages: missing, Query: media.Title}
	if h, size, err := moviehash.File(videoPath); err == nil {
		q.Hash, q.Size = h, size
	} else {
		logger.Warn("Subtitles: cannot hash", filepath.Base(videoPath), ":", err)
	}
	q.IMDbID = media.IMDBId
	if media.TMDBId != nil {
		q.TMDbID = strconv.Itoa(*media.TMDBId)
	}
	if season == 0 && episode == 0 && media.Year > 0 {
		q.Query = fmt.Sprintf("%s %d", media.Title, media.Year)
	}

	subs, err := b.SearchSubtitles(ctx, q)
	if errors.Is(err, scout.ErrNotFound) {
		logger.Info("Subtitles: none found for", filepath.Base(videoPath))
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	written := 0
	for _, lang := range missing {
		best := scout.BestSubtitle(subs, lang)
		if best == nil {
			logger.Info("Subtitles: no", lang, "subtitle for", filepath.Base(videoPath))
			continue
		}
		f, err := b.DownloadSubtitle(ctx, best.Provider, best.FileID)
		if errors.Is(err, scout.ErrQuotaExceeded) {
			return written, err
		}
		if err != nil {
			logger.Warn("Subtitles: failed to download", lang, "for", filepath.Base(videoPath), ":", err)
			continue
		}
		dest := base + "." + lang + ".srt"
		if err := writeAtomic(dest, f.Data); err != nil {
			return written, err
		}
		written++
		logger.Info(fmt.Sprintf("Subtitles: %s → %s (matched by %s)", lang, filepath.Base(dest), best.MatchedBy))
	}
	return written, nil
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
