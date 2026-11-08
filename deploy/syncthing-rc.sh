#!/data/data/com.termux/files/usr/bin/bash
# PocketServe - supervisor do Syncthing
# Mantém o Syncthing vivo, sobe no boot e reinicia se travar.
set -u

DIR="${POCKETSERVE_HOME:-$HOME/.pocketserve}"
SYNDIR="${SYNCTHING_HOME:-$HOME/.syncthing-rc}"
BIN="$(command -v syncthing 2>/dev/null || echo "${PREFIX:-/data/data/com.termux/files/usr}/bin/syncthing")"
PIDFILE="$SYNDIR/syncthing.pid"
WDPID="$SYNDIR/supervisor.pid"
LOG="$SYNDIR/syncthing.log"
WDLOG="$SYNDIR/supervisor.log"
STOPFLAG="$SYNDIR/stop"
GUI_PORT="${SYNCTHING_GUI_PORT:-8384}"
SYNC_PORT="${SYNCTHING_PORT:-22000}"

mkdir -p "$SYNDIR"

running() {
  [ -f "$PIDFILE" ] || return 1
  pid="$(cat "$PIDFILE" 2>/dev/null)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

supervisor_running() {
  [ -f "$WDPID" ] || return 1
  pid="$(cat "$WDPID" 2>/dev/null)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

gui_ok() {
  exec 3<>"/dev/tcp/127.0.0.1/$GUI_PORT" 2>/dev/null || return 1
  printf 'GET /rest/noauth/health HTTP/1.0\r\nHost: 127.0.0.1\r\n\r\n' >&3 2>/dev/null || { exec 3<&- 2>/dev/null; return 1; }
  IFS= read -r -t 4 line <&3 2>/dev/null || { exec 3<&- 2>/dev/null; return 1; }
  exec 3<&- 2>/dev/null
  case "$line" in *" 200 "*) return 0 ;; esac
  return 1
}

start() {
  if running; then
    echo "Syncthing já está rodando (PID $(cat "$PIDFILE"))."
    return 0
  fi
  if [ ! -x "$BIN" ]; then
    echo "Syncthing não encontrado. Instale com: pkg install syncthing"
    return 1
  fi
  rm -f "$PIDFILE"
  command -v termux-wake-lock >/dev/null 2>&1 && termux-wake-lock 2>/dev/null
  # --no-restart: o supervisor cuida do reinício, evitando dois laços.
  nohup "$BIN" serve \
    --home "$SYNDIR" \
    --no-browser \
    >>"$LOG" 2>&1 &
  echo $! >"$PIDFILE"
  sleep 2
  if running; then
    echo "Syncthing iniciado (PID $(cat "$PIDFILE"))."
    echo "Painel do Syncthing: http://127.0.0.1:$GUI_PORT (só no aparelho)"
    return 0
  fi
  echo "Não foi possível iniciar. Últimas linhas:"
  tail -n 12 "$LOG" 2>/dev/null
  return 1
}

stop() {
  if ! running; then
    echo "Syncthing não está rodando."
    rm -f "$PIDFILE"
    return 0
  fi
  pid="$(cat "$PIDFILE")"
  kill "$pid" 2>/dev/null
  n=0
  while kill -0 "$pid" 2>/dev/null && [ "$n" -lt 60 ]; do sleep 0.25; n=$((n+1)); done
  kill -9 "$pid" 2>/dev/null
  rm -f "$PIDFILE"
  echo "Syncthing parado."
}

status() {
  if running; then
    echo "Syncthing: rodando (PID $(cat "$PIDFILE"))"
    gui_ok && echo "  interface: respondendo em 127.0.0.1:$GUI_PORT" || echo "  interface: sem resposta"
  else
    echo "Syncthing: parado"
  fi
  if supervisor_running; then
    echo "Supervisor: ativo (PID $(cat "$WDPID"), checagem a cada 20s)"
  else
    echo "Supervisor: inativo"
  fi
}

watchdog_loop() {
  echo "$(date '+%F %T') supervisor do Syncthing iniciado (PID $$)"
  fails=0
  while :; do
    [ -f "$STOPFLAG" ] && break
    if ! running; then
      echo "$(date '+%F %T') processo ausente; reiniciando"
      start >>"$WDLOG" 2>&1
      fails=$((fails+1))
    elif ! gui_ok; then
      echo "$(date '+%F %T') vivo mas sem responder; reiniciando"
      stop >>"$WDLOG" 2>&1
      start >>"$WDLOG" 2>&1
      fails=$((fails+1))
    else
      [ "$fails" -gt 0 ] && echo "$(date '+%F %T') estável novamente"
      fails=0
    fi
    if [ "$fails" -ge 5 ]; then sleep 120
    elif [ "$fails" -ge 2 ]; then sleep 45
    else sleep 20; fi
  done
  echo "$(date '+%F %T') supervisor encerrado"
}

SELF="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

case "${1:-start}" in
  start)
    shift 2>/dev/null || true
    start
    ;;
  stop) stop ;;
  restart) stop; sleep 1; start ;;
  status) status ;;
  log) tail -n 80 "$LOG" 2>/dev/null ;;
  wdlog) tail -n 60 "$WDLOG" 2>/dev/null ;;
  gui) echo "http://127.0.0.1:$GUI_PORT" ;;
  supervisor)
    if supervisor_running; then
      echo "Supervisor já ativo (PID $(cat "$WDPID"))."
      exit 0
    fi
    rm -f "$STOPFLAG" "$WDPID"
    nohup "$SELF" _watchdog >>"$WDLOG" 2>&1 &
    echo $! >"$WDPID"
    sleep 0.5
    if supervisor_running; then
      echo "Supervisor do Syncthing ativado (PID $(cat "$WDPID"))."
    else
      echo "Falha ao ativar o supervisor. Veja $WDLOG"
      exit 1
    fi
    ;;
  unsupervisor)
    touch "$STOPFLAG"
    if supervisor_running; then
      pid="$(cat "$WDPID")"
      kill "$pid" 2>/dev/null
      n=0
      while kill -0 "$pid" 2>/dev/null && [ "$n" -lt 40 ]; do sleep 0.25; n=$((n+1)); done
      kill -9 "$pid" 2>/dev/null
    fi
    rm -f "$WDPID"
    echo "Supervisor do Syncthing desativado."
    ;;
  _watchdog) watchdog_loop ;;
  *)
    echo "uso: syncthing-rc [start|stop|restart|status|log|wdlog|gui|supervisor|unsupervisor]"
    exit 2
    ;;
esac
