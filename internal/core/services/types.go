package services

import (
	"reel/internal/clients/indexers"
	"reel/internal/config"
)

type IndexerClientWithMode struct {
	Client indexers.Client
	Source config.SourceConfig
}

type ClientStatus struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Status bool   `json:"status"`
}

type SubtitleTrack struct {
	Language string `json:"language"`
	Label    string `json:"label"`
	FilePath string `json:"-"` // Don't expose file path to frontend
}

type CalendarEvent struct {
	Title  string `json:"title"`
	Start  string `json:"start"`
	AllDay bool   `json:"allDay"`
}
