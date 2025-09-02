package indexers

import (
	"encoding/xml"
	"strconv"
)

type TorznabChannel struct {
	Title       string        `xml:"title"`
	Description string        `xml:"description"`
	Link        string        `xml:"link"`
	Language    string        `xml:"language"`
	WebMaster   string        `xml:"webMaster"`
	Items       []TorznabItem `xml:"item"`
}

type TorznabFeed struct {
	XMLName xml.Name       `xml:"rss"`
	Channel TorznabChannel `xml:"channel"`
}

type TorznabAttribute struct {
	XMLName xml.Name `xml:"attr"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:"value,attr"`
}

type TorznabItem struct {
	Title       string             `xml:"title"`
	Link        string             `xml:"link"`
	Comments    string             `xml:"comments"`
	PubDate     string             `xml:"pubDate"`
	Size        int64              `xml:"size"`
	Description string             `xml:"description"`
	GUID        string             `xml:"guid"`
	Attributes  []TorznabAttribute `xml:"attr"`
}

func (item *TorznabItem) GetIntAttr(name string) int {
	for _, attr := range item.Attributes {
		if attr.Name == name {
			val, _ := strconv.Atoi(attr.Value)
			return val
		}
	}
	return 0
}
