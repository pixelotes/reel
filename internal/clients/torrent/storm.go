package torrent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// StormClient implements TorrentClient for the Storm BitTorrent client.
// Storm wraps Rain (cenkalti/rain) with a REST API and supports per-torrent download paths.
type StormClient struct {
	host       string
	httpClient *http.Client
}

// Storm API response types

type stormTorrent struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	InfoHash      string      `json:"info_hash"`
	Status        string      `json:"status"`
	Progress      float64     `json:"progress"`
	Size          int64       `json:"size"`
	Downloaded    int64       `json:"downloaded"`
	Uploaded      int64       `json:"uploaded"`
	DownloadSpeed int         `json:"download_speed"`
	UploadSpeed   int         `json:"upload_speed"`
	ETA           *int        `json:"eta"`
	Peers         int         `json:"peers"`
	Label         string      `json:"label"`
	TargetPath    string      `json:"target_path"`
	SaveDir       string      `json:"save_dir"`
	Moved         bool        `json:"moved"`
	AddedAt       string      `json:"added_at"`
	Files         []stormFile `json:"files,omitempty"`
}

type stormFile struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Completed int64  `json:"completed"`
}

type stormAddRequest struct {
	URI        string `json:"uri"`
	TargetPath string `json:"target_path,omitempty"`
	Label      string `json:"label,omitempty"`
}

func stormDownloadDir(tor stormTorrent) string {
	if tor.Moved {
		return tor.TargetPath
	}
	return tor.SaveDir
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func NewStormClient(host string) *StormClient {
	host = strings.TrimRight(host, "/")
	return &StormClient{
		host:       host,
		httpClient: &http.Client{},
	}
}

func (s *StormClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("storm: failed to marshal request: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, s.host+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("storm: failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("storm: connection failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("storm: failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("storm error: %s", errResp.Error)
		}
		return nil, fmt.Errorf("storm error: HTTP %d", resp.StatusCode)
	}

	return data, nil
}

func (s *StormClient) AddTorrent(magnetLink string, downloadPath string) (string, error) {
	body := stormAddRequest{
		URI:        magnetLink,
		TargetPath: downloadPath,
	}

	data, err := s.doRequest("POST", "/api/torrents", body)
	if err != nil {
		return "", err
	}

	var resp stormTorrent
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("storm: failed to parse add response: %w", err)
	}

	return resp.ID, nil
}

func (s *StormClient) AddTorrentFile(fileContent []byte, downloadPath string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "torrent.torrent")
	if err != nil {
		return "", fmt.Errorf("storm: failed to create form file: %w", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		return "", fmt.Errorf("storm: failed to write file content: %w", err)
	}

	if err := writer.WriteField("target_path", downloadPath); err != nil {
		return "", fmt.Errorf("storm: failed to write target_path field: %w", err)
	}

	writer.Close()

	req, err := http.NewRequest("POST", s.host+"/api/torrents/file", &buf)
	if err != nil {
		return "", fmt.Errorf("storm: failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("storm: connection failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("storm: failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &errResp) == nil && errResp.Error != "" {
			return "", fmt.Errorf("storm error: %s", errResp.Error)
		}
		return "", fmt.Errorf("storm error: HTTP %d", resp.StatusCode)
	}

	var torrent stormTorrent
	if err := json.Unmarshal(data, &torrent); err != nil {
		return "", fmt.Errorf("storm: failed to parse response: %w", err)
	}

	return torrent.ID, nil
}

func (s *StormClient) GetTorrentStatus(id string) (TorrentStatus, error) {
	data, err := s.doRequest("GET", "/api/torrents/"+id, nil)
	if err != nil {
		return TorrentStatus{}, err
	}

	var tor stormTorrent
	if err := json.Unmarshal(data, &tor); err != nil {
		return TorrentStatus{}, fmt.Errorf("storm: failed to parse torrent status: %w", err)
	}

	isCompleted := tor.Status == "Seeding" || (tor.Status == "Stopped" && tor.Progress >= 1.0)

	var fileList []string
	for _, f := range tor.Files {
		fileList = append(fileList, f.Path)
	}

	// If no files in the torrent detail, fetch them separately
	if len(fileList) == 0 {
		if files, err := s.getFiles(id); err == nil {
			for _, f := range files {
				fileList = append(fileList, f.Path)
			}
		}
	}

	uploadRatio := 0.0
	if tor.Size > 0 {
		uploadRatio = float64(tor.Uploaded) / float64(tor.Size)
	}

	return TorrentStatus{
		Hash:         tor.InfoHash,
		Name:         tor.Name,
		Progress:     tor.Progress,
		Files:        fileList,
		DownloadDir:  stormDownloadDir(tor),
		IsCompleted:  isCompleted,
		DownloadRate: int64(tor.DownloadSpeed),
		UploadRate:   int64(tor.UploadSpeed),
		ETA:          derefInt(tor.ETA),
		UploadRatio:  uploadRatio,
	}, nil
}

func (s *StormClient) getFiles(id string) ([]stormFile, error) {
	data, err := s.doRequest("GET", "/api/torrents/"+id+"/files", nil)
	if err != nil {
		return nil, err
	}

	var files []stormFile
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (s *StormClient) RemoveTorrent(id string) error {
	_, err := s.doRequest("DELETE", "/api/torrents/"+id+"?delete_data=false", nil)
	return err
}

func (s *StormClient) AddTrackers(id string, trackers []string) error {
	for _, tracker := range trackers {
		body := map[string]string{"url": tracker}
		if _, err := s.doRequest("POST", "/api/torrents/"+id+"/trackers", body); err != nil {
			return fmt.Errorf("storm: failed to add tracker %s: %w", tracker, err)
		}
	}
	return nil
}

func (s *StormClient) HealthCheck() (bool, error) {
	data, err := s.doRequest("GET", "/api/health", nil)
	if err != nil {
		return false, err
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, err
	}

	return resp.Status == "ok", nil
}
