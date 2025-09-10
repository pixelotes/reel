package torrent

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type Aria2Client struct {
	host       string
	secret     string
	httpClient *http.Client
}

type aria2Request struct {
	Jsonrpc string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type aria2Response struct {
	ID      string      `json:"id"`
	Jsonrpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewAria2Client(host, secret string) *Aria2Client {
	return &Aria2Client{
		host:       host,
		secret:     secret,
		httpClient: &http.Client{},
	}
}

func (a *Aria2Client) sendRequest(method string, params ...interface{}) (interface{}, error) {
	tokenParams := []interface{}{"token:" + a.secret}
	tokenParams = append(tokenParams, params...)

	reqData := aria2Request{
		Jsonrpc: "2.0",
		ID:      "reel",
		Method:  method,
		Params:  tokenParams,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", a.host, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response aria2Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("aria2 error: %s (code: %d)", response.Error.Message, response.Error.Code)
	}

	return response.Result, nil
}

func (a *Aria2Client) AddTorrent(magnetLink string, downloadPath string) (string, error) {
	options := map[string]string{"dir": downloadPath}
	result, err := a.sendRequest("aria2.addUri", []string{magnetLink}, options)
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (a *Aria2Client) AddTorrentFile(fileContent []byte, downloadPath string) (string, error) {
	encodedMetainfo := base64.StdEncoding.EncodeToString(fileContent)
	options := map[string]string{"dir": downloadPath}
	result, err := a.sendRequest("aria2.addTorrent", encodedMetainfo, []string{}, options)
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (a *Aria2Client) GetTorrentStatus(hash string) (TorrentStatus, error) {
	// Fields to request from the tellStatus method
	statusFields := []string{
		"gid", "infoHash", "status", "totalLength", "completedLength", "uploadLength",
		"downloadSpeed", "uploadSpeed", "dir", "bittorrent",
	}
	statusResult, err := a.sendRequest("aria2.tellStatus", hash, statusFields)
	if err != nil {
		return TorrentStatus{}, err
	}

	// New call to get the list of files
	filesResult, err := a.sendRequest("aria2.getFiles", hash)
	if err != nil {
		return TorrentStatus{}, fmt.Errorf("could not get files for torrent: %w", err)
	}

	data := statusResult.(map[string]interface{})
	filesData := filesResult.([]interface{})

	// The base download directory for the torrent
	downloadDir := data["dir"].(string)

	// Parse numeric values from strings
	totalLength, _ := strconv.ParseFloat(data["totalLength"].(string), 64)
	completedLength, _ := strconv.ParseFloat(data["completedLength"].(string), 64)
	uploadLength, _ := strconv.ParseFloat(data["uploadLength"].(string), 64)
	downloadSpeed, _ := strconv.ParseFloat(data["downloadSpeed"].(string), 64)
	uploadSpeed, _ := strconv.ParseFloat(data["uploadSpeed"].(string), 64)

	// Correctly extract the torrent name from the bittorrent struct
	var name string
	if bittorrent, ok := data["bittorrent"].(map[string]interface{}); ok {
		if info, ok := bittorrent["info"].(map[string]interface{}); ok {
			if nameVal, ok := info["name"].(string); ok {
				name = nameVal
			}
		}
	}

	// Calculate Progress and Upload Ratio
	progress := 0.0
	if totalLength > 0 {
		progress = completedLength / totalLength
	}

	uploadRatio := 0.0
	if totalLength > 0 {
		uploadRatio = uploadLength / totalLength
	}

	// --- MODIFIED SECTION ---
	// Process the file list, converting absolute paths to relative paths
	var fileList []string
	for _, fileEntry := range filesData {
		fileMap := fileEntry.(map[string]interface{})
		if absolutePath, ok := fileMap["path"].(string); ok {
			// Make the path relative to the download directory
			relativePath := strings.TrimPrefix(absolutePath, downloadDir)
			relativePath = strings.TrimPrefix(relativePath, "/") // Remove leading slash
			fileList = append(fileList, relativePath)
		}
	}
	// --- END OF MODIFIED SECTION ---

	return TorrentStatus{
		Hash:         data["infoHash"].(string),
		Name:         name,
		Progress:     progress,
		IsCompleted:  data["status"].(string) == "complete",
		DownloadRate: int64(downloadSpeed),
		UploadRate:   int64(uploadSpeed),
		DownloadDir:  downloadDir,
		Files:        fileList, // Now contains relative paths
		UploadRatio:  uploadRatio,
		ETA:          0,
	}, nil
}

func (a *Aria2Client) RemoveTorrent(hash string) error {
	_, err := a.sendRequest("aria2.remove", hash)
	return err
}

func (a *Aria2Client) AddTrackers(hash string, trackers []string) error {
	// The correct way to add trackers is to use the changeOption RPC call
	// with the bt-tracker option. This directly modifies the active torrent.

	// First, get the current list of trackers
	result, err := a.sendRequest("aria2.getOption", hash, []string{"bt-tracker"})
	if err != nil {
		return fmt.Errorf("could not get current trackers for torrent %s: %w", hash, err)
	}

	options := result.(map[string]interface{})
	existingTrackersStr := options["bt-tracker"].(string)

	// Combine existing trackers with new ones, avoiding duplicates
	trackerSet := make(map[string]bool)
	for _, t := range strings.Split(existingTrackersStr, ",") {
		if t != "" {
			trackerSet[t] = true
		}
	}
	for _, t := range trackers {
		if t != "" {
			trackerSet[t] = true
		}
	}

	var allTrackers []string
	for t := range trackerSet {
		allTrackers = append(allTrackers, t)
	}

	newTrackersStr := strings.Join(allTrackers, ",")

	// Send the updated tracker list back to Aria2
	_, err = a.sendRequest("aria2.changeOption", hash, map[string]string{"bt-tracker": newTrackersStr})
	return err
}

func (a *Aria2Client) HealthCheck() (bool, error) {
	_, err := a.sendRequest("aria2.getVersion")
	return err == nil, err
}
