# Changelog

Todas as mudanças relevantes deste projeto. Formato [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/), versões [SemVer](https://semver.org/lang/pt-BR/).

## [Unreleased]

## [1.0.1] - 2026-10-15

### Corrigido

- `go install github.com/fabiodrneles/go-checker@vX.Y.Z` mostra a versão instalada, e não `dev (none)` (#26)

## [1.0.0] - 2026-10-15

Primeira versão estável: o protótipo virou uma ferramenta de verificação de saúde para o terminal, o CI e o monitoramento contínuo.

### Adicionado

- Núcleo reescrito: latência medida de verdade, códigos de saída 0/1/2, timeout e retentativas por check (backoff exponencial com jitter, só para erros de rede e 5xx), concorrência limitada, URLs como argumento e CLI em inglês (#12)
- Raio-x da requisição (DNS, TCP, TLS, TTFB e total) com `--trace`, e validade do certificado TLS com WARN abaixo de `tls_min_days` e DOWN quando expira (#13)
- Asserções em `expect`: status (um ou lista), `max_latency`, `body_contains`, `body_regex`, JSON path com `equals` e `exists`, e cabeçalhos (#14)
- Relatórios `--format json|junit|markdown`, `--output`, `--junit`, resumo automático no GitHub Actions e a GitHub Action `fabiodrneles/go-checker` (#16)
- `go-checker init`, com arquivo inicial comentado ou gerado a partir de uma spec OpenAPI 3 (`--from-openapi`) (#17)
- `go-checker watch`: métricas do Prometheus (up, latência p50/p95/p99, checks por estado, dias do certificado), `/healthz` e alertas para webhook e Slack com proteção contra oscilação (`--confirm`) (#20)
- `${VAR}` e `${VAR:-padrão}` no arquivo, autenticação basic e bearer, e checks `tcp://` e `dns://` (#21)
- Alertas no Discord, no Teams e no Telegram, e imagem Docker multi-arquitetura em `ghcr.io/fabiodrneles/go-checker` (#22)
- README completo em português e inglês, com 11 casos de uso, prints e referências (#23)

### Alterado

- O módulo agora é `github.com/fabiodrneles/go-checker` (o `go install` funciona) (#12)
- **Incompatível:** o processo sai com 1 quando um check falha (antes saía sempre com 0) (#12)
- Chave desconhecida no arquivo é erro com a linha e a sugestão da chave certa (#12, #23)

### Removido

- O binário `go-checker.exe` versionado no repositório (#12)
