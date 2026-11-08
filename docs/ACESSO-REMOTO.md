# Acesso remoto

Como abrir o PocketServe fora da rede Wi-Fi de casa — e por que a ordem abaixo importa.

## Escolha o caminho

| Caminho | Dificuldade | Exposição à internet | Recomendação |
|---|---|---|---|
| Wi-Fi local | nenhuma | nenhuma | uso em casa |
| Tailscale (VPN) | baixa | nenhuma porta aberta | **recomendado** |
| Cloudflare Tunnel | média | túnel de saída, sem porta aberta | bom para links públicos e HTTPS |
| Proxy reverso próprio | média/alta | porta 80/443 no proxy | só se você administra o proxy |
| TLS direto + port forwarding | alta | porta aberta no roteador | evitar |

A regra: **não encaminhe a porta 8080 no roteador**. O PocketServe foi desenhado para uma rede pessoal, não para ser um serviço público.

## 1. Wi-Fi local

O servidor escuta em `0.0.0.0`, então qualquer aparelho na mesma rede alcança:

```text
http://SEU_ENDERECO_IP:8080
```

Descubra o IP no Termux:

```bash
ip -4 addr show wlan0 | awk '/inet /{sub(/\/.*/,"",$2); print $2; exit}'
```

No painel, a tela **Acesso** mostra os endereços detectados e gera QR codes.

Cuidados:

- redes de convidados e Wi-Fi público costumam isolar os clientes; o celular não será alcançado;
- o IP pode mudar quando o roteador reinicia; use reserva DHCP;
- o HTTP na LAN não é criptografado. Em rede compartilhada, prefira Tailscale.

## 2. Tailscale

O Tailscale cria uma rede privada criptografada entre seus aparelhos, sem abrir porta no roteador e sem depender do IP da operadora.

### No celular

1. instale o app **Tailscale** (loja oficial) e entre na mesma conta dos outros aparelhos;
2. conecte a VPN;
3. ative **Always-on VPN** nas configurações do Android, se disponível;
4. permita o app em economia de bateria.

O PocketServe continua escutando em `0.0.0.0`; o tráfego chega pela interface `tailscale0`.

### No computador

1. instale o Tailscale e entre na **mesma conta**;
2. confira `tailscale status`;
3. abra `http://SEU_NOME:8080` (nome curto do MagicDNS) ou `http://NOME.tailXXXX.ts.net:8080`.

O nome curto só funciona com **MagicDNS** ativo no painel do Tailscale. O campo **Nome MagicDNS** do PocketServe serve apenas para exibir o endereço; ele não configura a VPN.

### Por que é o recomendado

- nenhuma porta exposta na internet;
- tráfego criptografado pelo WireGuard;
- funciona atrás de CGNAT e em dados móveis;
- autenticação por dispositivo, além da senha do PocketServe.

O custo é manter o app Tailscale conectado no celular. Se o Android matar a VPN, o acesso remoto para de funcionar até reconectar.

## 3. Cloudflare Tunnel

O `cloudflared` abre uma conexão **de dentro para fora** com a Cloudflare. Nenhuma porta é aberta no roteador. A Cloudflare termina o TLS e entrega a requisição em `127.0.0.1:8080`.

### Pré-requisitos

- um domínio cujo DNS você controla na Cloudflare;
- o binário `cloudflared` para **linux arm64** no Termux;
- o PocketServe rodando e respondendo em `127.0.0.1:8080`.

Confira:

```bash
cloudflared --version
curl -s http://127.0.0.1:8080/healthz
```

### 1. Autenticar

```bash
cloudflared tunnel login
```

O comando mostra uma URL. Abra em qualquer navegador, escolha a zona do domínio e autorize. O arquivo `cert.pem` fica em `~/.cloudflared/`.

Em aparelho sem navegador, copie a URL para o computador; o login é concluído no navegador e o `cert.pem` é gravado no aparelho que rodou o comando.

### 2. Criar o túnel

```bash
cloudflared tunnel create pocketserve
```

Saída esperada (valores fictícios):

```text
Tunnel credentials written to /data/data/com.termux/files/home/.cloudflared/SEU_TUNNEL_ID.json
Created tunnel pocketserve with id SEU_TUNNEL_ID
```

Guarde o ID. O JSON de credenciais é um **segredo**: não versione, não cole em issue, não mostre em print.

### 3. Apontar o DNS

```bash
cloudflared tunnel route dns pocketserve SEU_DOMINIO
```

Isso cria um CNAME `SEU_DOMINIO → SEU_TUNNEL_ID.cfargotunnel.com`. O hostname precisa pertencer a uma zona gerenciada pela Cloudflare.

### 4. Configurar

Copie o modelo e edite:

```bash
mkdir -p ~/.cloudflared
cp /sdcard/Download/PocketServe/cloudflared.example.yml ~/.cloudflared/config.yml
```

O modelo do repositório ([deploy/cloudflared.example.yml](../deploy/cloudflared.example.yml)) usa:

```yaml
tunnel: SEU_TUNNEL_ID
credentials-file: /data/data/com.termux/files/home/.cloudflared/SEU_TUNNEL_ID.json
metrics: 127.0.0.1:20241
no-autoupdate: true
edge-ip-version: "4"
protocol: http2
ingress:
  - hostname: SEU_DOMINIO
    service: http://127.0.0.1:8080
    originRequest:
      connectTimeout: 30s
      keepAliveTimeout: 90s
      httpHostHeader: SEU_DOMINIO
  - service: http_status:404
```

Por que esses valores:

- `127.0.0.1:8080`: o `cloudflared` conecta pelo loopback, então o túnel funciona mesmo com o servidor escutando em `0.0.0.0`;
- `edge-ip-version: "4"` e `protocol: http2`: redes móveis costumam bloquear QUIC/UDP e o resolvedor IPv6 do Android não funciona no Termux;
- `keepAliveTimeout: 90s`: conexões ociosas podem ser derrubadas pelo Android;
- `http_status:404` final: qualquer outro hostname não é encaminhado.

### 5. Testar em primeiro plano

```bash
cloudflared tunnel --config ~/.cloudflared/config.yml run
```

Abra `https://SEU_DOMINIO`. Se funcionar, pare com `Ctrl+C` e suba em segundo plano.

### 6. Rodar em segundo plano

O Termux não tem um gerenciador de serviços padrão. Uma opção simples:

```bash
nohup cloudflared tunnel --config ~/.cloudflared/config.yml run \
  >> ~/.cloudflared/tunnel.log 2>&1 &
echo $! > ~/.cloudflared/tunnel.pid
```

Confira:

```bash
kill -0 "$(cat ~/.cloudflared/tunnel.pid)" && echo rodando
tail -n 30 ~/.cloudflared/tunnel.log
```

Para subir no boot, acrescente a mesma linha ao script `~/.termux/boot/pocketserve.sh`, depois do `sleep 8`:

```sh
nohup cloudflared tunnel --config "$HOME/.cloudflared/config.yml" run \
  >> "$HOME/.cloudflared/tunnel.log" 2>&1 &
echo $! > "$HOME/.cloudflared/tunnel.pid"
```

Isso **não** reinicia o túnel se ele cair. Se precisar dessa garantia, use um supervisor próprio (systemd em um PC, por exemplo) ou aceite reiniciar manualmente. Não há script de supervisão do túnel neste repositório.

### 7. Segurança com Cloudflare

- mantenha o `cloudflared` apontando para `127.0.0.1:8080`, sem depender do IP da LAN;
- ative **2FA** no PocketServe;
- considere **Cloudflare Access** para exigir login antes de chegar ao servidor;
- não use “Trusted IPs” do Cloudflare como única defesa;
- revise **Verificação em duas etapas** (aparelhos confiáveis) e **Tokens de API**;
- o servidor lê `CF-Connecting-IP` quando a conexão vem de loopback, então o limite de tentativas e a atividade usam o IP real do visitante.

### Problemas comuns

| Sintoma | Causa provável | Solução |
|---|---|---|
| erro 502 | servidor parado ou porta errada | `pocketserve status`, `curl /healthz` |
| erro 1033 | túnel não está rodando | iniciar `cloudflared` e ver o log |
| DNS não resolve | rota não criada ou zona errada | `cloudflared tunnel route dns ...` |
| funciona em uma rede e não em outra | QUIC/UDP bloqueado | mantenha `protocol: http2` e `edge-ip-version: "4"` |
| login redireciona para HTTP | proxy não envia `X-Forwarded-Proto: https` | mantenha o proxy em loopback e revise o config |
| rate limit único | proxy sem IP real | confira se `CF-Connecting-IP` chega ao servidor |

## 4. Proxy reverso próprio

Se você já tem Caddy, Nginx ou Traefik, pode terminar o TLS nele. O detalhe importante: o PocketServe só confia em `X-Forwarded-Proto`, `X-Forwarded-For` e `CF-Connecting-IP` quando a conexão vem de **loopback**. Se o proxy estiver em outro computador, o servidor vê o IP do proxy e não reconhece HTTPS nem o IP real.

### Caddy no mesmo aparelho

```caddyfile
SEU_DOMINIO {
    reverse_proxy 127.0.0.1:8080
}
```

### Nginx no mesmo aparelho

```nginx
server {
    listen 443 ssl;
    server_name SEU_DOMINIO;

    ssl_certificate     /caminho/fullchain.pem;
    ssl_certificate_key /caminho/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 3600s;
        client_max_body_size 0;
    }
}
```

`client_max_body_size 0` evita o limite padrão de 1 MB do Nginx. `proxy_read_timeout` longo evita cortar uploads grandes.

Se o proxy estiver em outra máquina, a alternativa é usar TLS direto no PocketServe e apontar o proxy para `https://IP:8443`, ou aceitar que o servidor verá o proxy como cliente.

## 5. TLS direto no PocketServe

Com um certificado e uma chave PEM válidos:

```bash
~/.pocketserve/pocketserve -home ~/.pocketserve \
  -tls-cert /caminho/cert.pem \
  -tls-key /caminho/key.pem
```

O servidor mantém HTTP na porta normal e sobe HTTPS em `8443` (configurável em `tls_port`). O TLS mínimo é 1.2.

O certificado precisa corresponder ao nome usado pelo cliente. Um certificado autoassinado funciona, mas o navegador mostrará aviso; para uso sério, use um certificado de uma CA reconhecida.

Não há renovação automática. Certificados Let's Encrypt vencem em 90 dias; renove e reinicie.

## 6. O que não fazer

- **não** encaminhe `8080` no roteador;
- **não** use HTTP puro em internet aberta;
- **não** confie em “porta secreta” ou nome difícil;
- **não** deixe o painel do Syncthing (`8384`) exposto;
- **não** publique `cert.pem`, o JSON do túnel, o `config.json` nem prints com domínio/IP;
- **não** use a mesma senha do PocketServe em outros serviços.

## Checklist de acesso remoto

- [ ] 2FA ativo;
- [ ] senha longa e exclusiva;
- [ ] Tailscale conectado e “sempre ligado”, ou túnel rodando;
- [ ] proxy/túnel apontando para `127.0.0.1`, sem depender do IP da LAN;
- [ ] `https://` funcionando, quando exposto;
- [ ] limite de tentativas com IP real;
- [ ] QR code de acesso testado;
- [ ] acesso remoto removido quando não for mais necessário.
