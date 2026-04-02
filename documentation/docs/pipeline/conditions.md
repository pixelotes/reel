# Stage Conditions

Stages can have conditions that determine whether they execute. When a condition evaluates to `false`, the stage is skipped silently and the pipeline continues to the next stage. Conditions are set via the `condition` field in each stage's configuration.

## Available Conditions

### Boolean Conditions

| Condition | Description |
|---|---|
| `always` / `true` | Always execute the stage (default behavior) |
| `never` / `false` | Never execute the stage (effectively disables it) |

### Media Type Conditions

| Condition | Description |
|---|---|
| `is_movie` | Execute only when processing a movie |
| `is_tv` / `is_tvshow` / `is_tv_show` | Execute only when processing a TV show |
| `is_anime` | Execute only when processing anime |

### File Count Conditions

Compare the number of files identified for processing. Supports standard comparison operators.

| Condition | Description |
|---|---|
| `files > N` | More than N files |
| `files >= N` | N or more files |
| `files = N` | Exactly N files |
| `files < N` | Fewer than N files |
| `files <= N` | N or fewer files |
| `files != N` | Not exactly N files |

### File Size Conditions

Compare the size of the first file in the processing list. Supports `MB` and `GB` units.

| Condition | Description |
|---|---|
| `file_size > 100MB` | First file is larger than 100 MB |
| `file_size > 1GB` | First file is larger than 1 GB |
| `file_size < 500MB` | First file is smaller than 500 MB |

!!! note

    File size conditions evaluate against the **first file** in the `OriginalFiles` list, not the total size of all files. Size units are case-insensitive (`mb`, `MB`, `gb`, `GB` all work).

### Unknown Conditions

Any condition string that does not match a known pattern defaults to `true` and logs a warning. This prevents typos from silently disabling stages.

## Configuration

Set the `condition` field on any stage entry. If `condition` is omitted, the stage always executes.

```yaml
postprocessing:
  pipeline:
    enabled: true
    stages:
      - name: validate
        enabled: true
      - name: extract
        enabled: true
        condition: "files > 1"
      - name: duplicate_check
        enabled: true
        condition: "is_movie"
      - name: space_check
        enabled: true
        condition: "file_size > 1GB"
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true
      - name: subtitles
        enabled: true
        condition: "is_movie"
      - name: notify
        enabled: true
```

## Examples

### Skip subtitle downloads for TV shows

Only download subtitles for movies, where subtitle availability is typically better:

```yaml
- name: subtitles
  enabled: true
  condition: "is_movie"
```

### Extract only multi-file torrents

Archives are common in multi-file torrents but rare in single-file downloads:

```yaml
- name: extract
  enabled: true
  condition: "files > 1"
```

### Check disk space only for large files

Skip the space check for small downloads where disk pressure is unlikely:

```yaml
- name: space_check
  enabled: true
  condition: "file_size > 1GB"
```

### Disable a stage without removing it

Use the `never` condition or set `enabled: false` -- both prevent execution:

```yaml
- name: health_check
  enabled: true
  condition: "never"
```

!!! tip

    Prefer `enabled: false` over `condition: "never"` for permanently disabled stages. Use `condition: "never"` when you want to temporarily disable a stage while preserving its condition for later re-enabling.

## How Conditions Are Evaluated

1. The condition string is trimmed and lowercased.
2. Simple keyword conditions (`always`, `never`, `is_movie`, etc.) are matched first.
3. If the condition starts with `files `, it is parsed as a file count comparison.
4. If the condition starts with `file_size ` or `filesize `, it is parsed as a file size comparison.
5. If no pattern matches, the condition defaults to `true` and a warning is logged.

Conditions are evaluated at execution time, so they have access to the current processing context including the media type, file list, and file sizes on disk.
