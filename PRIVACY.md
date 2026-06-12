# Privacy and data protection

> How `skill-organizer` handles telemetry data. Read this alongside
> [`OBSERVABILITY.md`](OBSERVABILITY.md) for the full technical
> and legal picture.

---

## Field-by-field disclosure

The telemetry Event has exactly 5 fields. No other data is ever sent
from the binary. The table below lists each field, its type, and why
it is anonymous.

| Field | Type | Why it's anonymous |
|---|---|---|
| `command` | string | The cobra subcommand name (e.g. `check-security`, `enable`); intrinsic to the call, not user-configurable, and reveals nothing about the user's project or machine. |
| `exit_status` | integer | `0` on success, `1` on error — the cobra exit code, not user-supplied. |
| `timestamp` | string (RFC3339 UTC) | Bucketed server-side if the maintainer needs hour granularity. The timezone is always UTC; no local-clock skew or timezone is transmitted. |
| `version` | string | The CLI semver (e.g. `0.4.0`), a public string the user can see in `skill-organizer --version`. |
| `event_id` | string (32 hex chars) | 16 random bytes per event, generated with `crypto/rand`. Per-event, **not** per-user — it cannot link two events from the same machine. |

No fields beyond these 5 are ever emitted. See the
[Schema-change protocol](#schema-change-protocol) section for what we
will never collect.

---

## Legal basis and data retention

### Legal basis

We rely on **consent** (GDPR Art. 6(1)(a)). The first-run prompt asks
the user to opt in; the answer is sticky. The user can opt out at any
time by running `skill-organizer telemetry disable`.

### Data retention on the device

The on-device buffer (`<appDir>/telemetry-buffer.jsonl`) is a 1 MB
JSONL spool with FIFO eviction. Events that cannot be sent (network
down) stay in the buffer until they are sent or until the buffer
rotates. The buffer is deleted by `skill-organizer telemetry wipe`.

### Data retention on the backend

The chosen processor (New Relic) retains ingested events for 8 days
under the free-tier account. After 8 days, events are dropped from the
New Relic Insights UI; raw storage is deleted per the New Relic DPA.
We do not back up, mirror, or export the data to any other system.

---

## Data-controller statement

The **data controller** is the maintainer of the `skill-organizer` CLI
(see the `CODEOWNERS` file or the GitHub repository's `OWNERS` for the
current individual or team).

The **data processor** is New Relic, Inc. (US). The processor's DPA is
executed as part of the maintainer's New Relic account signup; the
maintainer can produce a copy on request.

**Contact:** open a GitHub issue on the
[skill-organizer repository](https://github.com/sergiocarracedo/skill-organizer).
For privacy-specific requests (data access, deletion, complaint), use
the issue title prefix `privacy:`.

---

## Schema-change protocol

The 5 fields above are the **only** fields the project will ever emit
in v0.x. The following are **explicitly excluded** and will not be
added without a new `OBSERVABILITY.md` and `PRIVACY.md` version, a new
event type, and a release-note entry:

- File paths
- Command arguments
- Environment variable values
- Usernames
- Hostnames
- Machine serials or hardware IDs
- IP addresses
- File contents
- Source code or text from the user's project
- Anything that could identify a person or organization
- Anything that could identify a specific file or machine

Any new field is a **breaking change** to the privacy contract. It
will be proposed via the standard `enhancement` issue template and
reviewed by the maintainer.

---

*This document is informational. It does not constitute legal advice.
For a binding data-processing agreement, contact the maintainer.*
