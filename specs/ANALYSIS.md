# Análise e verificação do go-checker

Feita em 2026-10-14 sobre o commit `64781ae` (a `main` antes da adoção do sdd-kit). **Verificado** = executado; **inferido** = leitura de código.

## 1. Resumo executivo

O go-checker era um protótipo de 190 linhas: verifica URLs em paralelo e imprime UP/DOWN. A ideia é boa, mas o resultado não era confiável: a latência impressa estava errada, o código de saída era 0 mesmo com serviços fora do ar (inútil em CI), não havia testes, README nem CI, e um binário Windows de 9,5 MB estava commitado. A decisão foi reescrever o núcleo e crescer até uma ferramenta de verificação de saúde que sirva tanto no terminal quanto no pipeline.

## 2. O que foi verificado

| Comando | Resultado |
|---|---|
| `go build ./...` | compila |
| `go run . check` com o `config.yaml` do repositório | toda latência impressa como `2562047h47m16.854s` (C1) |
| `go run . check; echo $?` com um site fora do ar | `0` (C2) |
| `go test ./...` | nenhum teste |
| `git ls-files` | `go-checker.exe` (9,5 MB) versionado |

## 3. Observações por severidade

### Críticas

- **C1** A latência é medida a partir de `time.Time{}` (o ano 1), depois da requisição: o número impresso não significa nada.
- **C2** O processo sai com 0 mesmo quando serviços estão fora: um pipeline nunca falha por causa dele.

### Altas

- **A1** Nenhum teste, nenhum CI.
- **A2** Timeout fixo de 10 s, sem retentativa: uma oscilação de rede vira DOWN.
- **A3** O módulo se chama `go-checker`: `go install github.com/fabiodrneles/go-checker@latest` não funciona.

### Médias

- **M1** Binário `go-checker.exe` commitado, sem `.gitignore`.
- **M2** Só verifica o status (< 400); não há como afirmar nada sobre o corpo, a latência ou o certificado.
- **M3** Mensagens em português misturadas com termos em inglês; a ferramenta deve falar inglês (decisão D1).

### Baixas

- **B1** Cores ANSI sempre ligadas, mesmo fora de um terminal.
- **B2** Concorrência sem limite: mil URLs abrem mil conexões de uma vez.

## 4. Pontos positivos (manter)

- Cobra para a CLI e YAML para a configuração.
- Concorrência com goroutines desde o início.
- O formato `sites: - url:` é simples; continua aceito (002 FR-7).

## 5. Avaliação do README

Não havia README.

## 6. Melhorias recomendadas (priorizadas)

1. Núcleo correto e testado: latência, códigos de saída, timeout, retentativas, CI (spec 002).
2. Diagnóstico que poucas ferramentas dão: tempo de DNS, TCP, TLS e primeiro byte, e validade do certificado (spec 003).
3. Asserções sobre status, latência, corpo, JSON e cabeçalhos (spec 004).
4. Saídas para CI (JSON, JUnit, Markdown) e uma GitHub Action (spec 005).
5. Gerar os checks a partir de uma spec OpenAPI (spec 006).
6. Modo monitor com métricas Prometheus e alertas sem oscilação (spec 007).
7. README em PT e EN e release v1.0.0 (spec 008).

## 7. Decisões respondidas pelo dono em 2026-10-14

- **D1** A ferramenta (comandos, mensagens, ajuda) é em inglês; o README tem versão em português e em inglês.
- **D2** Códigos de saída: 0 tudo no ar, 1 algum check falhou, 2 erro de uso ou de configuração.
- **D3** Reescrever o núcleo em pacotes `internal/` em vez de remendar o `checker/`.
- **D4** Sem dependências novas além de cobra e yaml.v3: métricas Prometheus e JSON path implementados no projeto.
