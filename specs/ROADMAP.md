# Roadmap e tarefas

Cada tarefa referencia a spec e os critérios de aceite que ela fecha. Ordem sugerida = ordem da lista. Épico #3.

A versão de cada fase não é fixada aqui: o go-release-manager a calcula pelos commits no fechamento, e o fechamento grava `→ vX.Y.Z` no cabeçalho da fase.

## Fase 0 — Decisões (antes de codar)

- [x] Responder as decisões D1–D4 de [ANALYSIS.md](ANALYSIS.md) e mover as specs para `Approved`.

## Fase 1 — Funcionar de verdade (P0) → `v1.0.0`

- [x] **T1** Adoção do sdd-kit, módulo, `.gitignore` e release com GoReleaser — 001 FR-1, FR-2, FR-3, FR-4, AC-1 (#4)
- [x] **T2** Latência, códigos de saída, timeout, retentativas, concorrência, CLI em inglês — 002 FR-1..FR-8, AC-1..AC-11 (#5)

## Fase 2 — Confiável (P1) → `v1.0.0`

- [x] **T3** Raio-x da requisição e validade do certificado — 003 AC-1, AC-2, AC-3, AC-4 (#6)
- [x] **T4** Asserções — 004 AC-1, AC-2, AC-3, AC-4, AC-5, AC-6, AC-7 (#7)
- [x] **T5** Saídas JSON, JUnit e Markdown, resumo do job e GitHub Action — 005 AC-1, AC-2, AC-3, AC-4, AC-5, AC-6 (#8)

- [x] **T9** Variáveis de ambiente, autenticação, checks TCP e DNS — 009 AC-1, AC-2, AC-3, AC-4, AC-5 (#18)
- [x] **T10** Discord, Teams e Telegram e imagem Docker — 010 AC-1, AC-2, AC-3, AC-4 (#19)

## Fase 3 — Profissional (P2) → `v1.0.0`

- [x] **T6** `init --from-openapi` — 006 AC-1, AC-2, AC-3, AC-4, AC-5 (#9)
- [x] **T7** `watch`, métricas Prometheus e alertas — 007 AC-1, AC-2, AC-3, AC-4, AC-5, AC-6 (#10)
- [x] **T8** README em PT e EN, exemplos e v1.0.0 — 008 FR-1, FR-2, FR-3, AC-1 (#11)
