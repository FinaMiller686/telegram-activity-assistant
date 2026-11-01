# Windows setup

[简体中文](WINDOWS.md) | [English](WINDOWS.en.md)

Windows uses Docker Desktop. You do not need to install Go, Node.js, or a database separately.

## Before you start

Install [Docker Desktop](https://docs.docker.com/desktop/setup/install/windows-install/), enable WSL 2 when prompted, and restart Windows if needed. Start Docker Desktop with WSL 2 and Linux containers.

From the [repository](https://github.com/FinaMiller686/telegram-activity-assistant), choose Code → Download ZIP and extract it to a local NTFS drive. Keep it outside OneDrive and other automatically synced folders.

## First launch

Open PowerShell in the project folder and run:

```powershell
.\scripts\windows.ps1 setup
```

This generates a database password and a master key in `.env` and `.local/windows/master.key`. Keep both files private.

Check Windows Device encryption or BitLocker for the drives containing the project and Docker's data. Docker's disk-image location is shown under Settings → Resources → Advanced.

After checking, edit `.env`:

```dotenv
TGTLDR_STORAGE_PROTECTED=true
```

This records your check; it does not turn encryption on. Do not connect Telegram until the storage is protected.

Start the app:

```powershell
.\scripts\windows.ps1 start
```

The first build downloads dependencies and may take some time. When ready, the script opens [http://localhost:3000](http://localhost:3000).

Set a web password, sign in to Telegram, link the Bot, and select groups. To use AI, enter an endpoint and key under Settings → Summary engine, test and save, then enable AI. See the [user guide](USAGE.en.md).

## Daily use

Keep Docker Desktop running. You can enable its Start when you sign in option. Containers that you stopped manually need another `start` command.

```powershell
.\scripts\windows.ps1 status
.\scripts\windows.ps1 stop
.\scripts\windows.ps1 start
```

Sleep, shutdown, and network loss pause monitoring. The app attempts to recover messages after resuming. Bot messages can be received on your phone, but no new results are generated while the computer is offline.

Windows has not been tested on a real installation yet. Script simulations, configuration checks, and builds have passed. Start with one group and check delivery before adding more. [Test notes](TESTING.en.md)

<details>
<summary>Troubleshooting</summary>

- **Script blocked:** check that it came from this repository and review it. You can allow local scripts for the current PowerShell session with `Set-ExecutionPolicy -Scope Process RemoteSigned`. A downloaded script may also need Unblock in its file properties. Contact your administrator for managed computers.
- **Docker Desktop is not ready:** start Docker Desktop and wait for its engine.
- **Switch Docker Desktop to Linux containers:** Windows containers are not supported.
- **Confirm encryption:** finish the disk check and `.env` setting above.
- **Master key is missing / invalid:** restore the original key; do not generate a replacement.
- **Web page unavailable:** run `.\scripts\windows.ps1 logs`. If the port is in use, change `TGTLDR_HOST_WEB_PORT` in `.env` and restart.
- **Local model unavailable:** Docker usually reaches the Windows host at `http://host.docker.internal:11434/v1`, not the container's own `127.0.0.1`.

Requires PowerShell 5.1+ and Compose 2.20+. Updating Docker Desktop updates Compose. `docker info --format '{{.OSType}}'` should return `linux`.

</details>

<details>
<summary>Updates, backups, and existing data</summary>

Stop the app before updating. A Git checkout can use `git pull --ff-only`, then `start`. For ZIP installs, replace program files while keeping `.env` and `.local/windows/`.

The database is in Docker volumes; the master key is in the project folder. Back up both and protect them separately. Source code alone is not a data backup, and a database backup without the key cannot decrypt stored credentials.

`stop` keeps your data. `docker compose down -v` deletes volumes. Keep the folder name unchanged where possible: Compose uses it to distinguish projects, and a rename may select a new empty database.

Running `setup` again preserves the existing password and key. A previous generic Compose installation needs its original key backed up and migrated before using this launcher.

The master key is mounted only into the backend, not into its image or database volume. Local file permissions are restricted, but a Compose mount does not encrypt the host. See the [WSL 2 notes](https://docs.docker.com/desktop/features/wsl/) and [Compose secrets documentation](https://docs.docker.com/compose/how-tos/use-secrets/).

</details>
