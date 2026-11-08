# Instalação

Do zero ao primeiro login, com comandos completos e saídas esperadas.

## 1. Checklist

### Celular

- [ ] Android **7 ou superior** (recomendado 10+). Testado em Android 15 / SDK 35.
- [ ] Arquitetura **arm64-v8a**. Confira em `uname -m` no Termux: deve mostrar `aarch64`.
- [ ] [Termux](https://fido.termux.dev/) instalado pela F-Droid ou pelo GitHub oficial. A versão da Play Store é antiga e não é recomendada.
- [ ] Acesso ao armazenamento (`termux-setup-storage`).
- [ ] Espaço livre suficiente para o binário e os dados (o binário tem cerca de 9 MB).
- [ ] Carregador e uma rede Wi-Fi estável para uso contínuo.

### Computador

- [ ] **Go 1.24+** (`go version`).
- [ ] `adb` (Android Platform Tools), se for copiar por USB.
- [ ] O código-fonte do PocketServe.

O servidor **não precisa de root**, **não precisa de SDK Android** e **não precisa de compilador C**.

---

## 2. Instalar o Termux e liberar o aparelho

### 2.1 Termux

Instale o Termux pela F-Droid ou pelo repositório oficial. Abra e execute:

```bash
pkg update && pkg upgrade -y
termux-setup-storage
```

O Android vai pedir permissão para o Termux acessar fotos, mídia e arquivos. **Aceite.** Sem isso, `/sdcard` não existe e o instalador recusa a execução.

Teste:

```bash
ls /sdcard
echo ok > /sdcard/Download/pocketserve-teste.txt
cat /sdcard/Download/pocketserve-teste.txt
rm /sdcard/Download/pocketserve-teste.txt
```

Se os três comandos funcionarem, o acesso está correto.

### 2.2 Bateria e inicialização

O Android encerra processos em segundo plano para economizar bateria. Para um servidor, isso é inimigo. Ajuste (os nomes variam por fabricante):

| Ajuste | Onde costuma ficar | Por quê |
|---|---|---|
| Sem restrições de bateria | Configurações → Apps → Termux → Bateria | evita congelamento do processo |
| Autostart | Configurações → Apps → Termux → Iniciar automaticamente | permite subir após reiniciar |
| Travar no recentes | botão de apps recentes → cadeado | evita encerramento acidental |
| Termux:Boot instalado | F-Droid | executa o script de boot |
| Wi-Fi sempre ativa | Configurações → Wi-Fi → Avançado | evita queda de rede com a tela apagada |

No Termux, ative o wake lock:

```bash
termux-wake-lock
```

O instalador também chama esse comando ao iniciar e no boot.

---

## 3. Compilar o binário (no computador)

```bash
git clone https://github.com/SEU_USUARIO/pocketserve.git
cd pocketserve

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags "-s -w" -o pocketserve-arm64 .
```

Verifique:

```bash
file pocketserve-arm64
```

Saída esperada (a ordem dos atributos pode variar):

```
pocketserve-arm64: ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), statically linked, stripped
```

Também é possível usar o alvo do Makefile:

```bash
make arm64
```

> **Por que `CGO_ENABLED=0`?** O binário fica estático e não depende de bibliotecas do Termux. Isso simplifica a instalação e evita problemas de linker no Android.

### Compilar dentro do Termux (alternativa)

É possível, mas consome mais bateria, tempo e espaço:

```bash
pkg install golang git -y
git clone https://github.com/SEU_USUARIO/pocketserve.git
cd pocketserve
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o pocketserve-arm64 .
```

O resultado é o mesmo. Para atualizações frequentes, compilar no PC é mais prático.

---

## 4. Copiar para o celular

### Opção A — USB com adb (recomendada)

No celular, ative **Opções do desenvolvedor → Depuração USB**. Conecte o cabo e autorize o computador. No PC:

```bash
adb devices
# List of devices attached
# SEU_DISPOSITIVO	device

adb shell mkdir -p /sdcard/Download/PocketServe
adb push pocketserve-arm64 /sdcard/Download/PocketServe/
adb push deploy/install.sh deploy/ctl.sh deploy/syncthing-rc.sh \
  deploy/rollback-guardar.sh deploy/backup-estado.sh \
  /sdcard/Download/PocketServe/
```

Confira:

```bash
adb shell ls -l /sdcard/Download/PocketServe/
```

Saída esperada (a ordem pode variar):

```
-rw-rw---- ... backup-estado.sh
-rw-rw---- ... ctl.sh
-rw-rw---- ... install.sh
-rwxrwx--- ... pocketserve-arm64
-rw-rw---- ... rollback-guardar.sh
-rw-rw---- ... syncthing-rc.sh
```

### Opção B — sem cabo

Copie `pocketserve-arm64`, `install.sh`, `ctl.sh` e, se for usar Syncthing, `syncthing-rc.sh` para `/sdcard/Download/PocketServe/` usando:

- gerenciador de arquivos do Android;
- `scp` para o SSH do Termux (`pkg install openssh`, `sshd`);
- Syncthing;
- qualquer pasta compartilhada.

O instalador exige `install.sh`, `ctl.sh` e `pocketserve-arm64` **na mesma pasta**. O `syncthing-rc.sh` é copiado se estiver presente; sem ele, o Syncthing não é supervisionado, mas o PocketServe funciona normalmente.

---

## 5. Instalar no Termux

```bash
bash /sdcard/Download/PocketServe/install.sh
```

O script:

1. confere se está no Termux e se `/sdcard` existe;
2. para uma instância antiga, se houver;
3. copia o binário para `~/.pocketserve/pocketserve`;
4. copia o `ctl.sh`;
5. cria a configuração inicial **sem sobrescrever** uma existente;
6. cria o comando `pocketserve` em `$PREFIX/bin`;
7. instala o supervisor do Syncthing, se o pacote estiver presente;
8. cria o script de boot em `~/.termux/boot/pocketserve.sh`;
9. inicia o servidor com supervisor.

Saída esperada (resumida):

```
== PocketServe - instalacao ==
Destino: /data/data/com.termux/files/home/.pocketserve
Configuracao existente preservada (senha e ajustes mantidos).
Arquivos instalados. Iniciando o servidor com supervisor...
PocketServe iniciado (PID 12345).
Painel local:    http://127.0.0.1:8080
Rede Wi-Fi:      http://SEU_ENDERECO_IP:8080

Instalacao concluida.

No primeiro acesso, o painel pede para criar a senha do administrador.
Comandos uteis: pocketserve status | restart | log | wdlog
```

> A linha "Configuracao existente preservada" aparece quando já existe `~/.pocketserve/config.json`. Em uma instalação nova, o script cria o arquivo e mostra "PocketServe configurado".

### Instalação limpa

Se quiser começar do zero (apaga senha, tokens, shares e atividade — **não apaga seus arquivos**):

```bash
pocketserve stop
rm -rf ~/.pocketserve
bash /sdcard/Download/PocketServe/install.sh
```

### Instalação manual (sem o script)

Para entender ou personalizar:

```bash
mkdir -p ~/.pocketserve
cp /sdcard/Download/PocketServe/pocketserve-arm64 ~/.pocketserve/pocketserve
chmod 700 ~/.pocketserve/pocketserve
~/.pocketserve/pocketserve -home ~/.pocketserve -init -root /sdcard -port 8080
~/.pocketserve/pocketserve -home ~/.pocketserve
```

O primeiro comando cria `config.json` e sai. O segundo sobe o servidor em primeiro plano. Para o supervisor, copie `ctl.sh` e use `ctl.sh start`.

---

## 6. Primeiro acesso

No próprio celular:

```bash
pocketserve status
pocketserve url
```

No computador, na mesma Wi-Fi, abra o endereço exibido:

```
http://SEU_ENDERECO_IP:8080
```

Na primeira vez, o painel mostra a tela de configuração:

1. escolha o **nome do servidor** (até 40 caracteres; o padrão é `PocketServe`);
2. crie uma **senha** com pelo menos 8 caracteres e confirme no campo seguinte;
3. clique em **Criar servidor**.

Você entra automaticamente. Depois, ative 2FA em **Configurações → Verificação em duas etapas** (recomendado).

### Testar a saúde do serviço

```bash
curl -s http://127.0.0.1:8080/healthz
```

Saída esperada:

```json
{"ok":true,"service":"pocketserve","version":"1.0.0"}
```

### Descobrir o IP

```bash
ip -4 addr show wlan0 | grep inet
```

Ou use o painel: **Acesso** mostra LAN, Tailscale, endereço curto e QR code.

---

## 7. Estrutura instalada

```
~/.pocketserve/
├── pocketserve              binário
├── ctl.sh                   controle
├── config.json              configuração (0600)
├── shares.json              links públicos
├── tokens.json              tokens de API
├── activity.jsonl           atividades
├── revoked_sessions.json    sessões revogadas
├── server.log               log do servidor
├── watchdog.log             log do supervisor
├── server.pid               PID atual
└── watchdog.pid             PID do supervisor

$PREFIX/bin/pocketserve      atalho para ctl.sh
~/.termux/boot/pocketserve.sh  inicialização no boot
```

O binário **não** fica em `/sdcard`; ele roda do armazenamento privado do Termux. Os arquivos servidos ficam em `/sdcard` (ou na raiz que você configurar).

---

## 8. Atualizar

```bash
# no PC
make arm64
adb push pocketserve-arm64 /sdcard/Download/PocketServe/

# no Termux: guardar o binário atual (opcional)
bash /sdcard/Download/PocketServe/rollback-guardar.sh

# instalar por cima, preservando config.json
bash /sdcard/Download/PocketServe/install.sh

# conferir
pocketserve status
curl -s http://127.0.0.1:8080/healthz
```

Procedimento completo, com backup de estado e rollback: [OPERACAO.md](OPERACAO.md).

---

## 9. Desinstalar

```bash
pocketserve stop
rm -f "$PREFIX/bin/pocketserve"
rm -f "$HOME/.termux/boot/pocketserve.sh"
rm -rf "$HOME/.pocketserve"
```

Isso remove o servidor e o estado. **Seus arquivos em `/sdcard` não são apagados.** Se quiser removê-los também, apague a pasta raiz que você configurou — com cuidado.

---

## 10. Problemas comuns na instalação

| Sintoma | Causa provável | Solução |
|---|---|---|
| `ERRO: execute este instalador dentro do Termux` | rodou em outro shell | abra o Termux e rode de novo |
| `ERRO: binario pocketserve-arm64 nao encontrado` | binário não está na mesma pasta do script | copie `install.sh`, `ctl.sh` e `pocketserve-arm64` juntos |
| `ERRO: /sdcard indisponivel` | permissão de armazenamento negada | rode `termux-setup-storage` e aceite |
| `Permission denied` ao executar | binário sem permissão | `chmod 700 ~/.pocketserve/pocketserve` |
| `address already in use` | outra instância na porta | `pocketserve stop`; se persistir, `pkill -f pocketserve` |
| `raiz "..." indisponível` | cartão/HD desconectado | reconecte; o supervisor tenta de novo |
| painel não abre no PC | firewall, IP errado ou Wi-Fi diferente | confira `pocketserve url`; teste `curl` no celular |
| login não aceita a senha | teclado adicionou espaço/maiúscula | digite devagar; o campo não corrige automaticamente |
| Android bloqueia `/sdcard/Android/data` | scoped storage (Android 11+) | comportamento do sistema; não há contorno sem root |

Mais casos em [FAQ.md](FAQ.md) e [OPERACAO.md](OPERACAO.md).
