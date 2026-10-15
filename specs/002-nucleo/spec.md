# 002 — Núcleo: medir, decidir e sair com o código certo

- **Prioridade:** P0
- **Status:** Done — entregue na `v1.0.0`
- **Código afetado:** `cmd/`, `internal/config`, `internal/probe`, `internal/runner`, `internal/report`
- **Resolve:** C1, C2, A2, M3, B1, B2

## Requisitos funcionais

- **FR-1 (C1)** A latência MUST cobrir a troca inteira: do início da conexão até o corpo lido.
- **FR-2 (D2)** O processo MUST sair com 0 se todos os checks passam, 1 se algum falha e 2 em erro de uso ou de configuração.
- **FR-3** Cada check MAY definir `timeout`, `retries`, `method`, `headers` e `body`; `defaults` vale para todos; `--timeout` e `--retries` sobrescrevem o arquivo.
- **FR-4** Erro de rede e status 5xx MUST ser tentados de novo até `retries` vezes, com espera exponencial e jitter; um 4xx MUST NOT ser tentado de novo.
- **FR-5** No máximo `--concurrency` checks MUST rodar ao mesmo tempo; o relatório MUST seguir a ordem do arquivo.
- **FR-6** URLs passadas como argumento MUST virar checks, sem arquivo.
- **FR-7** O formato antigo `sites:` MUST continuar aceito.
- **FR-8 (D1)** Toda mensagem MUST ser em inglês; as cores MUST desligar fora de um terminal, com `--no-color` ou com `NO_COLOR`.

## Critérios de aceite

- **AC-1** Dado um servidor que responde em 60 ms, quando o check roda, então a latência informada fica entre 60 ms e o timeout.
- **AC-2** Dados checks todos no ar, quando `check` roda, então sai com 0.
- **AC-3** Dado um check que responde 503, quando `check` roda, então sai com 1.
- **AC-4** Dado um arquivo inexistente ou inválido, uma flag desconhecida ou valores fora do limite, quando `check` roda, então sai com 2 e explica o erro no stderr.
- **AC-5** Dado um servidor que não responde e `timeout: 50ms`, quando o check roda, então falha por timeout em menos de 1 s.
- **AC-6** Dado um servidor que responde 503 duas vezes e depois 200, com `retries: 2`, quando o check roda, então fica UP em 3 tentativas; com 404, há uma tentativa só.
- **AC-7** Dado um arquivo com `sites: - url:`, quando é lido, então cada URL vira um check.
- **AC-8** Dado `timout:` (erro de digitação) no arquivo, quando é lido, então o erro cita a chave e a linha.
- **AC-9** Dados 8 checks e `--concurrency 3`, quando rodam, então no máximo 3 ficam em andamento ao mesmo tempo e os resultados seguem a ordem do arquivo.
- **AC-10** Dado `go-checker check URL1 URL2`, quando roda, então verifica as duas URLs.
- **AC-11** A espera antes da tentativa n MUST dobrar a cada tentativa, ter teto e variar (jitter).
