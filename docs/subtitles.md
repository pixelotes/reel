# Subtitles

Reel supports automatically downloading subtitles for Movies, TV Shows, and Anime using the **OpenSubtitles.com** API (v1).

## Features
- **Hash-based Search**: Accurately matches subtitles to your specific video file version.
- **Metadata Fallback**: If hash matching fails, it falls back to searching by Title, Year, Season, and Episode.
- **Multi-language Support**: Configure multiple languages (e.g., `["en", "es", "fr"]`).
- **Efficient**: Uses a lightweight client that reads only minimal file data (64KB head/tail) for hashing, suitable for low-resource devices like Raspberry Pi.

## Configuration

To enable subtitles, add the following section to your `config.yml`:

```yaml
subtitles:
  enabled: true
  api_key: "your_opensubtitles_api_key_here"
  languages: ["en"] # List of ISO 639-1 language codes
  download_path: "" # Optional. If empty, subtitles are saved next to the video file.
```

### Obtaining an API Key
1. Go to [OpenSubtitles.com](https://www.opensubtitles.com/) and create an account.
2. Visit the [Consumer API](https://www.opensubtitles.com/en/consumers) usage page to generate an API Key.

## How it Works
1. **Post-Processing**: Subtitle downloading is triggered automatically during the **Post-Processing** phase, immediately after a downloaded file is moved or renamed to its final destination.
2. **Search**: 
   - First, Reel computes the unique file hash.
   - It queries OpenSubtitles with this hash.
   - If no results are found, it queries using the media metadata (Title, Season, Episode).
3. **Download**: The highest-rated subtitle for your preferred language is downloaded.
4. **Saving**: The subtitle is saved with the language code appended, e.g., `MovieName (2023).en.srt`.

## Troubleshooting
- **No subtitles found**: Ensure the release is popular enough to have subtitles. Check logs for "No subtitles found".
- **API Errors**: Verify your API Key is correct and has not exceeded rate limits.
