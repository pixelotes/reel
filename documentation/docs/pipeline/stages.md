# Pipeline Stages

Reel provides 12 pipeline stages. Six are **core stages** included in the default pipeline, and six are **extended stages** available for opt-in use.

## Summary

| Stage | Default | Rollback | Fatal on Error |
|---|---|---|---|
| `validate` | Yes | No | Yes |
| `create_folders` | Yes | No | Yes |
| `move_files` | Yes | Yes | Yes |
| `rename` | Yes | Yes | Yes |
| `subtitles` | Yes | No | No |
| `notify` | Yes | No | No |
| `extract` | No | Yes | Yes |
| `health_check` | No | No | No |
| `enrich_metadata` | No | No | No |
| `duplicate_check` | No | No | No |
| `permission_check` | No | No | Yes |
| `space_check` | No | No | Yes |

---

## Core Stages

These stages form the default pipeline when no custom stages are configured.

### validate

Filters files by video extension and minimum size, removing non-video and too-small files from processing.

- **Accepted extensions:** `.mkv`, `.mp4`, `.avi`, `.mov`
- **Minimum size:** Configured via `postprocessing.min_file_size_mb` (default: 1 MB)
- **Behavior:** Updates `OriginalFiles` in the processing context to contain only valid video files. Fails if no valid files remain after filtering.
- **Rollback:** None.
- **When to use:** Always recommended as the first stage to ensure only legitimate video files proceed through the pipeline.

```yaml
- name: validate
  enabled: true
```

!!! note

    Validation requires `postprocessing.validate_files: true` in addition to the stage being enabled. If `validate_files` is `false`, the stage passes through without filtering.

---

### create_folders

Creates the destination directory structure based on media type, title, year, and season.

- **Movies:** `{destination_folder}/{Title} ({Year})/`
- **TV Shows / Anime with season:** `{destination_folder}/{Title} ({Year})/S{XX}/`
- **Title sanitization:** Special characters are removed from the title to produce filesystem-safe folder names.
- **Rollback:** None. Folders are kept on rollback because they may be shared by other media.
- **When to use:** Required before `move_files`. This stage sets the `DestinationDir` in the processing context.

```yaml
- name: create_folders
  enabled: true
```

---

### move_files

Moves or links files from the download location to the destination directory.

- **Move methods:** Tries methods in order from the `move_method` config for the media type. Supported methods: `hardlink`, `symlink`, `move`, `copy`.
- **Retry logic:** Configurable via `postprocessing.retry_attempts` (default: 3) and `postprocessing.retry_delay` (default: 5 seconds).
- **File availability:** Waits for files to appear on disk up to `postprocessing.wait_for_file_timeout` seconds (default: 30).
- **Rollback:** Removes copied or moved files from the destination directory.
- **When to use:** Core stage for transferring files to their final location. Must run after `create_folders`.

```yaml
- name: move_files
  enabled: true
```

!!! tip

    Configure move methods in order of preference. Hardlink is fastest and saves disk space, but only works on the same filesystem. A typical fallback chain is `["hardlink", "symlink", "move", "copy"]`.

---

### rename

Renames files at the destination using templates from the `file_renaming` configuration.

- **Available placeholders:** `{title}`, `{year}`, `{season}`, `{episode}`, `{quality}`
- **Quality detection:** Parsed automatically from the torrent name (e.g., `1080p`, `2160p`, `WEB-DL`).
- **Subtitle files skipped:** Files with `.srt`, `.sub`, or `.ass` extensions are not renamed.
- **Fallback naming:** If no template is configured, defaults to `Title (Year) [Quality].ext` for movies or `Title - S01E01 [Quality].ext` for series.
- **Rollback:** Restores original filenames using metadata stored during execution.
- **When to use:** Run after `move_files` to apply clean, standardized filenames.

```yaml
- name: rename
  enabled: true
```

Example renaming templates:

```yaml
file_renaming:
  movie_template: "{title} ({year}) [{quality}]"
  series_template: "{title} - S{season}E{episode} [{quality}]"
  anime_template: "{title} - S{season}E{episode} [{quality}]"
```

---

### subtitles

Downloads subtitles via the OpenSubtitles API for each video file at the destination.

- **Requirements:** `subtitles.enabled: true` and a valid `subtitles.api_key`.
- **Languages:** Downloads subtitles for all languages listed in `subtitles.languages`.
- **Non-fatal:** Errors are collected in the processing context but do not stop the pipeline.
- **Output path:** Subtitles are saved as `{video_name}.{lang}.srt` alongside the video file.
- **Rollback:** None. Downloaded subtitle files are not removed on rollback.
- **When to use:** Run after `rename` so subtitle filenames match the final video filenames.

```yaml
- name: subtitles
  enabled: true
```

---

### notify

Sends notifications to all configured notifiers (Telegram, Pushbullet) about the completed processing.

- **Execution:** Notifications are sent asynchronously (each notifier runs in its own goroutine).
- **Rollback:** None. Sent notifications cannot be recalled.
- **When to use:** Always the last stage in the pipeline.

```yaml
- name: notify
  enabled: true
```

---

## Extended Stages

These stages are not included in the default pipeline. Add them explicitly to your stage configuration.

### extract

Extracts `.zip` and `.rar` archives before other stages process the files.

- **ZIP extraction:** Built-in. Validates against ZipSlip path traversal attacks.
- **RAR extraction:** Requires the `unrar` binary to be available in `PATH`. If `unrar` is not found, extraction fails with an error.
- **Behavior:** Replaces `OriginalFiles` in the processing context with the extracted file list.
- **Rollback:** Removes extracted directories.
- **When to use:** Place early in the pipeline (after `validate`) when torrents may contain archived media files.

```yaml
- name: extract
  enabled: true
  condition: "files > 1"
```

!!! warning

    RAR extraction depends on the external `unrar` command. Make sure it is installed and available in your system `PATH`.

---

### health_check

Verifies file integrity before processing.

- **Readability check:** Opens each file to confirm it can be read.
- **Stability check:** If a file was modified within the last 5 seconds (possibly still being written), the stage waits before proceeding.
- **Header verification:** Reads the file header (first 8 KB) to verify the file is not empty or corrupt.
- **Non-fatal:** Verification errors are collected as warnings but do not stop the pipeline.
- **Rollback:** None (read-only stage).
- **When to use:** Place early in the pipeline to catch corrupt or incomplete downloads before moving files.

```yaml
- name: health_check
  enabled: true
```

---

### enrich_metadata

Adds processing metadata to the context for informational and debugging purposes.

- **Added metadata:**
    - `processing_time` -- Timestamp of when processing started
    - `torrent_hash` -- Hash of the source torrent
    - `torrent_name` -- Name of the source torrent
    - `quality` -- Detected quality (e.g., `1080p`, `2160p`)
    - `file_count` -- Number of files being processed
    - `total_size_mb` -- Total size of all files in megabytes
- **Rollback:** None (informational only).
- **When to use:** Useful for logging and debugging. Can be placed anywhere in the pipeline but is most informative early on.

```yaml
- name: enrich_metadata
  enabled: true
```

---

### duplicate_check

Checks whether the destination directory already contains video files with a similar size.

- **Detection threshold:** Files with less than 1% size difference are flagged as potential duplicates.
- **Behavior:** Sets a `duplicate_warning` flag in the processing context metadata. Does not block the pipeline.
- **Rollback:** None (read-only stage).
- **When to use:** Helpful for movies where re-downloads of the same content are common. Best placed before `move_files`.

```yaml
- name: duplicate_check
  enabled: true
  condition: "is_movie"
```

!!! note

    This stage only warns about potential duplicates. It does not prevent processing. Check your logs for `duplicate_warning` entries.

---

### permission_check

Verifies that Reel has the required filesystem permissions before processing.

- **Source files:** Checks read permission on every file in `OriginalFiles`.
- **Destination:** Creates and immediately removes a test file (`.reel_permission_test`) in the destination directory to verify write access.
- **Rollback:** None (non-destructive).
- **When to use:** Place before `move_files` to fail fast if permissions are misconfigured, rather than discovering the problem mid-transfer.

```yaml
- name: permission_check
  enabled: true
```

---

### space_check

Checks available disk space at the destination before transferring files.

- **Buffer requirement:** Requires 10% more free space than the total size of all files to be processed.
- **System call:** Uses `statfs` to query available disk space at the destination path.
- **Rollback:** None (read-only stage).
- **When to use:** Place before `move_files` to prevent partial transfers that fill the disk.

```yaml
- name: space_check
  enabled: true
  condition: "file_size > 1GB"
```

!!! tip

    Combine `space_check` with a file size condition to skip the check for small downloads where disk space is unlikely to be an issue.
