# Torrent Clients

Reel manages downloads through a single torrent client configured in the `torrent_client` section of your config file. It communicates with the client's API to add torrents, monitor progress, and remove completed downloads.

---

## Supported Clients

| Client | Default Port | Protocol | Auth Method |
|--------|-------------|----------|-------------|
| Transmission | 9091 | Web API (RPC) | Username/password (optional) |
| qBittorrent | 8080 | Web API (session cookies) | Username/password (required) |
| Aria2 | 6800 | JSON-RPC | RPC secret token |
| Rain | 7246 | JSON-RPC 2.0 | None |
| Deluge | 8112 | JSON-RPC | Username/password (required) |

---

## Transmission

Transmission exposes an RPC-based web API on port 9091. Authentication is optional and depends on your Transmission configuration.

```yaml
torrent_client:
  type: "transmission"
  host: "localhost:9091"
  username: ""       # optional
  password: ""       # optional
  download_path: "/downloads/media"
```

!!! tip
    If Transmission is running in Docker alongside Reel, use the container name as the host (e.g., `transmission:9091`).

---

## qBittorrent

qBittorrent uses a web API with session-based cookie authentication. Both username and password are required.

```yaml
torrent_client:
  type: "qbittorrent"
  host: "localhost:8080"
  username: "admin"
  password: "adminadmin"
  download_path: "/downloads/media"
```

!!! note
    The default qBittorrent credentials are `admin` / `adminadmin`. Change these in the qBittorrent web UI before exposing the service.

---

## Aria2

Aria2 uses a JSON-RPC protocol and authenticates with a secret token instead of a username/password pair. The `host` field must be the full JSON-RPC URL.

```yaml
torrent_client:
  type: "aria2"
  host: "http://localhost:6800/jsonrpc"
  secret: "your_rpc_secret_token"
  download_path: "/downloads/media"
```

!!! warning
    The `username` and `password` fields are ignored for Aria2. Use the `secret` field to set the RPC token.

---

## Rain

[Rain](https://github.com/cenkalti/rain) is a lightweight Go BitTorrent client, battle-tested in production at put.io since 2019. It exposes a JSON-RPC 2.0 API with no authentication. Ideal for resource-constrained environments like Raspberry Pi.

```yaml
torrent_client:
  type: "rain"
  host: "http://localhost:7246"
  download_path: "/downloads/media"
```

!!! warning "Download path"
    Rain does not support per-torrent download directories. All torrents download to the `datadir` configured in Rain's own `config.yaml`. Make sure `download_path` in Reel matches Rain's `datadir` setting.

!!! tip "Resource usage"
    Rain uses ~9 MiB RAM with tuned cache settings, making it the lightest option for ARM devices. See the [Storm container](https://github.com/pixelotes/storm) for a Pi-optimized Docker setup.

---

## Deluge

Deluge uses a JSON-RPC API with automatic reconnection. Both username and password are required.

```yaml
torrent_client:
  type: "deluge"
  host: "localhost:8112"
  username: "admin"
  password: "deluge"
  download_path: "/downloads/media"
```

!!! note
    The default Deluge web UI password is `deluge`. Change it on first login.

---

## Configuration Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Client type: `transmission`, `qbittorrent`, `aria2`, `rain`, or `deluge`. |
| `host` | string | Yes | Host and port, or full URL for the client API. |
| `username` | string | Depends | Username for authentication. Used by Transmission, qBittorrent, and Deluge. |
| `password` | string | Depends | Password for authentication. |
| `secret` | string | Depends | RPC secret token. Used by Aria2 only. |
| `download_path` | string | Yes | Directory where the torrent client saves downloaded files. |

---

## Choosing a Client

| Feature | Transmission | qBittorrent | Aria2 | Rain | Deluge |
|---------|-------------|-------------|-------|------|--------|
| Resource usage | Low | Medium | Very low | Very low | Medium |
| Web UI included | Yes | Yes | No | No | Yes |
| Auth optional | Yes | No | N/A (token) | N/A | No |
| Auto-reconnect | No | No | No | No | Yes |
| Per-torrent download path | Yes | Yes | Yes | No | Yes |
| Best for | Simplicity | Feature-rich UI | Headless/minimal | Pi/ARM minimal | Plugin ecosystem |

!!! tip "Raspberry Pi users"
    Rain and Aria2 have the lowest resource footprint, making them the best choices for ARM devices and single-board computers. Rain has the advantage of reliable session persistence and resume, unlike Aria2 which has known issues with BitTorrent session handling.
