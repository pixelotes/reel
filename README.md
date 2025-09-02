# Reel: A Lightweight Media Management Tool
Reel is a self-hosted media management tool designed for low-end devices like the Raspberry Pi 3+. It’s built to be lightweight, consuming less than 10MB of RAM and minimal CPU, making it a perfect starting point for your media automation needs.

## ⚠️ Disclaimer
This project is in its early stages. While it handles the basics, you may encounter bugs and unfinished features. The configuration tab in the UI, for example, is not yet functional, and multi-user support is not yet implemented. Think of this as a solid foundation, not a fully-featured alternative to more mature applications.

## 🚀 Getting Started
You can run Reel either from the source code or using Docker Compose.

## From Source
Clone the repository:

```bash
git clone https://github.com/your-username/reel.git
cd reel
```

Install dependencies:

```bash
go mod download
```

Run the application:

```bash
go run ./main.go -config /path/to/your/config.yml
```

### With Docker Compose
Create a docker-compose.yml file, similar to the example below. Be sure to update the volumes to match your setup.

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
```

Create your config.yml file inside the ./reel/config directory. You can start with the provided config.example.yml.

Start the container:

```bash
docker-compose up -d
```

## 🛠️ Configuration
Reel is configured using a config.yml file. Currently, it supports:

- Indexer: Scarf (with Jackett support planned).
- Download Client: Transmission (with qBittorrent support planned).0
- Sources: Torznab and RSS feeds.

## ✨ Future Features
Here’s a glimpse of what’s planned for the future:

- Expanded Support: More download clients and Torznab servers.
- Customization:
  - Configurable scoring for torrents.
  - Customizable file renaming patterns.
  - Configurable notifications for various events.
- Automation:
  - Automatic quality upgrades for movies.
  - (Done) Cleanup of torrents based on upload ratios.
  - (Done) Addition of external trackers to torrents.
- Integrations:
  - More metadata providers.
  - A built-in subtitle downloader.

## 📄 License
Reel is open-source software licensed under the MIT License.
