#!/data/data/com.termux/files/usr/bin/bash
# Backup do estado do PocketServe (configuração, shares, tokens, atividade,
# sessões revogadas e snapshots). Rode com o servidor parado ou aceite que
# arquivos em escrita no momento podem ficar inconsistentes.
set -eu
DIR="$HOME/.pocketserve"
OUT="/sdcard/Download/PocketServe"
TS="$(date +%Y%m%d-%H%M%S)"
FILE="$OUT/pocketserve-estado-$TS.tar.gz"

echo "== backup do estado =="
echo "origem: $DIR"

if [ ! -d "$DIR" ]; then
  echo "ERRO: $DIR nao existe"
  exit 1
fi

# config.json, shares.json, activity.jsonl, snapshots e packs do backup.
tar -czf "$FILE" -C "$HOME" .pocketserve 2>/dev/null || {
  echo "ERRO ao criar o pacote"
  exit 1
}

chmod 644 "$FILE" 2>/dev/null || true
echo "criado: $FILE"
ls -lh "$FILE"
echo
echo "conteudo:"
tar -tzf "$FILE" | head -20
echo
echo "sha256:"
sha256sum "$FILE"
