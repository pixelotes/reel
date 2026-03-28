# Postprocessing Pipeline

The postprocessing pipeline handles everything that happens after a torrent finishes downloading: validating files, moving them to your media library, renaming them, downloading subtitles, and sending notifications.

It uses a **stage-based architecture** where each step runs in order, with support for conditional execution, automatic rollback on failure, state recovery if interrupted, and detailed metrics.

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration Reference](#configuration-reference)
- [Pipeline Stages](#pipeline-stages)
- [Conditions](#conditions)
- [Rollback Behavior](#rollback-behavior)
- [State Recovery](#state-recovery)
- [Metrics](#metrics)
- [Examples](#examples)

## Quick Start

Minimal configuration to get started:

```yaml
postprocessing:
  enabled: true
  pipeline:
    enabled: true
```

This enables the pipeline with the default stage order: `validate` → `create_folders` → `move_files` → `rename` → `subtitles` → `notify`.

## Configuration Reference

### Top-Level Options

```yaml
postprocessing:
  enabled: true                    # Master switch (required)
  validate_files: true             # Enable file validation in the validate stage
  min_file_size_mb: 1              # Minimum video file size in MB (default: 1)
  wait_for_file_timeout: 30        # Seconds to wait for files to appear (default: 30)
  retry_attempts: 3                # Retries for file move/link operations (default: 3)
  retry_delay: 5                   # Seconds between retries (default: 5)
  cleanup_on_failure: true         # Delete processing state after success
  keep_failed_downloads_days: 7    # Days to keep failed download state for debugging
```

### Pipeline Options

```yaml
postprocessing:
  pipeline:
    enabled: true       # Use the pipeline architecture (false = legacy mode)
    rollback: true       # Auto-rollback all completed stages on failure (default: true)
    stages: []           # Ordered list of stages (empty = use defaults)
```

### Stage Options

Each stage in the `stages` list accepts:

| Field       | Type              | Description                                        |
|-------------|-------------------|----------------------------------------------------|
| `name`      | string            | Stage identifier (see table below)                 |
| `enabled`   | bool              | Enable or disable this stage                       |
| `condition` | string (optional) | Condition that must be met for the stage to run     |

## Pipeline Stages

Stages run in the order they appear in the `stages` list. If no stages are configured, the default set is used.

### Default Stages

These 6 stages run by default when `stages` is empty:

| Order | Name             | Description                                |
|-------|------------------|--------------------------------------------|
| 1     | `validate`       | Check files meet minimum requirements      |
| 2     | `create_folders` | Create destination directory structure     |
| 3     | `move_files`     | Move/link files to the library             |
| 4     | `rename`         | Apply naming templates                     |
| 5     | `subtitles`      | Download subtitles                         |
| 6     | `notify`         | Send notifications                         |

### Extended Stages

These additional stages can be added to the pipeline for extra safety and functionality:

| Name               | Description                                           |
|--------------------|-------------------------------------------------------|
| `permission_check` | Verify read/write permissions before processing       |
| `space_check`      | Verify sufficient disk space at destination           |
| `extract`          | Extract ZIP/RAR archives before processing            |
| `health_check`     | Verify file integrity and stability                   |
| `enrich_metadata`  | Attach processing metadata (hash, quality, size, etc) |
| `duplicate_check`  | Detect if media already exists at destination         |

### Stage Details

#### `validate`

Filters files by extension (`.mkv`, `.mp4`, `.avi`, `.mov`) and enforces the minimum file size set in `min_file_size_mb`. Read-only, no rollback needed.

#### `create_folders`

Creates the destination directory structure based on media type:

- Movies: `{destination_folder}/{Title} ({Year})/`
- TV Shows: `{destination_folder}/{Title} ({Year})/S{NN}/`
- Anime: `{destination_folder}/{Title} ({Year})/S{NN}/`

Uses the `destination_folder` from the `movies`, `tv-shows`, or `anime` config section.

#### `move_files`

Moves files to the destination using the `move_method` list defined per media type. Methods are tried in order until one succeeds:

- `hardlink` - Hard link (fastest, no extra space)
- `symlink` - Symbolic link
- `move` - Move file (frees source space)
- `copy` - Full copy (slowest, uses most space)

Respects `wait_for_file_timeout`, `retry_attempts`, and `retry_delay`. Rollback removes the processed files.

#### `rename`

Renames files using templates from `file_renaming`:

```yaml
file_renaming:
  movie_template: "{title} ({year}) [{quality}]"
  series_template: "{title} - S{season}E{episode} [{quality}]"
  anime_template: "{title} - {season}x{episode} [{quality}]"
```

Available variables: `{title}`, `{year}`, `{season}`, `{episode}`, `{quality}`. Quality is parsed automatically from the torrent name. Rollback restores original filenames.

#### `subtitles`

Downloads subtitles using the OpenSubtitles API. Requires the `subtitles` config section:

```yaml
subtitles:
  enabled: true
  api_key: "your-api-key"
  languages: ["en", "es"]
```

Files are named `{video}.{lang}.srt`. Failures are non-fatal and don't stop the pipeline.

#### `notify`

Sends notifications asynchronously using configured notifiers (e.g. Pushbullet). Non-blocking, no rollback.

#### `permission_check`

Verifies that source files are readable and the destination directory is writable (creates and deletes a test file). Fails the pipeline if permissions are insufficient.

#### `space_check`

Calculates total file size + 10% buffer and checks available disk space at the destination. Issues a warning but does not fail the pipeline.

#### `extract`

Extracts `.zip` and `.rar` archives found in the download. ZIP extraction is built-in; RAR requires the `unrar` command to be installed. Rollback removes extracted directories.

#### `health_check`

Checks file readability, modification time (waits if file was recently modified), and reads the first 1MB of video files for basic integrity verification. Non-fatal warnings only.

#### `enrich_metadata`

Attaches processing metadata to the pipeline context:

- `processing_time` - Timestamp
- `torrent_hash`, `torrent_name` - Torrent identifiers
- `quality` - Parsed quality level
- `file_count` - Number of files
- `total_size_mb` - Total size

#### `duplicate_check`

Scans the destination folder for existing video files and uses size comparison (< 1% difference = likely duplicate) to detect duplicates. Sets a `duplicate_warning` metadata flag. Read-only.

## Conditions

Any stage can have a `condition` that determines whether it runs. If the condition is not met, the stage is skipped.

### Boolean Conditions

| Condition         | Behavior       |
|-------------------|----------------|
| `always` / `true` | Always execute |
| `never` / `false` | Always skip    |

### Media Type Conditions

| Condition                          | Matches        |
|------------------------------------|----------------|
| `is_movie`                         | Movies         |
| `is_tv` / `is_tvshow` / `is_tv_show` | TV Shows    |
| `is_anime`                         | Anime          |

### File Count Conditions

Compare the number of files using operators `>`, `>=`, `=`, `<=`, `<`, `!=`:

```yaml
condition: "files > 1"     # Only if more than 1 file
condition: "files = 1"     # Only if exactly 1 file
```

### File Size Conditions

Compare the size of the first file. Supports `MB` and `GB` units:

```yaml
condition: "file_size > 100MB"
condition: "file_size >= 1GB"
condition: "file_size < 500MB"
```

### Default Behavior

Unknown conditions default to `true` (stage runs with a warning logged).

## Rollback Behavior

When `pipeline.rollback` is `true` (default) and a stage fails:

1. The pipeline stops immediately.
2. All previously completed stages are rolled back **in reverse order**.
3. Each stage cleans up its own changes:
   - `move_files` removes copied/linked files from the destination.
   - `rename` restores original filenames.
   - `extract` removes extracted directories.
4. Rollback errors are logged but don't prevent other stages from rolling back.
5. The failure and rollback details are saved to the processing state for debugging.

## State Recovery

The pipeline saves its progress to disk after each stage completes. If the process is interrupted (crash, restart, etc.):

1. On the next run, the pipeline detects the incomplete state.
2. Already-completed stages are skipped.
3. Execution resumes from the last incomplete stage.

State files are stored as JSON in the data directory (`{data_path}/states/{ID}.json`).

- Successful states are deleted if `cleanup_on_failure` is `true`.
- Failed states are preserved for debugging and cleaned up after `keep_failed_downloads_days`.

## Metrics

The pipeline tracks detailed metrics for every run:

**Per stage:** execution time, success/failure, rollback status.

**Per run:** total duration, stages run, stages failed, whether it was a resumed run.

**Aggregated:** total runs, success rate, average/min/max duration per stage. Historical metrics are saved to disk and old entries are cleaned up automatically.

## Examples

### Default Pipeline (Minimal)

```yaml
postprocessing:
  enabled: true
  pipeline:
    enabled: true
```

Runs the 6 default stages with default settings.

### Full Pipeline with All Stages

```yaml
postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 50
  wait_for_file_timeout: 60
  retry_attempts: 5
  retry_delay: 10
  cleanup_on_failure: true
  keep_failed_downloads_days: 14
  pipeline:
    enabled: true
    rollback: true
    stages:
      - name: permission_check
        enabled: true
      - name: space_check
        enabled: true
      - name: extract
        enabled: true
      - name: health_check
        enabled: true
      - name: validate
        enabled: true
      - name: enrich_metadata
        enabled: true
      - name: duplicate_check
        enabled: true
        condition: "is_movie"
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true
      - name: subtitles
        enabled: true
      - name: notify
        enabled: true
```

### Movies Only: Skip Subtitles for Small Files

```yaml
postprocessing:
  enabled: true
  min_file_size_mb: 100
  pipeline:
    enabled: true
    stages:
      - name: validate
        enabled: true
      - name: duplicate_check
        enabled: true
        condition: "is_movie"
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true
      - name: subtitles
        enabled: true
        condition: "file_size > 500MB"
      - name: notify
        enabled: true
```

### Anime Setup: Extract Archives, No Duplicate Check

```yaml
postprocessing:
  enabled: true
  pipeline:
    enabled: true
    stages:
      - name: permission_check
        enabled: true
      - name: extract
        enabled: true
        condition: "is_anime"
      - name: validate
        enabled: true
      - name: health_check
        enabled: true
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true
      - name: subtitles
        enabled: true
      - name: notify
        enabled: true
```

### Disable Rollback and Notifications

```yaml
postprocessing:
  enabled: true
  pipeline:
    enabled: true
    rollback: false
    stages:
      - name: validate
        enabled: true
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true
      - name: subtitles
        enabled: true
      - name: notify
        enabled: false
```

### Legacy Mode (No Pipeline)

```yaml
postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 50
  pipeline:
    enabled: false
```

Bypasses the pipeline entirely and uses the original monolithic post-processor.
