#!/bin/sh
# doc-commands: roda os blocos ```bash e ```sh da documentação marcados com
# <!-- doc-commands --> na linha anterior, para o README não mentir.
#
# Uso: sh scripts/doc-commands.sh [ARQUIVO.md ...]   (padrão: README.md)
# Cada bloco roda na raiz do repositório: `bash -eo pipefail` (```bash) ou
# `sh -e` (```sh). Blocos sem a
# marca (instalação, deploy) não rodam. Saída: uma linha por bloco; em falha,
# o bloco e o fim da saída dele. Códigos: 0 ok (ou nada marcado), 1 falha.
set -eu

[ $# -gt 0 ] || set -- README.md
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
n=0 failed=0
for f in "$@"; do
  [ -f "$f" ] || { echo "doc-commands: $f não existe; ignorado"; continue; }
  # Extrai cada bloco marcado para $tmp/<n>.sh e anota "<n> <arquivo>:<linha>".
  awk -v dir="$tmp" -v file="$f" -v start="$n" '
    on && /^```[[:space:]]*$/ { on = 0; close(out); next }
    on { print > out; next }
    /^```(bash|sh)[[:space:]]*$/ && prev ~ /^<!-- doc-commands -->[[:space:]]*$/ {
      on = 1; start++; out = dir "/" start ".sh"; printf "" > out
      print start " " file ":" NR + 1 " " ($0 ~ /bash/ ? "bash" : "sh") >> (dir "/index")
    }
    { prev = $0 }' "$f"
  [ ! -f "$tmp/index" ] || n="$(tail -n 1 "$tmp/index" | cut -d' ' -f1)"
done
[ "$n" -gt 0 ] || { echo "doc-commands: nenhum bloco marcado com <!-- doc-commands -->"; exit 0; }
while read -r i where shell; do
  if { if [ "$shell" = bash ]; then bash -eo pipefail "$tmp/$i.sh"; else sh -e "$tmp/$i.sh"; fi; } > "$tmp/$i.out" 2>&1; then
    echo "ok $where"
  else
    failed=$((failed + 1))
    echo "FALHOU $where"
    sed 's/^/  | /' "$tmp/$i.sh"
    tail -n 20 "$tmp/$i.out" | sed 's/^/  > /'
  fi
done < "$tmp/index"
[ "$failed" -eq 0 ] || { echo "doc-commands: $failed de $n bloco(s) falharam" >&2; exit 1; }
echo "doc-commands: $n bloco(s) ok"
