# Reel

Reel is a self-hosted media automation tool. It allows you to add movies and TV shows to a library, and it will automatically search for them on your chosen indexers, download them with your preferred torrent client, and organize them for you.

## How to Run

### Docker (Recommended)

The easiest way to run Reel is with Docker.

1.  **Create a `config.yml` file:**

    Start with the provided `config.example.yml` and customize it to your needs. You will need to provide your own API keys for TMDB and your indexer.

2.  **Run the Docker container:**

    ```bash
    docker run -d \
      --name=reel \
      -p 8081:8081 \
      -v /path/to/your/config.yml:/app/config/config.yml \
      -v /path/to/your/data:/app/data \
      --restart unless-stopped \
      reel:latest
    ```

    * Replace `/path/to/your/config.yml` with the actual path to your configuration file.
    * Replace `/path/to/your/data` with the path where you want to store the Reel database and other data.

### From Source

1.  **Clone the repository:**

    ```bash
    git clone [https://github.com/user/reel.git](https://github.com/user/reel.git)
    cd reel
    ```

2.  **Install dependencies:**

    ```bash
    go mod download
    ```

3.  **Build the binary:**

    ```bash
    go build ./cmd/reel
    ```

4.  **Run the application:**

    ```bash
    ./reel -config /path/to/your/config.yml
    ```

## Configuration

Reel is configured using a `config.yml` file. Here are some of the key settings:

* **`app`**: General application settings, including the port, data path, and UI password.
* **`indexer`**: Your torrent indexer settings (e.g., Scarf, Jackett).
* **`torrent_client`**: Your torrent client settings (e.g., Transmission, qBittorrent).
* **`metadata`**: Metadata provider settings (e.g., TMDB, IMDb).
* **`database`**: The path to the SQLite database file.
* **`automation`**: Automation settings, such as the search interval and quality preferences.

## API Usage

Reel provides a simple REST API for interacting with the application.

### Add Media

To add a new movie or TV show to your library, send a `POST` request to `/api/v1/media`:

```bash
curl -X POST http://localhost:8081/api/v1/media \
  -H "Content-Type: application/json" \
  -d '{
    "type": "movie",
    "title": "The Matrix",
    "year": 1999
  }'