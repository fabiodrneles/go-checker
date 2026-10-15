# 003 — Raio-x da requisição e validade do certificado

- **Prioridade:** P1
- **Status:** Done — entregue na `v1.0.0`
- **Código afetado:** `internal/probe`, `internal/runner`, `internal/report`

## Contexto

"Está lento" não diz onde. Separar DNS, conexão, TLS e servidor aponta o culpado; e certificado vencido derruba serviços que estavam "no ar" ontem.

## Requisitos funcionais

- **FR-1** Cada resposta MUST trazer os tempos de DNS, conexão TCP, handshake TLS, primeiro byte (TTFB) e total, medidos com `net/http/httptrace`.
- **FR-2** Em HTTPS, o resultado MUST trazer os dias até o certificado do servidor expirar.
- **FR-3** Abaixo de `tls_min_days` (padrão 14), o check MUST ficar WARN; com o certificado expirado, DOWN.
- **FR-4** `check --trace` MUST imprimir o raio-x de cada check.
- **FR-5** `insecure: true` MAY pular a verificação do certificado (serviços internos com certificado próprio); a validade MUST continuar sendo informada e avaliada.
- **FR-6** Cada requisição MUST abrir uma conexão nova, para que DNS, TCP e TLS entrem na medição como para um visitante novo.

## Critérios de aceite

- **AC-1** Dado um servidor HTTPS local, quando o check roda, então os tempos de conexão, TLS, TTFB e total são positivos e o total é o maior.
- **AC-2** Dado um certificado que vence em 5 dias e `tls_min_days: 14`, quando o check roda, então fica WARN e o motivo cita os dias.
- **AC-3** Dado um certificado expirado, quando o check roda, então fica DOWN.
- **AC-4** Dado `--trace`, quando `check` roda, então a saída mostra `dns`, `connect`, `tls`, `ttfb` e `total`.
