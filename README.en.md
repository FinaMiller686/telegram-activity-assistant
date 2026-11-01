# Telegram Activity Assistant

[简体中文](README.md) | [English](README.en.md)

Turn Telegram event notices into Chinese summaries sent to a private Bot. Save events you want to attend, mark registrations, and add events to your calendar.

The app runs on your computer. macOS uses a native launcher; Windows uses Docker Desktop.

## Features

- Choose which groups and channels to follow. Extract event times, venues, descriptions, and registration deadlines.
- Combine reposts of the same event, keep source links, and flag time conflicts.
- Browse upcoming events and manage your choices in the web interface or Bot.
- Receive a daily digest at 21:00 Singapore time and reminders before registration closes.
- Export `.ics` files for Apple Calendar, Google Calendar, and Outlook, or open a Google / Outlook draft and save it yourself.

Connect a local model or an OpenAI-compatible endpoint for Chinese summaries. Without a model, the app uses rules to extract some dates, venues, and links; it does not translate full notices. The web interface can be switched to English.

## Windows

Install [Docker Desktop](https://docs.docker.com/desktop/setup/install/windows-install/) with WSL 2 and Linux containers. Download and extract the project, open PowerShell in its folder, and run:

```powershell
.\scripts\windows.ps1 setup
```

Follow the [Windows guide](docs/WINDOWS.en.md) to check disk protection and update `.env`, then start:

```powershell
.\scripts\windows.ps1 start
```

The script builds the app and opens your browser when the services are ready. No separate Go, Node.js, or PostgreSQL installation is needed.

The scripts have passed simulated tests. Windows permissions, Docker execution, and a real Telegram login still need testing on Windows.

## macOS

Install [Homebrew](https://brew.sh/), then run:

```bash
git clone https://github.com/FinaMiller686/telegram-activity-assistant.git
cd telegram-activity-assistant
brew install go node postgresql@17
./scripts/macos-native.sh build
./scripts/initialize-encrypted-storage.sh
./scripts/macos-native.sh start
```

Requires Go 1.25+, Node.js 24+, and PostgreSQL 17. Tested on Apple Silicon; Intel Macs have not been tested.

Open [http://localhost:3000](http://localhost:3000):

1. Set a password for the web interface.
2. Get an API ID and API Hash from [my.telegram.org/apps](https://my.telegram.org/apps), then sign in to Telegram in the setup wizard.
3. Create a Bot with [BotFather](https://t.me/BotFather), enter its token, send it `/start`, and link the private chat.
4. Sync the group list and enable message saving for the groups you want to follow.
5. To use AI, enter an endpoint, API key, and model under Settings → Summary engine. Test and save, then enable AI in Activities.

To start automatically after signing in to your Mac:

```bash
./scripts/install-launch-agent.sh
```

Keep the installation folder in place. Monitoring pauses while the computer is asleep, offline, or shut down, then attempts to recover accessible messages after it resumes. See the [user guide](docs/USAGE.en.md) for daily use and troubleshooting.

## Docker

The [Compose configuration](docker-compose.yml) builds this version from source. Docker execution has not been tested here. Read the [Docker notes](docs/USAGE.en.md#docker) before connecting a Telegram account.

## Limits

Images are not read yet; only their text captions are processed. Extraction and deduplication can be wrong, so check the source before registering or attending. Calendar exports and drafts do not update themselves when a group post changes.

With external AI enabled, text from the selected groups is sent to your configured provider. The app does not collect direct chats, post to groups, or submit registration forms. See [data storage and deletion](docs/USAGE.en.md#data-storage-and-deletion).

## Project source and license

Based on [fr0der1c/tgtldr](https://github.com/fr0der1c/tgtldr), commit `88e98fc`, with event extraction, deduplication, Bot actions, and calendar exports added.

The upstream [PolyForm Noncommercial 1.0.0 license](LICENSE) and copyright notice are retained. Noncommercial use is allowed within its terms; commercial use requires separate permission. This is not an OSI-approved open-source license.

[Test notes](docs/TESTING.en.md)
