# 008 — Documentação e v1.0.0

- **Prioridade:** P1
- **Status:** In Progress
- **Código afetado:** `README.md`, `README.en.md`, `docs/`, `examples/`

## Requisitos funcionais

- **FR-1 (D1)** `README.md` (português) e `README.en.md` (inglês) MUST cobrir instalação, início rápido, referência da configuração, códigos de saída, uso em CI, monitor e dicas, com imagens da saída real.
- **FR-2** Os exemplos de comando do README MUST rodar no CI (`<!-- doc-commands -->`).
- **FR-3** `examples/` MUST trazer arquivos válidos, verificados por teste.

## Critérios de aceite

- **AC-1** Dado cada arquivo de `examples/`, quando é lido, então é válido.
