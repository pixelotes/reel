// pixelotes/reel/reel-912718c2894dddc773eede72733de790bc7912b3/internal/clients/torrent/qbittorrent.go
package torrent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// qBittorrentClient implements the TorrentClient interface.
type qBittorrentClient struct {
	host       string
	username   string
	password   string
	httpClient *http.Client
}

type qbTorrentProperties struct {
	Name        string  `json:"name"`
	Size        int64   `json:"size"`
	Progress    float64 `json:"progress"`
	Ratio       float64 `json:"ratio"`
	SavePath    string  `json:"save_path"`
	State       string  `json:"state"`
	ContentPath string  `json:"content_path"`
}

type qbTorrentFile struct {
	Name string `json:"name"`
}

func NewQBittorrentClient(host, username, password string) *qBittorrentClient {
	return &qBittorrentClient{
		host:       host,
		username:   username,
		password:   password,
		httpClient: &http.Client{},
	}
}

func (q *qBittorrentClient) AddTrackers(hash string, trackers []string) error {
	cookie, err := q.login()
	if err != nil {
		return err
	}

	addTrackersURL := fmt.Sprintf("%s/api/v2/torrents/addTrackers", q.host)
	data := url.Values{}
	data.Set("hash", hash)
	data.Set("urls", strings.Join(trackers, "\n"))

	req, err := http.NewRequest("POST", addTrackersURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.AddCookie(cookie)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add trackers with status: %s", resp.Status)
	}
	return nil
}

// login authenticates with the qBittorrent Web API and gets a session cookie.
func (q *qBittorrentClient) login() (*http.Cookie, error) {
	loginURL := fmt.Sprintf("%s/api/v2/auth/login", q.host)
	data := url.Values{}
	data.Set("username", q.username)
	data.Set("password", q.password)

	req, err := http.NewRequest("POST", loginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qbittorrent login failed with status: %s", resp.Status)
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" {
			return cookie, nil
		}
	}
	return nil, fmt.Errorf("SID cookie not found after login")
}

func (q *qBittorrentClient) AddTorrent(magnetLink string, downloadPath string) (string, error) {
	cookie, err := q.login()
	if err != nil {
		return "", err
	}

	addURL := fmt.Sprintf("%s/api/v2/torrents/add", q.host)
	data := url.Values{}
	data.Set("urls", magnetLink)
	data.Set("savepath", downloadPath)

	req, err := http.NewRequest("POST", addURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.AddCookie(cookie)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to add torrent with status: %s", resp.Status)
	}

	// For magnet links, parsing the info hash (btih) from the link itself is the most reliable method.
	// Example: magnet:?xt=urn:btih:HASH&dn=...
	lowerLink := strings.ToLower(magnetLink)
	btihIndex := strings.Index(lowerLink, "btih:")
	if btihIndex == -1 {
		return "", fmt.Errorf("info hash (btih) not found in magnet link")
	}

	hashStart := btihIndex + 5
	hashEnd := strings.Index(lowerLink[hashStart:], "&")
	if hashEnd == -1 {
		// If no '&', the hash is the rest of the string
		return lowerLink[hashStart:], nil
	}

	return lowerLink[hashStart : hashStart+hashEnd], nil
}

func (q *qBittorrentClient) AddTorrentFile(fileContent []byte, downloadPath string) (string, error) {
	cookie, err := q.login()
	if err != nil {
		return "", err
	}

	addURL := fmt.Sprintf("%s/api/v2/torrents/add", q.host)

	// Generate a unique tag to identify the torrent after adding it.
	tempTag := "reel-temp-" + uuid.New().String()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("torrents", "file.torrent")
	if err != nil {
		return "", err
	}
	part.Write(fileContent)
	writer.WriteField("savepath", downloadPath)
	writer.WriteField("tags", tempTag)
	writer.Close()

	req, err := http.NewRequest("POST", addURL, body)
	if err != nil {
		return "", err
	}

	req.AddCookie(cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to add torrent file with status: %s, body: %s", resp.Status, string(bodyBytes))
	}

	// Now, find the torrent by the unique tag to get its hash
	infoURL := fmt.Sprintf("%s/api/v2/torrents/info?filter=all&tags=%s", q.host, tempTag)
	req, err = http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return "", err
	}
	req.AddCookie(cookie)

	resp, err = q.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var torrents []struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(bodyBytes, &torrents); err != nil {
		return "", fmt.Errorf("failed to find torrent by tag: %w", err)
	}

	if len(torrents) == 0 {
		return "", fmt.Errorf("could not find added torrent by temporary tag")
	}
	hash := torrents[0].Hash

	// Clean up by removing the temporary tag
	removeTagsURL := fmt.Sprintf("%s/api/v2/torrents/removeTags", q.host)
	data := url.Values{}
	data.Set("hashes", hash)
	data.Set("tags", tempTag)

	req, err = http.NewRequest("POST", removeTagsURL, strings.NewReader(data.Encode()))
	if err != nil {
		// Non-critical error, just log it
		fmt.Printf("Warning: failed to remove temporary tag: %v\n", err)
	} else {
		req.AddCookie(cookie)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		q.httpClient.Do(req) // Fire and forget
	}

	return hash, nil
}

// GetTorrentStatus is a mock implementation. A full implementation would parse the torrent list from the API.
func (q *qBittorrentClient) GetTorrentStatus(hash string) (TorrentStatus, error) {
	cookie, err := q.login()
	if err != nil {
		return TorrentStatus{}, err
	}

	// First, get the main torrent properties
	propertiesURL := fmt.Sprintf("%s/api/v2/torrents/properties?hash=%s", q.host, hash)
	req, err := http.NewRequest("GET", propertiesURL, nil)
	if err != nil {
		return TorrentStatus{}, err
	}
	req.AddCookie(cookie)

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return TorrentStatus{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// If torrent not found, qBittorrent returns 404
		if resp.StatusCode == http.StatusNotFound {
			return TorrentStatus{}, fmt.Errorf("torrent with hash %s not found", hash)
		}
		return TorrentStatus{}, fmt.Errorf("failed to get torrent properties with status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return TorrentStatus{}, err
	}

	var props qbTorrentProperties
	if err := json.Unmarshal(body, &props); err != nil {
		return TorrentStatus{}, fmt.Errorf("failed to decode torrent properties: %w", err)
	}

	// --- New: Get the file list ---
	filesURL := fmt.Sprintf("%s/api/v2/torrents/files?hash=%s", q.host, hash)
	req, err = http.NewRequest("GET", filesURL, nil)
	if err != nil {
		return TorrentStatus{}, err
	}
	req.AddCookie(cookie)

	resp, err = q.httpClient.Do(req)
	if err != nil {
		return TorrentStatus{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TorrentStatus{}, fmt.Errorf("failed to get torrent files with status: %s", resp.Status)
	}

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return TorrentStatus{}, err
	}

	var files []qbTorrentFile
	if err := json.Unmarshal(body, &files); err != nil {
		return TorrentStatus{}, fmt.Errorf("failed to decode torrent files: %w", err)
	}

	var fileList []string
	for _, f := range files {
		fileList = append(fileList, f.Name)
	}
	// --- End of new section ---

	return TorrentStatus{
		Hash:        hash,
		Name:        props.Name,
		Progress:    props.Progress,
		IsCompleted: props.Progress >= 1.0,
		DownloadDir: props.SavePath,
		UploadRatio: props.Ratio,
		Files:       fileList, // Populate the files list
	}, nil
}

func (q *qBittorrentClient) RemoveTorrent(hash string) error {
	cookie, err := q.login()
	if err != nil {
		return err
	}

	removeURL := fmt.Sprintf("%s/api/v2/torrents/delete", q.host)
	data := url.Values{}
	data.Set("hashes", hash)
	data.Set("deleteFiles", "true")

	req, err := http.NewRequest("POST", removeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.AddCookie(cookie)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to remove torrent with status: %s", resp.Status)
	}
	return nil
}

func (q *qBittorrentClient) HealthCheck() (bool, error) {
	_, err := q.login()
	if err != nil {
		return false, err
	}
	return true, nil
}
