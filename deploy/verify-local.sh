#!/usr/bin/env bash
# Verificação HTTP de ponta a ponta em uma instância descartável.
#
# Sobe um servidor limpo (home e raiz temporárias), espera o /healthz e roda
# deploy/verify-all.sh. Nada toca na instalação real do PocketServe.
#
# Requisitos: Go (ou POCKETSERVE_BIN apontando para um binário já compilado),
# curl e python3. Rode a partir da raiz do repositório:
#
#   bash deploy/verify-local.sh
#   POCKETSERVE_BIN=./pocketserve-arm64 bash deploy/verify-local.sh
set -eu

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${PORT:-19600}"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/pocketserve-verify.XXXXXX")"
HOME_DIR="$WORK/home"
ROOT_DIR="$WORK/root"
BIN="$WORK/pocketserve"
LOG="$WORK/server.log"
PID=""

cleanup() {
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    for _ in $(seq 1 50); do
      kill -0 "$PID" 2>/dev/null || break
      sleep 0.1
    done
    kill -9 "$PID" 2>/dev/null || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

fail() {
  echo "FALHA: $*" >&2
  if [ -f "$LOG" ]; then
    echo "--- últimas linhas de $LOG ---" >&2
    tail -n 30 "$LOG" >&2 || true
  fi
  exit 1
}

for cmd in curl python3; do
  command -v "$cmd" >/dev/null 2>&1 || fail "$cmd não encontrado no PATH"
done

mkdir -p "$HOME_DIR" "$ROOT_DIR/docs"
printf 'conteudo de teste\n' > "$ROOT_DIR/docs/a.txt"

if [ -n "${POCKETSERVE_BIN:-}" ]; then
  [ -x "$POCKETSERVE_BIN" ] || fail "POCKETSERVE_BIN não é executável: $POCKETSERVE_BIN"
  cp -f "$POCKETSERVE_BIN" "$BIN"
else
  command -v "${GO:-go}" >/dev/null 2>&1 || fail "go não encontrado; defina POCKETSERVE_BIN"
  ( cd "$REPO_DIR" && CGO_ENABLED=0 "${GO:-go}" build -trimpath -o "$BIN" . ) || fail "build falhou"
fi
chmod 700 "$BIN"

"$BIN" -home "$HOME_DIR" -init -root "$ROOT_DIR" -port "$PORT" -name "Verify" >/dev/null \
  || fail "não foi possível criar a configuração de teste"

"$BIN" -home "$HOME_DIR" -root "$ROOT_DIR" -port "$PORT" >"$LOG" 2>&1 &
PID=$!

READY=0
for _ in $(seq 1 100); do
  if curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; then
    READY=1
    break
  fi
  kill -0 "$PID" 2>/dev/null || fail "o servidor encerrou durante a subida"
  sleep 0.1
done
[ "$READY" = "1" ] || fail "o servidor não respondeu em /healthz na porta $PORT"

echo "Instância de teste: http://127.0.0.1:$PORT (home: $HOME_DIR)"
BASE="http://127.0.0.1:$PORT" ROOT="$ROOT_DIR" bash "$REPO_DIR/deploy/verify-all.sh"
