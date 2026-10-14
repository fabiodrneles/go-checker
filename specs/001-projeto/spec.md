# 001 — Projeto, CI e release

- **Prioridade:** P0
- **Status:** In Progress
- **Código afetado:** `main.go`, `.goreleaser.yml`, `.github/workflows/`
- **Resolve:** A1, A3, M1

## Contexto

Sem CI, sem release e com o binário commitado, ninguém consegue instalar nem confiar no projeto.

## Requisitos funcionais

- **FR-1** O módulo MUST ser `github.com/fabiodrneles/go-checker`, para o `go install` funcionar.
- **FR-2** Binários e saídas de build MUST ficar fora do git (`.gitignore`).
- **FR-3** O CI MUST rodar `make ci` (lint, testes com race detector, cobertura ≥ 80%, build) e os testes em Linux, macOS e Windows.
- **FR-4** O workflow *Release tag* MUST calcular a versão pelo go-release-manager e publicar, com o GoReleaser, binários para Linux, macOS (amd64 e arm64) e Windows (amd64), com `checksums.txt`.
- **FR-5** O binário MUST informar a versão e o commit do build.

## Critérios de aceite

- **AC-1** Dado um binário de release, quando `go-checker version` (ou `--version`) roda, então imprime `go-checker vX.Y.Z (commit)`.

## Fora de escopo

- Pacotes de sistema (Homebrew, apt) e imagem Docker.
