#!/usr/bin/env bash
# Restaura um snapshot do PocketServe para uma pasta de preparo.
#
# Uso:
#   bash deploy/restore-snapshot.sh SNAPSHOT.json PASTA_DE_DESTINO
#
# Não toca nos arquivos atuais: extrai tudo para PASTA_DE_DESTINO, preservando
# os caminhos exibidos no manifesto (inclusive apelidos de volume, como
# "cartao/..."). Depois de conferir, mova manualmente para o lugar definitivo.
#
# O manifesto e os packs vêm de:
#   <raiz principal>/.pocketserve-backups/arquivos/
#     snapshots/<id>.json
#     packs/<sha256>.pack
set -eu

if [ "$#" -ne 2 ]; then
  echo "uso: $0 SNAPSHOT.json PASTA_DE_DESTINO" >&2
  exit 2
fi

SNAP="$1"
DEST="$2"

[ -f "$SNAP" ] || { echo "ERRO: snapshot não encontrado: $SNAP" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "ERRO: python3 não encontrado" >&2; exit 1; }

PACKS="$(cd "$(dirname "$SNAP")/../packs" 2>/dev/null && pwd)" \
  || { echo "ERRO: pasta packs não encontrada ao lado do snapshot" >&2; exit 1; }
mkdir -p "$DEST"

python3 - "$SNAP" "$PACKS" "$DEST" <<'PY'
import hashlib
import json
import os
import posixpath
import sys
import tarfile

snap_path, packs, dest = sys.argv[1:4]
with open(snap_path, "r", encoding="utf-8") as f:
    snap = json.load(f)

files = snap.get("files") or []
if not files:
    print("ERRO: o snapshot não contém arquivos", file=sys.stderr)
    raise SystemExit(1)

restored = 0
failed = 0

for item in files:
    rel = str(item.get("path", "")).replace("\\", "/")
    norm = posixpath.normpath(rel)
    if not rel or norm in (".", "..") or norm.startswith("../") or norm.startswith("/"):
        print(f"IGNORADO (caminho inseguro): {rel!r}", file=sys.stderr)
        failed += 1
        continue

    sha = str(item.get("sha", ""))
    pack = os.path.join(packs, sha + ".pack")
    if not sha or not os.path.isfile(pack):
        print(f"FALTA pack: {rel} ({sha or 'sem hash'})", file=sys.stderr)
        failed += 1
        continue

    out = os.path.join(dest, *[p for p in norm.split("/") if p])
    os.makedirs(os.path.dirname(out), exist_ok=True)
    tmp = out + ".restore-tmp"

    try:
        h = hashlib.sha256()
        with tarfile.open(pack, "r:gz") as tf:
            member = tf.next()
            if member is None:
                raise ValueError("pack vazio")
            src = tf.extractfile(member)
            if src is None:
                raise ValueError("entrada não é arquivo")
            with open(tmp, "wb") as w:
                while True:
                    chunk = src.read(1024 * 1024)
                    if not chunk:
                        break
                    h.update(chunk)
                    w.write(chunk)
        if h.hexdigest() != sha:
            raise ValueError("hash não confere")
        os.replace(tmp, out)
        restored += 1
    except Exception as exc:
        print(f"FALHA: {rel}: {exc}", file=sys.stderr)
        failed += 1
        try:
            os.remove(tmp)
        except FileNotFoundError:
            pass

print(f"snapshot {snap.get('id', '?')}: {restored} arquivo(s) restaurado(s), {failed} falha(s)")
print(f"destino: {dest}")
raise SystemExit(1 if failed else 0)
PY
