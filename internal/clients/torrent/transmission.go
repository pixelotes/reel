package torrent

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type TransmissionClient struct {
    host       string
    username   string
    password   string
    sessionID  string
    httpClient *http.Client
}

type TorrentClient interface {
    AddTorrent(magnetLink string, downloadPath string) (string, error)
    GetTorrentStatus(hash string) (TorrentStatus, error)
    RemoveTorrent(hash string) error
}

type TorrentStatus struct {
    Hash         string  `json:"hash"`
    Name         string  `json:"name"`
    Progress     float64 `json:"progress"`
    IsCompleted  bool    `json:"is_completed"`
    DownloadRate int64   `json:"download_rate"`
    UploadRate   int64   `json:"upload_rate"`
    ETA          int     `json:"eta"`
}

func NewTransmissionClient(host, username, password string) *TransmissionClient {
    return &TransmissionClient{
        host:       host,
        username:   username,
        password:   password,
        httpClient: &http.Client{},
    }
}

func (t *TransmissionClient) AddTorrent(magnetLink string, downloadPath string) (string, error) {
    method := "torrent-add"
    args := map[string]interface{}{
        "filename":     magnetLink,
        "download-dir": downloadPath,
    }
    
    response, err := t.sendRequest(method, args)
    if err != nil {
        return "", err
    }
    
    // Extract torrent hash from response
    if arguments, ok := response["arguments"].(map[string]interface{}); ok {
        if torrentAdded, ok := arguments["torrent-added"].(map[string]interface{}); ok {
            if hashString, ok := torrentAdded["hashString"].(string); ok {
                return hashString, nil
            }
        }
    }
    
    return "", fmt.Errorf("could not extract torrent hash from response")
}

func (t *TransmissionClient) GetTorrentStatus(hash string) (TorrentStatus, error) {
    method := "torrent-get"
    args := map[string]interface{}{
        "fields": []string{"hashString", "name", "percentDone", "status", "rateDownload", "rateUpload", "eta"},
        "ids":    []string{hash},
    }
    
    response, err := t.sendRequest(method, args)
    if err != nil {
        return TorrentStatus{}, err
    }
    
    if arguments, ok := response["arguments"].(map[string]interface{}); ok {
        if torrents, ok := arguments["torrents"].([]interface{}); ok && len(torrents) > 0 {
            if torrent, ok := torrents[0].(map[string]interface{}); ok {
                status := TorrentStatus{
                    Hash:     hash,
                    Name:     torrent["name"].(string),
                    Progress: torrent["percentDone"].(float64),
                }
                
                if status.Progress >= 1.0 {
                    status.IsCompleted = true
                }
                
                return status, nil
            }
        }
    }
    
    return TorrentStatus{}, fmt.Errorf("torrent not found")
}

func (t *TransmissionClient) RemoveTorrent(hash string) error {
    method := "torrent-remove"
    args := map[string]interface{}{
        "ids":               []string{hash},
        "delete-local-data": false,
    }
    
    _, err := t.sendRequest(method, args)
    return err
}

func (t *TransmissionClient) sendRequest(method string, args interface{}) (map[string]interface{}, error) {
    reqData := map[string]interface{}{
        "method":    method,
        "arguments": args,
    }
    
    jsonData, err := json.Marshal(reqData)
    if err != nil {
        return nil, err
    }
    
    url := fmt.Sprintf("http://%s/transmission/rpc", t.host)
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Content-Type", "application/json")
    if t.username != "" && t.password != "" {
        req.SetBasicAuth(t.username, t.password)
    }
    if t.sessionID != "" {
        req.Header.Set("X-Transmission-Session-Id", t.sessionID)
    }
    
    resp, err := t.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Handle session ID requirement
    if resp.StatusCode == 409 {
        t.sessionID = resp.Header.Get("X-Transmission-Session-Id")
        return t.sendRequest(method, args) // Retry with session ID
    }
    
    var response map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, err
    }
    
    if result, ok := response["result"].(string); !ok || result != "success" {
        return nil, fmt.Errorf("transmission request failed: %v", response["result"])
    }
    
    return response, nil
}
