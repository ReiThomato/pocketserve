#!/data/data/com.termux/files/usr/bin/bash
# PocketServe - instalador para Termux
set -eu

NAME="${NAME:-PocketServe}"
PORT="${PORT:-8080}"
ROOT="${ROOT:-/sdcard}"
TS_NAME="${TS_NAME:-}"   # opcional: seu nome MagicDNS do Tailscale
DIR="${POCKETSERVE_HOME:-$HOME/.pocketserve}"
SELF="$(cd "$(dirname "$0")" && pwd)"

say() { printf '%s\n' "$*"; }

if [ -z "${PREFIX:-}" ] || [ ! -d "$PREFIX" ]; then
  say "ERRO: execute este instalador dentro do Termux."
  exit 1
fi
if [ ! -f "$SELF/pocketserve-arm64" ]; then
  say "ERRO: binario pocketserve-arm64 nao encontrado em $SELF"
  exit 1
fi
if [ ! -d /sdcard ]; then
  say "ERRO: /sdcard indisponivel. Rode primeiro: termux-setup-storage"
  exit 1
fi

# Descobre o nome MagicDNS automaticamente, se o Tailscale estiver presente.
if [ -z "$TS_NAME" ] && command -v tailscale >/dev/null 2>&1; then
  TS_NAME="$(tailscale status --json 2>/dev/null |
    sed -n 's/.*"DNSName"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  TS_NAME="${TS_NAME%.}"
fi

say "== PocketServe - instalacao =="
say "Destino: $DIR"
mkdir -p "$DIR"

# Para uma instancia antiga, se existir.
if [ -x "$DIR/ctl.sh" ]; then
  "$DIR/ctl.sh" stop >/dev/null 2>&1 || true
elif [ -f "$DIR/server.pid" ]; then
  oldpid="$(cat "$DIR/server.pid" 2>/dev/null || true)"
  if [ -n "$oldpid" ]; then
    kill "$oldpid" 2>/dev/null || true
    sleep 1
  fi
  rm -f "$DIR/server.pid"
fi

# Binario e scripts de controle.
cp -f "$SELF/pocketserve-arm64" "$DIR/pocketserve.new"
chmod 700 "$DIR/pocketserve.new"
mv -f "$DIR/pocketserve.new" "$DIR/pocketserve"
cp -f "$SELF/ctl.sh" "$DIR/ctl.sh"
chmod 700 "$DIR/ctl.sh"

# Configuracao inicial (nunca sobrescreve uma existente).
if [ ! -f "$DIR/config.json" ]; then
  "$DIR/pocketserve" -home "$DIR" -init -root "$ROOT" -port "$PORT" -name "$NAME" -ts-name "$TS_NAME"
else
  say "Configuracao existente preservada (senha e ajustes mantidos)."
fi

# Comando curto: pocketserve start|stop|restart|status|url|log
mkdir -p "$PREFIX/bin"
cat > "$PREFIX/bin/pocketserve" <<EOF
#!$PREFIX/bin/bash
exec "$DIR/ctl.sh" "\$@"
EOF
chmod 700 "$PREFIX/bin/pocketserve"

# Syncthing: supervisor dedicado, se o pacote estiver instalado.
if command -v syncthing >/dev/null 2>&1 && [ -f "$SELF/syncthing-rc.sh" ]; then
  SYNC_HOME="$HOME/.pocketserve-sync"
  mkdir -p "$SYNC_HOME"
  cp -f "$SELF/syncthing-rc.sh" "$SYNC_HOME/syncthing-rc.sh"
  chmod 700 "$SYNC_HOME/syncthing-rc.sh"
  say "Syncthing detectado: supervisor instalado."
elif [ -f "$SELF/syncthing-rc.sh" ]; then
  say "Syncthing nao instalado. Para ativar depois: pkg install syncthing"
  say "e rode o instalador novamente."
fi

# Inicio automatico no boot (Termux:Boot), agora com supervisor.
mkdir -p "$HOME/.termux/boot"
cat > "$HOME/.termux/boot/pocketserve.sh" <<EOF
#!$PREFIX/bin/sh
termux-wake-lock 2>/dev/null || true
sleep 8
"$DIR/ctl.sh" watchdog >> "$DIR/boot.log" 2>&1
if [ -x "$HOME/.pocketserve-sync/syncthing-rc.sh" ]; then
  "$HOME/.pocketserve-sync/syncthing-rc.sh" supervisor >> "$DIR/boot.log" 2>&1
fi
EOF
chmod 700 "$HOME/.termux/boot/pocketserve.sh"

say "Arquivos instalados. Iniciando o servidor com supervisor..."
if ! "$DIR/ctl.sh" watchdog; then
  say ""
  say "A instalacao terminou, mas o servidor nao iniciou."
  say "Veja o log: $DIR/server.log"
  exit 1
fi

say ""
say "Instalacao concluida."
say ""
say "No primeiro acesso, o painel pede para criar a senha do administrador."
say "Comandos uteis: pocketserve status | restart | log | wdlog"
say ""
if [ -x "$HOME/.pocketserve-sync/syncthing-rc.sh" ]; then
  say ""
  say "Syncthing: pocketserve sync status | start | stop | log"
fi
say ""
say "O supervisor reinicia o servidor sozinho se ele cair ou travar,"
say "e volta a subir tudo automaticamente quando o celular reinicia."
say ""
say "Para acesso remoto: abra o app Tailscale no celular e conecte a VPN."
say "Para sobreviver ao gerenciamento de bateria, permita o Termux em"
say "'Autostart' e use 'Sem restricoes' na economia de bateria (consulte docs/OPERACAO.md no repositório)."
