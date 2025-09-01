package torrent

import (
	"fmt"
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
	// This is a placeholder. A real implementation would need to query the
	// /api/v2/torrents/info endpoint and find the torrent by its hash.
	return TorrentStatus{
		Hash:        hash,
		Name:        "Mock qBittorrent Download",
		Progress:    0.5, // Mock 50% progress
		IsCompleted: false,
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
