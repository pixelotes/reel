package services

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"reel/internal/clients/torrent"
	"reel/internal/database"
	"reel/internal/database/models"
)

// fakeTorrentClient is a TorrentClient that records removals.
type fakeTorrentClient struct {
	mu        sync.Mutex
	status    torrent.TorrentStatus
	removed   []string
	removeErr error
}

func (f *fakeTorrentClient) AddTorrent(string, string) (string, error)     { return "h", nil }
func (f *fakeTorrentClient) AddTorrentFile([]byte, string) (string, error) { return "h", nil }
func (f *fakeTorrentClient) AddTrackers(string, []string) error            { return nil }
func (f *fakeTorrentClient) HealthCheck() (bool, error)                    { return true, nil }
func (f *fakeTorrentClient) GetTorrentStatus(string) (torrent.TorrentStatus, error) {
	return f.status, nil
}
func (f *fakeTorrentClient) RemoveTorrent(hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, "remove:"+hash)
	return f.removeErr
}
func (f *fakeTorrentClient) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.removed...)
}

// fakeKeepDataClient additionally supports removal that keeps data.
type fakeKeepDataClient struct{ fakeTorrentClient }

func (f *fakeKeepDataClient) RemoveTorrentKeepData(hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, "keep:"+hash)
	return f.removeErr
}

func newTestRepo(t *testing.T) *models.MediaRepository {
	t.Helper()
	db, err := database.NewSQLite(filepath.Join(t.TempDir(), "reel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(db, testLogger()); err != nil {
		t.Fatal(err)
	}
	return models.NewMediaRepository(db, testLogger())
}

func strp(s string) *string { return &s }

func newTestDownloader(t *testing.T, client torrent.TorrentClient, pp *PostProcessor) (*DownloaderService, *models.MediaRepository) {
	t.Helper()
	repo := newTestRepo(t)
	cfg := testConfig(t.TempDir(), t.TempDir())
	d := NewDownloaderService(cfg, testLogger(), client, repo, pp, nil)
	return d, repo
}

func createMovie(t *testing.T, repo *models.MediaRepository, status models.MediaStatus, hash string) *models.Media {
	t.Helper()
	m := &models.Media{Type: models.MediaTypeMovie, Title: "Big Buck Bunny", Year: 2008,
		MinQuality: "360p", MaxQuality: "2160p", Status: status}
	if err := repo.Create(m); err != nil {
		t.Fatal(err)
	}
	if hash != "" {
		if err := repo.UpdateDownloadInfo(m.ID, status, &hash, strp("name")); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.GetByID(m.ID)
	if err != nil || got == nil {
		t.Fatalf("reload media: %v", err)
	}
	return got
}

// countingPostProcessor builds a PostProcessor whose pipeline counts executions
// and blocks each one until release is closed.
func countingPostProcessor(t *testing.T, release <-chan struct{}) (*PostProcessor, *int32) {
	t.Helper()
	var runs int32
	stage := &spyStage{name: "count", executeFunc: func(*ProcessingContext) error {
		atomic.AddInt32(&runs, 1)
		<-release
		return nil
	}}
	cfg := testConfig(t.TempDir(), t.TempDir())
	pp := &PostProcessor{config: cfg, logger: testLogger(), pipeline: NewPipeline(cfg, testLogger(), []Stage{stage})}
	return pp, &runs
}

func completedStatus(hash string) torrent.TorrentStatus {
	return torrent.TorrentStatus{Hash: hash, Name: "Big.Buck.Bunny.1080p", Progress: 1,
		IsCompleted: true, DownloadDir: "/dl", Files: []string{"a.mp4", "b.mp4"}}
}

func TestProcessDownload_SingleFlight(t *testing.T) {
	release := make(chan struct{})
	pp, runs := countingPostProcessor(t, release)
	media := models.Media{ID: 9001, Type: models.MediaTypeMovie, Title: "Dup"}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := pp.ProcessDownload(media, completedStatus("h"), 0, 0, "/dl"); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	time.Sleep(100 * time.Millisecond) // let all callers reach the claim
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(runs); got != 1 {
		t.Fatalf("expected pipeline to run once, ran %d times", got)
	}

	// Once finished, the same media can be processed again (claim was released).
	if err := pp.ProcessDownload(media, completedStatus("h"), 0, 0, "/dl"); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(runs); got != 2 {
		t.Fatalf("expected a second sequential run, got %d runs", got)
	}
}

func TestProcessDownload_DifferentEpisodesRunConcurrently(t *testing.T) {
	release := make(chan struct{})
	pp, runs := countingPostProcessor(t, release)
	media := models.Media{ID: 9002, Type: models.MediaTypeTVShow, Title: "Show"}

	var wg sync.WaitGroup
	for ep := 1; ep <= 3; ep++ {
		wg.Add(1)
		go func(ep int) {
			defer wg.Done()
			pp.ProcessDownload(media, completedStatus("h"), 1, ep, "/dl")
		}(ep)
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(runs); got != 3 {
		t.Fatalf("expected 3 runs for 3 episodes, got %d", got)
	}
}

func TestUpdateDownloadStatus_MovieProcessedOnce(t *testing.T) {
	release := make(chan struct{})
	pp, runs := countingPostProcessor(t, release)
	client := &fakeTorrentClient{status: completedStatus("hash1")}
	d, repo := newTestDownloader(t, client, pp)
	m := createMovie(t, repo, models.StatusDownloading, "hash1")

	d.UpdateDownloadStatus()
	d.UpdateDownloadStatus() // next scheduler tick while processing still runs
	time.Sleep(100 * time.Millisecond)
	close(release)
	time.Sleep(100 * time.Millisecond)

	if got := atomic.LoadInt32(runs); got != 1 {
		t.Fatalf("expected 1 post-processing run, got %d", got)
	}
	got, _ := repo.GetByID(m.ID)
	if got.Status != models.StatusDownloaded {
		t.Errorf("expected status downloaded, got %s", got.Status)
	}
}

func TestRemoveMediaFromClient_Movie(t *testing.T) {
	client := &fakeKeepDataClient{}
	d, repo := newTestDownloader(t, client, nil)
	m := createMovie(t, repo, models.StatusDownloading, "abc")

	d.RemoveMediaFromClient(m)

	got := client.calls()
	if len(got) != 1 || got[0] != "keep:abc" {
		t.Fatalf("expected data-preserving removal of abc, got %v", got)
	}
}

func TestRemoveMediaFromClient_NoHashNoCall(t *testing.T) {
	client := &fakeKeepDataClient{}
	d, repo := newTestDownloader(t, client, nil)
	m := createMovie(t, repo, models.StatusPending, "")

	d.RemoveMediaFromClient(m)

	if got := client.calls(); len(got) != 0 {
		t.Fatalf("expected no removal, got %v", got)
	}
}

func TestRemoveMediaFromClient_FallsBackToRemoveTorrent(t *testing.T) {
	client := &fakeTorrentClient{} // no KeepDataRemover
	d, repo := newTestDownloader(t, client, nil)
	m := createMovie(t, repo, models.StatusDownloading, "abc")

	d.RemoveMediaFromClient(m)

	if got := client.calls(); len(got) != 1 || got[0] != "remove:abc" {
		t.Fatalf("expected RemoveTorrent fallback, got %v", got)
	}
}

func TestRemoveMediaFromClient_ErrorDoesNotPanicOrPropagate(t *testing.T) {
	client := &fakeKeepDataClient{fakeTorrentClient{removeErr: errors.New("boom")}}
	d, repo := newTestDownloader(t, client, nil)
	m := createMovie(t, repo, models.StatusDownloading, "abc")

	d.RemoveMediaFromClient(m) // must return normally (no error return value)

	if got := client.calls(); len(got) != 1 {
		t.Fatalf("expected removal attempt, got %v", got)
	}
}

func TestRemoveMediaFromClient_TVEpisodesDeduped(t *testing.T) {
	client := &fakeKeepDataClient{}
	d, repo := newTestDownloader(t, client, nil)

	show := &models.TVShow{Status: "Running"}
	if err := repo.CreateTVShow(show); err != nil {
		t.Fatal(err)
	}
	season := &models.Season{ShowID: show.ID, SeasonNumber: 1}
	if err := repo.CreateSeason(season); err != nil {
		t.Fatal(err)
	}
	media := &models.Media{Type: models.MediaTypeTVShow, Title: "Show", Year: 2020,
		MinQuality: "360p", MaxQuality: "2160p", Status: models.StatusDownloading, TVShowID: &show.ID}
	if err := repo.Create(media); err != nil {
		t.Fatal(err)
	}
	for ep, hash := range map[int]string{1: "h1", 2: "h2", 3: "h1"} { // 1 and 3 share a pack
		e := &models.Episode{SeasonID: season.ID, EpisodeNumber: ep, Title: "e", Status: models.StatusDownloading}
		if err := repo.CreateEpisode(e); err != nil {
			t.Fatal(err)
		}
		if err := repo.UpdateEpisodeDownloadInfo(media.ID, 1, ep, models.StatusDownloading, strp(hash), strp("n")); err != nil {
			t.Fatal(err)
		}
	}

	d.RemoveMediaFromClient(media)

	got := map[string]int{}
	for _, c := range client.calls() {
		got[c]++
	}
	if len(got) != 2 || got["keep:h1"] != 1 || got["keep:h2"] != 1 {
		t.Fatalf("expected one data-preserving removal each of h1,h2; got %v", got)
	}
}

func TestRemoveMediaFromClient_EbookUsesDirectClient(t *testing.T) {
	torrentC := &fakeKeepDataClient{}
	directC := &fakeKeepDataClient{}
	d, repo := newTestDownloader(t, torrentC, nil)
	d.directClient = directC

	m := &models.Media{Type: models.MediaTypeEbook, Title: "Book", Year: 2000,
		MinQuality: "any", MaxQuality: "any", Status: models.StatusDownloading}
	if err := repo.Create(m); err != nil {
		t.Fatal(err)
	}
	m.TorrentHash = strp("ebook1")

	d.RemoveMediaFromClient(m)

	if got := directC.calls(); len(got) != 1 || got[0] != "keep:ebook1" {
		t.Fatalf("expected direct client removal, got %v", got)
	}
	if got := torrentC.calls(); len(got) != 0 {
		t.Fatalf("torrent client must not be used for ebooks, got %v", got)
	}
}
