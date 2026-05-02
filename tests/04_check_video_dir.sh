#!/bin/bash
# 04_check_video_dir.sh - Verifica ou cria o diretório de vídeos configurado

echo "[TEST 04] Verificando diretório de vídeos..."

# Pega o diretório do config.yaml
VIDEO_DIR=$(grep 'video_dir' ../config.yaml | cut -d '"' -f 2)

if [ -z "$VIDEO_DIR" ]; then
    VIDEO_DIR="./videos"
fi

# Ajusta path relativo para o teste rodar de tests/ ou root
if [[ "$VIDEO_DIR" == "./"* ]]; then
    VIDEO_DIR="../${VIDEO_DIR:2}"
fi

if [ -d "$VIDEO_DIR" ]; then
    echo "OK: Diretório de vídeos ($VIDEO_DIR) existe."
else
    echo "AVISO: Diretório de vídeos não existe. Criando $VIDEO_DIR..."
    mkdir -p "$VIDEO_DIR"
    echo "OK: Diretório criado."
fi

exit 0
