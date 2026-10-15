# Changelog

Todas as mudanças relevantes deste projeto. Formato [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/), versões [SemVer](https://semver.org/lang/pt-BR/).

## [Unreleased]

## [1.0.0] - 2026-10-15

### Adicionado

- adopt the sdd-kit and rewrite the core (latency, exit codes, retries) (#12)
- request trace (DNS, TCP, TLS, TTFB) and certificate expiry (#13)
- assertions on status, latency, body, JSON and headers (#14)
- JSON, JUnit and Markdown reports, job summary and a GitHub Action (#16)
- generate checks from an OpenAPI spec with init (#17)
- watch mode with Prometheus metrics and flap-protected alerts (#20)
- env variables, basic and bearer auth, TCP and DNS checks (#21)
- Discord, Teams and Telegram alerts and a Docker image (#22)
