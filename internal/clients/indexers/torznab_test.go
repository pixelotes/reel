package indexers

import (
	"encoding/xml"
	"testing"
)

func TestTorznabItem_GetIntAttr(t *testing.T) {
	item := TorznabItem{
		Attributes: []TorznabAttribute{
			{Name: "seeders", Value: "42"},
			{Name: "leechers", Value: "5"},
		},
	}

	if got := item.GetIntAttr("seeders"); got != 42 {
		t.Errorf("GetIntAttr(seeders) = %d, want 42", got)
	}
	if got := item.GetIntAttr("leechers"); got != 5 {
		t.Errorf("GetIntAttr(leechers) = %d, want 5", got)
	}
}

func TestTorznabItem_GetIntAttr_Missing(t *testing.T) {
	item := TorznabItem{}

	if got := item.GetIntAttr("seeders"); got != 0 {
		t.Errorf("GetIntAttr on missing attr = %d, want 0", got)
	}
}

func TestTorznabFeed_Unmarshal(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed">
<channel>
  <title>Test Channel</title>
  <item>
    <title>Movie 1080p</title>
    <link>magnet:?xt=urn:btih:hash1</link>
    <size>1500000000</size>
    <torznab:attr name="seeders" value="10"/>
  </item>
</channel>
</rss>`

	var feed TorznabFeed
	if err := xml.Unmarshal([]byte(xmlData), &feed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if feed.Channel.Title != "Test Channel" {
		t.Errorf("channel title = %q, want 'Test Channel'", feed.Channel.Title)
	}
	if len(feed.Channel.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Channel.Items))
	}

	item := feed.Channel.Items[0]
	if item.Title != "Movie 1080p" {
		t.Errorf("item title = %q, want 'Movie 1080p'", item.Title)
	}
	if item.Size != 1500000000 {
		t.Errorf("item size = %d, want 1500000000", item.Size)
	}
	if item.GetIntAttr("seeders") != 10 {
		t.Errorf("seeders = %d, want 10", item.GetIntAttr("seeders"))
	}
}
