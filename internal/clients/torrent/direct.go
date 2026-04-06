package torrent

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"reel/internal/utils"
)

// DirectDownloadClient implements TorrentClient for direct HTTP downloads.
// Used for downloading files (ebooks, etc.) directly from URLs without a torrent client.
type DirectDownloadClient struct {
	downloads map[string]*directDownload
	mu        sync.RWMutex
	logger    *utils.Logger
	client    *http.Client
	mangaSem  chan struct{} // serializes manga chapter downloads
}

type directDownload struct {
	status      TorrentStatus
	url         string
	destPath    string
	totalBytes  int64
	writtenBytes int64
	err         error
	done        bool
}

func NewDirectDownloadClient(logger *utils.Logger) *DirectDownloadClient {
	return &DirectDownloadClient{
		downloads: make(map[string]*directDownload),
		logger:    logger,
		client:    &http.Client{Timeout: 30 * time.Minute},
		mangaSem:  make(chan struct{}, 1), // only one manga chapter at a time
	}
}

func (c *DirectDownloadClient) AddTorrent(downloadURL string, downloadPath string) (string, error) {
	if err := os.MkdirAll(downloadPath, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create download dir: %w", err)
	}

	hash := fmt.Sprintf("%x", sha1.Sum([]byte(downloadURL+time.Now().String())))

	fileName := filenameFromURL(downloadURL)
	filePath := filepath.Join(downloadPath, fileName)

	dl := &directDownload{
		url:      downloadURL,
		destPath: filePath,
		status: TorrentStatus{
			Hash:        hash,
			Name:        fileName,
			Progress:    0,
			Files:       []string{fileName},
			DownloadDir: downloadPath,
			IsCompleted: false,
		},
	}

	c.mu.Lock()
	c.downloads[hash] = dl
	c.mu.Unlock()

	go c.downloadFile(dl)

	c.logger.Info("DirectDownload: Starting download:", fileName)
	return hash, nil
}

func (c *DirectDownloadClient) AddTorrentFile(fileContent []byte, downloadPath string) (string, error) {
	return "", fmt.Errorf("DirectDownloadClient does not support torrent files")
}

func (c *DirectDownloadClient) GetTorrentStatus(hash string) (TorrentStatus, error) {
	c.mu.RLock()
	dl, ok := c.downloads[hash]
	c.mu.RUnlock()

	if !ok {
		return TorrentStatus{}, fmt.Errorf("download not found: %s", hash)
	}

	if dl.err != nil {
		return TorrentStatus{}, dl.err
	}

	return dl.status, nil
}

func (c *DirectDownloadClient) RemoveTorrent(hash string) error {
	c.mu.Lock()
	delete(c.downloads, hash)
	c.mu.Unlock()
	return nil
}

func (c *DirectDownloadClient) AddTrackers(hash string, trackers []string) error {
	return nil // No-op for direct downloads
}

func (c *DirectDownloadClient) HealthCheck() (bool, error) {
	return true, nil
}

// chapterDelay is the delay between downloading consecutive manga chapters.
const chapterDelay = 30 * time.Second

// pageDelay is the delay between downloading individual pages within a chapter.
const pageDelay = 200 * time.Millisecond

func (c *DirectDownloadClient) downloadFile(dl *directDownload) {
	// Handle MangaDex chapter downloads (pages -> CBZ)
	// Serialize manga downloads so we don't flood the server.
	if strings.HasPrefix(dl.url, "mangadex://chapter/") {
		c.mangaSem <- struct{}{} // wait for our turn
		c.downloadMangaChapter(dl)
		time.Sleep(chapterDelay) // respect rate limit before releasing
		<-c.mangaSem
		return
	}

	resp, err := c.client.Get(dl.url)
	if err != nil {
		dl.err = fmt.Errorf("download failed: %w", err)
		c.logger.Error("DirectDownload: HTTP request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dl.err = fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
		c.logger.Error("DirectDownload: Bad status:", resp.StatusCode)
		return
	}

	// Update filename from the final URL (after redirects) for a proper extension
	finalURL := resp.Request.URL.String()
	if finalURL != dl.url {
		newName := filenameFromURL(finalURL)
		dir := filepath.Dir(dl.destPath)
		dl.destPath = filepath.Join(dir, newName)
		c.mu.Lock()
		dl.status.Name = newName
		dl.status.Files = []string{newName}
		c.mu.Unlock()
		c.logger.Info("DirectDownload: Resolved filename after redirect:", newName)
	}

	dl.totalBytes = resp.ContentLength

	outFile, err := os.Create(dl.destPath)
	if err != nil {
		dl.err = fmt.Errorf("failed to create file: %w", err)
		return
	}

	// Track progress during download
	writer := &progressWriter{
		file: outFile,
		dl:   dl,
	}

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		outFile.Close()
		os.Remove(dl.destPath)
		dl.err = fmt.Errorf("download write failed: %w", err)
		return
	}

	if err = outFile.Sync(); err != nil {
		outFile.Close()
		os.Remove(dl.destPath)
		dl.err = fmt.Errorf("sync failed: %w", err)
		return
	}
	outFile.Close()

	// Mark as completed
	c.mu.Lock()
	dl.status.Progress = 1.0
	dl.status.IsCompleted = true
	dl.done = true
	c.mu.Unlock()

	c.logger.Info("DirectDownload: Completed:", dl.status.Name)
}

// progressWriter wraps a file writer to track download progress.
type progressWriter struct {
	file *os.File
	dl   *directDownload
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.file.Write(p)
	if err != nil {
		return n, err
	}

	pw.dl.writtenBytes += int64(n)
	if pw.dl.totalBytes > 0 {
		pw.dl.status.Progress = float64(pw.dl.writtenBytes) / float64(pw.dl.totalBytes)
	}

	return n, nil
}

// filenameFromURL extracts a filename from a URL, falling back to a default.
func filenameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		path := parsed.Path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			name := path[idx+1:]
			if name != "" {
				decoded, err := url.QueryUnescape(name)
				if err == nil {
					return decoded
				}
				return name
			}
		}
	}
	return fmt.Sprintf("download_%d", time.Now().UnixNano())
}

// downloadMangaChapter downloads all pages from a MangaDex chapter and packages them as a CBZ file.
func (c *DirectDownloadClient) downloadMangaChapter(dl *directDownload) {
	chapterID := strings.TrimPrefix(dl.url, "mangadex://chapter/")

	// 1. Get page URLs from MangaDex at-home API
	atHomeURL := fmt.Sprintf("https://api.mangadex.org/at-home/server/%s", chapterID)
	req, err := http.NewRequest("GET", atHomeURL, nil)
	if err != nil {
		dl.err = fmt.Errorf("failed to create at-home request: %w", err)
		return
	}
	req.Header.Set("User-Agent", "Reel/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		dl.err = fmt.Errorf("at-home request failed: %w", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dl.err = fmt.Errorf("at-home error: HTTP %d", resp.StatusCode)
		return
	}

	var atHome struct {
		BaseURL string `json:"baseUrl"`
		Chapter struct {
			Hash      string   `json:"hash"`
			Data      []string `json:"data"`
			DataSaver []string `json:"dataSaver"`
		} `json:"chapter"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&atHome); err != nil {
		dl.err = fmt.Errorf("failed to decode at-home response: %w", err)
		return
	}

	// Use data-saver (lower bandwidth), fallback to full quality
	files := atHome.Chapter.DataSaver
	quality := "data-saver"
	if len(files) == 0 {
		files = atHome.Chapter.Data
		quality = "data"
	}
	if len(files) == 0 {
		dl.err = fmt.Errorf("no pages found for chapter %s", chapterID)
		return
	}

	dl.totalBytes = int64(len(files))

	// 2. Create CBZ file (zip of images)
	cbzPath := dl.destPath
	if !strings.HasSuffix(cbzPath, ".cbz") {
		cbzPath = strings.TrimSuffix(cbzPath, filepath.Ext(cbzPath)) + ".cbz"
		dl.destPath = cbzPath
		dl.status.Name = filepath.Base(cbzPath)
		dl.status.Files = []string{filepath.Base(cbzPath)}
	}

	cbzFile, err := os.Create(cbzPath)
	if err != nil {
		dl.err = fmt.Errorf("failed to create CBZ file: %w", err)
		return
	}

	zipWriter := zip.NewWriter(cbzFile)

	// 3. Download each page and add to zip
	for i, filename := range files {
		pageURL := fmt.Sprintf("%s/%s/%s/%s", atHome.BaseURL, quality, atHome.Chapter.Hash, filename)

		ext := filepath.Ext(filename)
		if ext == "" {
			ext = ".jpg"
		}
		pageName := fmt.Sprintf("%03d%s", i+1, ext)

		if err := c.downloadPageToZip(zipWriter, pageURL, pageName); err != nil {
			zipWriter.Close()
			cbzFile.Close()
			os.Remove(cbzPath)
			dl.err = fmt.Errorf("failed to download page %d: %w", i+1, err)
			return
		}

		dl.writtenBytes = int64(i + 1)
		dl.status.Progress = float64(i+1) / float64(len(files))

		c.logger.Info(fmt.Sprintf("MangaDex: Downloaded page %d/%d", i+1, len(files)))

		// Rate limit: delay between pages
		if i < len(files)-1 {
			time.Sleep(pageDelay)
		}
	}

	zipWriter.Close()
	cbzFile.Sync()
	cbzFile.Close()

	// Mark as completed
	c.mu.Lock()
	dl.status.Progress = 1.0
	dl.status.IsCompleted = true
	dl.done = true
	c.mu.Unlock()

	c.logger.Info(fmt.Sprintf("MangaDex: Chapter downloaded as CBZ: %s (%d pages)", filepath.Base(cbzPath), len(files)))
}

func (c *DirectDownloadClient) downloadPageToZip(zw *zip.Writer, pageURL, pageName string) error {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Reel/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("page download failed: HTTP %d", resp.StatusCode)
	}

	w, err := zw.Create(pageName)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, resp.Body)
	return err
}
