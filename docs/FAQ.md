# Perguntas frequentes

Respostas curtas para dúvidas e problemas comuns. Cada resposta aponta para o documento onde o assunto é tratado por inteiro.

## Instalação e primeiro acesso

### Preciso de root?

Não. O PocketServe roda como um aplicativo comum dentro do Termux. Root não é necessário para instalar, servir arquivos, usar HTTPS em portas altas, TOTP, backups ou múltiplas raízes.

Sem root você **não** consegue contornar bloqueios do Android, como `Android/data` e `Android/obb` (restritos desde o Android 11) ou a política de bateria do fabricante. Isso é limitação do sistema, não do servidor.

### Preciso compilar no celular?

Não. O caminho recomendado é compilar no computador com `CGO_ENABLED=0 GOOS=linux GOARCH=arm64` e enviar o binário estático para o Termux. Compilar no aparelho funciona, mas consome mais tempo, bateria e espaço.

### `go: command not found` no computador

Instale o Go 1.24 ou superior. Confira:

```bash
go version
```

Se o comando não existir, siga a instalação oficial em [go.dev/dl](https://go.dev/dl/). O `Makefile` assume `go` no `PATH`; para usar outro binário, rode `make arm64 GO=/caminho/para/go`.

### `ERRO: execute este instalador dentro do Termux`

O `install.sh` confere se `$PREFIX` existe e aponta para dentro de `com.termux`. Esse erro aparece quando ele é executado em outro shell (Linux do PC, adb shell, SSH de outra máquina).

Abra o **Termux no próprio celular** e rode:

```bash
bash /sdcard/Download/PocketServe/install.sh
```

### `ERRO: binario pocketserve-arm64 nao encontrado`

O script procura o binário **na mesma pasta dele**. Copie `install.sh`, `ctl.sh` e `pocketserve-arm64` juntos para `/sdcard/Download/PocketServe/` e rode de novo.

### `ERRO: /sdcard indisponivel. Rode primeiro: termux-setup-storage`

O Termux não tem permissão de armazenamento. Rode:

```bash
termux-setup-storage
```

Aceite o pedido do Android. Depois confirme:

```bash
ls /sdcard
```

### O painel não abre no computador

Verifique, nesta ordem:

1. `pocketserve status` mostra o servidor rodando?
2. `curl -s http://127.0.0.1:8080/healthz` funciona **no celular**?
3. O endereço usado é o da rede Wi-Fi (`pocketserve url`), não `127.0.0.1`?
4. Celular e computador estão no **mesmo** Wi-Fi (não um na rede 2,4 GHz e outro na 5 GHz com isolamento)?
5. O roteador tem "isolamento de clientes" (AP isolation) ligado?

Se o `curl` local funciona e o acesso externo não, o problema é rede/firewall, não o servidor.

### Esqueci a senha. E agora?

Com acesso físico ao Termux:

```bash
pocketserve stop
~/.pocketserve/pocketserve -home ~/.pocketserve -reset-access
pocketserve start
```

Isso remove senha, TOTP e aparelhos confiáveis. **Não** apaga arquivos, compartilhamentos, lixeira nem snapshots. Não existe recuperação remota, por decisão de segurança.

### Perdi o autenticador (2FA)

Use um dos **10 códigos de recuperação** gerados na ativação. Cada um serve uma vez.

Se não tiver os códigos, o caminho é o `-reset-access` acima, no próprio aparelho.

## Uso diário

### A lixeira não desliga quando coloco 0

A partir da versão pública 1.0.0, `trash_days: 0` desliga a lixeira de verdade: a exclusão passa a ser definitiva. Antes, um defeito convertia `0` em `30` a cada carga da configuração.

Se o seu `config.json` foi escrito por uma versão antiga, confira o valor:

```bash
grep trash_days ~/.pocketserve/config.json
```

A interface (**Configurações → Lixeira (dias)**) aceita `0` a `3650`. O padrão, quando a chave não existe, é `30`.

### Por que o arquivo que apaguei não está mais na lixeira?

Três motivos possíveis:

1. a lixeira está desligada (`trash_days = 0`);
2. passaram mais de `trash_days` dias — a limpeza roda a cada 6 horas;
3. o item foi apagado definitivamente pela tela da lixeira ou por "Esvaziar lixeira".

Nesses casos, tente **Versões (backup)**, se houver snapshot daquele caminho.

### Excluí uma pasta inteira. A lixeira guarda tudo?

Sim: mover para a lixeira é um `rename` da árvore inteira para o volume de origem, então a estrutura é preservada. A restauração devolve no caminho original; se o nome já existir, cria `nome (1)`.

### `Pasta de destino inválida` ao enviar para `DCIM` ou `Download`

Esse era um defeito real da linhagem privada 1.5: com mais de uma raiz configurada, nomes de pasta sem ponto eram confundidos com apelidos de volume. Hoje a ordem de resolução é:

1. apelido exato tem prioridade (`cartao/...`);
2. pasta que existe na raiz principal é tratada como pasta normal;
3. só recusa se o nome não existe e é parecido com um apelido (por exemplo `cartoa`).

Se você vir essa mensagem na versão 1.0.0, verifique se o caminho realmente existe e se o volume está montado.

### Por que mover entre interno e cartão demora?

`os.Rename` não funciona entre sistemas de arquivos diferentes: retorna `EXDEV`. O servidor copia os bytes e só remove a origem depois que a cópia termina.

Consequências:

- o progresso não é retomável;
- se o processo morrer no meio, a origem permanece intacta e o destino pode ficar parcial;
- pastas grandes podem demorar e consumir bateria.

### O editor recusa meu arquivo

Limites reais:

| Operação | Limite |
|---|---|
| leitura no editor | 2 MiB (acima disso o conteúdo vem truncado e o botão Editar desaparece) |
| gravação | 5 MiB |
| corpo JSON das APIs | 1 MiB |

Arquivos binários são detectados e não abrem no editor. Para arquivos grandes, edite no PC depois de baixar.

### O upload falha com `Sem espaço` mas ainda há espaço livre

Existe uma **reserva de disco** por volume (padrão 2048 MB) que nenhum upload pode ocupar. Se o volume estiver abaixo da reserva, qualquer gravação é recusada com `507 no_space`, mesmo que sobre espaço.

Opções: liberar espaço ou reduzir a reserva em **Configurações → Reserva de disco (MB)**. `0` desliga a reserva — não recomendado em exFAT.

### `Android/data` e `Android/obb` não abrem

O Android 11+ bloqueia essas pastas para todos os aplicativos, inclusive o Termux, sem root. O painel mostra uma mensagem explícita em vez de "não encontrado".

Use `Android/media` ou outra pasta. Não há contorno sem root.

### A busca não encontra um arquivo

- mínimo de 2 caracteres;
- máximo de **400 resultados**;
- limite de **6 segundos**;
- a partir da raiz, varre todos os volumes; a partir de uma subpasta, só aquela árvore;
- arquivos e pastas ocultos são ignorados;
- `Android` é pulado na raiz.

Para acervos grandes, reduza o escopo: busque dentro da pasta provável.

### O painel mostra CPU como "—"

Esperado no Android 15 e em várias versões recentes. O sistema bloqueia `/proc/stat`, `/proc/loadavg` e `/proc/uptime` para aplicativos. O PocketServe usa a syscall `sysinfo` para uptime e carga, mas o percentual de CPU pode ficar indisponível.

### A bateria não aparece

O Android pode bloquear `/sys/class/power_supply/battery/*`. O servidor tenta dois caminhos:

1. leitura direta do `sysfs`;
2. `termux-battery-status`, se o app **Termux:API** estiver instalado.

Se ambos falharem, o painel mostra `Sem acesso à bateria`. Instale o Termux:API (F-Droid) e conceda a permissão.

## Rede e acesso remoto

### Posso abrir uma porta no roteador?

Tecnicamente sim, mas **não é recomendado**. O modelo assume LAN ou VPN. Se precisar de acesso externo, prefira:

1. **Tailscale** (VPN WireGuard, sem abrir porta);
2. **Cloudflare Tunnel** ou outro túnel de saída;
3. só em último caso, port forwarding — e apenas com HTTPS, senha forte e 2FA.

Leia [ACESSO-REMOTO.md](ACESSO-REMOTO.md) e [SEGURANCA.md](SEGURANCA.md).

### O Chrome mostra erro de SSL

O Chrome força HTTPS e recusa HTTP puro em muitos contextos. Duas saídas:

- usar o endereço HTTP explicitamente na LAN, com `http://`;
- configurar HTTPS em `8443` com certificado local (`-tls-cert`/`-tls-key`).

Certificado autoassinado gera aviso de segurança. Para acesso público, deixe o túnel ou proxy terminar o TLS.

### O endereço curto do Tailscale não resolve

`http://celular:8080` só funciona com **MagicDNS** ativo e a VPN conectada. O campo `tailscale_name` no PocketServe serve apenas para **exibir** o endereço; ele não configura o Tailscale.

Confira no aparelho:

```bash
tailscale status
```

### O IP real do visitante aparece como 127.0.0.1

Isso acontece quando um proxy local (cloudflared, Caddy, nginx) conecta em loopback e não encaminha o IP original. O servidor aceita, nesta ordem, quando a conexão vem do loopback:

1. `CF-Connecting-IP`;
2. `X-Forwarded-For`;
3. `X-Real-IP`.

Fora do loopback esses cabeçalhos são ignorados de propósito, para impedir falsificação. Se o seu proxy não envia nenhum deles, o problema é de configuração do proxy.

Sem IP real, o limite de tentativas de login vira um balde único: um atacante pode bloquear o login de todos.

### Dois celulares na mesma rede, cada um com PocketServe

Funciona, desde que usem portas diferentes. Se ambos tentarem `8080`, o segundo falha com `address already in use`.

Troque a porta em **Configurações → Servidor** ou use `-port 8081`.

## Backup e recuperação

### Onde ficam os snapshots?

Em `<raiz principal>/.pocketserve-backups/arquivos/` — no mesmo volume dos arquivos, não em `~/.pocketserve`. A pasta é oculta na interface e excluída do próprio backup.

```text
<raiz>/.pocketserve-backups/arquivos/
├── snapshots/<id>.json
├── packs/<sha256>.pack
└── index.json
```

### `backup-estado.sh` guarda os snapshots?

Não. Ele empacota `~/.pocketserve` (configuração, shares, tokens, atividade, sessões revogadas e logs). Os snapshots de arquivos ficam na raiz e precisam ser copiados separadamente.

### Arquivo grande não aparece no histórico de versões

Arquivos acima de **512 MB** não entram nos snapshots. Eles aparecem no campo `missing` do manifesto, mas não têm versão.

Para preservar versões de arquivos grandes, a cópia externa é a única opção.

### O relatório de duplicatas mostra dados antigos

O relatório é calculado a partir do **snapshot mais recente**, não do disco ao vivo. Se o último backup é de ontem, o relatório reflete ontem.

Rode um snapshot manual (**Configurações → Backups → Rodar agora**) antes de limpar duplicatas.

### Restaurar um snapshot inteiro

Não há botão para isso. A restauração pela interface é **por arquivo**.

Para o snapshot inteiro, use o script auxiliar:

```bash
bash deploy/restore-snapshot.sh SNAPSHOT.json PASTA_DE_PREPARO
```

Ele extrai tudo para uma pasta separada, verifica o SHA-256 de cada arquivo e **não** toca nas raízes ao vivo. Depois de conferir, mova manualmente.

### O backup roda com o celular desligado?

Não. O snapshot diário é verificado a cada 30 minutos **enquanto o servidor está ligado**; se a última execução tem mais de 23 horas, roda. Aparelho desligado não recupera o snapshot perdido no dia seguinte — ele roda quando o servidor voltar.

## Desempenho e estabilidade

### O Android mata o servidor

Sintomas: o supervisor reinicia várias vezes, ou tudo para quando a tela apaga.

Causas comuns:

- economia de bateria do Termux;
- "phantom process killer" do Android;
- otimizador de memória do fabricante;
- Termux removido dos recentes;
- wake lock desativado.

Mitigações (sem garantia):

```bash
termux-wake-lock
```

E no Android: **Sem restrições** na bateria, autostart permitido, Termux travado nos recentes.

Não existe solução garantida sem root.

### O servidor esquenta

É esperado sob carga contínua: celular sem resfriamento ativo reduz frequência ao atingir limites térmicos (no aparelho de referência, 85 °C para CPU e 50 °C para bateria). Se a temperatura em repouso passar de ~45 °C, há algo errado: capa grossa, sol direto, carga rápida ou processo em loop.

Veja [HARDWARE.md](HARDWARE.md).

### A bateria está sempre em 100%. É problema?

Sim, a longo prazo. Carga contínua acelera o desgaste e, em casos ruins, causa inchaço. Se o aparelho permitir limite de carga (root ou recurso do fabricante), use. Caso contrário, monitore temperatura e troque a bateria preventivamente.

A bateria também funciona como uma pequena UPS: quedas curtas de energia não derrubam o servidor.

### Quantos usuários simultâneos suporta?

Não há número medido. O projeto é para **um usuário** e poucos acessos simultâneos. O gargalo costuma ser o Wi-Fi e o cartão, não a CPU. Não foi feito teste de carga prolongada — veja [LIMITACOES.md](LIMITACOES.md).

### Serve para hospedar um site ou banco de dados?

Não. O PocketServe é um servidor de arquivos pessoal. Não há banco de dados, multiusuário, replicação ou garantia de disponibilidade. Para isso, use um servidor de verdade.

## Segurança

### O tráfego é criptografado?

Só com HTTPS configurado ou dentro de uma VPN (Tailscale/WireGuard). Em HTTP puro na LAN, quem estiver na mesma rede pode ler o tráfego.

### O que acontece se eu perder o celular?

Quem tiver o aparelho desbloqueado tem acesso aos arquivos. Proteções:

- criptografia de disco do Android;
- senha de tela forte;
- 2FA no PocketServe;
- cópia externa dos dados;
- revogar tokens e shares de outro dispositivo (se você tiver o estado).

O backup local não ajuda nesse cenário — ele está no aparelho perdido.

### O link de compartilhamento é seguro?

O token tem 16 bytes aleatórios e é a única credencial. Quem tiver o link acessa. Trate-o como segredo:

- use validade curta;
- use senha quando o link circular por canais inseguros;
- revogue quando não precisar mais.

O token aparece em históricos de navegador e logs de proxy.

### Como revogar todos os acessos?

- **Senha**: trocar a senha invalida todas as sessões (a versão da senha muda).
- **Tokens de API**: revogue individualmente em **Configurações → Tokens de API**.
- **Aparelhos confiáveis**: revogue em **Configurações → Verificação em duas etapas**.
- **Shares**: revogue em **Compartilhamentos**.
- **Sessão atual**: **Sair** revoga no servidor, não só apaga o cookie.

Não existe botão "encerrar todas as sessões" — é uma limitação conhecida.

### O que é o cabeçalho `X-PS`?

É a proteção CSRF. Toda mutação autenticada por cookie exige `X-PS: 1`, e um formulário HTML cross-site não consegue enviar cabeçalho customizado. Requisições com Bearer token não precisam dele.

Se você chama a API com `curl` e recebe `403 csrf`, adicione `-H 'X-PS: 1'`.

## Desenvolvimento

### Como rodar os testes?

```bash
make test     # go test ./...
make race     # go test -race ./...
make cover    # cobertura
make vet      # go vet ./...
make fmt      # confere gofmt
```

A verificação HTTP completa sobe uma instância descartável:

```bash
make verify-http
```

Resultados e o que cada etapa cobre: [VERIFICACAO.md](VERIFICACAO.md).

### Posso usar a API em um script?

Sim, com um token de API:

```bash
curl -H "Authorization: Bearer ps_SEU_TOKEN" \
  "http://127.0.0.1:8080/api/files?path="
```

Os escopos são `files:read`, `files:write`, `files:delete`, `shares:manage` e `admin`. A API não tem CORS nem versionamento. Veja [API.md](API.md).

### Como contribuir?

Leia [CONTRIBUTING.md](../CONTRIBUTING.md). O projeto é pequeno: mudanças devem vir com teste e sem dependências novas desnecessárias.

### Por que o código tem comentários em português?

O projeto nasceu como ferramenta pessoal em português. A interface, a documentação e o código mantêm o idioma. Isso não impede contribuições em outros idiomas — só mantenha o estilo do arquivo que está editando.
