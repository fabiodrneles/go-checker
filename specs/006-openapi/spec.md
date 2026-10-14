# 006 — Gerar checks a partir de OpenAPI

- **Prioridade:** P2
- **Status:** Approved
- **Código afetado:** `internal/openapi`, `cmd/`

## Requisitos funcionais

- **FR-1** `go-checker init --from-openapi ARQUIVO` MUST ler OpenAPI 3 em YAML ou JSON e gerar um arquivo de checks.
- **FR-2** Cada operação GET sem parâmetro de caminho obrigatório MUST virar um check, com o nome do `operationId` (ou método e caminho) e `expect.status` do primeiro 2xx documentado.
- **FR-3** A URL base MUST vir de `--base-url` ou, sem ela, do primeiro `servers[].url`.
- **FR-4** Sem `--from-openapi`, `init` MUST gerar um arquivo de exemplo comentado.
- **FR-5** `init` MUST NOT sobrescrever um arquivo existente sem `--force`.

## Critérios de aceite

- **AC-1** Dada uma spec com `GET /health`, `GET /users/{id}` e `POST /users`, quando `init --from-openapi` roda, então só `/health` vira check.
- **AC-2** Dado `responses: {"204": ...}`, quando o check é gerado, então `expect.status` é 204.
- **AC-3** Dado `--base-url http://localhost:8080`, quando o arquivo é gerado, então as URLs usam essa base.
- **AC-4** Dado um arquivo gerado, quando `check -f` o lê, então ele é válido.
- **AC-5** Dado um arquivo que já existe, quando `init` roda sem `--force`, então sai com 2 e não o altera.
