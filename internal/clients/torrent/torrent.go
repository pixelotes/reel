package torrent

type TorrentClient interface {
	AddTorrent(magnetLink string, downloadPath string) (string, error)
	AddTorrentFile(fileContent []byte, downloadPath string) (string, error)
	GetTorrentStatus(hash string) (TorrentStatus, error)
	RemoveTorrent(hash string) error
}

type TorrentStatus struct {
	Hash         string   `json:"hash"`
	Name         string   `json:"name"`
	Progress     float64  `json:"progress"`
	Files        []string `json:"files"`
	DownloadDir  string   `json:"download_dir"`
	IsCompleted  bool     `json:"is_completed"`
	DownloadRate int64    `json:"download_rate"`
	UploadRate   int64    `json:"upload_rate"`
	ETA          int      `json:"eta"`
	UploadRatio  float64  `json:"upload_ratio"`
}
