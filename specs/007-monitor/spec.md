# 007 — Modo monitor, métricas e alertas

- **Prioridade:** P2
- **Status:** In Progress
- **Código afetado:** `internal/watch`, `internal/metrics`, `internal/notify`, `cmd/`

## Requisitos funcionais

- **FR-1** `go-checker watch` MUST rodar os checks a cada `--interval` até receber SIGINT/SIGTERM.
- **FR-2** Com `--metrics-addr`, MUST servir `/metrics` no formato de exposição do Prometheus: `go_checker_up`, `go_checker_latency_seconds` (p50, p95 e p99 numa janela das últimas `--window` medições), `go_checker_checks_total` por estado e `go_checker_tls_days_left`; e `/healthz`.
- **FR-3** Uma mudança de estado MUST ser confirmada por `--confirm N` rodadas seguidas antes de valer (proteção contra oscilação).
- **FR-4** Cada mudança confirmada MUST ser enviada para `--webhook URL` (JSON) e `--slack URL` (mensagem do Slack).
- **FR-6** A primeira rodada MUST definir o estado de partida de cada check, sem alerta; uma rodada interrompida pelo encerramento MUST NOT gerar alertas.
- **FR-5** Falha ao enviar um alerta MUST ser registrada no stderr sem parar o monitor.

## Critérios de aceite

- **AC-1** Dadas as latências 1..100 ms na janela, quando `/metrics` é lido, então p50 = 50 ms, p95 = 95 ms e p99 = 99 ms.
- **AC-2** Dado `--confirm 2`, quando o check falha uma vez e volta, então nenhum alerta sai; quando falha duas vezes seguidas, sai um alerta.
- **AC-3** Dado `--webhook`, quando o estado muda, então o webhook recebe JSON com `name`, `from`, `to` e `reasons`.
- **AC-4** Dado `--slack`, quando o estado muda, então o Slack recebe `{"text": ...}`.
- **AC-5** Dado o contexto cancelado, quando `watch` roda, então termina sem erro.
- **AC-6** Dado `/metrics`, quando é lido, então as linhas seguem o formato `# HELP`, `# TYPE` e `nome{rótulos} valor`.
