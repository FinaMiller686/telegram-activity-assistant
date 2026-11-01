# User guide

[简体中文](../使用说明.md) | [English](USAGE.en.md)

## First launch

On Windows, use the [Windows guide](WINDOWS.en.md). The shell commands in the next paragraph are for macOS.

On a Mac, follow the [README](../README.en.md#macos), then start the app:

```bash
./scripts/macos-native.sh start
```

Open [http://localhost:3000](http://localhost:3000). The web interface is available only on this computer. The native launcher uses ports 3000, 18080, and 55433; resolve any conflicts before starting.

Get a Telegram API ID and API Hash from [my.telegram.org/apps](https://my.telegram.org/apps). Enter them in the local setup wizard, followed by your phone number, login code, and two-step verification password if enabled. Enter passwords and keys locally.

## Choose groups

In Groups, turn on message saving for each group or channel you want to follow, then save. No groups are monitored by default. Direct chats are excluded.

After joining a new group, go to Settings → Telegram account → Sync chats, then enable it in Groups. Syncing the list does not enable every group.

To load older posts, use Load historical messages next to a group and select 7 days, 30 days, or a date range. If AI is on, backfilled text is also processed. Expired events remain in the web history without sending new expired-event cards.

## Set up AI

AI is off by default. In Settings → Summary engine, enter the endpoint and API key, fetch the model list, choose a model, and test and save. Then enable AI in Activities. Group AI status follows that switch.

- OpenAI: `https://api.openai.com/v1`. Select a model available to your account.
- Local OpenAI-compatible service: often `http://127.0.0.1:11434/v1`. Install and start the model first.
- A model on the host computer, accessed from Docker: often `http://host.docker.internal:11434/v1`.

AI receives the selected groups' original text, publication time, and Singapore timezone. It decides whether a post is relevant and extracts event details. Image files are not sent; captions are processed as text.

External providers may charge for usage. Changing providers requires entering a key for the new endpoint. Connection tests send fixed test text, not group messages.

Without a model, local rules can extract some notice formats but do not translate full messages. Names, dates, and links may be missing or wrong. Use the original-message link to check them.

## Set up the private Bot

Create a Bot with [BotFather](https://t.me/BotFather), enter its token in the web interface, send it `/start`, and link the private chat in the wizard.

With one signed-in Telegram account, the app sets that account as the allowed Bot user. If the owner field is empty, enter the same account's Telegram user ID in Activities. The Bot checks the sender, private chat, and login status before showing data or accepting actions.

Event card buttons let you:

- Add an event to your list and get its registration link, if one was found.
- Decide later or dismiss an event.
- Mark registration as completed.
- Open the original Telegram message.

These actions update your list. They do not fill out forms or post to groups. Repeated clicks do not create another event.

Commands: `/week` for the next 7 days, `/going` for selected and registered events, and `/calendar` for calendar options.

### Delivery history

The activities page shows messages awaiting extraction, extraction failures, pending deliveries, retries, and the latest history checks across monitored groups. Expand a record to compare its text, timestamp, and Telegram message ID.

An unchanged card for the same activity is not sent again. Real content changes are still delivered. A failed send does not block other activities and is retried later. After five extraction failures, use **Retry failed extraction**.

**Accepted by Telegram** means the API acknowledged the send, not that it was read on your phone. If a network interruption loses the acknowledgement, the record remains unconfirmed; retrying can cause a duplicate. Counts cover saved text only. Image-only notices are not recognized, and AI can miss activities, so these counts do not prove that every activity was delivered.

## Add to a calendar

Both the web interface and Bot can export `.ics` files for Apple Calendar, Google Calendar, and Outlook. In the Bot, choose Add to calendar, or send `/calendar` for older cards.

- **Apple Calendar:** open the `.ics` attachment and confirm the import. If Telegram on iPhone only previews it, try opening the attachment through Mail.
- **Google Calendar:** use the Google button to open a draft, review it, and save. Alternatively, import the `.ics` file from Settings → Import & export on a computer. [Google instructions](https://support.google.com/calendar/answer/37118)
- **Outlook:** choose the work/school or personal-account button. Review the draft before saving, or import the `.ics` file. [Outlook instructions](https://support.microsoft.com/en-us/outlook/import-or-subscribe-to-a-calendar-in-outlook-com-or-outlook-on-the-web)

Calendar files explicitly use Asia/Singapore (UTC+8). GMT+8 is the same offset. Generate a new file to replace an older attachment.

Exact start and end times are required. You can enter missing times in the web card using Singapore time; they apply only to that export. If the content is too long for a draft link, use the file.

Clicking a Google or Outlook button sends that event's title, times, location, and description to the chosen website. You still choose where to save it. The assistant does not access your calendar account, create events automatically, or sync later changes. Bot callbacks require the assistant to be running.

## Start, stop, and restart

Windows commands are in the [Windows guide](WINDOWS.en.md#daily-use). On macOS:

```bash
./scripts/macos-native.sh status
./scripts/macos-native.sh start
```

Enable startup after signing in:

```bash
./scripts/install-launch-agent.sh
```

With autostart installed, stopped processes are restarted. To keep the app stopped, remove the launch agent first:

```bash
launchctl bootout "gui/$(id -u)/local.tgtldr.activity"
rm "$HOME/Library/LaunchAgents/local.tgtldr.activity.plist"
./scripts/macos-native.sh stop
```

Without autostart, run `./scripts/macos-native.sh stop` directly.

Monitoring pauses during sleep, shutdown, or network loss. It attempts recovery on reconnect and every 15 minutes. The first automatic backfill covers up to 7 days; later recovery can go back up to 30 days. Use manual backfill for older ranges. Deleted or inaccessible messages may not be recoverable.

The daily digest defaults to 21:00 Asia/Singapore and can be changed in Activities. If the app resumes later that day, it generates that day's digest; it does not send a digest for every missed offline day. Reminders require the app to be running. Network retries can occasionally produce duplicate deliveries.

## Data storage and deletion

The Mac launcher stores messages, events, the database, and logs in an AES-256 encrypted disk image. Unlock keys and the credential master key are kept in macOS Keychain; FileVault is not required. The image is `.local/native/activity-data.sparsebundle`, initially limited to 4 GB.

The Keychain entries depend on the project path. Before moving the installation or changing computers, back up the image and the corresponding entries: `local.activity-assistant.volume.v1` and `local.activity-assistant.master.v1`. Losing them may prevent recovery. Once unlocked, data can still be accessed by programs with sufficient local permissions.

For Windows storage, see the [Windows guide](WINDOWS.en.md).

Raw messages are kept for 14 days by default. Event records and personal statuses remain. Delete history in the web interface pauses monitoring, clears local messages, events, statuses, digests, and media, and sets a new collection cutoff. Accounts, keys, Bot settings, and group selection remain. Backfill cannot restore posts before that cutoff. Bot messages already sent to Telegram, backups, and system snapshots must be handled separately.

Do not commit `.env`, `.local/`, sessions, keys, logs, or exports. `python3 scripts/check-shareable.py` checks common credential patterns and local-file paths, but cannot find every possible leak.

## Docker

Windows should use the PowerShell launcher, which mounts a separate master key. For a shell-based deployment:

```bash
./scripts/setup-env.sh
docker compose up -d --build
```

Confirm that the Docker data storage is encrypted and that keys are protected separately, then set `TGTLDR_STORAGE_PROTECTED=true` in `.env` and restart. This flag records your deployment check; it does not encrypt anything. Docker and the native Mac launcher use different databases.

```bash
docker compose ps
docker compose logs --tail=100 app
docker compose down
```

`down` keeps volumes; `down -v` deletes them. Docker Desktop must itself start after login for containers to resume. Docker execution has not been tested here.

## Other notes

Two events can use the same room. Deduplication checks names, dates/times, organizers, and other evidence; a shared venue alone is not enough. Uncertain cases may still appear as separate cards.

Campus-only model endpoints need a network connection permitted by their provider. The optional campus network scripts are not part of normal installation, and their connection path still needs validation.

The optional text exporter reads up to 30 days of accessible group/channel history, excludes direct chats and media files, and does not call AI or change monitored groups. Build it with `cd app && go build -o bin/export-text ./cmd/export-text`, then run `scripts/export-telegram-text.sh` from the project root. Its HTML, JSONL, ZIP, and completion list are stored in the encrypted data area; rerunning can resume an unfinished export.

This version uses Activities instead of the upstream group-summary and quick-review flows. The daily activity digest is assembled locally without an extra model call.

See the [test notes](TESTING.en.md) for completed checks and remaining real-device tests. The upstream [noncommercial license](../LICENSE) and the terms of Telegram and your model provider apply.
