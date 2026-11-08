# Limitações

O que o PocketServe **não** faz, o que ele faz com limites e o que você precisa aceitar antes de confiar seus arquivos a ele. Esta lista é deliberadamente conservadora: é melhor descobrir um limite aqui do que no meio de uma recuperação.

> Funcionalidades planejadas ficam em [ROADMAP.md](ROADMAP.md). Nada aqui deve ser lido como promessa de implementação.

## Escopo do produto

| Limitação | Consequência | Alternativa |
|---|---|---|
| **um único usuário administrador** | não há contas separadas, permissões por usuário nem auditoria por pessoa | um servidor por pessoa ou outra ferramenta |
| **sem WebDAV** | não monta como unidade de rede no Windows/Explorer/Finder | Syncthing, rsync, `adb`, ou montar via outro servidor |
| **sem aplicativo móvel nativo** | o acesso é pelo navegador; o `manifest.json` permite adicionar à tela inicial | não há app publicado; não há comportamento offline |
| **sem sincronização embutida** | o servidor não replica arquivos entre aparelhos | Syncthing, rsync ou scripts pela API |
| **sem multiusuário/quotas por usuário** | todos os arquivos pertencem ao mesmo administrador | — |
| **sem compartilhamento com escrita** | links públicos são somente leitura | upload pelo painel autenticado |
| **sem edição colaborativa** | o editor salva o arquivo inteiro; não há bloqueio | combine com Syncthing e aceite conflitos |
| **sem lixeira global única** | cada volume tem sua própria lixeira | veja [ARMAZENAMENTO.md](ARMAZENAMENTO.md) |

## Backup e recuperação

| Limitação | Consequência | Alternativa |
|---|---|---|
| backup **local**, no mesmo aparelho | perda/roubo do celular ou morte do cartão leva os snapshots | cópia externa periódica (3-2-1) |
| retenção fixa de **14 snapshots** | não há retenção por idade, tamanho ou importância | ajuste a frequência ou copie snapshots para fora |
| arquivos acima de **512 MB** ficam fora | vídeos grandes não têm versão nem entram no snapshot | cópia externa manual |
| **sem restauração integral pela interface** | recuperar um snapshot inteiro exige o script auxiliar | `deploy/restore-snapshot.sh` para uma pasta de preparo |
| **sem criptografia** | quem tiver acesso ao armazenamento lê os packs e o estado | criptografe o volume ou a cópia externa |
| **sem verificação automática de todos os packs** | a integridade é conferida no download/restauração | teste de restauração periódico |
| snapshot automático depende do servidor ligado | aparelho desligado não “recupera” o snapshot perdido | rode manualmente ao voltar |
| metadados não são preservados | restauração aplica `0644`; dono/ACL/xattr/timestamps não voltam | — |
| diretórios vazios não entram | estrutura vazia não é recriada | recrie manualmente |
| symlinks não são seguidos | link simbólico não é copiado nem restaurado | copie o destino real |
| **sem backup do estado pela interface** | `config.json`, tokens e shares exigem script/manual | `deploy/backup-estado.sh` |

## Arquivos e armazenamento

| Limitação | Valor | Observação |
|---|---|---|
| raízes extras | até **8** | além da principal |
| upload | configurável; `0` = sem limite | limitado pelo espaço e pela reserva |
| corpo JSON | **1 MiB** | login: 4 KiB; setup/senha: 8 KiB |
| leitura no editor | **2 MiB** | acima disso, `truncated: true` |
| gravação no editor | **5 MiB** | acima disso, `413` |
| thumbnail | arquivo até **64 MiB**, **80 MP**, 4 simultâneas | apenas JPG/JPEG/PNG/GIF |
| busca | **400 resultados** e **6 s** | sem índice; varredura em tempo real |
| descompactar ZIP | **20 GiB** e **20.000 arquivos** | cota por operação |
| log exibido | **160 KiB** finais | não é rotacionado automaticamente |
| atividade | **500 eventos** em memória; arquivo até ~512 KiB | não é um SIEM |
| lixeira | 0 a **3.650 dias**; `0` desliga | padrão 30 |
| reserva de disco | 0 a **1.048.576 MB** por volume | padrão 2.048 MB |

Outras limitações:

- **sem upload retomável**: se a conexão cair, o upload recomeça;
- **sem cota por pasta**: a reserva é por volume;
- **sem indexação**: a busca percorre o disco a cada consulta;
- **sem deduplicação de quase-duplicatas**: só hash exato;
- **sem preview de Office, HEIC ou vídeo**: miniaturas só para imagens comuns; PDF/imagem/áudio/vídeo abrem no visualizador do navegador;
- **`Android/data` e `Android/obb`** são bloqueados pelo Android 11+; nem o Termux consegue ler sem root;
- **nomes com `/` ou `..`** são sanitizados; isso pode mudar o nome enviado;
- **sem controle de versão de arquivos fora dos snapshots**: renomear quebra o histórico por caminho;
- **sem “desfazer” para mover/copiar**: a lixeira cobre exclusão, não a origem de uma cópia.

## Segurança

| Limitação | Risco | Mitigação |
|---|---|---|
| **HTTP puro na LAN** | tráfego legível por quem estiver na mesma rede | use Tailscale ou TLS |
| **2FA opcional** | senha comprometida = acesso total | ative TOTP antes de expor |
| **sem lista de sessões ativas** | não dá para ver/derrubar uma sessão específica | logout revoga a sessão atual; trocar a senha invalida todas |
| **sem bloqueio por conta** | o limite é por IP | use proxy que preserve o IP real e senha forte |
| **sem WAF/fail2ban** | ataques volumétricos podem derrubar o serviço | proteja na borda (Cloudflare/Tailscale) |
| **sem CORS** | a API não é feita para outro domínio | use o mesmo origin ou um proxy |
| **token de share na URL** | aparece em histórico, logs de proxy e prints | trate o link como segredo; revogue quando necessário |
| **senha de share mínima de 4 caracteres** | senha curta é fraca | use senhas longas |
| **sem criptografia em repouso** | acesso físico/ao cartão lê os arquivos | criptografia de disco do Android + cópia externa segura |
| **sem antivírus** | arquivos infectados podem ser armazenados e baixados | não é um scanner; baixe em cliente com proteção |
| **logs locais** | sem envio externo, sem alerta em tempo real | monitore o painel ou use monitoração externa |
| **cookie legado sem ID** | sessões antigas não são revogáveis individualmente | expiram sozinhas; trocar a senha invalida todas |
| **`CF-Connecting-IP` só no loopback** | proxy em outra máquina não tem o IP real reconhecido | coloque o proxy no mesmo aparelho ou aceite o IP do proxy |

O modelo assume **um usuário, uma rede confiável e nenhuma porta aberta no roteador**. Expor à internet sem TLS, 2FA e uma camada de acesso é decisão consciente de risco. Leia [SEGURANCA.md](SEGURANCA.md) e [ACESSO-REMOTO.md](ACESSO-REMOTO.md).

## Desempenho e escala

| Limitação | Efeito |
|---|---|
| CPU de celular com throttling térmico | uploads grandes e hashing sustentado reduzem a frequência |
| FUSE do Android | cada operação de arquivo tem custo extra |
| exFAT sem journal | menos robusto a queda de energia que ext4 |
| cartão microSD | vazão e durabilidade muito inferiores a SSD/NVMe |
| Wi-Fi | latência e estabilidade inferiores a Ethernet |
| sem cache de diretório | listagens grandes leem o disco |
| sem busca indexada | 400 resultados/6 s é o teto, não a meta |
| sem compressão de upload/download | ZIP é gerado/descompactado no servidor |
| sem balanceamento | um processo, um aparelho |

**Não use como NAS de produção**, servidor de banco de dados, hospedagem de sites ou armazenamento de equipe. O PocketServe é para uso pessoal e doméstico.

## Plataforma Android

| Limitação | Consequência |
|---|---|
| Termux pode ser morto pelo Android | supervisor e servidor param juntos; boot só no próximo reinício |
| sem systemd/serviço nativo | o supervisor é um processo shell, não um serviço do sistema |
| Termux:Boot é opcional | sem ele, nada sobe automaticamente após reiniciar |
| wake lock não é garantia | fabricantes agressivos podem encerrar mesmo assim |
| bateria em carga contínua | degradação e risco de inchaço |
| sem resfriamento ativo | carga contínua aquece e reduz frequência |
| sem watchdog de hardware | se o Android travar, não há reinício remoto |
| armazenamento compartilhado pode ser remontado | caminhos `/storage/XXXX-XXXX` mudam ao reformatar |
| permissões mudam entre versões | `Android/data` pode ficar inacessível |
| sem root | não há como contornar bloqueios do sistema |

## Interface e acessibilidade

- interface em **português** apenas;
- sem tema claro (o painel é escuro);
- sem leitor de tela dedicado ou auditoria WCAG completa;
- navegação por teclado existe nos controles nativos, mas não foi auditada em todos os fluxos;
- sem aplicativo nativo; o `manifest.json` permite "adicionar à tela inicial", mas sem service worker não há funcionamento offline nem cache;
- sem notificações push;
- sem edição de planilhas/Office;
- sem visualização de vídeo com transcodificação (depende do codec do navegador).

## API

- **sem versionamento** (`/v1`): a API muda com o servidor;
- **sem paginação** em listagens de arquivos;
- **sem CORS**;
- **sem webhooks**;
- **sem endpoint para restaurar snapshot inteiro**;
- tokens não acessam configuração, sistema, log, TOTP, aparelhos confiáveis ou outros tokens;
- não há idempotência explícita em operações de escrita;
- não há limite de requisições por token (só o limite de login por IP).

## O que não é limitação

Para evitar confusão:

- **não precisa de root** — o servidor roda como app comum;
- **não precisa de internet** — LAN e Tailscale funcionam sem nuvem;
- **não precisa de banco de dados** — o estado é JSON;
- **não precisa de Node.js** — o frontend é embutido;
- **não precisa de compilador C** — o build é Go puro com `CGO_ENABLED=0`;
- **não é somente leitura** — upload, edição, renomear, mover, copiar e excluir funcionam;
- **não é um brinquedo** — há autenticação, CSRF, rate limit, validação de caminho e testes; mas também não é um produto enterprise.

## Antes de confiar seus arquivos

1. Leia [BACKUP-E-RECUPERACAO.md](BACKUP-E-RECUPERACAO.md).
2. Faça um snapshot e **teste a restauração** de um arquivo.
3. Configure uma cópia externa.
4. Ative 2FA se o servidor sair da LAN.
5. Aceite que o aparelho pode morrer e que o backup local não é suficiente.

Se algum desses pontos for inaceitável, o PocketServe não é a ferramenta certa para o seu caso.
