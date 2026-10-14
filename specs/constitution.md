# Constituição do go-checker

Princípios que toda spec e todo PR devem respeitar. Mudá-los exige uma spec própria.

1. **Testado.** Todo critério de aceite tem teste automatizado (`NNN AC-n` citado no teste); o CI bloqueia merge vermelho e a cobertura mínima é 80%.
2. **Falhar alto.** Erros são reportados com contexto e código de saída diferente de zero; uma chave desconhecida no YAML é erro, nunca um padrão silencioso.
3. **O código de saída é contrato.** 0, 1 e 2 têm o significado do README (D2) e só mudam com versão major.
4. **Medir de verdade.** Todo número impresso (latência, dias do certificado) vem de uma medição testada contra um servidor real (`httptest`).
5. **Sem rede nos testes.** Os testes só falam com servidores locais; nada depende da internet.
6. **Poucas dependências.** Só a biblioteca padrão, cobra e yaml.v3 (D4).
7. **Fala inglês, documenta em dois idiomas.** A CLI é em inglês; o README existe em português e em inglês (D1).
8. **Multiplataforma.** Os testes passam em Linux, macOS e Windows; a release publica binários para as três.
