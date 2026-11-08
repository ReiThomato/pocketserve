#!/usr/bin/env bash
# Verificação funcional completa do PocketServe.
# Exercita cada endpoint e reporta OK/FALHA por função.
set -u

BASE="${BASE:-http://127.0.0.1:19600}"
CK=$(mktemp)
PASS=0; FAIL=0; SKIP=0
declare -a FAILED

chk() { # chk "nome" "esperado" "obtido"
  local name="$1" want="$2" got="$3"
  if [ "$got" = "$want" ]; then
    PASS=$((PASS+1)); printf "  \033[32mok\033[0m   %-42s %s\n" "$name" "$got"
  else
    FAIL=$((FAIL+1)); FAILED+=("$name (esperado $want, veio $got)")
    printf "  \033[31mFALHA\033[0m %-41s esperado %s, veio %s\n" "$name" "$want" "$got"
  fi
}

code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
body() { curl -s "$@"; }

echo "=================================================================="
echo " VERIFICACAO FUNCIONAL — PocketServe"
echo " alvo: $BASE"
echo "=================================================================="

# ---------------- 1. config / estado ----------------
echo
echo "[1] config.go / json.go / sistema basico"
chk "GET /healthz (publico)"            200 "$(code "$BASE/healthz")"
chk "GET /api/state (publico)"          200 "$(code "$BASE/api/state")"
V=$(body "$BASE/healthz" | python3 -c 'import sys,json;print(json.load(sys.stdin)["version"])' 2>/dev/null)
chk "versao reportada"                  "1.0.0" "$V"
chk "GET / (painel)"                    200 "$(code "$BASE/")"
chk "GET /styles.css"                   200 "$(code "$BASE/styles.css")"
chk "GET /app.js"                       200 "$(code "$BASE/app.js")"
chk "GET /files.js"                     200 "$(code "$BASE/files.js")"
chk "GET /manifest.json"                200 "$(code "$BASE/manifest.json")"

# ---------------- 2. auth ----------------
echo
echo "[2] auth.go / trust.go / totp.go"
# Aceita 200 (instancia limpa) ou 403 already_setup (ja configurada).
SETUP=$(code -c "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"password":"senha12345","server_name":"Audit"}' "$BASE/api/setup")
if [ "$SETUP" = "200" ] || [ "$SETUP" = "403" ]; then
  PASS=$((PASS+1)); printf "  \033[32mok\033[0m   %-42s %s\n" "POST /api/setup" "$SETUP"
else
  FAIL=$((FAIL+1)); FAILED+=("POST /api/setup (veio $SETUP)")
  printf "  \033[31mFALHA\033[0m %-41s %s\n" "POST /api/setup" "$SETUP"
fi
chk "POST /api/login (senha errada)"    401 "$(code -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"password":"errada"}' "$BASE/api/login")"
chk "POST /api/login (senha certa)"     200 "$(code -c "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"password":"senha12345"}' "$BASE/api/login")"
chk "login sem header X-PS bloqueado"   403 "$(code -H 'Content-Type: application/json' \
  -d '{"password":"senha12345"}' "$BASE/api/login")"
chk "GET /api/whoami (com sessao)"      200 "$(code -b "$CK" "$BASE/api/whoami")"
chk "GET /api/whoami (sem credencial)"  401 "$(code "$BASE/api/whoami")"
chk "rota protegida sem cookie"         401 "$(code "$BASE/api/files")"
chk "GET /api/totp"                     200 "$(code -b "$CK" "$BASE/api/totp")"
chk "GET /api/trusted"                  200 "$(code -b "$CK" "$BASE/api/trusted")"

# ---------------- 3. raizes / config ----------------
echo
echo "[3] config.go / roots.go / settings.go"
chk "GET /api/settings"                 200 "$(code -b "$CK" "$BASE/api/settings")"
chk "POST /api/settings"                200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"server_name":"Audit","port":19600,"root":"'"$ROOT"'","session_days":30,"max_upload_mb":0,"min_free_mb":0,"trash_days":30,"backup_daily":false,"roots":[]}' \
  "$BASE/api/settings")"
chk "settings exige sessao"             401 "$(code "$BASE/api/settings")"

# ---------------- 4. listagem ----------------
echo
echo "[4] files.go — listagem"
chk "listar raiz"                       200 "$(code -b "$CK" "$BASE/api/files?path=")"
chk "listar subpasta"                   200 "$(code -b "$CK" "$BASE/api/files?path=docs")"
chk "listar pasta inexistente"          404 "$(code -b "$CK" "$BASE/api/files?path=naoexiste")"
chk "listar arquivo como pasta"         400 "$(code -b "$CK" "$BASE/api/files?path=docs/a.txt")"

# ---------------- 5. upload ----------------
echo
echo "[5] files.go — upload"
echo "conteudo de teste" > /tmp/_audit.txt
chk "upload na raiz"                    200 "$(code -b "$CK" -H 'X-PS: 1' -F "file=@/tmp/_audit.txt" "$BASE/api/upload?path=")"
chk "upload em subpasta"                200 "$(code -b "$CK" -H 'X-PS: 1' -F "file=@/tmp/_audit.txt" "$BASE/api/upload?path=docs")"
chk "upload com overwrite"              200 "$(code -b "$CK" -H 'X-PS: 1' -F "file=@/tmp/_audit.txt" "$BASE/api/upload?path=docs&overwrite=1")"
chk "upload cria pasta nova"            200 "$(code -b "$CK" -H 'X-PS: 1' -F "file=@/tmp/_audit.txt" "$BASE/api/upload?path=nova/pasta")"

# ---------------- 6. leitura / download ----------------
echo
echo "[6] files.go — leitura"
chk "GET /api/download"                 200 "$(code -b "$CK" "$BASE/api/download?path=docs/_audit.txt")"
chk "GET /api/raw"                      200 "$(code -b "$CK" "$BASE/api/raw?path=docs/_audit.txt")"
chk "GET /api/text"                     200 "$(code -b "$CK" "$BASE/api/text?path=docs/_audit.txt")"
chk "GET /api/checksum"                 200 "$(code -b "$CK" "$BASE/api/checksum?path=docs/_audit.txt")"
chk "GET /api/thumb (arquivo nao-imagem)" 404 "$(code -b "$CK" "$BASE/api/thumb?path=docs/_audit.txt")"

# ---------------- 7. busca ----------------
echo
echo "[7] files.go — busca"
chk "busca com 1 caractere recusada"    400 "$(code -b "$CK" "$BASE/api/search?q=a")"
chk "busca com 2+ caracteres"           200 "$(code -b "$CK" "$BASE/api/search?q=au")"
chk "busca em subpasta"                 200 "$(code -b "$CK" "$BASE/api/search?q=au&path=docs")"

# ---------------- 8. operacoes ----------------
echo
echo "[8] files.go — operacoes"
chk "mkdir"                             200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"path":"","name":"criada"}' "$BASE/api/mkdir")"
chk "mkdir duplicado"                   409 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"path":"","name":"criada"}' "$BASE/api/mkdir")"
chk "rename"                            200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"path":"criada","name":"renomeada"}' "$BASE/api/rename")"
chk "copy"                              200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"paths":["docs/_audit.txt"],"dest":"renomeada"}' "$BASE/api/copy")"
chk "move"                              200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"paths":["renomeada/_audit.txt"],"dest":"docs"}' "$BASE/api/move")"
chk "save (editor)"                     200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"path":"docs/editado.txt","content":"texto"}' "$BASE/api/save")"
chk "delete"                            200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"paths":["docs/editado.txt"]}' "$BASE/api/delete")"

# ---------------- 9. zip ----------------
echo
echo "[9] files.go — zip"
chk "GET /api/zip (pasta)"              200 "$(code -b "$CK" "$BASE/api/zip?path=docs")"
chk "GET /api/zip (multiplos)"          200 "$(code -b "$CK" "$BASE/api/zip?path=docs&path=nova")"
chk "zip sem path"                      400 "$(code -b "$CK" "$BASE/api/zip")"

# ---------------- 10. lixeira ----------------
echo
echo "[10] trash.go"
chk "GET /api/trash"                    200 "$(code -b "$CK" "$BASE/api/trash")"
TID=$(body -b "$CK" "$BASE/api/trash" | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d["items"][0]["id"] if d["items"] else "")' 2>/dev/null)
if [ -n "$TID" ]; then
  chk "POST /api/trash/restore"         200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' -d "{\"id\":\"$TID\"}" "$BASE/api/trash/restore")"
else
  SKIP=$((SKIP+1)); echo "  skip  trash/restore (lixeira vazia)"
fi
chk "POST /api/trash/empty"             200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' -d '{}' "$BASE/api/trash/empty")"

# ---------------- 11. backup ----------------
echo
echo "[11] backup.go"
chk "POST /api/backups/run"             200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' -d '{}' "$BASE/api/backups/run")"
chk "GET /api/backups"                  200 "$(code -b "$CK" "$BASE/api/backups")"
SNAP=$(body -b "$CK" "$BASE/api/backups" | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d["snapshots"][0]["id"] if d["snapshots"] else "")' 2>/dev/null)
if [ -n "$SNAP" ]; then
  chk "GET /api/backups/file"           200 "$(code -b "$CK" "$BASE/api/backups/file?snapshot=$SNAP&path=docs/a.txt")"
else
  SKIP=$((SKIP+1)); echo "  skip  backups/file (sem snapshot)"
fi

# ---------------- 12. versoes ----------------
echo
echo "[12] versions.go"
chk "GET /api/versions"                 200 "$(code -b "$CK" "$BASE/api/versions?path=docs/a.txt")"
chk "GET /api/versions (inexistente)"   200 "$(code -b "$CK" "$BASE/api/versions?path=nada.txt")"
chk "GET /api/versions/download"        404 "$(code -b "$CK" "$BASE/api/versions/download?snapshot=x&path=y")"
chk "POST /api/versions/restore (snap invalido)" 404 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"snapshot":"x","path":"y"}' "$BASE/api/versions/restore")"

# ---------------- 13. duplicatas ----------------
echo
echo "[13] duplicates.go"
chk "GET /api/duplicates"               200 "$(code -b "$CK" "$BASE/api/duplicates")"
chk "POST /api/duplicates/cleanup vazio" 400 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' -d '{"paths":[]}' "$BASE/api/duplicates/cleanup")"

# ---------------- 14. shares ----------------
echo
echo "[14] shares.go"
chk "POST /api/shares"                  200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"path":"docs","expires_hours":24}' "$BASE/api/shares")"
chk "GET /api/shares"                   200 "$(code -b "$CK" "$BASE/api/shares")"
STOK=$(body -b "$CK" "$BASE/api/shares" | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d["shares"][0]["token"] if d["shares"] else "")' 2>/dev/null)
if [ -n "$STOK" ]; then
  chk "GET /s/{token} (pagina publica)" 200 "$(code "$BASE/s/$STOK")"
  chk "GET /s/{token}/zip"              200 "$(code "$BASE/s/$STOK/zip")"
  chk "DELETE /api/shares/{token}"      200 "$(code -b "$CK" -X DELETE -H 'X-PS: 1' "$BASE/api/shares/$STOK")"
else
  SKIP=$((SKIP+2)); echo "  skip  share publico (nao criado)"
fi
chk "share de caminho invalido"         404 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"path":"naoexiste"}' "$BASE/api/shares")"

# ---------------- 15. tokens ----------------
echo
echo "[15] tokens.go"
chk "POST /api/tokens"                  200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"name":"audit","scopes":["files:read"],"expires_days":1}' "$BASE/api/tokens")"
chk "GET /api/tokens"                   200 "$(code -b "$CK" "$BASE/api/tokens")"
TK=$(body -b "$CK" "$BASE/api/tokens" | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d["tokens"][0]["id"] if d["tokens"] else "")' 2>/dev/null)
chk "token sem escopo recusado"         400 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"name":"x","scopes":[]}' "$BASE/api/tokens")"
chk "token com escopo invalido"         400 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"name":"x","scopes":["root"]}' "$BASE/api/tokens")"
if [ -n "$TK" ]; then
  chk "POST /api/tokens/{id}/revoke"    200 "$(code -b "$CK" -H 'X-PS: 1' -H 'Content-Type: application/json' -d '{}' "$BASE/api/tokens/$TK/revoke")"
  chk "DELETE /api/tokens/{id}"         200 "$(code -b "$CK" -X DELETE -H 'X-PS: 1' "$BASE/api/tokens/$TK")"
fi

# ---------------- 16. sistema / atividade ----------------
echo
echo "[16] system.go / activity.go"
chk "GET /api/system"                   200 "$(code -b "$CK" "$BASE/api/system")"
chk "GET /api/activity"                 200 "$(code -b "$CK" "$BASE/api/activity")"
chk "GET /api/log"                      200 "$(code -b "$CK" "$BASE/api/log")"
chk "GET /api/qr"                       200 "$(code -b "$CK" "$BASE/api/qr?text=teste")"

# ---------------- 17. seguranca ----------------
echo
echo "[17] seguranca"
chk "traversal ../etc/passwd"           404 "$(code -b "$CK" -G "$BASE/api/files" --data-urlencode 'path=../etc/passwd')"
chk "traversal aninhado"                404 "$(code -b "$CK" -G "$BASE/api/files" --data-urlencode 'path=docs/../../etc/passwd')"
chk "mutacao sem CSRF"                  403 "$(code -b "$CK" -H 'Content-Type: application/json' -d '{"path":"","name":"x"}' "$BASE/api/mkdir")"
chk "upload com nome traversal"         200 "$(code -b "$CK" -H 'X-PS: 1' -F "file=@/tmp/_audit.txt;filename=../../pwn.txt" "$BASE/api/upload?path=docs")"

# ---------------- 18. logout ----------------
echo
echo "[18] auth.go — logout"
chk "POST /api/logout"                  200 "$(code -b "$CK" -H 'X-PS: 1' -X POST "$BASE/api/logout")"
chk "sessao invalida apos logout"       401 "$(code -b "$CK" "$BASE/api/files")"

echo
echo "=================================================================="
printf " resultado: \033[32m%d ok\033[0m" "$PASS"
[ "$FAIL" -gt 0 ] && printf ", \033[31m%d falha(s)\033[0m" "$FAIL"
[ "$SKIP" -gt 0 ] && printf ", %d skip" "$SKIP"
echo
if [ "$FAIL" -gt 0 ]; then
  echo
  echo " FALHAS:"
  for f in "${FAILED[@]}"; do echo "   - $f"; done
fi
echo "=================================================================="
rm -f "$CK" /tmp/_audit.txt
[ "$FAIL" -gt 0 ] && exit 1
exit 0
