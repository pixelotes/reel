# Changelog

## [Unreleased]

### Performance

- **Pre-compile regex patterns at initialization** — All regex in `MatcherService` and `TorrentSelector` are now compiled once at construction time instead of on every call. This affects `Normalize()`, `IsIgnorable()`, `Matches()`, `filterByRejectPatterns()`, and `filterByEpisodeNumber()`. Estimated 5-10x speedup in torrent matching/filtering hot paths.

- **Move lookup maps to package level** — `techTerms` and `stopWords` maps in `matcher.go` are now package-level variables instead of being recreated on every `IsIgnorable()` call.

- **Pre-allocate filter slices** — All `filterBy*` functions in `TorrentSelector` now pre-allocate result slices with `make([]T, 0, len(input))` to reduce GC pressure.

### Refactor

- **Deduplicate search term logic** — Extracted `getSearchTerms()` in `SearcherService`, replacing 4 identical blocks across `performSearch()`, `PerformSearch()`, `PerformEpisodeSearch()`, and `FindBestTorrent()`.

- **Split magnet-to-torrent into separate binary** — Moved `anacrolix/torrent` dependency into a standalone helper binary (`cmd/magnet2torrent/`). The main binary now invokes it via `exec.Command` when needed. This removes ~60 transitive dependencies (including the entire pion/WebRTC stack) from the main binary. The helper is only spawned when `magnet_to_torrent_enabled` is true and the download URL is a magnet link. If the helper is missing, it falls back gracefully to passing the magnet link directly to the torrent client.

- **Replace gopsutil with syscall** — Replaced `github.com/shirou/gopsutil` (and its transitive deps `go-ole`, `wmi`) with a direct `syscall.Statfs` call for disk space checking. Only used for a single `disk.Usage()` call.

- **Migrate deprecated ioutil** — Replaced all `io/ioutil` usage with modern equivalents (`io.ReadAll`, `io.NopCloser`) across `api.go`, `qbittorrent.go`, and `tmdb.go`.

### Binary size

Measured on linux/arm64 with `-ldflags="-s -w"` (stripped):

| Binary | Before | After | Change |
|--------|--------|-------|--------|
| Main (reel) | 18.4 MB | 11.8 MB | **-35%** |
| Helper (magnet2torrent) | — | 12.9 MB | new |

The main binary, which runs as a long-lived daemon, is 6.6 MB smaller and no longer loads anacrolix/pion/webrtc into memory at startup. The helper binary only lives for the duration of a magnet conversion (seconds) and exits.
