#!/bin/bash
# 10_cleanup_test.sh - Limpa os artefatos de teste gerados

echo "[TEST 10] Limpando artefatos de teste..."

cd ..

# Remove binário
if [ -f "crom-mediastream-test" ]; then
    rm crom-mediastream-test
    echo "OK: Binário de teste removido."
fi

# Remove vídeo mock
VIDEO_DIR=$(grep 'video_dir' config.yaml | cut -d '"' -f 2)
if [ -z "$VIDEO_DIR" ]; then VIDEO_DIR="./videos"; fi

if [ -f "${VIDEO_DIR}/test_mock_01.mp4" ]; then
    rm "${VIDEO_DIR}/test_mock_01.mp4"
    echo "OK: Vídeo mock removido."
fi

exit 0
