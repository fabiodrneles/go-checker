# 009 — Acima da média: segredos, autenticação, TCP e DNS

- **Prioridade:** P1
- **Status:** Done — entregue na `v1.0.0`
- **Código afetado:** `internal/config`, `internal/probe`
- **Origem:** pesquisa de mercado de 2026-10-15 (Gatus, Uptime Kuma, Blackbox Exporter, Hurl, StepCI, Checkly, k6)

## Contexto

As ferramentas da categoria leem segredos do ambiente, têm atalhos de autenticação e verificam mais que HTTP. Sem isso, o token vai para o YAML e um banco de dados ou um DNS ficam de fora do monitoramento.

## Requisitos funcionais

- **FR-1** `${VAR}` e `${VAR:-padrão}` MUST ser substituídos pelo ambiente antes da leitura; `$${` escreve `${`; linhas de comentário ficam como estão; só a forma com chaves é expandida (o `$` de regex e de JSON path não muda).
- **FR-2** Uma variável sem valor e sem padrão MUST ser erro de configuração, citando a linha.
- **FR-3** `auth: {basic: {username, password}}` e `auth: {bearer: TOKEN}` MUST gerar o cabeçalho `Authorization`; os dois juntos é erro.
- **FR-4** `tcp://host:porta` MUST verificar que a porta aceita conexão, medindo o tempo de conexão.
- **FR-5** `dns://nome` MUST verificar que o nome resolve; os endereços, ordenados, são o corpo, para `body_contains` e `body_regex`.
- **FR-6** Checks TCP e DNS MUST aceitar só as asserções que fazem sentido (`max_latency`; no DNS, também as de corpo); o resto é erro de configuração.

## Critérios de aceite

- **AC-1** Dado `url: https://${HOST}/health` e `HOST=api.example.com`, quando o arquivo é lido, então a URL é `https://api.example.com/health`, e `^ok$` numa regex não muda.
- **AC-2** Dado `${API_TOKEN}` sem a variável definida, quando o arquivo é lido, então sai com 2 e o erro cita a linha e a variável.
- **AC-3** Dado `auth: {basic: {username: ana, password: pw}}`, quando o check roda, então envia `Authorization: Basic YW5hOnB3`.
- **AC-4** Dada uma porta aberta, quando o check `tcp://` roda, então fica UP; com a porta fechada, DOWN.
- **AC-5** Dado um nome que resolve, quando o check `dns://` roda, então fica UP e o corpo tem os endereços; um nome inexistente fica DOWN.
