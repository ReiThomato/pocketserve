# Política de segurança

## Versões suportadas

| Versão | Suporte |
|---|---|
| 1.0.x | Sim |
| &lt; 1.0 (linhagem privada) | Não |

## Como relatar uma vulnerabilidade

**Não abra uma issue pública para falhas de segurança.**

Use o recurso **Security Advisories** do GitHub no repositório:

1. Abra a aba **Security**.
2. Escolha **Report a vulnerability**.
3. Descreva o impacto, a versão afetada e um passo a passo de reprodução.
4. Se possível, inclua uma prova de conceito mínima, sem dados reais.

Se o recurso não estiver disponível, entre em contato pelo canal indicado na página do projeto. Não publique detalhes antes de uma correção.

Você pode esperar:

- confirmação de recebimento em até 7 dias;
- avaliação de impacto e prazo de correção;
- crédito na divulgação, se você quiser.

## Escopo

Fazem parte do escopo:

- bypass de autenticação, sessão, CSRF, TOTP ou tokens de API;
- execução de comandos, leitura/escrita fora das raízes configuradas, traversal ou symlink escape;
- exposição de arquivos, configuração, credenciais, cookies ou logs;
- XSS, injeção de HTML/JS nas páginas de compartilhamento ou no painel;
- negação de serviço remota que não exija autenticação;
- falhas no tratamento de ZIP (zip bomb, path traversal, cota);
- vazamento de segredos em backups, versões ou respostas da API.

**Fora do escopo** (por design, documentado):

- servidor HTTP puro na rede local — o modelo assume LAN confiável ou VPN; use HTTPS/Tailscale se não confiar na rede;
- acesso físico ao aparelho desbloqueado ou malware no Android;
- ausência de multiusuário, WebDAV, criptografia própria ou replicação;
- força bruta ilimitada (há bloqueio progressivo, mas não CAPTCHA);
- relatar que "não há rate limit distribuído" ou "não há WAF";
- engenharia social contra o operador.

## Modelo de ameaças resumido

O PocketServe foi desenhado para **um único usuário**, com acesso por:

- Wi-Fi local considerada confiável;
- Tailscale/WireGuard (VPN cifrada);
- ou proxy HTTPS de confiança (Cloudflare Tunnel, Caddy) falando com o servidor por `127.0.0.1`.

Nesse modelo:

| Atacante | Risco | Defesa |
|---|---|---|
| Alguém na mesma Wi-Fi | médio | senha forte, 2FA, HTTPS, bloqueio de tentativas |
| Quem tem um link de compartilhamento | baixo | token de 128 bits, validade, senha opcional, somente leitura |
| Malware no celular | alto, fora do controle do app | sandbox do Android, senha e 2FA não protegem contra app com acesso root |
| Celular perdido/roubado | médio | criptografia do Android + PIN/biometria |
| Internet pública | nulo por padrão | nenhuma porta aberta; túnel é opt-in |

## Recomendações mínimas

1. Senha longa e exclusiva (12+ caracteres), gerenciador de senhas.
2. TOTP ativado e códigos de recuperação guardados fora do celular.
3. Acesso remoto por Tailscale ou túnel HTTPS; nunca encaminhe a porta 8080 no roteador.
4. Mantenha Android e Termux atualizados.
5. Faça backup externo periódico; o snapshot no mesmo cartão não protege contra falha do cartão.
6. Revise **Configurações → Verificação em duas etapas** e revogue aparelhos confiáveis que não reconhecer.
7. Se o aparelho for perdido, troque a senha em outro acesso ou use `-reset-access` no Termux (exige acesso físico).

## Divulgação responsável

Pedimos um prazo razoável para correção antes de qualquer publicação. Não acesse, altere ou exfiltre dados de terceiros durante testes. Testes devem ser feitos em uma instância sua, com dados fictícios.

## Sem garantias

Este é um projeto pessoal distribuído sob MIT, **sem garantia**. Leia [LICENSE](LICENSE) e [docs/LIMITACOES.md](docs/LIMITACOES.md).
