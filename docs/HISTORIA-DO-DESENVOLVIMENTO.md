# História do desenvolvimento

Este documento registra problemas reais encontrados durante o desenvolvimento do PocketServe, a causa identificada, a solução implementada, o resultado dos testes e o que continua limitado. Ele não é uma lista de funcionalidades: é o histórico de erros e decisões que explicam por que o código tem a forma atual.

> **Escopo:** o projeto nasceu como servidor pessoal em um celular Android e foi desenvolvido em ciclos curtos entre 06 e 08/11/2026. A versão pública **1.0.0** reorganiza a linhagem privada **1.10.x** com numeração própria. Nomes de domínio, IPs, identificadores de túnel e dados do aparelho foram removidos.

## Linha do tempo

| Marco | O que entrou | Problema que motivou |
|---|---|---|
| 1.0 | servidor Go, painel embutido, arquivos, shares, sistema | substituir acesso manual por ADB/MTP |
| 1.1 | lixeira, snapshots, SHA-256, TOTP, raiz ausente | exclusão acidental, edição errada, exposição do `/` |
| 1.2 | aparelhos confiáveis, recuperação, HTTPS, Syncthing | atrito do 2FA, perda de acesso, TLS, sincronização |
| 1.3 | múltiplas raízes, apelidos, cartão SD | interno e cartão ao mesmo tempo |
| 1.4 | memória decimal, bateria, endereço curto | números que não fechavam, bateria invisível |
| 1.4.1 | soma de todos os volumes | dashboard ignorava o cartão |
| 1.5 | correção do apelido, `EXDEV`, lixeira por volume | uploads no interno recusados; mover entre volumes falhava |
| 1.6 | tokens de API com escopo | automação sem compartilhar a senha |
| 1.7 | reserva de disco por volume | upload enchia o cartão e falhava no fim |
| 1.8 | relatório de duplicatas | espaço recuperável sem varrer tudo de novo |
| 1.9 | versões por arquivo | recuperar versão antiga já presente nos packs |
| 1.10 | overflow mobile, revogação de sessão | logout não invalidava o cookie |
| 1.10.2 | correções finais e verificação 80/80 | fechamento da linhagem privada |
| 1.0.0 pública | anonimização, 4 correções, documentação | publicação independente e reproduzível |

## 1.0 — o começo

O ponto de partida foi um servidor HTTP em Go rodando dentro do Termux, com o painel embutido no binário via `go:embed`. As decisões vieram das restrições do ambiente, não de preferência de arquitetura:

- **binário único**: no Android, cada runtime extra é uma fonte de quebra;
- **sem banco de dados**: JSON legível e fácil de copiar;
- **sem framework no frontend**: menos RAM no navegador do celular;
- **sem root**: o servidor é um app comum, sujeito às regras do Android;
- **uma dependência**: `skip2/go-qrcode`, apenas para QR codes.

O primeiro ciclo já tinha arquivos, upload, ZIP, shares, TOTP e painel de sistema. Ainda não havia lixeira, backup, múltiplas raízes nem testes de autenticação.

## 1.1 — lixeira, backup e a raiz que caía para `/`

### Lixeira e snapshots

**Problema:** excluir pelo painel era definitivo; uma edição errada não tinha volta.

**Solução:** exclusão passou a mover para `.pocketserve-trash` no mesmo volume, com restauração e retenção de 30 dias. O backup passou a ser um snapshot content-addressed: cada arquivo vira um pack `SHA-256`, compartilhado entre snapshots. O manifesto guarda `caminho → tamanho, hash, permissão`.

**Resultado:** 14 snapshots com custo próximo ao de um espelho, sem dependência externa. A restauração de um arquivo verifica o hash antes de entregar.

**Limite:** o backup fica no mesmo aparelho. Se o cartão morrer, os snapshots morrem junto. A cópia externa continua sendo obrigatória.

### A raiz indisponível

**Problema:** se o cartão ou HD da raiz fosse removido, o servidor caía para `/` e expunha o sistema de arquivos inteiro.

**Causa:** o fallback de inicialização usava o diretório atual quando a raiz configurada não existia. Em alguns cenários, o diretório atual era `/`.

**Solução:** o servidor **encerra** com erro quando a raiz não existe. Para diagnóstico consciente existe `POCKETSERVE_ALLOW_MISSING_ROOT=1`, que sobe no diretório atual — recusando `/`, `/system`, `/proc`, `/data` (fora do Termux) e outras árvores de sistema — e avisa no log.

**Teste:** inicialização com raiz inexistente falha; com a variável, sobe no diretório de trabalho e registra o aviso.

**Limite:** o supervisor tenta reiniciar, mas se o volume não voltar, o servidor fica em ciclo de falha. Isso é preferível a expor `/`.

### TOTP

**Problema:** uma senha única era o único fator.

**Solução:** TOTP RFC 6238, 6 dígitos, período de 30 s, 10 códigos de recuperação com hash e desativação exigindo senha + código.

**Limite:** o TOTP depende do relógio do aparelho e do autenticador. Relógio errado recusa códigos válidos.

## 1.2 — acesso, HTTPS e Syncthing

### “Lembrar deste aparelho”

**Problema:** com 2FA ativo, todo login pedia o celular.

**Solução:** aparelho confiável por 30 dias, com lista, revogação e limpeza ao trocar a senha ou desativar o 2FA.

**Limite:** o cookie de aparelho confiável é um segundo fator persistente. Se ele vazar, o atacante ainda precisa da senha; se a senha também vazar, o 2FA não protege naquele aparelho.

### Recuperação de acesso

**Problema:** senha esquecida ou autenticador perdido deixavam o painel inacessível.

**Solução:** `-reset-access` remove senha, 2FA e aparelhos confiáveis, sem tocar em arquivos, shares, lixeira ou backups. Exige acesso físico ao Termux.

**Limite:** não há recuperação pela rede, por decisão de segurança.

### HTTPS opcional

**Problema:** o Chrome força HTTPS e mostra erro em HTTP puro.

**Solução:** TLS opcional em `8443` (portas abaixo de 1024 exigem root), mantendo HTTP em `8080` para a LAN. O certificado é local; não há renovação automática.

**Limite:** certificado autoassinado gera aviso; Let's Encrypt vence em 90 dias. Para acesso público, um túnel ou proxy que termina TLS é mais prático.

### Syncthing

**Problema:** não havia sincronização contínua.

**Solução:** script separado `syncthing-rc.sh`, com supervisor próprio, interface em `127.0.0.1:8384` e porta de sincronização `22000`. O PocketServe não implementa sincronização; apenas supervisiona o Syncthing quando ele está instalado.

**Limite:** o supervisor do Syncthing não é um serviço do Android; se o Termux morrer, os dois param.

### Teclado do Android

**Problema:** o teclado capitalizava a primeira letra da senha e adicionava espaço no código.

**Solução:** `autocapitalize="none"`, `autocorrect="off"`, `spellcheck="false"` nos campos e remoção de espaços do código. A mensagem de erro passou a lembrar maiúsculas, acentos e espaço no fim.

## 1.3 — múltiplas raízes

**Problema:** o servidor mostrava uma única pasta. Interno e cartão não podiam ser usados ao mesmo tempo.

**Solução:** até 8 raízes extras, cada uma com apelido curto (`cartao/...`). O caminho exibível carrega o apelido; a validação de traversal e symlink continua contra todas as raízes.

**Resultado:** abas de volume no painel, configuração sem reiniciar e caminhos estáveis.

**Limite:** renomear o apelido quebra links e histórico de versões daquele caminho. O conteúdo no disco não é movido.

## 1.4 — números que não fechavam

### Memória em GB decimal

**Sintoma:** o painel mostrava `7,5 GB` de RAM em um aparelho de 8 GB.

**Causa:** o frontend dividia por 1024³ (GiB), enquanto o Android anuncia em 1000³ (GB).

**Solução:** formatação em base decimal. O percentual sempre esteve correto porque usava `MemAvailable`, que inclui cache recuperável.

**Limite:** `MemFree` sozinho faria o aparelho parecer sem memória; a métrica exibida é `MemAvailable`.

### Bateria invisível

**Sintoma:** campos vazios, com a mensagem “Use o app Termux:API”.

**Causa:** o Android bloqueia `/sys/class/power_supply/battery/*` para processos fora do Termux em algumas versões. O servidor roda no Termux, mas uma atualização do sistema pode mudar isso.

**Solução:** fallback para `termux-battery-status` e mensagem explícita “O Android bloqueou a leitura” quando ambos falham.

**Limite:** sem Termux:API e sem sysfs, não há leitura de bateria.

### Endereço curto

**Descoberta:** com MagicDNS ativo, `http://nome-do-aparelho:8080` resolve sem o sufixo `.tailXXXX.ts.net`. O nome curto vem do campo `tailscale_name`; o PocketServe não configura a VPN.

**Limite:** o nome curto só funciona com Tailscale conectado e MagicDNS ativo. Se o Android matar a VPN, o endereço para de resolver.

### 1.4.1 — soma dos volumes

**Sintoma:** o dashboard mostrava 217 GB livres, mas havia também um cartão de 128 GB.

**Causa:** o card somava apenas a raiz principal.

**Solução:** soma de todos os volumes, com detalhamento por volume. A deduplicação usa caminho normalizado, não `fsid`: no FUSE do Android, dois volumes podem reportar o mesmo identificador.

**Resultado medido:** 340 GB livres de 361 GB no aparelho de referência, contra 217 GB antes.

## 1.5 — o bug que recusava uploads no interno

**Sintoma:** com o cartão configurado, qualquer upload para `DCIM`, `Download`, `Projetos` ou `01_DOCUMENTOS` voltava com “Pasta de destino inválida”. Só `cartao/...` funcionava.

**Causa:** `aliasLike` considerava suspeito qualquer primeiro segmento sem ponto e sem espaço quando havia mais de uma raiz. Nomes reais de pastas do celular eram tratados como apelidos inválidos.

**Solução:** a ordem de resolução passou a ser:

1. apelido exato tem prioridade;
2. pasta que existe na raiz principal é pasta normal;
3. só recusa se o nome não existe e é quase igual a um apelido — `cartoa` sugere `cartao`.

**Testes:** `roots_test.go` e `volumes_test.go` cobrem pastas reais com duas raízes, prioridade do apelido, sugestão de erro de digitação e nomes derivados (`cartao-bkp`).

### Outros três problemas do mesmo ciclo

| Problema | Causa | Solução |
|---|---|---|
| mover entre interno e cartão não funcionava | `os.Rename` entre sistemas de arquivos retorna `EXDEV` | cópia + remoção, só remove a origem após a cópia completa |
| excluir no cartão era definitivo | lixeira só existia na raiz principal | lixeira por volume, com `root` no `meta.json` |
| busca no cartão devolvia `../rcB/...` | caminho relativo em vez do caminho exibível | `displayPath` com apelido e busca em todos os volumes |

**Limite:** a cópia entre volumes pode demorar e não é retomável. Se o processo morrer no meio, a origem permanece; o destino pode ficar parcial.

## 1.6 — tokens de API

**Problema:** automação precisava da senha ou de um cookie de sessão.

**Solução:** tokens com escopos (`files:read`, `files:write`, `files:delete`, `shares:manage`, `admin`), validade opcional, hash SHA-256 em disco e revogação individual. A chave aparece uma única vez.

**Decisão:** SHA-256 puro, não PBKDF2. O token tem 256 bits de entropia real; não há dicionário a proteger, e PBKDF2 custaria ~200 ms por requisição.

**Limite:** `admin` não dá acesso a configuração, sistema, log, TOTP, aparelhos confiáveis ou outros tokens. Não há CORS: a API é para o mesmo origin.

## 1.7 — reserva de disco

**Problema:** nada impedia um upload de encher o cartão até o último byte. Em exFAT, a alocação degrada; o Android fica instável quando o armazenamento compartilhado zera. Pior: um upload por Tailscale recebia gigabytes pela rede para descobrir no fim que não cabia.

**Solução:** reserva configurável por volume (padrão 2 GB), checada antes do primeiro byte, antes de cada arquivo, depois da gravação, no editor e no unzip. Resposta `507 Insufficient Storage` com código `no_space`. Alerta a partir de 90% de uso, no máximo uma vez por dia por volume.

**Bug encontrado pelo teste:** com `needed == 0`, a checagem passava porque só comparava `needed > usable`. Corrigido para recusar sempre que o volume já está abaixo da reserva, independentemente do tamanho do arquivo.

**Limite:** a reserva é por volume, não por pasta. Dois uploads simultâneos podem passar juntos na janela entre a checagem e a escrita; a checagem pós-escrita remove o arquivo que furou a reserva.

## 1.8 — duplicatas sem varrer o disco

**Problema:** o backup já calculava SHA-256 de todos os arquivos, mas essa informação não era usada para encontrar cópias.

**Solução:** relatório a partir do snapshot mais recente, agrupado por hash, com sugestão determinística de cópia a manter (caminho mais curto; empate alfabético). A limpeza move para a lixeira e tem duas travas no backend: nunca apaga a cópia sugerida e nunca deixa um grupo sem cópia.

**Decisão:** não usar symlinks para deduplicar. O `resolveExisting` usa `EvalSymlinks` para impedir escape de raiz; symlinks entre volumes quebrariam a validação.

**Limite:** depende do snapshot (relatório reflete a data do último backup), compara apenas conteúdo exato e ignora arquivos vazios. Não detecta quase-duplicatas (fotos com metadados diferentes, versões levemente editadas).

## 1.9 — versões por arquivo e o cache que devolvia conteúdo antigo

**Ideia:** como o backup é content-addressed, a versão antiga de um arquivo sobrescrito já está no disco. Bastava cruzar os manifestos por caminho.

**Solução:** `GET /api/versions`, download de uma versão e restauração. A restauração extrai para um temporário no mesmo diretório, verifica SHA-256 e renomeia; a versão atual vai para a lixeira quando a lixeira está ligada.

### O bug de perda de dados

**Sintoma:** dois snapshots seguidos mostravam o mesmo hash para conteúdos diferentes.

**Causa:** o cache de hash usava `mtime.Unix()` (precisão de segundos). Duas edições no mesmo segundo, com o mesmo tamanho (`"AAAA"` → `"BBBB"`), batiam no cache e o snapshot gravava o pack do conteúdo anterior. No cartão exFAT, a granularidade é de 2 segundos, piorando a janela.

**Solução:** nanossegundos + `ctime` + janela de acomodação de 10 segundos. Arquivos recém-modificados nunca vêm do cache.

**Testes:** `TestBackupDoesNotReuseHashAfterQuickEdit`, `TestFileHistoryDetectsVersions` e `TestManifestCacheSkipsRecentFiles`.

**Custo:** o cache fica menos eficiente para arquivos recém-modificados. No snapshot diário, sobre arquivos parados há horas, o impacto é nulo.

**Limite:** renomear um arquivo “quebra” o histórico, porque o cruzamento é por caminho exibível. Não há diff; baixe a versão e compare fora.

## 1.10 — auditoria, overflow e logout

### Overflow no celular

**Problema:** o painel criava rolagem horizontal em telas estreitas.

**Solução:** correções de CSS responsivo e testes de layout (`responsive_test.go`). O painel passou a caber em 390 px sem rolagem horizontal.

### Auditoria independente

Uma varredura completa encontrou:

| Achado | Gravidade | Resolução |
|---|---|---|
| exposição pública com fator único | P0 | 2FA disponível; recomendação de ativar antes de expor |
| autenticação com 0% de teste | P1 | testes de PBKDF2, login, rate limit, sessão e TOTP |
| cobertura geral de 28% | P1 | subiu para 33,8% no fechamento da linhagem |
| `CF-Connecting-IP` não lido | P2 | corrigido na versão pública 1.0.0 |
| rota duplicada derrubava o processo | P2 | `TestRoutesRegister` trava o registro |
| `shares.go` quase sem teste | P3 | testes de hash não exposto e páginas públicas |
| estado global mutável | P3 | dívida conhecida; um processo = um `App` |

A auditoria também confirmou path traversal, upload com nome malicioso, XSS e concorrência:

- 6 variações de traversal recusadas;
- upload com `../` sanitizado;
- nome `<img src=x onerror=...>.txt` exibido como texto;
- `go test -race` limpo e 10 uploads simultâneos sem corrupção.

### Logout que não invalidava

**Sintoma:** o logout removia o cookie do navegador, mas o cookie antigo continuava válido no servidor.

**Causa:** a sessão era sem estado (`HMAC(exp, versão_da_senha)`) e `handleLogout` só emitia um `Set-Cookie` vazio. O servidor não guardava nada.

**Agravante:** o payload não tinha ID. Dois logins no mesmo segundo geravam **cookie idêntico**. Revogar “o cookie” derrubaria todas as sessões daquele segundo — sair do notebook deslogaria o celular.

**Solução:** payload com ID aleatório de 16 bytes e lista de revogação persistida em `revoked_sessions.json`, guardando o **hash** do ID, com expiração igual à da sessão e limpeza horária. Sessão sem estado continua sendo o caminho normal; a lista só cresce com logouts.

**Testes:** 13 testes em `sessions_test.go`, incluindo logout, reutilização do cookie, persistência após reinício, isolamento entre sessões, cookie legado sem ID e limite de crescimento.

**Resultado medido:** 99 testes, cobertura 33,8%, `go test -race` limpo e 80/80 verificações HTTP. O cookie revogado devolveu `401` após `kill -9` e reinício; o cookie legítimo continuou `200`.

**Limite residual:** cookies emitidos antes da correção não têm ID e não são revogáveis individualmente; expiram sozinhos. A lista fica em `~/.pocketserve`, no armazenamento privado do Termux, e não é copiada no backup dos arquivos do usuário.

## 1.10.2 — fechamento da linhagem privada

O último marco privado incluiu:

- script de rollback;
- verificação que aceita instância já configurada;
- correção do logout;
- 80/80 verificações HTTP;
- documentação de riscos e rollback.

## 1.0.0 pública — auditoria, anonimização e quatro correções

A versão pública foi criada em uma cópia isolada, sem o histórico git privado, sem binários, sem dados de execução e sem documentação interna. O código funcional é o da linhagem 1.10.2, com renomeações de identidade e quatro correções encontradas na revisão final:

### 1. Hash de senha de share exposto

**Problema:** `GET /api/shares` e a resposta de criação devolviam o campo `password_hash` do link.

**Solução:** a representação pública passou a expor apenas `has_password`; o hash continua somente em `shares.json`. O frontend passou a usar `s.has_password`.

**Testes:** `TestSharePublicDoesNotExposePasswordHash` e `TestShareListDoesNotExposePasswordHash`.

### 2. IP real atrás de túnel

**Problema:** atrás de um túnel que conecta em `127.0.0.1`, o servidor via todos os clientes como loopback. O limite de tentativas virava um balde único e um atacante podia bloquear o login de todos.

**Solução:** quando a conexão vem de loopback, o servidor aceita `CF-Connecting-IP`, depois `X-Forwarded-For` e `X-Real-IP`. Fora do loopback, esses cabeçalhos continuam ignorados.

**Testes:** cabeçalhos de rede ignorados, `CF-Connecting-IP` aceito no loopback, fallback para `X-Forwarded-For` e cabeçalho malformado.

**Limite:** se o proxy local não encaminhar nenhum desses cabeçalhos, o problema persiste. A configuração do proxy é responsabilidade do operador.

### 3. `trash_days = 0` não desligava a lixeira

**Problema:** a interface e a documentação diziam que `0` desliga a lixeira, mas `fillDefaults` convertia `0` em `30` em toda carga e gravação. Na prática, a lixeira nunca podia ser desligada.

**Solução:** `config.json` sem a chave recebe 30; a chave presente com valor `0` é preservada. O comportamento agora corresponde ao que a interface anuncia.

**Testes:** `TestConfigExplicitZeroTrashDaysIsPreserved` e `TestConfigMissingTrashDaysUsesDefault`.

### 4. Escape hatch de raiz podia expor o sistema

**Problema:** `POCKETSERVE_ALLOW_MISSING_ROOT=1` prometia subir "no diretório atual, nunca em `/`", mas o código usava `os.Getwd()` sem validar. Iniciado com `cwd=/`, o servidor exporia o sistema de arquivos inteiro — o oposto do que a checagem de raiz existe para impedir.

**Solução:** `fallbackRoot` recusa `/`, `/system`, `/proc`, `/sys`, `/dev`, `/vendor`, `/etc`, `/root` e qualquer caminho sob `/data` fora do Termux. O diretório do Termux é aceito porque é o `cwd` normal do app.

**Teste:** `TestFallbackRootRefusesSystemTrees`, com dez caminhos.

## O que os erros ensinaram

1. **Timestamp de segundo não identifica versão.** O bug do cache de backup só apareceu porque o teste editou o mesmo arquivo duas vezes no mesmo segundo. Em produção, teria destruído dados silenciosamente.
2. **Teste de caminho não é teste de permissão.** O bug do `aliasLike` recusava pastas reais; a validação de traversal estava correta, mas a resolução de apelido não.
3. **Logout sem estado não é logout.** Sem um ID de sessão, não há o que revogar.
4. **Interface e código precisam da mesma verdade.** O `0` da lixeira era aceito pela interface, mas anulado pela configuração.
5. **Documentação não é garantia.** O comentário dizia "nunca em `/`" e o código não verificava; o escape hatch de diagnóstico era, na prática, um caminho para expor o sistema.
6. **O ambiente é parte do software.** FUSE, `EXDEV`, exFAT, seccomp, `/proc` bloqueado e o gerenciador de processos do Android explicam boa parte do código.
7. **Backup local não é recuperação de desastre.** O snapshot protege contra erro humano, não contra perda do aparelho.

## O que continua em aberto

- cobertura de testes em `files.go`, `system.go`, `shares.go`, `trust.go` e `tls.go`;
- `files.go` concentra listagem, upload, download, ZIP, editor, thumbnails e busca;
- não há teste automatizado de navegador para todas as telas;
- não há restauração integral de snapshot pela interface;
- não há cópia externa automática;
- não há lista de sessões ativas no painel;
- não há teste de carga prolongada nem medição de vazão;
- a exposição pública continua exigindo decisão consciente do operador (TLS, 2FA, senha forte e, de preferência, uma camada de acesso).

Consulte [LIMITACOES.md](LIMITACOES.md) e [VERIFICACAO.md](VERIFICACAO.md) para o estado atual.
