# Spec: Adicionar opção --timeout ao comando check

## Functional Requirements

### FR-1: O comando check deve aceitar uma opção --timeout
O comando `check` deve aceitar um parâmetro `--timeout` que especifica o tempo máximo de execução em segundos.

### FR-2: Valor padrão do timeout
Se --timeout não for especificado, deve usar um valor padrão razoável (ex: 120 segundos).

### FR-3: Validação do valor
O valor do timeout deve ser um número inteiro positivo.

## Acceptance Criteria

### AC-1: Comando check com --timeout funciona
`go-checker check --timeout 30` deve executar com timeout de 30 segundos.

### AC-2: Comando check sem --timeout usa valor padrão
`go-checker check` deve executar com timeout padrão (120s).

### AC-3: Valor inválido retorna erro
`go-checker check --timeout -5` deve retornar erro de validação.

## Decisões

- **D-1** (decisão do usuário) como seguir no ticket «Testes para a cobertura chegar a 80%» → o valor do timeout não precisa ser um número inteiro
