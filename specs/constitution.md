# Constituição do go-checker

Princípios que toda spec e todo PR devem respeitar. Mudá-los exige uma spec própria. Escreva de 5 a 10 princípios **verificáveis** depois da análise do repositório (skill `sdd-delivery`, fase de descoberta).

1. **Testado.** Todo critério de aceite tem teste automatizado; o CI bloqueia merge vermelho.
2. **Falhar alto.** Erros são reportados com contexto e código de saída diferente de zero; nada é descartado em silêncio.
