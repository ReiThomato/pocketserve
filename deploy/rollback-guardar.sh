#!/data/data/com.termux/files/usr/bin/bash
# Guarda o binário atualmente instalado antes de atualizar.
# Uso: bash rollback-guardar.sh
set -eu

DIR="$HOME/.pocketserve"
DEST="/sdcard/Download/PocketServe"

if [ ! -x "$DIR/pocketserve" ]; then
  echo "ERRO: $DIR/pocketserve nao existe"
  exit 1
fi

VER=$("$DIR/pocketserve" -version 2>/dev/null | awk '{print $NF}')
[ -n "$VER" ] || VER="desconhecida"
OUT="$DEST/pocketserve-arm64-ANTES-$VER"

cp -f "$DIR/pocketserve" "$OUT"
chmod 644 "$OUT" 2>/dev/null || true

echo "binario guardado para rollback:"
echo "  $OUT"
ls -lh "$OUT"
echo
echo "sha256:"
sha256sum "$OUT"
