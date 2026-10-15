# 004 — Asserções

- **Prioridade:** P1
- **Status:** Done — entregue na `v1.0.0`
- **Código afetado:** `internal/assert`, `internal/config`, `internal/runner`

## Contexto

Um 200 com a página de erro do balanceador, ou um JSON com `"status": "degraded"`, passa por "no ar". As asserções dizem o que "certo" significa.

## Requisitos funcionais

- **FR-1** O bloco `expect` MAY ter `status` (um código ou lista), `max_latency`, `body_contains`, `body_regex`, `json` (lista de `path` com `equals` ou `exists`) e `headers` (o valor contém o texto).
- **FR-2** Sem `expect.status`, um status < 400 MUST passar (o comportamento de antes).
- **FR-3** Cada asserção que falha MUST virar um motivo com o esperado e o encontrado.
- **FR-4** `max_latency` estourada MUST deixar o check WARN; as demais asserções, DOWN.
- **FR-5** `body_regex` inválida e `path` mal formado MUST ser erro de configuração (saída 2).

## Critérios de aceite

- **AC-1** Dado `status: [200, 204]` e a resposta 201, quando o check roda, então fica DOWN com "status 201, want 200 or 204".
- **AC-2** Dado `body_contains: ok` e o corpo `fail`, quando o check roda, então fica DOWN.
- **AC-3** Dado `body_regex: "v[0-9]+"` e o corpo `v12`, quando o check roda, então passa.
- **AC-4** Dado `json: [{path: $.data.items[1].name, equals: b}]` e um corpo que bate, quando o check roda, então passa; com outro valor, falha citando o valor encontrado.
- **AC-5** Dado `headers: {Content-Type: json}` e `Content-Type: text/html`, quando o check roda, então falha.
- **AC-6** Dado `max_latency: 10ms` e uma resposta em 50 ms, quando o check roda, então fica WARN.
- **AC-7** Dada uma regex ou um path inválidos, quando o arquivo é lido, então sai com 2.
