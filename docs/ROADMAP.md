# Roadmap

Este documento lista **ideias e trabalho planejado**. Nada aqui está implementado, a menos que esteja marcado como concluído. Não trate esta lista como compromisso de data ou de entrega.

> A versão pública atual é **1.0.0**. Ela consolida a linhagem privada 1.10.x e não promete compatibilidade retroativa com a numeração privada.

## Como as prioridades são decididas

Não existe equipe nem financiamento. A ordem abaixo segue três critérios, nesta sequência:

1. **risco de perda de dados** — qualquer coisa que possa destruir arquivo do usuário;
2. **correção de comportamento prometido** — quando a interface ou a documentação dizem uma coisa e o código faz outra;
3. **custo de operação** — o que reduz manutenção e suporte.

Funcionalidade nova só entra depois que os dois primeiros itens estão cobertos. Isso explica por que a lista de "features" é curta e a de robustez é maior.

## Concluído na 1.0.0 pública

| Item | Situação |
|---|---|
| Auditoria completa do código e da documentação privada | concluído |
| Renomeação de identidade (`pocketserve`, `POCKETSERVE_HOME`, `ps_session`, `.pocketserve-*`) | concluído |
| Histórico git independente, sem dados privados | concluído |
| Correção: hash de senha de share não é mais exposto na API | concluído |
| Correção: `CF-Connecting-IP` aceito de proxy local | concluído |
| Correção: `trash_days: 0` desliga a lixeira de fato | concluído |
| Remoção dos scripts de túnel específicos do provedor | concluído |
| Exemplo genérico de túnel (`deploy/cloudflared.example.yml`) | concluído |
| Documentação pública completa | concluído |
| Script de verificação em instância descartável (`deploy/verify-local.sh`) | concluído |
| Script de restauração de snapshot para pasta de preparo | concluído |
| CI com `gofmt`, `vet`, testes, `-race` e build arm64 | concluído |

## Alta prioridade — robustez

| Item | Por quê | Risco de não fazer | Estado |
|---|---|---|---|
| **Teste de restauração automatizado** | o backup só vale se restaurar | descobrir a falha no dia do desastre | planejado |
| **Verificação periódica de packs** | detectar corrupção antes de precisar | servir erro só na recuperação | planejado |
| **Cópia externa assistida** | o backup local não é desastre-proof | perda total com o aparelho | planejado |
| **Cobertura de teste em `files.go`, `system.go`, `shares.go`** | são os maiores arquivos e os menos testados | regressão silenciosa em upload/download | planejado |
| **Teste de carga prolongada** | ninguém mediu semanas de uso | instabilidade só aparece em produção | não iniciado |

## Média prioridade — operação

| Item | Por quê | Observação |
|---|---|---|
| **Lista de sessões ativas no painel** | hoje não dá para ver ou derrubar uma sessão específica | exige endpoint e decisão sobre identificação |
| **"Encerrar todas as sessões"** | hoje só trocando a senha | poderia usar um `session_epoch` |
| **Rotação de log integrada** | `server.log` cresce sem limite | hoje é manual |
| **Retenção de snapshots configurável** | 14 é fixo | exigiria UI e validação |
| **Agendamento de snapshot por horário** | hoje é "a cada 23 h, se ligado" | depende de o Android não matar o processo |
| **Alerta de bateria/temperatura** | hoje só disco tem alerta | precisa limiar e canal |
| **Exportar atividade** | o log é só local e limitado | útil para auditoria pessoal |
| **Restauração integral pela interface** | hoje só por arquivo ou script | decisão de UX e de segurança |

## Baixa prioridade — funcionalidades

| Item | Situação | Por que está aqui |
|---|---|---|
| **WebDAV** | não implementado | mudaria o modelo de autenticação e o volume de código |
| **Multiusuário com permissões** | não implementado | o projeto é de um usuário; exigiria repensar tudo |
| **Aplicativo móvel nativo** | não implementado | o PWA resolve; app exigiria manutenção de loja |
| **Sincronização embutida** | não implementado | Syncthing já resolve melhor |
| **Edição colaborativa** | não implementado | fora do escopo de um servidor pessoal |
| **Compartilhamento com escrita** | não implementado | aumenta muito a superfície de ataque |
| **Criptografia em repouso** | não implementado | depende do Android; fazer no app seria frágil |
| **Quase-duplicatas (hash perceptual)** | não implementado | complexidade alta para ganho incerto |
| **Preview de Office/HEIC** | não implementado | exige conversores externos |
| **API versionada (`/v1`)** | não implementado | mudaria todas as rotas |

## Fora de escopo

Estes itens foram considerados e recusados de forma consciente:

- **hospedagem de sites ou bancos de dados** — não é o propósito;
- **NAS de produção com RAID** — um celular não tem redundância;
- **CDN, cache distribuído, cluster** — um processo, um aparelho;
- **telemetria ou analytics** — o projeto não envia dados para fora;
- **login social / OAuth** — um administrador, uma senha;
- **marketplace de extensões** — superfície de ataque sem demanda.

## Como propor algo

1. Abra uma issue descrevendo o problema, não a solução.
2. Diga qual é o custo de **não** fazer.
3. Se for código, inclua teste e mantenha a dependência externa em uma.

Leia [CONTRIBUTING.md](../CONTRIBUTING.md) antes de enviar uma alteração.

## O que este roadmap não promete

- datas;
- ordem rígida;
- que todo item será feito;
- compatibilidade com a numeração privada 1.10.x;
- que uma funcionalidade listada aqui exista na próxima versão.

O estado real de cada versão publicada está no [CHANGELOG](../CHANGELOG.md). O que **não** existe hoje está em [LIMITACOES.md](LIMITACOES.md).
