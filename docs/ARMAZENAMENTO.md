# Armazenamento

Como o PocketServe enxerga os volumes do celular, como os caminhos funcionam e o que acontece quando o espaço acaba.

## Modelo de raízes

O servidor expõe **uma raiz principal** e, opcionalmente, **até 8 raízes extras**. Cada raiz extra tem um **apelido curto**.

```text
raiz principal:  /sdcard
raiz extra:      /storage/XXXX-XXXX/PocketServe   apelido: cartao
```

Caminhos:

```text
DCIM/Camera/foto.jpg              → raiz principal
cartao/04_MIDIA/aula.mp4          → raiz extra
cartao/                           → raiz do cartão
```

O primeiro segmento escolhe a raiz **somente quando é um apelido conhecido**. Isso mantém a compatibilidade: quem usa uma raiz só continua vendo os caminhos como antes.

### Ordem de resolução

1. **apelido exato** vence (`cartao/...`);
2. **pasta que existe de fato** na raiz principal é pasta normal, mesmo que pareça um apelido (`DCIM`, `Download`, `Projetos`);
3. nome inexistente e muito parecido com um apelido gera sugestão (`você quis dizer "cartao"?`) em vez de criar uma pasta no lugar errado;
4. qualquer outro nome é uma pasta nova na raiz principal.

Essa ordem corrigiu um bug real: com duas raízes, nomes comuns do interno passaram a ser recusados como “apelido inválido”. Detalhes em [HISTORIA-DO-DESENVOLVIMENTO.md](HISTORIA-DO-DESENVOLVIMENTO.md).

### Apelidos

- normalizados para minúsculas;
- aceitam letras `a-z`, números, `-` e `_`;
- a interface limita a 16 caracteres;
- não podem repetir outro apelido; em conflito, o servidor acrescenta `-2`;
- o apelido pode ser igual ao nome de uma pasta da raiz principal; nesse caso o apelido tem prioridade, porque a escolha foi explícita.

### Raiz dentro de raiz

Se uma raiz extra estiver fisicamente dentro da principal, ela **não aparece como subpasta** na listagem. O volume já tem aba própria; mostrá-lo duas vezes faria o mesmo arquivo aparecer em dois caminhos.

## Adicionar um volume

### Pelo painel

1. **Configurações → Armazenamento**;
2. o servidor lista os volumes encontrados, com espaço livre;
3. clique em **Usar como “...”** ou informe caminho e apelido;
4. **Adicionar**.

A mudança vale na hora, sem reiniciar.

### Caminhos comuns no Android

| Caminho | O que é |
|---|---|
| `/sdcard` | armazenamento compartilhado (interno) |
| `/storage/emulated/0` | mesmo volume, por outro caminho |
| `/storage/XXXX-XXXX` | cartão microSD montado pelo Android |
| `~/storage/external-1` | link criado pelo `termux-setup-storage` |
| `/data/data/com.termux/files/home` | área privada do Termux |

O `XXXX-XXXX` é o UUID do volume. Ele pode mudar se o cartão for reformatado ou trocado; nesse caso, atualize o caminho em Configurações.

Para descobrir no Termux:

```bash
ls -la /storage
ls -la ~/storage
df -h /sdcard /storage/XXXX-XXXX 2>/dev/null
```

### Formatar o cartão

Recomenda-se formatar pelo próprio Android:

**Configurações → Armazenamento → Cartão SD → Formatar**.

Isso garante exFAT correto e a estrutura esperada. **Formate antes de copiar dados** e faça backup: o processo apaga tudo.

Um cartão que aparece como `/storage/0000-0000` provavelmente não foi formatado pelo aparelho. Funciona, mas pode ter formatação de outro dispositivo.

## O que o Android bloqueia

A partir do Android 11 (scoped storage), o Termux não consegue atravessar:

```text
/sdcard/Android/data
/sdcard/Android/obb
```

Essas pastas existem, mas pertencem a outros aplicativos. O servidor responde `403` com uma explicação, em vez de dizer que a pasta não existe. **Não há contorno sem root.**

Também não são acessíveis:

- partições de sistema;
- áreas privadas de outros apps;
- arquivos cuja permissão o Android negou ao Termux.

Se o Termux não tem acesso ao armazenamento compartilhado, `/sdcard` nem existe. Rode:

```bash
termux-setup-storage
```

e aceite o pedido.

## Lixeira por volume

A lixeira fica **dentro de cada volume**:

```text
<raiz>/.pocketserve-trash/<id>/
├── meta.json     caminho original, volume, tamanho, data
└── data/         conteúdo movido por rename
```

Por quê: restaurar por `rename` só é instantâneo no mesmo sistema de arquivos. Se houvesse uma lixeira única no interno, excluir um arquivo do cartão exigiria copiar o conteúdo inteiro — ou seria definitivo.

- a lixeira é invisível na listagem, mesmo com “mostrar ocultos”;
- a retenção padrão é **30 dias** (`trash_days`);
- `0` desliga a lixeira e a exclusão passa a ser definitiva;
- uma rotina a cada 6 horas remove itens vencidos;
- itens antigos guardam o volume de origem para restaurar no lugar certo.

## Backup e armazenamento

Os snapshots ficam **na raiz principal**:

```text
<raiz principal>/.pocketserve-backups/arquivos/
├── snapshots/<id>.json
├── packs/<sha256>.pack
└── index.json
```

O backup **percorre todos os volumes**, mas grava os packs apenas na raiz principal. O manifesto usa caminhos com apelido (`cartao/...`), então a restauração encontra o arquivo no volume correto.

Consequências práticas:

- o espaço necessário para o backup sai do volume principal;
- se o cartão tem 100 GB e o interno só 20 GB livres, um backup do cartão pode não caber;
- o backup **não substitui** uma cópia externa: um defeito no cartão ou no aparelho leva os dois.

Detalhes em [BACKUP-E-RECUPERACAO.md](BACKUP-E-RECUPERACAO.md).

## Reserva de disco

O Android fica instável quando o armazenamento compartilhado zera, e o exFAT degrada a alocação. Por isso existe uma **reserva por volume**, padrão **2048 MB (2 GB)**.

A checagem acontece:

| Momento | Por quê |
|---|---|
| antes de receber o upload | evita baixar gigabytes que não cabem |
| antes de cada arquivo da fila | o anterior pode ter consumido o espaço |
| depois de gravar | se o volume ficou abaixo da reserva, o arquivo é removido |
| no editor | texto também ocupa espaço |
| no unzip | soma o tamanho declarado antes de extrair |

Resposta quando não cabe:

```http
HTTP/1.1 507 Insufficient Storage
Content-Type: application/json

{"code":"no_space","error":"Sem espaço: 1.8 GB livres, mas 2.0 GB são reserva de segurança. Libere espaço ou reduza a reserva em Ajustes."}
```

`0` desliga a reserva. Isso é útil em um volume dedicado, mas não recomendado no cartão.

A checagem é **por volume**, não por pasta: não há cota por diretório. Entre a checagem e a escrita existe uma janela de corrida; a checagem pós-escrita remove o arquivo que furou a reserva.

### Alertas

- o cartão de armazenamento marca o volume com aviso quando ele está abaixo da reserva;
- a partir de **90% de uso**, o servidor grava `disk_alert` na atividade;
- o aviso é emitido uma vez ao entrar no estado crítico e no máximo a cada 24 h;
- a verificação automática roda a cada hora e também logo após subir.

## Números do aparelho de teste

Medições feitas no aparelho usado no desenvolvimento (sem identificar marca ou modelo):

| Volume | Nominal | Visível para o Android | Livre no momento da medição |
|---|---|---|---|
| Interno | 256 GB | ~221 GB | ~202 GB |
| microSD | 128 GB | ~115 GB | ~115 GB |

Por que os números não batem com o rótulo:

- fabricantes contam `1 GB = 1.000.000.000` bytes; o sistema conta em potências de 1024;
- o sistema de arquivos gasta metadados (tabela de alocação, journal, inodes);
- há partições reservadas (boot, recuperação, firmware, `metadata`);
- o Android mantém espaço de segurança.

É normal. O painel soma **todos os volumes configurados** e mostra o detalhamento por volume. A versão que somava apenas a raiz principal foi corrigida na linhagem 1.4.1.

## Remover um volume

Em **Configurações → Armazenamento**, clique em **Remover**. Isso remove a raiz da configuração; **nada é apagado do disco**. Os arquivos continuam acessíveis pelo caminho normal do Android.

Se o volume removido tinha lixeira ou backup, esses dados continuam no disco, mas deixam de aparecer no painel. Para apagá-los de vez, remova as pastas `.pocketserve-trash` e `.pocketserve-backups` manualmente — com cuidado.

## Cartão removido durante a operação

Se a raiz principal desaparecer (cartão removido, HD desconectado), o processo **encerra** em vez de cair para `/`. O supervisor tenta subir de novo; quando o volume volta, o servidor sobe com os dados intactos.

Para diagnóstico, é possível permitir a subida sem a raiz:

```bash
POCKETSERVE_ALLOW_MISSING_ROOT=1 ~/.pocketserve/pocketserve -home ~/.pocketserve
```

Nesse modo, a raiz efetiva vira o diretório atual do processo e o log avisa em destaque. O servidor recusa `/`, `/system`, `/proc`, `/sys`, `/dev`, `/vendor`, `/etc`, `/root` e `/data` fora do Termux. **Não use em produção.**

## Checklist de armazenamento

- [ ] `termux-setup-storage` executado e permissão concedida;
- [ ] raiz principal existe e é acessível;
- [ ] cartão formatado pelo Android (recomendado);
- [ ] apelido curto e sem ambiguidade;
- [ ] reserva de disco configurada (2 GB é um bom padrão);
- [ ] lixeira com retenção adequada ao espaço livre;
- [ ] backup externo planejado — o snapshot local não protege contra falha física.
