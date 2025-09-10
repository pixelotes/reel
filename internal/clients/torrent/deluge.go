package torrent

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
)

// DelugeClient implements the TorrentClient interface for Deluge.
type DelugeClient struct {
	host       string
	password   string
	httpClient *http.Client
	reqID      int
	mu         sync.Mutex // To protect reqID
}

// delugeRequest defines the structure for a Deluge JSON-RPC request.
type delugeRequest struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
	ID     int           `json:"id"`
}

// delugeResponse defines the structure for a Deluge JSON-RPC response.
type delugeResponse struct {
	Result interface{} `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
	ID int `json:"id"`
}

// NewDelugeClient creates and authenticates a new client for Deluge.
// The host URL should be the path to the JSON endpoint, e.g., "http://localhost:8112/json".
func NewDelugeClient(host, password string) (*DelugeClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &DelugeClient{
		host:     host,
		password: password,
		httpClient: &http.Client{
			Jar: jar,
		},
		reqID: 1,
	}

	if err := client.login(); err != nil {
		return nil, fmt.Errorf("deluge login failed: %w", err)
	}

	return client, nil
}

// login authenticates with the Deluge daemon.
func (d *DelugeClient) login() error {
	_, err := d.sendRequest("auth.login", []interface{}{d.password})
	return err
}

// sendRequest is a helper to handle the Deluge JSON-RPC protocol.
func (d *DelugeClient) sendRequest(method string, params []interface{}) (interface{}, error) {
	d.mu.Lock()
	reqID := d.reqID
	d.reqID++
	d.mu.Unlock()

	reqBody := delugeRequest{
		Method: method,
		Params: params,
		ID:     reqID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", d.host, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res delugeResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if res.Error != nil {
		// If auth error, try to log in again and retry the request once.
		if strings.Contains(res.Error.Message, "Not authenticated") {
			if err := d.login(); err == nil {
				return d.sendRequest(method, params)
			}
		}
		return nil, fmt.Errorf("deluge API error for method '%s': %s", method, res.Error.Message)
	}

	return res.Result, nil
}

// HealthCheck verifies the connection to Deluge.
func (d *DelugeClient) HealthCheck() (bool, error) {
	_, err := d.sendRequest("web.connected", []interface{}{})
	return err == nil, err
}

// AddTorrent adds a magnet link to Deluge.
func (d *DelugeClient) AddTorrent(magnetLink string, downloadPath string) (string, error) {
	options := map[string]string{"download_location": downloadPath}
	result, err := d.sendRequest("core.add_torrent_magnet", []interface{}{magnetLink, options})
	if err != nil {
		return "", err
	}
	// Deluge directly returns the info hash.
	return result.(string), nil
}

// AddTorrentFile adds a .torrent file to Deluge.
func (d *DelugeClient) AddTorrentFile(fileContent []byte, downloadPath string) (string, error) {
	encodedContent := base64.StdEncoding.EncodeToString(fileContent)
	options := map[string]string{"download_location": downloadPath}
	result, err := d.sendRequest("core.add_torrent_file", []interface{}{"file.torrent", encodedContent, options})
	if err != nil {
		return "", err
	}
	// Deluge directly returns the info hash.
	return result.(string), nil
}

// GetTorrentStatus retrieves the full status of a torrent.
func (d *DelugeClient) GetTorrentStatus(hash string) (TorrentStatus, error) {
	keys := []string{
		"name", "progress", "state", "save_path", "total_size",
		"ratio", "download_payload_rate", "upload_payload_rate", "files",
	}
	filter := map[string][]string{"hash": {hash}}

	result, err := d.sendRequest("core.get_torrents_status", []interface{}{filter, keys})
	if err != nil {
		return TorrentStatus{}, err
	}

	torrents := result.(map[string]interface{})
	if len(torrents) == 0 {
		return TorrentStatus{}, fmt.Errorf("torrent with hash %s not found", hash)
	}

	data := torrents[hash].(map[string]interface{})

	var fileList []string
	if files, ok := data["files"].([]interface{}); ok {
		for _, file := range files {
			if fileMap, ok := file.(map[string]interface{}); ok {
				if path, ok := fileMap["path"].(string); ok {
					fileList = append(fileList, path)
				}
			}
		}
	}

	return TorrentStatus{
		Hash:         hash,
		Name:         data["name"].(string),
		Progress:     data["progress"].(float64) / 100.0, // Deluge progress is 0-100
		IsCompleted:  data["progress"].(float64) >= 100.0,
		DownloadDir:  data["save_path"].(string),
		UploadRatio:  data["ratio"].(float64),
		DownloadRate: int64(data["download_payload_rate"].(float64)),
		UploadRate:   int64(data["upload_payload_rate"].(float64)),
		Files:        fileList,
	}, nil
}

// RemoveTorrent removes a torrent and its data.
func (d *DelugeClient) RemoveTorrent(hash string) error {
	_, err := d.sendRequest("core.remove_torrent", []interface{}{hash, true}) // true to remove data
	return err
}

// AddTrackers adds new trackers to an existing torrent.
func (d *DelugeClient) AddTrackers(hash string, trackers []string) error {
	var trackerDicts []map[string]interface{}
	// First, get existing trackers to determine the tier
	status, err := d.GetTorrentStatus(hash)
	if err != nil {
		return fmt.Errorf("could not get existing trackers: %w", err)
	}

	// This part is a simplification. A full implementation would need to parse existing trackers.
	// For now, we add new trackers at the highest tier.
	for _, trackerURL := range trackers {
		trackerDicts = append(trackerDicts, map[string]interface{}{"url": trackerURL, "tier": len(status.Files)})
	}

	_, err = d.sendRequest("core.set_torrent_trackers", []interface{}{hash, trackerDicts})
	return err
}
