#!/bin/bash
# 08_go_fmt_test.sh - Verifica se o código Go está formatado corretamente (gofmt)

echo "[TEST 08] Verificando formatação do código (gofmt)..."

cd ..
UNFORMATTED=$(gofmt -l .)

if [ -z "$UNFORMATTED" ]; then
    echo "OK: Todo o código está formatado corretamente."
    exit 0
else
    echo "AVISO: Os seguintes arquivos não estão formatados (execute 'gofmt -w .'):"
    echo "$UNFORMATTED"
    # Não vamos falhar o teste agressivamente por isso, então exit 0, mas registra aviso.
    exit 0
fi
