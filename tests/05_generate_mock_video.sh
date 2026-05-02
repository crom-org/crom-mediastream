#!/bin/bash
# 05_generate_mock_video.sh - Gera um vídeo de teste MP4 curto para uso posterior

echo "[TEST 05] Gerando vídeo MP4 de teste mock..."

VIDEO_DIR=$(grep 'video_dir' ../config.yaml | cut -d '"' -f 2)
if [ -z "$VIDEO_DIR" ]; then VIDEO_DIR="./videos"; fi
if [[ "$VIDEO_DIR" == "./"* ]]; then VIDEO_DIR="../${VIDEO_DIR:2}"; fi

mkdir -p "$VIDEO_DIR"
TEST_FILE="$VIDEO_DIR/test_mock_01.mp4"

# Usa ffmpeg para gerar um vídeo de 2 segundos com cor sólida (testsrc) e ruído
if ffmpeg -y -f lavfi -i testsrc=duration=2:size=1280x720:rate=30 -f lavfi -i anoisesrc=d=2 -c:v libx264 -c:a aac "$TEST_FILE" &> /dev/null; then
    echo "OK: Vídeo mock gerado em $TEST_FILE"
    exit 0
else
    echo "ERRO: Falha ao gerar vídeo mock via FFmpeg."
    exit 1
fi
