# Troubleshooting Guide

This guide helps you troubleshoot common issues with media processing, including nonstandard packing, numbering, and merged files.

## Common Issues

### Unrecognized Files or Missing Episodes

If Reel fails to detect files or reports missing episodes, check the following:

1.  **Check Logs:** Enable debug logging in your `config.yml` (`app.debug: true`) and re-run the process. Look for "unrecognized file" or "pattern mismatch" messages.
2.  **Verify Naming:** Reel expects a recognizable pattern. If your files are named unexpectedly (e.g., `ShowName.01.mkv` instead of `ShowName.S01E01.mkv`), it may fail to parse them.
3.  **Nonstandard Packing:** For series that use absolute numbering (e.g., episode 500 instead of S10E10), you may need to configure a custom naming template or rename the files manually if they don't match standard series patterns.
4.  **Merged Audio/Video:** If audio and video are separate (e.g., `.mkv` video and `.flac` audio), Reel may not automatically merge them. Ensure the video file contains the primary audio track or manually merge them before processing.

### Pipeline Failures

If the pipeline fails repeatedly:

1.  **Check `rollback` Status:** Review the logs to see if a rollback occurred and why the original stage failed.
2.  **Manual Intervention:** If a stage is stuck, you can try to resolve the underlying issue (e.g., file permissions, disk space) and restart Reel. The crash recovery mechanism should pick up from the last completed stage.
3.  **Database Inconsistency:** If you suspect the database state is corrupt, examine the processing state files in `{data_path}/processing_states/` to identify the stuck item and remove the stale state file if necessary.

!!! tip
    If you're still stuck, join our community forums or check the GitHub issue tracker to see if others have encountered similar packing or numbering conventions.
