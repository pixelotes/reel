# Automation

Reel runs background tasks on configurable intervals using cron syntax. These tasks handle searching, downloading, processing, and cleanup automatically.

## Scheduled Tasks

| Task             | Config Key                      | Default       | Description                                          |
|------------------|---------------------------------|---------------|------------------------------------------------------|
| Search indexers  | `search_interval`               | `@every 30m`  | Searches configured indexers for wanted media         |
| RSS processing   | `rss_processing_interval`       | `@every 1h`   | Processes RSS feed sources                            |
| Download status  | `download_status_interval`      | `@every 10s`  | Checks torrent client for completed downloads         |
| Episode check    | `new_episodes_check_interval`   | `@every 6h`   | Checks metadata providers for new episodes            |
| Cleanup          | `cleanup_interval`              | `@every 24h`  | Removes old completed/seeded torrents                 |
| Retry failed     | `retry_failed_interval`         | `@every 1h`   | Retries failed downloads                              |

## Cron Syntax

Reel uses Go's `robfig/cron` format for scheduling intervals.

| Expression     | Meaning           |
|----------------|-------------------|
| `@every 30m`   | Every 30 minutes  |
| `@every 1h`    | Every hour        |
| `@every 6h`    | Every 6 hours     |
| `@every 24h`   | Every 24 hours    |

!!! tip
    You can use any valid duration unit: `s` (seconds), `m` (minutes), `h` (hours).

## Torrent Cleanup

Control when completed torrents are removed from the client:

| Setting                    | Description                                          |
|----------------------------|------------------------------------------------------|
| `keep_torrents_for_days`   | Remove completed torrents after N days               |
| `keep_torrents_seed_ratio` | Remove once seed ratio is reached (e.g. `1.2`)       |

!!! note
    Both conditions are evaluated independently. A torrent is removed when **either** condition is met.

## Episode Delay

| Setting                          | Description                                              |
|----------------------------------|----------------------------------------------------------|
| `episode_download_delay_hours`   | Wait N hours after air date before searching             |

!!! info
    This is useful to let higher quality releases appear before Reel starts searching. For example, setting this to `6` gives uploaders time to release proper encodes instead of early, lower-quality versions.

## Full Configuration Example

```yaml
automation:
  search_interval: "@every 30m"
  rss_processing_interval: "@every 1h"
  download_status_interval: "@every 10s"
  new_episodes_check_interval: "@every 6h"
  cleanup_interval: "@every 24h"
  retry_failed_interval: "@every 1h"

  keep_torrents_for_days: 7
  keep_torrents_seed_ratio: 1.2

  episode_download_delay_hours: 6
```
