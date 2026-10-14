# Usage analytics

Release builds with `AnalyticsAPIKey` configured send usage events to PostHog EU.
The existing **Settings > Experiments > Usage analytics** preference controls all
of these events, including performance summaries. Local builds without the
project key create no client. CPU profiling is independent of usage analytics.

Schema version 2 includes the original input **file name**, with folder paths
removed. Native document-picker and drag-and-drop names take precedence over
private cache names and embedded guest titles. Names are limited to 255 Unicode
characters and exclude control characters. No file contents, full paths, raw
logs, screenshots, or memory dumps are included in analytics.

Each installation has a persisted random `distinct_id`. Each foreground period
has a fresh UUIDv7 `$session_id`; each input open has a random `title_run_id`.
Continuing the same title after backgrounding retains its title run ID. Title
events carry `file_name`, `sha256`, `image_sha256`, `profile_id`, `input_format`,
and `mode` (`application` or `system`). Unknown hashes are empty: an early load
failure can still be diagnosed by its file name and title run ID.

## Events

| Event | Meaning |
| --- | --- |
| `app_started` | Process/shell launch; resuming does not send another launch event. |
| `session_started` | Foreground analytics session; `start_reason` is `launch`, `resume`, `analytics_enabled`, or `rotation`. |
| `title_milestone` | Input open (`inspecting`) or a changed frontend execution/error state; not a claim of first frame or playability. |
| `session_ended` | Foreground duration, last state, title context, and `end_reason` (`background`, `exit`, or `rotation`). |
| `performance_summary` | Aggregated guest execution and host work for one title, session, and settings configuration. |

`session_ended.had_error` means that the last title state was a guest/load error.
The legacy `crashed` property has the same meaning for existing insights; neither
is a native process crash detector.

## Android lifecycle

The existing native `Pause`/`Resume` bridge calls `Shell.SetHostActive`.
Backgrounding emits a final performance summary followed by `session_ended`,
then flushes/closes the client on a worker with a 1.5-second shutdown deadline.
It works even after the Ebitengine game loop has been suspended and does not
wait for network I/O on the Activity thread. Repeated pause callbacks cannot
duplicate session endings. Resume creates a new client and session; time spent
in the background is excluded.

Long foreground sessions, including an idle launcher, rotate after 23 hours to
stay within PostHog's 24-hour limit for custom UUIDv7 sessions.

Desktop exit flushes synchronously with the same bounded deadline. Forced
process termination, browser tab closure, or network failure can still lose
queued events; analytics is best-effort rather than durable storage.

## Performance summaries

An active title sends one summary after 60 seconds of runnable guest time. A
pause, modal panel, settings change, title close/change, background transition,
or orderly exit sends the remaining partial window. Idle windows create no
performance events. Counters reset per window; modal and background time do not
count as poor emulator performance. A drifting measured display refresh rate
updates the weighted target without flooding events.

| Property | Meaning |
| --- | --- |
| `duration_seconds` | Runnable host wall time in this window. |
| `guest_seconds`, `measured_speed` | Completed emulated time, and guest time divided by runnable host time. |
| `requested_speed`, `effective_speed` | User speed setting, and the time-weighted pacing target, including display sync. |
| `achieved_percent` | Completed guest time divided by the integrated pacing target, multiplied by 100. |
| `frame_work_count`, `mean_frame_work_ms`, `max_frame_work_ms` | Completed guest quanta and their mean/max processing time; not rendered FPS. |
| `audio_underruns`, `audio_overruns`, `audio_dropped_frames` | Host audio counter increments within this window. |
| `cpu_choice` | Selected CPU setting, such as `fastest`; not the resolved CPU backend. |
| `display_sync`, `display_sync_active`, `vsync_disabled`, `ui_priority` | Settings used for this execution window. |
| `summary_reason` | `interval`, `paused`, `settings_changed`, `title_closed`, `title_changed`, `background`, `exit`, `rotation`, or an ending execution/error state. |

Audio counters are read at most once per second during normal updates and once
at window/lifecycle boundaries. Frame-work counters already produced by the
worker are aggregated locally; individual frames are never sent as events.
Turning analytics off discards any unfinished summary and prevents collection
of further performance samples.

Useful PostHog breakdowns are `app_version`, `platform`, `file_name`,
`profile_id`, and `cpu_choice`. Weight speed and frame means by duration/frame
count when combining windows, rather than averaging short and long windows
equally. Synthetic analytics tests use a recording client and never send events
to the production project.
