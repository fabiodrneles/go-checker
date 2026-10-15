# 010 — Acima da média: mais canais de alerta e imagem Docker

- **Prioridade:** P1
- **Status:** In Progress
- **Código afetado:** `internal/notify`, `cmd/watch.go`, `.goreleaser.yml`, `Dockerfile`
- **Origem:** pesquisa de mercado de 2026-10-15

## Requisitos funcionais

- **FR-1** `watch` MUST aceitar `--discord URL`, `--teams URL` e `--telegram TOKEN:CHAT_ID` (repetíveis), com o mesmo filtro contra oscilação.
- **FR-2** A release MUST publicar uma imagem multi-arquitetura (amd64 e arm64) em `ghcr.io/fabiodrneles/go-checker`, com `latest` e a versão.

## Critérios de aceite

- **AC-1** Dado `--discord`, quando o estado muda, então o Discord recebe `{"content": ...}`.
- **AC-2** Dado `--teams`, quando o estado muda, então o Teams recebe uma mensagem com o título do check e o estado.
- **AC-3** Dado `--telegram TOKEN:CHAT`, quando o estado muda, então a API do Telegram recebe `sendMessage` com o `chat_id`.
- **AC-4** Dado `--telegram` sem `:`, quando `watch` roda, então sai com 2.
