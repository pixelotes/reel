# Recovery and Rollback

Reel provides two mechanisms to handle failures during post-processing: **rollback on failure** and **crash recovery via state persistence**. Together, they ensure that failed or interrupted processing does not leave the filesystem in an inconsistent state.

## Rollback on Failure

When a stage fails and rollback is enabled, all previously completed stages are rolled back in reverse order.

### Configuration

Rollback is enabled by default. To disable it:

```yaml
postprocessing:
  pipeline:
    enabled: true
    rollback: true  # default, set to false to disable
```

### How It Works

1. Stage A completes successfully.
2. Stage B completes successfully.
3. Stage C fails.
4. Rollback triggers: Stage B rolls back, then Stage A rolls back.
5. The pipeline returns an error describing which stage failed.

!!! example "Rollback in action"

    Suppose the pipeline runs `validate -> create_folders -> move_files -> rename` and `rename` fails:

    1. `rename` fails -- pipeline stops.
    2. `move_files` rollback runs -- removes copied/moved files from the destination.
    3. `create_folders` rollback runs -- no-op (folders are kept for other media).
    4. `validate` rollback runs -- no-op (nothing to undo).

### Rollback Behavior by Stage

Not all stages have rollback logic. Stages that perform read-only checks or irreversible actions have no rollback.

| Stage | Rollback Behavior |
|---|---|
| `validate` | None |
| `create_folders` | None (folders kept for other media) |
| `move_files` | Removes files that were copied or moved to the destination |
| `rename` | Restores original filenames |
| `subtitles` | None |
| `notify` | None (cannot unsend notifications) |
| `extract` | Removes extracted directories |
| `health_check` | None |
| `enrich_metadata` | None |
| `duplicate_check` | None |
| `permission_check` | None |
| `space_check` | None |

!!! warning

    Rollback errors are logged but do not halt the recovery process. If a rollback step fails (for example, a file was already deleted externally), the pipeline logs the error and continues rolling back remaining stages.

## Crash Recovery (State Persistence)

The pipeline saves its progress to disk after each stage. If Reel crashes or restarts mid-processing, it can detect the incomplete state and resume from where it left off.

### How State Is Stored

State files are saved as JSON to `{data_path}/processing_states/{id}.json`, where `{id}` is derived from the media ID and torrent hash.

A state file contains:

| Field | Description |
|---|---|
| `id` | Unique identifier (`{media_id}_{torrent_hash}`) |
| `media_id` | ID of the media being processed |
| `media_title` | Title of the media (for logging) |
| `torrent_hash` | Hash of the source torrent |
| `start_time` | When processing started |
| `last_update` | When the state was last saved |
| `status` | One of: `in_progress`, `completed`, `failed` |
| `completed_stages` | List of stages that finished successfully |
| `current_stage` | Stage currently executing (or last attempted) |
| `failed_stage` | Stage that caused the failure (if any) |
| `error` | Error message (if failed) |
| `processed_files` | List of files that have been processed so far |
| `metadata` | Additional metadata (rollback info, etc.) |

### State Lifecycle

```
1. Pipeline starts
   -> State created with status "in_progress"

2. Each stage completes
   -> State updated: stage added to completed_stages list
   -> ProcessedFiles updated with current file paths

3a. Pipeline succeeds
    -> State marked "completed"
    -> State file deleted (if cleanup_on_failure is true)

3b. Pipeline fails
    -> State marked "failed" with error details
    -> If rollback enabled, rollback metadata added to state
```

### Resuming After a Crash

On restart, Reel checks for incomplete processing states:

1. The state manager scans `{data_path}/processing_states/` for JSON files.
2. States with status `in_progress` are identified as incomplete.
3. When the same media/torrent combination is processed again, the existing state is loaded.
4. Stages listed in `completed_stages` are **skipped** -- the pipeline resumes from the first incomplete stage.

!!! note

    Crash recovery resumes from the last **completed** stage, not the last **attempted** stage. If a stage was in progress when the crash occurred, it will be re-executed from the beginning.

### Cleanup

State files are managed automatically:

- **Successful processing:** The state file is deleted after completion if `postprocessing.cleanup_on_failure` is `true`.
- **Old failed states:** Cleaned up periodically based on age. States with status `completed` or `failed` that have not been updated recently are removed.

```yaml
postprocessing:
  enabled: true
  cleanup_on_failure: true  # delete state files after successful completion
  pipeline:
    enabled: true
    rollback: true
```

### Example State File

A state file for an in-progress pipeline looks like this:

```json
{
  "id": "42_abc123def456",
  "media_id": 42,
  "media_title": "Inception",
  "torrent_hash": "abc123def456",
  "start_time": "2026-04-02T10:30:00Z",
  "last_update": "2026-04-02T10:30:15Z",
  "status": "in_progress",
  "completed_stages": ["validate", "create_folders", "move_files"],
  "current_stage": "rename",
  "failed_stage": "",
  "processed_files": [
    "/media/movies/Inception (2010)/inception.2010.1080p.mkv"
  ],
  "metadata": {
    "quality": "1080p",
    "file_count": "1"
  }
}
```

If Reel restarts, it will load this state and skip `validate`, `create_folders`, and `move_files`, resuming directly at the `rename` stage.

## Combining Both Mechanisms

Rollback and crash recovery work together:

- If a stage fails normally, **rollback** handles cleanup immediately.
- If Reel crashes mid-processing, **state persistence** enables resumption on restart.
- If Reel crashes during a rollback, the state file records which rollback steps succeeded via metadata entries (`rollback_count`, `rollback_errors`, `rollback_error_{stage_name}`).

!!! tip

    Keep both `rollback` and state persistence enabled for maximum reliability. The overhead is minimal -- state files are small JSON documents written once per stage completion.
