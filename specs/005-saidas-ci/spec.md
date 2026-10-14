# 005 — Saídas para CI e GitHub Action

- **Prioridade:** P1
- **Status:** Approved
- **Código afetado:** `internal/report`, `cmd/`, `action.yml`

## Requisitos funcionais

- **FR-1** `--format` MUST aceitar `text` (padrão), `json`, `junit` e `markdown`; `--output` grava em arquivo.
- **FR-2** O JSON MUST ter um esquema estável: `version`, `summary` e `checks[]` com `name`, `url`, `state`, `status`, `latency_ms`, `attempts`, `reasons`, `timings` e `tls_days_left`.
- **FR-3** O JUnit MUST seguir o formato aceito por GitHub, GitLab e Jenkins (um `testcase` por check, `failure` para DOWN).
- **FR-4** Com `GITHUB_STEP_SUMMARY` definido, o `check` MUST acrescentar a tabela Markdown ao resumo do job.
- **FR-5** `action.yml` MUST instalar o binário da release, rodar o `check` e expor `up`, `warn`, `down` e `report` como saídas.

## Critérios de aceite

- **AC-1** Dado `--format json`, quando `check` roda, então a saída é JSON válido com os campos do FR-2.
- **AC-2** Dado `--format junit`, quando há um check DOWN, então o XML tem `failures="1"` e o motivo no `failure`.
- **AC-3** Dado `--format markdown`, quando `check` roda, então sai uma tabela com uma linha por check.
- **AC-4** Dado `GITHUB_STEP_SUMMARY=arquivo`, quando `check` roda, então a tabela é acrescentada ao arquivo.
- **AC-5** Dado `--output arquivo`, quando `check` roda, então o relatório vai para o arquivo e o terminal mostra o texto.
- **AC-6** Dado um formato desconhecido, quando `check` roda, então sai com 2.
