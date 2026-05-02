#!/bin/bash
# 01_check_deps.sh - Verifica se as dependências do sistema estão instaladas

echo "[TEST 01] Verificando dependências do sistema..."

# Verifica Go
if ! command -v go &> /dev/null; then
    echo "ERRO: Golang não encontrado."
    exit 1
fi
echo "OK: Golang instalado ($(go version))."

# Verifica FFmpeg
if ! command -v ffmpeg &> /dev/null; then
    echo "ERRO: FFmpeg não encontrado."
    exit 1
fi
echo "OK: FFmpeg instalado."

exit 0
