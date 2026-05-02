#!/bin/bash
# 03_validate_config.sh - Verifica a integridade do arquivo config.yaml

echo "[TEST 03] Validando config.yaml..."

CONFIG_FILE="../config.yaml"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "ERRO: Arquivo $CONFIG_FILE não encontrado."
    exit 1
fi

if grep -q "stream_url" "$CONFIG_FILE" && grep -q "stream_key" "$CONFIG_FILE"; then
    echo "OK: Arquivo config.yaml contém chaves obrigatórias."
    exit 0
else
    echo "ERRO: Chaves stream_url ou stream_key ausentes no config.yaml."
    exit 1
fi
