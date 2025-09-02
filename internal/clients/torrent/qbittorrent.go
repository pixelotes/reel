// pixelotes/reel/reel-912718c2894dddc773eede72733de790bc7912b3/internal/clients/torrent/qbittorrent.go
package torrent

import (
	"encoding/json" // Added line
	"fmt"
	"io/ioutil" // Added line
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

type qbTorrentProperties struct { // Added struct
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

// GetTorrentStatus is a mock implementation. A full implementation would parse the torrent list from the API.
func (q *qBittorrentClient) GetTorrentStatus(hash string) (TorrentStatus, error) {
	cookie, err := q.login() // Added line
	if err != nil {          // Added line
		return TorrentStatus{}, err // Added line
	} // Added line

	propertiesURL := fmt.Sprintf("%s/api/v2/torrents/properties?hash=%s", q.host, hash) // Added line
	req, err := http.NewRequest("GET", propertiesURL, nil)                              // Added line
	if err != nil {                                                                     // Added line
		return TorrentStatus{}, err // Added line
	} // Added line
	req.AddCookie(cookie) // Added line

	resp, err := q.httpClient.Do(req) // Added line
	if err != nil {                   // Added line
		return TorrentStatus{}, err // Added line
	} // Added line
	defer resp.Body.Close() // Added line

	if resp.StatusCode != http.StatusOK { // Added line
		return TorrentStatus{}, fmt.Errorf("failed to get torrent properties with status: %s", resp.Status) // Added line
	} // Added line

	body, err := ioutil.ReadAll(resp.Body) // Added line
	if err != nil {                        // Added line
		return TorrentStatus{}, err // Added line
	} // Added line

	var props qbTorrentProperties                        // Added line
	if err := json.Unmarshal(body, &props); err != nil { // Added line
		return TorrentStatus{}, fmt.Errorf("failed to decode torrent properties: %w", err) // Added line
	} // Added line

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
