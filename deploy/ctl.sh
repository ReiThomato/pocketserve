#!/data/data/com.termux/files/usr/bin/bash
# PocketServe - controle do servidor
set -u

SELF="$(cd "$(dirname "$0")" 2>/dev/null && pwd)/$(basename "$0")"
DIR="${POCKETSERVE_HOME:-$HOME/.pocketserve}"
BIN="$DIR/pocketserve"
PIDFILE="$DIR/server.pid"
WDPID="$DIR/watchdog.pid"
SYNCTL="$HOME/.pocketserve-sync/syncthing-rc.sh"
LOG="$DIR/server.log"
WDLOG="$DIR/watchdog.log"
STOPFLAG="$DIR/watchdog.stop"
LOCKDIR="$DIR/.start.lock"

running() {
  [ -f "$PIDFILE" ] || return 1
  pid="$(cat "$PIDFILE" 2>/dev/null)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

watchdog_running() {
  [ -f "$WDPID" ] || return 1
  pid="$(cat "$WDPID" 2>/dev/null)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

port() {
  sed -n 's/.*"port"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$DIR/config.json" 2>/dev/null | head -n 1
}

tailscale_name() {
  sed -n 's/.*"tailscale_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$DIR/config.json" 2>/dev/null | head -n 1
}

urls() {
  p="$(port)"; [ -n "$p" ] || p=8080
  echo "Painel local:    http://127.0.0.1:$p"
  ip4="$(ip -4 addr show wlan0 2>/dev/null | awk '/inet /{sub(/\/.*/,"",$2); print $2; exit}')"
  [ -n "$ip4" ] && echo "Rede Wi-Fi:      http://$ip4:$p"
  ts="$(tailscale_name)"
  [ -n "$ts" ] && echo "Tailscale:       http://$ts:$p"
  return 0
}

# Testa a porta sem depender de curl nem wget.
health_ok() {
  p="$(port)"; [ -n "$p" ] || p=8080
  exec 3<>"/dev/tcp/127.0.0.1/$p" 2>/dev/null || return 1
  printf 'GET /healthz HTTP/1.0\r\nHost: 127.0.0.1\r\n\r\n' >&3 2>/dev/null || { exec 3<&- 2>/dev/null; return 1; }
  IFS= read -r -t 3 line <&3 2>/dev/null || { exec 3<&- 2>/dev/null; return 1; }
  exec 3<&- 2>/dev/null
  case "$line" in
    *" 200 "*) return 0 ;;
  esac
  return 1
}

# Mata processos do pocketserve que ficaram sem PID file (orfãos de um deploy
# anterior ou de um kill externo). Sem isso, o orfao segura a porta e todo
# start() novo falha com "address already in use".
reap_orphans() {
  for d in /proc/[0-9]*; do
    pid="${d#/proc/}"
    # Compara o executavel real, nao a linha de comando: evita matar um shell
    # cujo texto do comando contenha o caminho por acaso.
    exe="$(readlink "$d/exe" 2>/dev/null)" || continue
    case "$exe" in
      "$BIN"|"$BIN (deleted)") ;;
      *) continue ;;
    esac
    [ "$pid" = "$$" ] && continue
    [ "$pid" = "$PPID" ] && continue
    if [ -f "$PIDFILE" ] && [ "$pid" = "$(cat "$PIDFILE" 2>/dev/null)" ]; then
      continue
    fi
    kill "$pid" 2>/dev/null
  done
}

# O supervisor e o instalador podem chamar start() ao mesmo tempo. Sem um
# lock, os dois sobem o binario e um perde o bind da porta.
acquire_lock() {
  n=0
  while ! mkdir "$LOCKDIR" 2>/dev/null; do
    n=$((n + 1))
    if [ "$n" -ge 100 ]; then
      # Lock preso (processo morto no meio): remove e tenta de novo.
      rmdir "$LOCKDIR" 2>/dev/null
      n=0
    fi
    sleep 0.1
  done
}

release_lock() {
  rmdir "$LOCKDIR" 2>/dev/null
}

start() {
  acquire_lock
  if ! running; then
    reap_orphans
    sleep 0.3
  fi
  if running; then
    echo "PocketServe ja esta rodando (PID $(cat "$PIDFILE"))."
    urls
    release_lock
    return 0
  fi
  rm -f "$PIDFILE"
  command -v termux-wake-lock >/dev/null 2>&1 && termux-wake-lock 2>/dev/null
  ( cd "$HOME" 2>/dev/null || cd /; nohup "$BIN" -home "$DIR" >>"$LOG" 2>&1 & echo $! >"$PIDFILE" )
  sleep 1
  if running; then
    echo "PocketServe iniciado (PID $(cat "$PIDFILE"))."
    urls
    release_lock
  else
    echo "Nao foi possivel iniciar. Ultimas linhas do log:"
    tail -n 15 "$LOG" 2>/dev/null
    release_lock
    return 1
  fi
}

stop() {
  if ! running; then
    echo "PocketServe nao esta rodando."
    rm -f "$PIDFILE"
    return 0
  fi
  pid="$(cat "$PIDFILE")"
  kill "$pid" 2>/dev/null
  n=0
  while kill -0 "$pid" 2>/dev/null && [ "$n" -lt 50 ]; do
    sleep 0.1
    n=$((n + 1))
  done
  if kill -0 "$pid" 2>/dev/null; then
    kill -9 "$pid" 2>/dev/null
  fi
  rm -f "$PIDFILE"
  command -v termux-wake-unlock >/dev/null 2>&1 && termux-wake-unlock 2>/dev/null
  echo "PocketServe parado."
}

status() {
  if running; then
    echo "PocketServe: rodando (PID $(cat "$PIDFILE"))"
    urls
  else
    echo "PocketServe: parado"
  fi
  if watchdog_running; then
    echo "Supervisor:  ativo (PID $(cat "$WDPID"), checagem a cada 15s)"
  else
    echo "Supervisor:  inativo"
  fi
}

# ---------- supervisor ----------

watchdog_loop() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') supervisor iniciado (PID $$)"
  fails=0
  while :; do
    [ -f "$STOPFLAG" ] && break
    if ! running; then
      echo "$(date '+%Y-%m-%d %H:%M:%S') servidor ausente; reiniciando"
      start >>"$WDLOG" 2>&1
      fails=$((fails + 1))
    elif ! health_ok; then
      echo "$(date '+%Y-%m-%d %H:%M:%S') servidor vivo mas sem responder em /healthz; reiniciando"
      stop >>"$WDLOG" 2>&1
      start >>"$WDLOG" 2>&1
      fails=$((fails + 1))
    else
      if [ "$fails" -gt 0 ]; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') servidor estavel novamente"
      fi
      fails=0
    fi
    # Recuos progressivos evitam um laco apertado se o servidor nao subir.
    if [ "$fails" -ge 5 ]; then
      sleep 60
    elif [ "$fails" -ge 2 ]; then
      sleep 30
    else
      sleep 15
    fi
  done
  echo "$(date '+%Y-%m-%d %H:%M:%S') supervisor encerrado"
}

start_watchdog() {
  if watchdog_running; then
    echo "Supervisor ja esta ativo (PID $(cat "$WDPID"))."
    return 0
  fi
  rm -f "$STOPFLAG" "$WDPID"
  nohup "$SELF" _watchdog >>"$WDLOG" 2>&1 &
  echo $! >"$WDPID"
  sleep 0.3
  if watchdog_running; then
    echo "Supervisor ativado (PID $(cat "$WDPID"))."
  else
    echo "Nao foi possivel ativar o supervisor. Veja $WDLOG"
    return 1
  fi
}

stop_watchdog() {
  if ! watchdog_running; then
    rm -f "$WDPID"
    echo "Supervisor ja esta inativo."
    return 0
  fi
  touch "$STOPFLAG"
  pid="$(cat "$WDPID")"
  kill "$pid" 2>/dev/null
  n=0
  while kill -0 "$pid" 2>/dev/null && [ "$n" -lt 30 ]; do
    sleep 0.1
    n=$((n + 1))
  done
  kill -9 "$pid" 2>/dev/null
  rm -f "$WDPID"
  echo "Supervisor desativado."
}

sync_ctl() {
  # Delega o controle do Syncthing ao script dedicado, se ele existir.
  if [ -x "$SYNCTL" ]; then
    "$SYNCTL" "$@"
  else
    echo "Syncthing não está instalado (execute o instalador novamente)."
    return 1
  fi
}

case "${1:-start}" in
  start) start; sync_ctl supervisor >/dev/null 2>&1 || true ;;
  sync) shift; sync_ctl "$@" ;;
  stop) stop_watchdog; stop ;;
  restart) stop_watchdog; stop; sleep 1; start; start_watchdog ;;
  status) status ;;
  url | urls) urls ;;
  log) tail -n 100 "$LOG" 2>/dev/null ;;
  watchdog) start_watchdog ;;
  unwatchdog) stop_watchdog ;;
  wdlog) tail -n 60 "$WDLOG" 2>/dev/null ;;
  _watchdog) watchdog_loop ;;
  *)
    echo "uso: pocketserve [start|stop|restart|status|url|log|watchdog|unwatchdog|wdlog|sync ...]"
    exit 2
    ;;
esac
