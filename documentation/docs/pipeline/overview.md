# Pipeline Overview

The post-processing pipeline runs after a torrent download completes. It processes media files through a sequence of configurable stages, handling everything from file validation and extraction to renaming, subtitle downloads, and notifications.

## How It Works

When a download finishes, Reel identifies the media files and passes them through a series of **stages**. Each stage performs a specific task and hands off a shared `ProcessingContext` to the next stage in the sequence.

```
Download Complete
  -> Identify media files
  -> [validate] -> [create_folders] -> [move_files] -> [rename] -> [subtitles] -> [notify]
  -> Done (or Rollback on failure)
```

!!! info "Key Concepts"

    - Stages execute **sequentially** -- each stage receives a `ProcessingContext` containing media info, file paths, and metadata.
    - If a stage **fails**, subsequent stages do not run.
    - If **rollback** is enabled, completed stages roll back in reverse order on failure.
    - **State persistence** saves progress to disk as JSON after each stage, surviving crashes and allowing resumption.

## Pipeline vs Legacy Mode

Reel supports two post-processing modes:

| Feature | Legacy Mode | Pipeline Mode |
|---|---|---|
| Enabled by | `postprocessing.enabled: true` only | `postprocessing.enabled: true` + `postprocessing.pipeline.enabled: true` |
| Behavior | Create folders, move, rename, notify (monolithic) | Configurable stage sequence |
| Rollback | No | Yes (reverse-order rollback on failure) |
| Crash recovery | No | Yes (state persisted to disk as JSON) |
| Custom stages | No | Yes (12 available stages) |
| Stage conditions | No | Yes (per-stage conditional execution) |

!!! warning

    Without `postprocessing.pipeline.enabled: true`, Reel falls back to the simpler legacy mode which runs create folders, move, rename, and notify as a single monolithic operation with no rollback or recovery support.

## Default Stages

When pipeline mode is enabled but no stages are explicitly configured, Reel uses the following default sequence:

1. `validate` -- Filter files by video extension and minimum size
2. `create_folders` -- Create the destination directory structure
3. `move_files` -- Move or link files from download to destination
4. `rename` -- Rename files using configured templates
5. `subtitles` -- Download subtitles via OpenSubtitles
6. `notify` -- Send notifications to configured notifiers

## Minimal Configuration

Enable the pipeline with default stages:

```yaml
postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 1
  pipeline:
    enabled: true
    rollback: true
```

This activates pipeline mode with all six default stages, rollback on failure, and state persistence for crash recovery.

## Custom Stage Configuration

Define exactly which stages run and in what order:

```yaml
postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 1
  retry_attempts: 3
  retry_delay: 5
  wait_for_file_timeout: 30
  pipeline:
    enabled: true
    rollback: true
    stages:
      - name: validate
        enabled: true
      - name: extract
        enabled: true
        condition: "files > 1"
      - name: health_check
        enabled: true
      - name: permission_check
        enabled: true
      - name: space_check
        enabled: true
        condition: "file_size > 1GB"
      - name: duplicate_check
        enabled: true
        condition: "is_movie"
      - name: enrich_metadata
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

!!! tip

    You can disable any stage by setting `enabled: false` without removing it from the configuration. This makes it easy to toggle stages on and off during troubleshooting.

## Stage Ordering

Stage order matters. Some stages depend on the output of previous stages:

- `create_folders` must run before `move_files` (it sets the destination directory).
- `move_files` must run before `rename` (rename operates on files at the destination).
- `validate` and `extract` should run early to filter and prepare files before other stages process them.
- `notify` should run last so it reports on the final state.

!!! danger

    Placing `move_files` before `create_folders` will cause the pipeline to fail because no destination directory has been set.
