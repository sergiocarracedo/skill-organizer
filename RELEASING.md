# Releasing

A short checklist for cutting a new `v*` release of `skill-organizer`.

## Preflight (required before pushing a tag)

The release workflow at `.github/workflows/release.yml` reads two values
from the repository to bake into the binary at build time:

| Variable       | Type        | Source                                        |
| -------------- | ----------- | --------------------------------------------- |
| `NR_ENDPOINT`  | repository variable | `Settings → Secrets and variables → Actions → Variables` |
| `NR_API_KEY`   | repository variable | same tab — **Variables**, not Secrets         |

> **Why variables, not secrets?** The endpoint is a public URL and the
> Insert key is a per-account, rate-limited credential, not a critical
> secret. Variables are visible in the workflow log for debugging and
> don't need the additional ceremony. If you ever decide a value does
> need to be a secret, update both this file and the workflow line in
> `vars.NR_*` → `secrets.NR_*`.

A preflight step in the workflow fails the build with a clear
`::error::` annotation if either variable is empty, so a misconfigured
release will be caught at build time and never reach a download.

## Where to find the values

### `NR_ENDPOINT`

New Relic Insights Events API URL with the account ID substituted:

- **US**: `https://insights-collector.newrelic.com/v1/accounts/<ID>/events`
- **EU**: `https://insights-collector.eu01.nr-data.net/v1/accounts/<ID>/events`

The `<ID>` is the New Relic account number — visible in the browser URL
when you open the account in the NR dashboard, or in the account
switcher (top-right).

### `NR_API_KEY`

The **Insert key**, not the regular API key:

1. Open the New Relic dashboard.
2. Click the account dropdown in the top-right.
3. **API keys** → **Insert keys** tab.
4. Create one scoped to this CLI (e.g. `skill-organizer`) if it
   doesn't exist yet.

Insert keys are scoped to the Insights Events API and have separate
rate limits from the regular API key. They can be rotated at any time
from the same screen.

## How the credentials reach the binary

`.goreleaser.yaml` injects them as Go `-ldflags` at build time:

```
-X .../telemetry.NewRelicEndpoint={{ .Env.NR_ENDPOINT }}
-X .../telemetry.NewRelicAPIKey={{ .Env.NR_API_KEY }}
```

The user never configures these — there is no `endpoint:` or
`api_key:` field in the user's `telemetry.*` config (Phase 5 REQ-10
collapsed the schema to one key, `telemetry.enabled`). The factory in
`internal/telemetry/recorder.go` falls back to `NoopRecorder` if
either build-time var is empty; that is the intended safety net for
dev builds, not a bug.

## Verifying a release

After cutting a release, the matching binary's `telemetry status`
output should look like:

```
INFO  Enabled:        true
INFO  Recorder:       NewRelicRecorder
INFO  Endpoint:       https://insights-collector.newrelic.com/v1/accounts/3223638/events
INFO  Credentials:    yes
INFO  Default model:  (none)
INFO  Buffer:         /home/<user>/.config/skill-organizer/telemetry-buffer.jsonl (0 bytes, 0 events)
```

If you see `Recorder: NoopRecorder` together with a misconfiguration
warning pointing at this file, the binary was built without the
variables — a new release is required to fix it on the user side.
