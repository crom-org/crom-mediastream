#!/bin/bash
# 09_go_vet_test.sh - Executa o go vet para achar potenciais problemas lógicos no código

echo "[TEST 09] Analisando código estático com 'go vet'..."

cd ..
if go vet ./...; then
    echo "OK: 'go vet' não encontrou anomalias lógicas."
    exit 0
else
    echo "ERRO: 'go vet' detectou potenciais problemas."
    exit 1
fi
