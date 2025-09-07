package torrent

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	fields := []string{"gid", "infoHash", "status", "totalLength", "completedLength", "downloadSpeed", "uploadSpeed", "connections", "numSeeders", "bittorrent"}
	result, err := a.sendRequest("aria2.tellStatus", hash, fields)
	if err != nil {
		return TorrentStatus{}, err
	}

	data := result.(map[string]interface{})
	totalLength, _ := strconv.ParseFloat(data["totalLength"].(string), 64)
	completedLength, _ := strconv.ParseFloat(data["completedLength"].(string), 64)
	downloadSpeed, _ := strconv.ParseFloat(data["downloadSpeed"].(string), 64)
	uploadSpeed, _ := strconv.ParseFloat(data["uploadSpeed"].(string), 64)
	//numSeeders, _ := strconv.ParseFloat(data["numSeeders"].(string), 64)

	var name string
	if bittorrent, ok := data["bittorrent"].(map[string]interface{}); ok {
		if info, ok := bittorrent["info"].(map[string]interface{}); ok {
			name = info["name"].(string)
		}
	}

	progress := 0.0
	if totalLength > 0 {
		progress = completedLength / totalLength
	}

	return TorrentStatus{
		Hash:         data["infoHash"].(string),
		Name:         name,
		Progress:     progress,
		IsCompleted:  data["status"].(string) == "complete",
		DownloadRate: int64(downloadSpeed),
		UploadRate:   int64(uploadSpeed),
		ETA:          0, // aria2 does not provide ETA directly
		UploadRatio:  0, // aria2 does not provide upload ratio directly
	}, nil
}

func (a *Aria2Client) RemoveTorrent(hash string) error {
	_, err := a.sendRequest("aria2.remove", hash)
	return err
}

func (a *Aria2Client) AddTrackers(hash string, trackers []string) error {
	// The official way to add trackers in aria2 is to provide them when adding the torrent.
	// This is a workaround to add them after the fact.
	uri, err := a.sendRequest("aria2.getUris", hash)
	if err != nil {
		return err
	}
	uriList := uri.([]interface{})
	if len(uriList) == 0 {
		return fmt.Errorf("could not get URI for torrent %s", hash)
	}

	uriMap := uriList[0].(map[string]interface{})
	magnetURI := uriMap["uri"].(string)

	for _, tracker := range trackers {
		magnetURI += "&tr=" + tracker
	}

	_, err = a.sendRequest("aria2.changeUri", hash, 1, []string{magnetURI}, []string{}, 0)
	return err
}

func (a *Aria2Client) HealthCheck() (bool, error) {
	_, err := a.sendRequest("aria2.getVersion")
	return err == nil, err
}
