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

func NewQBittorrentClient(host, username, password string) *qBittorrentClient {
	return &qBittorrentClient{
		host:       host,
		username:   username,
		password:   password,
		httpClient: &http.Client{},
	}
}

func (q *qBittorrentClient) AddTrackers(hash string, trackers []string) error { // Added function
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

	// qBittorrent does not immediately return the hash in a simple way.
	// We are returning the magnet link as a placeholder for the hash.
	// A more robust implementation would parse the magnet link to get the hash.
	return strings.Split(strings.Split(magnetLink, "btih:")[1], "&")[0], nil
}

func (q *qBittorrentClient) AddTorrentFile(fileContent []byte, downloadPath string) (string, error) { // Added function
	cookie, err := q.login()
	if err != nil {
		return "", err
	}

	addURL := fmt.Sprintf("%s/api/v2/torrents/add", q.host)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("torrents", "file.torrent")
	if err != nil {
		return "", err
	}
	part.Write(fileContent)
	writer.WriteField("savepath", downloadPath)
	writer.Close()

	req, err := http.NewRequest("POST", addURL, body)
	if err != nil {
		return "", err
	}

	req.AddCookie(cookie)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to add torrent file with status: %s", resp.Status)
	}

	// This is a simplification; a more robust method would be needed to get the hash
	// after adding a file, as qBittorrent doesn't return it directly.
	return "hash-from-file-not-retrieved", nil
}

// GetTorrentStatus is a mock implementation. A full implementation would parse the torrent list from the API.
func (q *qBittorrentClient) GetTorrentStatus(hash string) (TorrentStatus, error) {
	cookie, err := q.login()
	if err != nil {
		return TorrentStatus{}, err
	}

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

	return TorrentStatus{ // Modified line
		Hash:        hash,
		Name:        props.Name,
		Progress:    props.Progress,
		IsCompleted: props.Progress >= 1.0,
		DownloadDir: props.SavePath,
		UploadRatio: props.Ratio,
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
