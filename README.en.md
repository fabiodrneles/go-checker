# go-checker

**Health checks for websites and APIs: in your terminal, in your CI, or as a continuous monitor.**

[![CI](https://github.com/fabiodrneles/go-checker/actions/workflows/ci.yml/badge.svg)](https://github.com/fabiodrneles/go-checker/actions/workflows/ci.yml)
[![Cross-platform](https://github.com/fabiodrneles/go-checker/actions/workflows/cross-platform.yml/badge.svg)](https://github.com/fabiodrneles/go-checker/actions/workflows/cross-platform.yml)
[![Release](https://img.shields.io/github/v/release/fabiodrneles/go-checker)](https://github.com/fabiodrneles/go-checker/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/fabiodrneles/go-checker)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[🇧🇷 Português](README.md) · 🇺🇸 English

![go-checker checking seven services: three up, two with warnings and two down, each failure with its reason](docs/images/check.png)

A single dependency-free binary that answers the question every team asks several times a day: **"is everything working?"**. It goes further than "it returned 200". go-checker checks that the answer is **right** (the JSON, the body, the headers) and **fast**, and that the **certificate** is not about to expire. It also tells you **where** the time went. Its exit code gates your pipeline, and it alerts your team when something is really down.

## Contents

- [Why go-checker](#why-go-checker)
- [Install](#install)
- [One-minute start](#one-minute-start)
- [Use cases](#use-cases)
- [Command reference](#command-reference)
- [Configuration reference](#configuration-reference)
- [Exit codes](#exit-codes)
- [Prometheus metrics](#prometheus-metrics)
- [Tips](#tips)
- [Comparison](#comparison)
- [How it works](#how-it-works)
- [Development](#development)

## Why go-checker

| Without go-checker | With go-checker |
|---|---|
| The deploy passes CI and the API comes up with the database disconnected: `/health` returns 200 with `"db": false`. | The assertion `json: [{path: $.db.ok, equals: true}]` fails the deploy in seconds. |
| "The site is slow" and nobody knows whether it is DNS, the network, TLS or the server. | `--trace` splits DNS, connect, TLS and server response time. |
| The certificate expires on a Sunday and the site goes down. | The check turns **WARN** 14 days ahead (configurable) and **DOWN** once expired. |
| One lost packet pages the on-call engineer at 3 a.m. | Alerts fire only after N rounds in a row (`--confirm`). |
| Writing checks for 40 endpoints. | `go-checker init --from-openapi` generates them from the API spec. |
| Heavy monitoring tools with a database and a UI. | A binary of a few MB and a YAML file versioned next to your code. |

### Highlights

- 🎯 **Assertions:** status (one or several), max latency, body text and regex, JSON values (`$.data.items[0].id`) and headers.
- 🔬 **Request trace:** DNS, TCP, TLS, time to first byte (TTFB) and total.
- 🔐 **TLS certificate:** days until expiry, with a configurable warning.
- 🔁 **Smart retries:** exponential backoff with jitter, for transient errors only (network and 5xx).
- 🚦 **CI exit codes:** 0 all good, 1 something failed, 2 configuration error.
- 📄 **Reports:** colored text, JSON, JUnit (GitHub, GitLab, Jenkins), Markdown, and an automatic GitHub Actions job summary.
- 🧩 **Ready-made GitHub Action**.
- 📈 **Monitor mode:** Prometheus metrics (p50, p95, p99) and alerts to Slack, Discord, Teams, Telegram or any webhook, with flap protection.
- 🪄 **Generation from OpenAPI 3**.
- 🔌 **Beyond HTTP:** TCP ports (databases, caches, SMTP) and DNS resolution.
- 🔑 **Secrets out of the file:** `${VAR}` from the environment, plus basic and bearer auth.
- 🛡️ **Strict configuration:** a mistyped key is an error with its line and the right suggestion.
- 📦 **Distribution:** Linux, macOS and Windows (amd64 and arm64), a Docker image, and `go install`.

## Install

**With Go (1.24 or newer):**

```bash
go install github.com/fabiodrneles/go-checker@latest
```

**Prebuilt binary:** download your platform's archive from [Releases](https://github.com/fabiodrneles/go-checker/releases/latest), verify it against `checksums.txt` and put `go-checker` on your `PATH`. On Linux and macOS:

```bash
VERSION=$(curl -s https://api.github.com/repos/fabiodrneles/go-checker/releases/latest | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)
OS=$(uname -s); ARCH=$(uname -m); [ "$ARCH" = aarch64 ] && ARCH=arm64
curl -fsSLO "https://github.com/fabiodrneles/go-checker/releases/download/${VERSION}/go-checker_${VERSION#v}_${OS}_${ARCH}.tar.gz"
tar -xzf go-checker_*.tar.gz go-checker && sudo mv go-checker /usr/local/bin/
go-checker version
```

**Docker:**

```bash
docker run --rm ghcr.io/fabiodrneles/go-checker check https://example.com
```

**GitHub Actions:** see the use case [Stop a broken deploy in CI](#1-stop-a-broken-deploy-in-ci).

## One-minute start

Check any URL, no file needed:

```bash
go-checker check https://example.com https://api.github.com
```

![Quick check of a site, an API, a TCP port and a DNS name, all up](docs/images/quick.png)

For a fixed set of checks, create the file and run it:

```bash
go-checker init          # writes a commented go-checker.yaml
go-checker check         # reads go-checker.yaml, checks.yaml or config.yaml
```

Try it without installing anything, from this repository:

<!-- doc-commands -->
```bash
go run . version
go run . init -f - | sed -n 1,12p
go run . init --from-openapi examples/openapi.yaml -f - | grep -c "url:"
```

## Use cases

Each case gives the problem, what go-checker does, a command to copy and paste, and real output.

### 1. Stop a broken deploy in CI

**Problem:** the pipeline ships a release and the tests pass, but in production the API returns errors or a dependency is down.
**Solution:** after the deploy, go-checker checks the endpoints. Any failure exits with code 1 and the job turns red. The reason shows up in the log and in the job summary.

```yaml
# .github/workflows/deploy.yml (after the deploy step)
- name: Smoke test
  uses: fabiodrneles/go-checker@v1.0.0
  with:
    file: go-checker.yaml          # or urls: https://app.example.com/health
    args: --retries 2              # tolerate the API still starting
    junit: reports/smoke.xml       # optional: results as tests
```

The same works in any CI (GitLab, Jenkins, Azure DevOps or a plain script), because the contract is the exit code:

```bash
go-checker check -f go-checker.yaml --retries 2 || exit 1
```

![A 503 check: three attempts and exit code 1](docs/images/exit.png)

The Action exposes the outputs `up`, `warn`, `down`, `report` (the JSON) and `exit-code`. With `fail: "false"` it only reports and does not fail the job.

### 2. Know the API is right, not just up

**Problem:** `/health` returns 200 even with the database disconnected, or the load balancer serves an error page with status 200.
**Solution:** assertions on the content. Each failure states what was expected and what was found.

```yaml
checks:
  - name: Checkout API
    url: https://api.example.com/checkout/health
    expect:
      status: 200
      headers: {Content-Type: application/json}
      json:
        - {path: $.status, equals: ok}
        - {path: $.db.ok, equals: true}
        - {path: $.error, exists: false}
```

```bash
go-checker check -f go-checker.yaml
```

In the image at the top, *Checkout API* returns 200 but is **DOWN**, because `$.status` is `"degraded"` and `$.db.ok` is `false`.

### 3. Find out why it is slow

**Problem:** "the site is slow" does not say where.
**Solution:** `--trace` shows every phase of the request and the certificate details. Slow DNS, a slow TLS handshake and a slow server (high TTFB) each call for a different fix.

```bash
go-checker check https://example.com --trace
```

![Trace with DNS, connect, TLS, first byte and total times, plus the certificate details](docs/images/trace.png)

To turn slowness into a signal, use `max_latency`. Going over the limit makes the check **WARN**, not DOWN, because a slow site is still up:

```yaml
expect:
  max_latency: 800ms
```

### 4. Never let a certificate expire again

**Problem:** the TLS certificate expires and the site disappears for every browser.
**Solution:** every HTTPS check reads the expiry. Below `tls_min_days` (default 14) the check turns **WARN**; once expired, **DOWN**. Run it daily in CI or keep it in `watch`, with the `go_checker_tls_days_left` metric.

```yaml
defaults:
  tls_min_days: 21
checks:
  - url: https://www.example.com
  - url: https://api.example.com
```

```bash
go-checker check --fail-on-warn   # also fail when expiry is close
```

Internal service with its own certificate? `insecure: true` skips chain verification, and the expiry is still evaluated.

### 5. Monitor 24/7 with Prometheus, Grafana and alerts

**Problem:** you want to know when something goes down, without running a monitoring platform.
**Solution:** `go-checker watch` runs the checks on an interval, exposes Prometheus metrics and tells your team when a state really changes.

```bash
go-checker watch -f go-checker.yaml --interval 30s --confirm 2 \
  --metrics-addr :9090 --slack "$SLACK_WEBHOOK"
```

![Monitor mode: a one-round blip is ignored and only the confirmed outage becomes an alert](docs/images/watch.png)

- **Flap protection:** with `--confirm 2`, one bad round raises no alert; two in a row do. The same applies to the recovery.
- **Alert channels**, all repeatable:
  - `--slack URL`
  - `--discord URL`
  - `--teams URL`
  - `--telegram BOT_TOKEN:CHAT_ID`
  - `--webhook URL`: JSON with `name`, `url`, `from`, `to`, `reasons` and `at`, for PagerDuty, Opsgenie or your own system.
- **Metrics** on `/metrics`, and the monitor's own health on `/healthz`:

![Prometheus metrics: up, latency quantiles, checks by state](docs/images/metrics.png)

Prometheus configuration:

```yaml
scrape_configs:
  - job_name: go-checker
    static_configs:
      - targets: ["go-checker:9090"]
```

Suggested alerting rules:

```yaml
- alert: EndpointDown
  expr: go_checker_up == 0
  for: 2m
- alert: EndpointSlow
  expr: go_checker_latency_seconds{quantile="0.95"} > 1
  for: 10m
- alert: CertificateExpiring
  expr: go_checker_tls_days_left < 14
```

### 6. Run it as a service on Docker or Kubernetes

**Problem:** the monitor has to run on its own, close to the services.
**Solution:** the official image is distroless, runs as non-root and supports amd64 and arm64.

```bash
docker run -d --name go-checker -p 9090:9090 \
  -v "$PWD/go-checker.yaml:/go-checker.yaml:ro" -e SLACK_WEBHOOK \
  ghcr.io/fabiodrneles/go-checker watch -f /go-checker.yaml --metrics-addr :9090 --slack "$SLACK_WEBHOOK"
```

On Kubernetes, use the same image in a `Deployment`, with the file in a `ConfigMap`, secrets in a `Secret` (read as `${VAR}`) and `/healthz` as the `livenessProbe`.

### 7. Start from your OpenAPI spec

**Problem:** the API has dozens of endpoints and nobody wants to write a check for each one.
**Solution:** `init --from-openapi` reads the spec (YAML or JSON) and generates one check per GET operation that can be called without made-up values, expecting the documented status. Operations that need parameters are listed in the file header, for you to complete by hand.

```bash
go-checker init --from-openapi openapi.yaml --base-url https://staging.example.com
go-checker check
```

![File generated from the spec, with the skipped operations listed in the header](docs/images/init.png)

### 8. Check databases, caches and DNS

**Problem:** not everything is HTTP. The app is up but Postgres refuses connections, or DNS points to the wrong place after a migration.
**Solution:** `tcp://` and `dns://` checks in the same file, with the same report, retries and alerts.

```yaml
checks:
  - name: Postgres
    url: tcp://db.internal:5432
  - name: Redis
    url: tcp://cache.internal:6379
    expect: {max_latency: 50ms}
  - name: Site DNS
    url: dns://www.example.com
    expect:
      body_contains: 93.184.215.14   # the address it should resolve to
```

### 9. Reports for people and for machines

**Problem:** every destination wants its own format: the terminal, the CI test panel, a script, a PR comment.
**Solution:** `--format` picks the output, and `--output` writes it to a file while the terminal keeps the text.

| Format | For |
|---|---|
| `text` (default) | People. Colors only on a terminal; `--no-color` or `NO_COLOR=1` turn them off. |
| `json` | Scripts and dashboards. A stable schema: `summary` and `checks[]`, with state, status, latency, timings and certificate. |
| `junit` | The test panels of GitHub, GitLab and Jenkins. One case per check. `--junit file` writes it alongside another format. |
| `markdown` | PR comments, wikis and the GitHub Actions job summary (automatic when `GITHUB_STEP_SUMMARY` is set). |

```bash
go-checker check --format json --output report.json --junit junit.xml
jq '.checks[] | select(.state != "UP") | {name, reasons}' report.json
```

![JSON report with the summary and, per check, state, latency and per-phase timings](docs/images/json.png)

![JUnit report with one case per check and the failure with its reason](docs/images/junit.png)

Markdown example, as it appears in the job summary:

> ### go-checker: 3 up, 2 warn, 2 down
>
> | | Check | Status | Latency | Notes |
> |---|---|---|---|---|
> | 🟢 UP | Storefront | 200 | 37ms | |
> | 🔴 DOWN | Checkout API | 200 | 63ms | $.status is "degraded", want "ok" |
> | 🟡 WARN | Search | 200 | 1.24s | latency 1.243s above max_latency 800ms |

### 10. Secrets and authentication without leaking tokens

**Problem:** the endpoint needs a token, and the token must not go into the repository.
**Solution:** `${VAR}` reads from the environment (in CI, from secrets). `${VAR:-default}` sets a fallback. A missing variable is a clear error with its line, never an empty header sent silently.

```yaml
checks:
  - name: Admin API
    url: https://${API_HOST:-api.example.com}/admin/health
    auth:
      bearer: ${API_TOKEN}          # or basic: {username: ..., password: ${PASS}}
```

```bash
API_TOKEN=... go-checker check -f secure.yaml
```

![With the variable set the check passes; without it, an error with the line and the missing variable](docs/images/env.png)

### 11. A configuration that lets no mistake through

**Problem:** a mistyped `timout:` and the check silently runs with the default.
**Solution:** every unknown key is an error (exit code 2) with its line and the closest valid key. CI catches the mistake before it becomes a false "all good".

![Typo reported with its line and the suggested key](docs/images/typo.png)

## Command reference

### `go-checker check [URL...]`

Runs the checks once and exits with the [exit code](#exit-codes). It reads the URLs given as arguments or the file given with `-f`. With neither, it looks for `go-checker.yaml`, `checks.yaml` and `config.yaml`, in that order.

| Flag | Default | What it does |
|---|---|---|
| `-f, --file` | lookup | Check file |
| `--timeout` | `10s` | Time limit per attempt (overrides the file) |
| `--retries` | `0` | Retries after a network error or 5xx (overrides the file) |
| `-c, --concurrency` | `10` | Checks running at the same time |
| `--trace` | | Show DNS, connect, TLS, TTFB and the certificate |
| `-o, --format` | `text` | `text`, `json`, `junit` or `markdown` |
| `--output` | | Write the report to a file (the terminal still shows text) |
| `--junit` | | Also write a JUnit report |
| `--fail-on-warn` | | Exit 1 on warnings too |
| `--no-color` | | No colors (also `NO_COLOR=1`) |

### `go-checker watch [URL...]`

Runs the checks every `--interval` until Ctrl+C or SIGTERM. It takes the input and execution flags of `check` (`-f`, `--timeout`, `--retries`, `--concurrency`), plus:

| Flag | Default | What it does |
|---|---|---|
| `--interval` | `30s` | Time between rounds (at least 1s) |
| `--confirm` | `2` | Rounds in a row before a state change counts |
| `--metrics-addr` | | Address for `/metrics` and `/healthz` (e.g. `:9090`) |
| `--window` | `100` | Responses used for the latency quantiles |
| `--slack`, `--discord`, `--teams` | | Channel webhook URL (repeatable) |
| `--telegram` | | `BOT_TOKEN:CHAT_ID` (repeatable) |
| `--webhook` | | Receives each change as JSON (repeatable) |

### `go-checker init`

| Flag | Default | What it does |
|---|---|---|
| `--from-openapi` | | OpenAPI 3 spec (YAML or JSON) to generate the checks from |
| `--base-url` | first `servers[]` | API base URL |
| `-f, --file` | `go-checker.yaml` | File to write (`-` for standard output) |
| `--force` | | Overwrite an existing file |

### `go-checker version`

Prints the version and the build commit (also `--version`).

## Configuration reference

```yaml
defaults:                    # applies to every check
  timeout: 10s               # per attempt: 500ms, 5s, 1m
  retries: 0                 # 0 to 10, only after a network error or 5xx
  tls_min_days: 14           # WARN when the certificate expires sooner
  headers: {X-Team: sre}     # merged with each check's headers

checks:
  - name: Checkout API       # default: the URL
    url: https://api.example.com/checkout/health   # http(s)://, tcp://host:port, dns://name
    method: GET              # GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS
    headers: {Accept: application/json}
    body: '{"ping": true}'
    timeout: 5s
    retries: 2
    tls_min_days: 30
    insecure: false          # true: accept a self-signed certificate (expiry still evaluated)
    auth:                    # basic or bearer
      bearer: ${API_TOKEN}
    expect:
      status: [200, 204]     # without status: any code below 400 passes
      max_latency: 800ms     # exceeded: WARN
      body_contains: '"ok"'
      body_regex: 'version":"v[0-9]+'
      headers: {Content-Type: json}   # the value contains the text
      json:
        - {path: $.status, equals: ok}          # numbers, strings, booleans, lists and objects
        - {path: $.items[0].id, exists: true}
        - {path: $.error, exists: false}
```

- **JSON path:** `$.a.b`, `$.list[0]`, `$["key with spaces"]` and combinations.
- **Variables:** `${VAR}` and `${VAR:-default}` in any value; `$${` writes `${`. Comment lines are not expanded.
- **Legacy format:** `sites: - url:` is still accepted.
- **States:** **UP** (all good), **WARN** (slow, or the certificate is close to expiry: passes, but warns) and **DOWN** (failed).

## Exit codes

| Code | Meaning | Example |
|---|---|---|
| `0` | All checks passed (WARN too, without `--fail-on-warn`) | Deploy approved |
| `1` | At least one check failed | API down, broken assertion, expired certificate |
| `2` | Usage or configuration error | Invalid file, unknown key, wrong flag |

Code 2 separates "the system is down" from "the check is badly written", so the two alerts never get mixed up.

## Prometheus metrics

| Metric | Type | Measures |
|---|---|---|
| `go_checker_up{check}` | gauge | 1 if the last round passed (UP or WARN), 0 if DOWN |
| `go_checker_warn{check}` | gauge | 1 if it passed with a warning |
| `go_checker_latency_seconds{check,quantile}` | summary | p50, p95 and p99 of the last `--window` responses, plus `_sum` and `_count` |
| `go_checker_checks_total{check,state}` | counter | Checks by state |
| `go_checker_tls_days_left{check}` | gauge | Days until the certificate expires |
| `go_checker_last_round_timestamp_seconds` | gauge | Time of the last round (alert if the monitor stops) |

## Tips

- **Deep health:** point at an endpoint that touches the dependencies (database, queue) and assert on its JSON. A `/ping` that only answers "pong" proves nothing.
- **CI right after a deploy:** `--retries 2` and `timeout: 5s` cover the window while the new version is still starting.
- **Monitoring without false alarms:** `--confirm 2` with `--interval 30s` alerts within about a minute and ignores isolated blips.
- **One file per environment:** `go-checker.staging.yaml` and `go-checker.prod.yaml`, or a single file with `https://${HOST}/...`.
- **Daily certificate routine:** a scheduled workflow (`schedule:`) running `go-checker check --fail-on-warn` warns you weeks ahead.
- **Filter the JSON:** `jq '.checks[] | select(.state=="DOWN") | .name' report.json`.
- **Many endpoints:** raise `--concurrency`; results stay in file order.
- **No colors in logs:** output is plain outside a terminal; `NO_COLOR=1` forces it.

## Comparison

| | go-checker | Gatus | Uptime Kuma | Blackbox Exporter | Hurl |
|---|---|---|---|---|---|
| Single binary, YAML config | ✅ | ✅ | ❌ (UI and database) | ✅ | ✅ (own format) |
| CI exit code | ✅ | ❌ | ❌ | ❌ | ✅ |
| JUnit, JSON, Markdown and job summary | ✅ | ❌ | ❌ | ❌ | partial |
| JSON assertions | ✅ | ✅ | partial | ❌ | ✅ |
| Per-phase timings (DNS, TCP, TLS, TTFB) | ✅ | ❌ | ❌ | ✅ | ✅ |
| Certificate expiry | ✅ | ✅ | ✅ | ✅ | ✅ |
| Generation from OpenAPI | ✅ | ❌ | ❌ | ❌ | ❌ |
| Prometheus and flap-protected alerts | ✅ | ✅ | ✅ | via Alertmanager | ❌ |
| Status page | ❌ | ✅ | ✅ | ❌ | ❌ |

go-checker is built for checks as code: the same configuration works in the terminal, in the pipeline and in the monitor. If what you need is a public status page, Gatus and Uptime Kuma are better fits.

## How it works

```text
go-checker.yaml ──► config (validation, ${VAR}, defaults)
                        │
                        ▼
                 runner (concurrency, retries with backoff and jitter)
                        │  for each check
                        ▼
                 probe (HTTP with httptrace · TCP · DNS) ──► timings, certificate
                        │
                        ▼
                 assert (status, body, JSON, headers, latency) ──► UP · WARN · DOWN
                        │
          ┌─────────────┴─────────────┐
          ▼                           ▼
  check: reports and            watch: metrics, flap filter
  exit code                     and alerts
```

Everything uses the Go standard library, plus `cobra` (CLI) and `yaml.v3`. The Prometheus format and the JSON path are implemented in the project.

## Development

```text
make ci        # lint, tests with the race detector, coverage ≥ 80% and build
make docs      # markdownlint
```

The project follows Spec Driven Development with the [sdd-kit](https://github.com/fabiodrneles/sdd-kit). Each feature has a spec in [`specs/`](specs/README.md) with acceptance criteria, and a test cites each criterion. Tests only talk to local servers (`httptest`) and run on Linux, macOS and Windows. Releases come from the Conventional Commits through the [go-release-manager](https://github.com/fabiodrneles/go-release-manager), and GoReleaser publishes the binaries.

Contributions are welcome: see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE) © Fabio Dorneles
