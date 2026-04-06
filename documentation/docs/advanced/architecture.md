# Architecture

Reel is designed from the ground up to run on a **Raspberry Pi 3B with 1 GB of RAM**. Every architectural decision prioritizes low memory usage, small binary size, and minimal CPU overhead — while keeping the system flexible enough to handle movies, TV shows, anime, ebooks, and manga.

Think of it as a personal, stripped-down Flexget that does exactly what you need and nothing more.

## Design principles

### Single binary, no runtime dependencies

Reel compiles to a single static binary with no external runtime dependencies. The database is SQLite (pure Go implementation, no CGO). The web UI is vanilla HTML/CSS/JS served directly by the binary. There is no Node.js, no Python, no Java — just one binary and a YAML config file.

This matters on a Pi 3B where every megabyte of RAM counts. A Go binary with static linking loads fast, uses predictable memory, and doesn't fight with garbage collectors from multiple runtimes.

### Vanilla web UI

The frontend is hand-written HTML, CSS, and JavaScript — no React, no Vue, no build toolchain. This keeps the served assets small (a few hundred KB total) and avoids the need for a Node.js build step.

The tradeoff is obvious: it's not a modern SPA with component libraries and state management. But for a media management dashboard that shows a list of media, a calendar, and some forms, it's more than enough. The UI loads instantly even on slow connections because there's almost nothing to download.

### stdlib over dependencies

Reel prefers Go's standard library over third-party packages. Some examples:

| Need | Approach | Why not the alternative |
|------|----------|------------------------|
| Disk space check | `syscall.Statfs` | `gopsutil` pulls in `go-ole`, `wmi`, and platform-specific bloat for a single function call |
| HTTP routing | `gorilla/mux` | Lightweight and battle-tested. Go 1.22+ `http.ServeMux` is an option for the future |
| JSON logging | Manual `json.Marshal` | Full logging frameworks (zap, zerolog) add binary size for features we don't use |
| Config parsing | `gopkg.in/yaml.v3` | No alternative for YAML; the stdlib only handles JSON |
| File operations | `os`, `io`, `path/filepath` | Always prefer stdlib for file I/O |

### Heavy dependencies in separate binaries

The magnet-to-torrent conversion feature requires `anacrolix/torrent`, which pulls in ~60 transitive dependencies including the entire pion/WebRTC stack. Instead of paying this cost in the main binary (which runs 24/7 as a daemon), the conversion logic lives in a **separate helper binary** (`magnet2torrent`) that is only spawned on demand.

```
reel              (main daemon, ~12 MB stripped)
                      │
                      │  exec.Command (only when needed)
                      ▼
magnet2torrent    (helper, ~13 MB stripped)
                      │
                      │  DHT peer discovery
                      ▼
                  .torrent file bytes → stdout
```

The main binary searches for the helper next to its own executable, then falls back to `$PATH`. If the helper is missing, the magnet link is passed directly to the torrent client (which usually handles magnets natively). This means:

- The main daemon's memory footprint stays small
- The helper process lives only for the duration of the conversion (seconds) and exits
- The feature is opt-in (`magnet_to_torrent_enabled` in config)
- The system degrades gracefully if the helper isn't installed

### Pre-compiled regex and static allocations

All regex patterns in the torrent matching and filtering hot paths are compiled once at initialization and reused. Lookup maps for technical terms, stop words, and resolution synonyms are package-level variables, not per-call allocations.

This matters because the matching engine runs against every search result from every indexer — potentially hundreds of titles per search cycle. On a Pi 3B, avoiding repeated regex compilation and map allocation directly reduces GC pressure and CPU usage.

### Pipeline architecture with rollback

Post-processing uses a stage-based pipeline that supports:

- **Conditional execution** — stages can be skipped based on media type, file count, or file size
- **Crash recovery** — pipeline state is persisted to disk, so interrupted processing resumes where it left off
- **Rollback on failure** — if a stage fails, previous stages can undo their changes

This is more complex than a simple "move and rename" script, but it prevents the most common failure mode in media automation: a crash leaves files in an inconsistent state (half-moved, partially renamed, missing from the database).

### SQLite with pure Go driver

The database is SQLite via `modernc.org/sqlite`, a pure Go implementation that doesn't require CGO. This means:

- Cross-compilation to ARM works without a C cross-compiler
- No shared library dependencies at runtime
- The database is a single file that's easy to back up

The tradeoff is slightly lower performance than the CGO-based `mattn/go-sqlite3`, but for Reel's workload (dozens of media entries, not millions), this is irrelevant.

## What Reel is not

Reel is not trying to be Sonarr, Radarr, or Flexget. It doesn't have:

- User management or multi-user support
- A plugin system
- API compatibility with other tools
- Mobile apps

It's a personal tool that does one thing well: automate media downloads on hardware that most software ignores.
