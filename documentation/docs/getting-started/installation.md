# Installation

## Docker Compose (recommended)

Create a `docker-compose.yml`:

```yaml
services:
  reel:
    image: pixelotes/reel
    container_name: reel
    restart: unless-stopped
    ports:
      - "8081:8081"
    volumes:
      - ./reel/config:/app/config
      - ./reel/data:/app/data
      - /path/to/media:/media
      - /path/to/downloads:/downloads
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Europe/Madrid
```

Create your config file at `./reel/config/config.yml` (see [Quick Start](quick-start.md)), then start:

```bash
docker compose up -d
```

Access the UI at `http://your-host:8081`.

### ARM32v7 (Raspberry Pi)

A dedicated ARM image is available:

```yaml
services:
  reel:
    image: pixelotes/reel:arm32v7
    # ... same config as above
```

### Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PUID` | User ID for file permissions | `1000` |
| `PGID` | Group ID for file permissions | `1000` |
| `TZ` | Timezone | `UTC` |

## From source

Requirements: Go 1.25+

```bash
git clone https://github.com/pixelotes/reel.git
cd reel
go mod download
go build -o reel .
./reel -config /path/to/config.yml
```

## Docker Compose with Scarf and Transmission

A full stack example with Reel, Scarf (indexer), and Transmission (torrent client):

```yaml
services:
  reel:
    image: pixelotes/reel
    container_name: reel
    restart: unless-stopped
    ports:
      - "8081:8081"
    volumes:
      - ./reel/config:/app/config
      - ./reel/data:/app/data
      - ./usb/media:/media
      - ./usb/downloads:/downloads
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Europe/Madrid

  scarf:
    image: pixelotes/scarf
    container_name: scarf
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./scarf/definitions:/app/definitions
    environment:
      - JWT_SECRET=your-secret
      - UI_PASSWORD=changeme

  transmission:
    image: linuxserver/transmission
    container_name: transmission
    restart: unless-stopped
    ports:
      - "9091:9091"
      - "51413:51413"
      - "51413:51413/udp"
    volumes:
      - ./usb/downloads:/downloads
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Europe/Madrid
```
