#!/bin/bash
# 02_build_binary.sh - Tenta compilar o binário Go para garantir que não há erros de compilação

echo "[TEST 02] Compilando o projeto Go..."

cd ..
# Tenta fazer o build do main.go ou do diretório raiz
if go build -o crom-mediastream-test ./cmd/crom; then
    echo "OK: Compilação concluída com sucesso."
    exit 0
else
    # Se ./cmd/crom não existir ainda, testa no raiz
    if go build -o crom-mediastream-test .; then
        echo "OK: Compilação na raiz concluída."
        exit 0
    else
        echo "ERRO: Falha na compilação do projeto."
        exit 1
    fi
fi
