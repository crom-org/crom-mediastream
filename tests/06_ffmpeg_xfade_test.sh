#!/bin/bash
# 06_ffmpeg_xfade_test.sh - Testa se a versão instalada do ffmpeg suporta o filtro xfade

echo "[TEST 06] Verificando suporte ao filtro xfade no FFmpeg..."

if ffmpeg -filters 2>/dev/null | grep -q 'xfade'; then
    echo "OK: Filtro xfade suportado pelo FFmpeg local."
    exit 0
else
    echo "ERRO: O filtro xfade (necessário para transições) não é suportado por esta versão do FFmpeg."
    exit 1
fi
