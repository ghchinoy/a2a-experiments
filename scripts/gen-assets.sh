#!/usr/bin/env bash
set -euo pipefail

# Output directory
DIR="cmd/multimodal/testdata/assets"
mkdir -p "$DIR"

echo "Generating deterministic test assets in $DIR..."

# Check if ffmpeg is available
if ! command -v ffmpeg &> /dev/null; then
    echo "Error: ffmpeg is required to generate media assets but was not found." >&2
    exit 1
fi

# 1. sample.png (100x100 solid blue image)
ffmpeg -y -f lavfi -i color=c=blue:size=100x100:d=1 -vframes 1 -update 1 "$DIR/sample.png" 2>/dev/null
echo "  [+] Generated sample.png"

# 2. sample.wav (1 second 1kHz PCM mono audio)
ffmpeg -y -f lavfi -i sine=frequency=1000:duration=1 -acodec pcm_s16le "$DIR/sample.wav" 2>/dev/null
echo "  [+] Generated sample.wav"

# 3. sample.mp3 (1 second 1kHz MP3 audio)
ffmpeg -y -f lavfi -i sine=frequency=1000:duration=1 -acodec libmp3lame "$DIR/sample.mp3" 2>/dev/null
echo "  [+] Generated sample.mp3"

# 4. sample.mp4 (1 second solid color video + audio)
ffmpeg -y -f lavfi -i color=c=blue:size=100x100:d=1 -f lavfi -i sine=frequency=1000:duration=1 -shortest -c:v libx264 -pix_fmt yuv420p -c:a aac "$DIR/sample.mp4" 2>/dev/null
echo "  [+] Generated sample.mp4"

# 5. sample.pdf (valid minimal 1-page PDF)
cat << 'EOF' > "$DIR/sample.pdf"
%PDF-1.4
1 0 obj
<<<Type/Catalog/Pages 2 0 R>>>
endobj
2 0 obj
<<<Type/Pages/Kids[3 0 R]/Count 1>>>
endobj
3 0 obj
<<<Type/Page/Parent 2 0 R/Resources<<>>/MediaBox[0 0 595 842]/Contents 4 0 R>>>
endobj
4 0 obj
<<Length 41>>
stream
BT /F1 12 Tf 72 712 Td (A2A Reference PDF) Tj ET
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000056 00000 n 
0000000111 00000 n 
0000000212 00000 n 
trailer
<<Size 5/Root 1 0 R>>
startxref
302
%%EOF
EOF
echo "  [+] Generated sample.pdf"

echo "All assets generated successfully!"
