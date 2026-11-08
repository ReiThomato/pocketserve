# Uso

Guia das telas e operações do painel. O painel é uma aplicação web embutida no binário; funciona em qualquer navegador moderno, no computador ou no celular.

## Primeiro acesso

Ao abrir o endereço pela primeira vez, o painel pede:

1. **nome do servidor** (até 40 caracteres; pode manter `PocketServe`);
2. **senha** com pelo menos 8 caracteres;
3. confirmação da senha.

Depois de criar (**Criar servidor**), a sessão já fica autenticada. Recomenda-se ativar 2FA em **Configurações → Verificação em duas etapas**.

Se você chamar `POST /api/login` antes de concluir a configuração, a API responde `428 setup_required` com a mensagem `Crie a senha do administrador primeiro.` No navegador, a tela de configuração aparece automaticamente.

## Visão geral (Início)

Mostra:

- nome e estado do servidor;
- espaço livre total e bateria;
- cartões de **armazenamento** (soma dos volumes, com detalhamento), **bateria**, **memória** e **rede**;
- ações rápidas: enviar arquivos, nova pasta, explorar, compartilhar;
- atividade recente e endereços de acesso.

O cartão de armazenamento avisa quando um volume está abaixo da reserva de disco. O de bateria mostra `Sem acesso à bateria` se o Android bloquear a leitura; nesse caso, instalar o Termux:API e conceder a permissão costuma resolver.

## Arquivos

A tela principal. No topo:

- **abas de volume** (principal + raízes extras);
- **caminho** (breadcrumbs);
- **busca** (mínimo 2 caracteres; varre a pasta atual ou todos os volumes quando a busca parte da raiz);
- **filtros** (tudo, pastas, imagens, vídeos, áudio, documentos, compactados);
- **ordenar** (nome, tamanho, data);
- **mostrar ocultos**;
- **grade ou lista**;
- **Enviar** e **Nova pasta**.

### Navegar

- Clique/toque em uma pasta para abrir.
- Clique em um arquivo para visualizar (imagem, vídeo, áudio, PDF ou texto).
- O botão **⋮** abre o menu do item.
- Na barra de seleção, é possível baixar, mover, copiar e excluir vários itens.
- **Voltar** sobe um nível; o caminho clicável permite saltar para qualquer ancestral.

### Enviar arquivos

O botão **Enviar** abre um menu:

| Opção | O que faz |
|---|---|
| **Escolher arquivos** | abre o seletor do sistema; aceita múltiplos |
| **Enviar uma pasta** | envia a árvore inteira, preservando subpastas |
| **Enviar para os dois** | aparece na raiz quando há ao menos um volume extra; distribui os arquivos pelo espaço livre |
| **Enviar pasta para os dois** | manda a árvore inteira para o volume com mais espaço |
| **Nova pasta aqui** | cria uma pasta no diretório atual |

Também é possível **arrastar e soltar** arquivos na janela. No celular, o seletor de arquivos permite escolher fotos, vídeos e documentos.

Durante o envio, um painel mostra o progresso por arquivo, a velocidade e os erros. É possível **tentar de novo** um item que falhou. O envio para os dois volumes só divide **arquivos soltos**, e apenas quando o envio parte da raiz; uma pasta inteira vai para um único volume para não quebrar a estrutura.

Limites:

- `max_upload_mb` (Configurações) limita cada arquivo; `0` = sem limite;
- a reserva de disco é checada antes, durante e depois da gravação;
- se o volume ficar abaixo da reserva, o arquivo é removido e a resposta é `507`.

### Download e ZIP

- **Baixar** em um arquivo usa `/api/download`.
- **Abrir em nova aba** usa `/api/raw` (útil para imagens, PDF e vídeo).
- **Baixar pasta (.zip)** compacta a pasta inteira em streaming.
- Selecionando vários itens, **Baixar** gera um ZIP com todos.

O ZIP é gerado sob demanda, sem arquivo temporário. O nome do arquivo inclui a pasta ou `selecao.zip`.

### Renomear, mover, copiar

- **Renomear**: nome novo no mesmo diretório; recusa nome vazio ou duplicado.
- **Mover para…**: abre o seletor de destino, com atalhos para cada volume.
- **Copiar para…**: mesma navegação, mantendo a origem.
- Entre volumes diferentes, a operação copia e só remove a origem depois que a cópia termina (o `rename` do Android falha com `EXDEV` entre FUSE distintos).

### Excluir e lixeira

**Excluir** move para a lixeira do volume de origem (se `trash_days > 0`). A lixeira é invisível na listagem, mesmo com “mostrar ocultos”.

Com a lixeira desligada (`trash_days = 0`), a exclusão é definitiva e o painel avisa.

### Editor de texto

Ao abrir um arquivo de texto, o botão **Editar** aparece quando o conteúdo não foi truncado. O editor:

- lê até **2 MiB**;
- grava até **5 MiB**;
- salva de forma atômica (arquivo temporário + `rename`);
- respeita a reserva de disco.

Arquivos binários são detectados e não abrem no editor.

### Versões (backup)

No menu do arquivo, **Versões (backup)** lista as versões distintas encontradas nos snapshots:

- **Baixar** extrai aquela versão sem tocar na atual;
- **Restaurar** coloca a versão no lugar; a versão atual vai para a lixeira (quando ligada);
- a versão marcada `atual` é a do snapshot mais recente que contém o caminho.

Requer que o backup tenha rodado ao menos uma vez. Detalhes em [BACKUP-E-RECUPERACAO.md](BACKUP-E-RECUPERACAO.md).

### Extrair ZIP

O menu de um `.zip` oferece **Extrair ZIP**. Limites: **20 GiB** de conteúdo e **20.000 arquivos** por extração. Caminhos internos com `..` são normalizados e contidos na pasta de destino.

### Busca

- mínimo de 2 caracteres;
- até **400 resultados**;
- limite de **6 segundos**;
- a partir da raiz, varre todos os volumes e evita duplicar caminhos;
- resultados mostram o caminho exibível com apelido (`cartao/...`).

## Compartilhamentos

Lista os links públicos criados. Cada cartão mostra:

- caminho compartilhado;
- validade;
- se tem senha (`Protegido`) ou não;
- contagem de downloads;
- botões **Abrir**, **QR code** e **Revogar**.

Para criar: abra o menu de um arquivo/pasta → **Compartilhar** → escolha a validade (1 h, 24 h, 7 dias, 30 dias ou nunca) → senha opcional (mínimo 4 caracteres) → **Criar link**.

O link tem a forma:

```text
http://SEU_ENDERECO_IP:8080/s/TOKEN
```

A página pública é somente leitura. Em pasta, permite navegar e baixar; em arquivo, permite baixar. Com senha, o visitante precisa digitá-la uma vez; o cookie de desbloqueio é assinado com o `session_secret`.

Revogar invalida o link imediatamente. A expiração é checada a cada acesso.

## Atividade

Histórico local das últimas **500** operações, com filtros **Acessos** e **Arquivos**. Eventos incluem login, falha de login, logout, upload, download, exclusão, backup, restauração, criação/revogação de share e token.

O arquivo fica em `~/.pocketserve/activity.jsonl`. O painel mostra no máximo 250 por vez e a API aceita `limit` de 1 a 500.

> O log é local. Ele não é enviado para nenhum serviço externo.

## Lixeira

Lista os itens excluídos de todos os volumes:

- caminho original, nome, tamanho, data e volume;
- **Restaurar** devolve ao caminho de origem; se o nome já existir, cria `nome (1)`;
- **Apagar** remove definitivamente um item;
- **Esvaziar lixeira** remove todos.

A retenção é configurável (`trash_days`, padrão 30; `0` desliga a lixeira). Uma rotina a cada 6 horas remove itens mais antigos que o limite.

A lixeira fica **dentro de cada volume** (`.pocketserve-trash`), porque restaurar por `rename` só é instantâneo no mesmo sistema de arquivos.

## Duplicados

Mostra grupos de arquivos com **conteúdo idêntico** (mesmo SHA-256), calculados a partir do snapshot mais recente do backup.

- total recuperável, número de grupos e de arquivos;
- cada grupo mostra as cópias, o volume e a cópia sugerida para manter;
- a sugestão é determinística: caminho mais curto vence; empate resolve por ordem alfabética;
- **Limpar selecionados** move as cópias extras para a lixeira.

Duas travas no servidor:

1. a cópia sugerida **nunca** é apagada, mesmo que o cliente peça;
2. cada grupo preserva pelo menos uma cópia.

O relatório depende do backup: se o snapshot é de ontem, ele reflete ontem. Arquivos vazios são ignorados. Não há detecção de quase-duplicatas (fotos com metadados diferentes, versões levemente editadas).

## Acesso

Mostra os endereços pelos quais o servidor pode ser alcançado:

- **LAN** (`http://SEU_ENDERECO_IP:8080`, por exemplo);
- **Tailscale** (`http://celular.tailXXXX.ts.net:8080` ou o nome curto);
- endereço local (`http://127.0.0.1:8080`);
- QR code para abrir no celular.

O domínio público configurado (`public_host`) aparece no log de inicialização, não nesta tela. Se você usa um túnel ou proxy, abra o domínio diretamente no navegador.

O endereço curto do Tailscale (`http://celular:8080`) funciona quando o MagicDNS está ativo. O campo **Nome MagicDNS** em Configurações só afeta o que o painel exibe; não configura a VPN.

## Configurações

Dividida em:

- **Servidor** — nome, porta, raiz, sessão, upload, reserva, lixeira, backup diário, Tailscale;
- **Armazenamento** — raízes extras e volumes detectados;
- **Segurança** — troca de senha;
- **Verificação em duas etapas** — ativar, desativar, códigos de recuperação, aparelhos confiáveis;
- **Tokens de API** — criar, listar, revogar e remover;
- **Backups** — rodar agora, listar snapshots, baixar arquivo, excluir snapshot;
- **Manutenção** — ver log, reiniciar servidor, sair.

Alterações de **porta** ou **pasta raiz** pedem reinício; o painel oferece **Reiniciar agora**.

## Atalhos e detalhes de interface

- O menu **⋮** de um item concentra visualizar/baixar, compartilhar, renomear, mover, copiar, detalhes, versões e excluir.
- **Copiar caminho** nos detalhes usa a área de transferência quando o navegador permite (HTTPS ou localhost).
- O tema é escuro; a interface se adapta a telas estreitas (celular) e largas (desktop).
- O painel não tem service worker: não funciona offline e não guarda arquivos em cache.
- O botão **Sair** revoga a sessão no servidor; o cookie antigo passa a devolver `401`.

## O que o painel não faz

- não edita arquivos binários;
- não reproduz formatos que o navegador não suporta (ex.: MKV com codec exótico, HEIC em alguns navegadores);
- não mostra diff entre versões;
- não restaura um snapshot inteiro pela interface (a restauração é por arquivo);
- não tem multiusuário, WebDAV ou aplicativo nativo.

Lista completa em [LIMITACOES.md](LIMITACOES.md).
