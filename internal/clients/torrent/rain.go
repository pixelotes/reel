package torrent

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// RainClient implements TorrentClient for the Rain BitTorrent client (cenkalti/rain).
// Rain uses JSON-RPC 2.0 with "Session.*" method namespace on its RPC port (default 7246).
// Note: Rain does not support per-torrent download paths. All torrents download to the
// datadir configured in Rain's config.yaml. The downloadPath parameter is stored and
// returned in GetTorrentStatus for Reel's post-processor to use.
type RainClient struct {
	host         string
	downloadPath string
	httpClient   *http.Client
	nextID       int
}

type rainRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

type rainResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

// Rain RPC types (matching cenkalti/rain/internal/rpctypes)

type rainTorrent struct {
	ID       string `json:"ID"`
	Name     string `json:"Name"`
	InfoHash string `json:"InfoHash"`
	Port     int    `json:"Port"`
}

type rainStats struct {
	InfoHash string `json:"InfoHash"`
	Status   string `json:"Status"`
	Name     string `json:"Name"`
	Error    string `json:"Error"`
	Pieces   struct {
		Have  uint32 `json:"Have"`
		Total uint32 `json:"Total"`
	} `json:"Pieces"`
	Bytes struct {
		Total      int64 `json:"Total"`
		Completed  int64 `json:"Completed"`
		Downloaded int64 `json:"Downloaded"`
		Uploaded   int64 `json:"Uploaded"`
	} `json:"Bytes"`
	Speed struct {
		Download int `json:"Download"`
		Upload   int `json:"Upload"`
	} `json:"Speed"`
	ETA int `json:"ETA"`
}

type rainFile struct {
	Path   string `json:"Path"`
	Length int64  `json:"Length"`
}

func NewRainClient(host, downloadPath string) *RainClient {
	return &RainClient{
		host:         host,
		downloadPath: downloadPath,
		httpClient:   &http.Client{},
	}
}

func (r *RainClient) call(method string, params interface{}, result interface{}) error {
	r.nextID++

	reqData := rainRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      r.nextID,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("rain: failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", r.host, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("rain: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("rain: connection failed: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp rainResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("rain: failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("rain error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	if result != nil && rpcResp.Result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("rain: failed to unmarshal result: %w", err)
		}
	}

	return nil
}

func (r *RainClient) AddTorrent(magnetLink string, downloadPath string) (string, error) {
	params := map[string]interface{}{
		"URI": magnetLink,
	}

	var resp struct {
		Torrent rainTorrent `json:"Torrent"`
	}
	if err := r.call("Session.AddURI", params, &resp); err != nil {
		return "", err
	}

	return resp.Torrent.ID, nil
}

func (r *RainClient) AddTorrentFile(fileContent []byte, downloadPath string) (string, error) {
	encoded := base64.StdEncoding.EncodeToString(fileContent)
	params := map[string]interface{}{
		"Torrent": encoded,
	}

	var resp struct {
		Torrent rainTorrent `json:"Torrent"`
	}
	if err := r.call("Session.AddTorrent", params, &resp); err != nil {
		return "", err
	}

	return resp.Torrent.ID, nil
}

func (r *RainClient) GetTorrentStatus(id string) (TorrentStatus, error) {
	params := map[string]string{"ID": id}

	var statsResp struct {
		Stats rainStats `json:"Stats"`
	}
	if err := r.call("Session.GetTorrentStats", params, &statsResp); err != nil {
		return TorrentStatus{}, err
	}

	var filesResp struct {
		Files []rainFile `json:"Files"`
	}
	if err := r.call("Session.GetTorrentFiles", params, &filesResp); err != nil {
		return TorrentStatus{}, fmt.Errorf("rain: could not get files for torrent: %w", err)
	}

	stats := statsResp.Stats

	// Calculate progress
	progress := 0.0
	if stats.Bytes.Total > 0 {
		progress = float64(stats.Bytes.Completed) / float64(stats.Bytes.Total)
	}

	// Calculate upload ratio
	uploadRatio := 0.0
	if stats.Bytes.Total > 0 {
		uploadRatio = float64(stats.Bytes.Uploaded) / float64(stats.Bytes.Total)
	}

	// Rain statuses: Downloading, Seeding, Stopped, Verifying, Allocating, etc.
	isCompleted := stats.Status == "Seeding" || (stats.Status == "Stopped" && progress >= 1.0)

	// Build file list with relative paths
	var fileList []string
	for _, f := range filesResp.Files {
		path := f.Path
		// Strip the download path prefix if present
		path = strings.TrimPrefix(path, r.downloadPath)
		path = strings.TrimPrefix(path, "/")
		fileList = append(fileList, path)
	}

	return TorrentStatus{
		Hash:         stats.InfoHash,
		Name:         stats.Name,
		Progress:     progress,
		Files:        fileList,
		DownloadDir:  r.downloadPath,
		IsCompleted:  isCompleted,
		DownloadRate: int64(stats.Speed.Download),
		UploadRate:   int64(stats.Speed.Upload),
		ETA:          stats.ETA,
		UploadRatio:  uploadRatio,
	}, nil
}

func (r *RainClient) RemoveTorrent(id string) error {
	params := map[string]interface{}{
		"ID": id,
	}
	return r.call("Session.RemoveTorrent", params, nil)
}

func (r *RainClient) AddTrackers(id string, trackers []string) error {
	for _, tracker := range trackers {
		params := map[string]string{
			"ID":  id,
			"URL": tracker,
		}
		if err := r.call("Session.AddTracker", params, nil); err != nil {
			return fmt.Errorf("rain: failed to add tracker %s: %w", tracker, err)
		}
	}
	return nil
}

func (r *RainClient) HealthCheck() (bool, error) {
	var resp struct {
		Torrents []rainTorrent `json:"Torrents"`
	}
	err := r.call("Session.ListTorrents", struct{}{}, &resp)
	return err == nil, err
}
