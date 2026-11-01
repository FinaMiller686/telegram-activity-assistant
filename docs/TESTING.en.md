# Test notes

[简体中文](activity/TESTS.md) | [English](TESTING.en.md)

## Run locally

Backend integration tests need two separate, disposable PostgreSQL databases. They clear data; never point them at your working database.

```bash
cd app
ACTIVITY_TEST_DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/activity_test?sslmode=disable' \
TGTLDR_TEST_DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/upstream_test?sslmode=disable' \
go test -p 1 -count=1 ./...
```

Database tests are skipped when those variables are missing. Bot and model tests use local mock services, not real credentials.

Frontend:

```bash
cd web
node --test lib/calendar.test.mjs
npm run build
```

Windows script checks: `powershell -NoProfile -File scripts/windows-tests.ps1`. Sharing check: `python3 scripts/check-shareable.py`. These run manually; no GitHub test workflow is configured.

## Coverage

- English and relative dates, overnight events, and missing times.
- Reposts, edits, deduplication, conflicts, repeated actions, and saved statuses.
- Different events in the same room remain separate; a venue alone is not enough to merge them.
- Unchanged-card suppression, real changes and reversions, concurrent sends, saved receipts, failure isolation, and retries after restart.
- Owner-only Bot access, calendar files, Google / Outlook draft links, Singapore timezone, equivalent UTC instants, Unicode, and long-link fallbacks.
- Provider-key changes, selected-group and direct-chat boundaries, collection cutoffs, and history deletion.
- Windows setup, existing-key preservation, missing-file failures, paths, and Docker arguments. ACL and Docker calls are mocked.

The calendar suite has seven passing tests. Unit and disposable-database tests passed for the activity, Bot, and API backend packages. Other completed checks include web builds, Linux x64 / ARM64 backend builds, Windows script simulations, and Compose configuration validation. Real text extraction, model processing, private Bot delivery, and `.ics` sending have been checked on an Apple Silicon Mac.

Still untested on real devices: Windows and Intel installations, Docker execution, live message edits/deletions and full sleep recovery, mobile calendar import, and saving drafts in signed-in Google / Outlook accounts. Image recognition is not implemented. Mock tests and successful API receipts do not prove those actions have succeeded.
