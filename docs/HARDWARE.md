# Hardware e ambiente

Este documento descreve o aparelho de referência usado para desenvolver e medir o PocketServe, sem marca, modelo comercial, codinome ou identificadores únicos. O objetivo é permitir que você compare com o seu próprio celular e entenda os limites reais da plataforma.

> **Importante:** os números abaixo são de **um aparelho específico**, medidos em 08/11/2026. Não são requisitos mínimos nem garantia de desempenho. Cada celular, versão do Android e cartão microSD se comporta de forma diferente.

## Como ler as tabelas

| Rótulo | Significado |
|---|---|
| **Medido** | lido do aparelho por comando ou observação direta na data indicada |
| **Declarado** | valor nominal do fabricante ou do sistema (ex.: “8 GB” de RAM) |
| **Não verificado** | não foi medido de forma confiável; não use como garantia |

Valores de capacidade usam **GB decimal** (1 GB = 1.000.000.000 bytes), a mesma base do Android. Por isso um aparelho “de 8 GB” mostra cerca de 7,87 GB em `/proc/meminfo` e um “de 256 GB” mostra cerca de 221 GB utilizáveis.

## Resumo do aparelho de referência

| Item | Valor | Origem |
|---|---|---|
| SoC | MediaTek **MT6769V/CZ** | medido (`/proc/cpuinfo`) |
| Arquitetura | **arm64-v8a**, AArch64 | medido |
| Núcleos | **8** (2 grandes + 6 eficientes) | medido |
| Núcleos grandes | 2× **Cortex-A75** (`0xd0a`), até **2,0 GHz** | medido |
| Núcleos eficientes | 6× **Cortex-A55** (`0xd05`), até **1,8 GHz** | medido |
| RAM | **7.869.816 kB** (~7,87 GB) | medido; 8 GB declarados |
| Armazenamento interno | **221 GB visíveis**, ~19 GB usados | medido; 256 GB declarados |
| Cartão microSD | **128 GB** nominais, **115 GB livres**, **exFAT** | medido |
| Android | **15** (API/SDK **35**) | medido |
| Ambiente Linux | **Termux 0.118.3**, sem root | medido |
| Wi-Fi | **5 GHz**, **802.11ac**, link de **433 Mbps** | medido (taxa negociada) |
| Alimentação | USB, bateria em **100%** | medido no momento |
| Bateria | **5.000 mAh** nominais, **35 °C** | medido / declarado |
| Temperatura em repouso leve | CPU **39,6 °C**; superfície **38,4 °C** | medido |
| Limites térmicos do sistema | CPU **85 °C**; bateria **50 °C** | lido da configuração térmica |

## Como reproduzir as medições

Os comandos abaixo rodam no **Termux**, sem root. Alguns podem falhar conforme a versão do Android; a falha também é informação.

### Sistema e arquitetura

```bash
uname -a
getprop ro.build.version.release
getprop ro.build.version.sdk
getprop ro.product.cpu.abi
nproc
termux-info
```

Saída esperada (valores do aparelho de referência, resumidos):

```text
Linux localhost 5.x ... aarch64 Android
15
35
arm64-v8a
8
Termux 0.118.3
```

> O Android 15 bloqueia a leitura de `/proc/stat`, `/proc/loadavg` e `/proc/uptime` para aplicativos. O PocketServe contorna com a syscall `sysinfo` e mostra “—” quando o percentual de CPU não está disponível. Isso é limitação do sistema, não defeito do servidor.

### CPU

```bash
grep -E 'processor|Hardware|CPU part|CPU implementer' /proc/cpuinfo
for c in /sys/devices/system/cpu/cpu[0-9]*; do
  printf '%s: ' "$(basename "$c")"
  cat "$c/cpufreq/cpuinfo_max_freq" 2>/dev/null || echo indisponível
done
```

No aparelho de referência:

- 8 entradas `processor`;
- 6 núcleos com `CPU part : 0xd05` (Cortex-A55);
- 2 núcleos com `CPU part : 0xd0a` (Cortex-A75);
- frequências máximas de 1,8 GHz e 2,0 GHz.

A frequência máxima não é a frequência sustentada. Sob carga contínua, o Android reduz a frequência para controlar a temperatura.

### Memória

```bash
grep -E 'MemTotal|MemAvailable|MemFree' /proc/meminfo
free -h 2>/dev/null || true
```

No aparelho de referência:

```text
MemTotal:        7869816 kB
MemAvailable:    ...
```

`MemTotal` é a RAM utilizável pelo Linux, não a capacidade comercial. O Android reserva parte para o sistema, câmera, modem e outros componentes.

O PocketServe usa pouca memória: a auditoria da linhagem privada mediu **12,4 MB RSS** em produção (versão 1.10.0). O binário arm64 tem **9,2 MiB** (9.633.976 bytes). Isso deixa a maior parte da RAM livre para o Android.

### Armazenamento

```bash
df -h /sdcard
ls -la /storage/
df -h /storage/* 2>/dev/null
mount | grep -E ' /sdcard | /storage/'
```

No aparelho de referência, na data da medição:

| Volume | Nominal | Visível | Livre | Sistema de arquivos |
|---|---|---|---|---|
| Interno | 256 GB | 221 GB | ~202 GB | FUSE/emulated |
| microSD | 128 GB | ~123 GB | 115 GB | exFAT |

O interno não mostra 256 GB porque o Android reserva partições de sistema, recuperação, firmware e metadados. A diferença de ~35 GB é normal.

O cartão foi formatado pelo próprio Android em **exFAT**. O identificador do volume (`XXXX-XXXX`) foi omitido de propósito: ele é único e não deve ser publicado.

### Rede

```bash
ip -4 addr show wlan0
iw dev wlan0 link 2>/dev/null || true
cat /sys/class/net/wlan0/address 2>/dev/null | sed 's/./X/g'
```

No aparelho de referência:

- interface `wlan0`;
- frequência de 5 GHz;
- padrão 802.11ac;
- taxa de link negociada de **433 Mbps**.

**433 Mbps é a taxa de link física**, não a vazão real. A vazão de um upload depende do roteador, da distância, da interferência, do cartão e do próprio Android. Não foi medida de forma padronizada.

O endereço MAC foi omitido. O endereço IP da rede local é tratado como placeholder (`SEU_ENDERECO_IP`) em toda a documentação.

### Bateria e energia

Com o app **Termux:API** instalado:

```bash
pkg install termux-api
termux-battery-status
```

Alternativa (pode ser bloqueada em algumas versões):

```bash
dumpsys battery
cat /sys/class/power_supply/battery/capacity
cat /sys/class/power_supply/battery/status
cat /sys/class/power_supply/battery/temp
```

No aparelho de referência, no momento da medição:

```json
{
  "health": "GOOD",
  "percentage": 100,
  "plugged": "USB",
  "status": "FULL",
  "temperature": 35.0
}
```

A capacidade nominal da bateria é **5.000 mAh** (valor declarado). O consumo elétrico em watts **não foi medido** com medidor de tomada; não há número confiável para publicar.

### Temperatura

```bash
for z in /sys/class/thermal/thermal_zone*; do
  printf '%s: ' "$z"
  cat "$z/type" 2>/dev/null
  printf '  '
  cat "$z/temp" 2>/dev/null
done
```

No aparelho de referência, em repouso e com carga leve:

| Ponto | Temperatura |
|---|---|
| CPU | 39,6 °C |
| Superfície/carcaça | 38,4 °C |
| Bateria | 35 °C |

Os limites lidos da configuração térmica do Android foram:

| Componente | Limite |
|---|---|
| CPU | 85 °C |
| Bateria | 50 °C |

Ao atingir esses limites, o Android reduz frequência e/ou corrente de carga. O valor exato depende da versão do Android e do fabricante. **Não trate 85 °C como temperatura de operação normal**: é o ponto em que o sistema começa a se proteger.

## Por que um celular pode ser servidor

Um celular moderno é um computador com CPU ARM, RAM, armazenamento, rede e uma bateria que funciona como mini-UPS. Para o perfil do PocketServe — um usuário, poucos acessos simultâneos, arquivos pessoais — isso é suficiente:

- **CPU:** 8 núcleos dão conta de HTTP, I/O e hashing de arquivos pequenos;
- **RAM:** o servidor usa cerca de 12 MB; o gargalo é o navegador do cliente, não o servidor;
- **Rede:** Wi-Fi 5 GHz e 802.11ac são suficientes para uso doméstico;
- **Energia:** a bateria mantém o servidor vivo em quedas curtas de energia;
- **Custo:** um aparelho antigo parado vira servidor sem comprar hardware novo.

O que **não** torna um celular equivalente a um servidor:

| Limite | Efeito prático |
|---|---|
| Sem ECC | um bit errado na RAM não é detectado pelo sistema |
| Sem RAID | um cartão morto perde os dados daquele volume |
| exFAT sem journal | queda de energia durante escrita pode corromper metadados |
| FUSE do Android | cada operação de arquivo passa por uma camada extra; latência maior |
| Gerenciamento agressivo de processos | o Android pode matar o Termux; o supervisor só funciona se o Termux sobreviver |
| Sem resfriamento ativo | carga contínua aquece e reduz frequência |
| Bateria em carga contínua | degradação acelerada; risco de inchaço |
| Sem IPMI/watchdog de hardware | se o Android travar, não há reinício remoto garantido |
| Sem hot-swap | trocar o cartão exige parar e reconectar |
| Wi-Fi, não Ethernet | latência e estabilidade inferiores a cabo |

## Operação contínua

Para deixar o aparelho ligado 24 horas por dia:

1. **Mantenha o carregador conectado** e use carregador de qualidade. O consumo do servidor é baixo, mas a tela, o Wi-Fi e o Android continuam consumindo.
2. **Não deixe a bateria em 100% para sempre se puder evitar.** Carga contínua acelera o desgaste. Se o aparelho permitir limite de carga (root ou recurso do fabricante), use. Caso contrário, monitore temperatura e inchaço e troque a bateria preventivamente.
3. **Ventile.** Não deixe o aparelho dentro de gaveta, embaixo de almofada ou no sol. Superfície plana e rígida, sem capa grossa.
4. **Não bloqueie o Termux.** Em Configurações → Apps → Termux: permita autostart, escolha “Sem restrições” na economia de bateria e trave o app nos recentes.
5. **Ative o wake lock** (`termux-wake-lock`). O instalador e o `start` tentam fazer isso.
6. **Monitore a temperatura** no painel e no Android. Se passar de ~45 °C em repouso, há algo errado (capa, sol, carga rápida, processo em loop).
7. **Tenha uma cópia externa.** O backup embutido fica no mesmo aparelho; não protege contra perda, roubo ou cartão morto.
8. **Aceite a manutenção.** Reinicie o aparelho periodicamente, atualize o Termux e verifique o espaço livre.

### O que fazer se o Android matar o servidor

Sintomas:

- `pocketserve status` mostra “parado”;
- o supervisor reinicia várias vezes no `watchdog.log`;
- o painel para de responder quando a tela apaga.

Verifique, nesta ordem:

```bash
pocketserve status
tail -n 40 ~/.pocketserve/watchdog.log
tail -n 40 ~/.pocketserve/server.log
```

Se o processo morre sem erro no log, quase sempre é o Android:

- economia de bateria do Termux;
- “phantom process killer” (limite de processos em segundo plano);
- otimizador de memória do fabricante;
- Termux removido dos recentes.

Não existe solução garantida sem root. As mitigações são wake lock, isenção de bateria, autostart e manter o app aberto/travado.

## O que não foi medido

Para não transformar estimativa em fato, estes pontos ficaram **não verificados**:

- potência e modelo do carregador;
- consumo em watts na tomada;
- vazão real de upload/download em MB/s;
- durabilidade e classe de velocidade do cartão microSD;
- saúde da bateria (capacidade efetiva após ciclos);
- temperatura e frequência sob carga contínua por horas;
- estabilidade por semanas sem reiniciar;
- comportamento com 3 ou mais volumes simultâneos;
- desempenho com dezenas de milhares de arquivos;
- latência e vazão por Tailscale e por túnel;
- resistência do aparelho a quedas de energia repetidas.

Se você medir esses pontos no seu aparelho, registre o método e a data. Números sem método não são úteis.

## Ficha técnica resumida

```text
SoC:             MediaTek MT6769V/CZ
CPU:             8 núcleos (2× Cortex-A75 2,0 GHz + 6× Cortex-A55 1,8 GHz)
Arquitetura:     arm64-v8a / AArch64
RAM:             7.869.816 kB (~7,87 GB); 8 GB nominais
Interno:         221 GB visíveis; 256 GB nominais; ~19 GB usados
microSD:         128 GB nominais; ~115 GB livres; exFAT
Android:         15 (SDK 35)
Termux:          0.118.3, sem root
Wi-Fi:           5 GHz, 802.11ac, link 433 Mbps
Bateria:         5.000 mAh nominais; 100% e 35 °C na medição
Temperatura:     CPU 39,6 °C; superfície 38,4 °C (repouso leve)
Limites:         CPU 85 °C; bateria 50 °C
Binário:         9,2 MiB (9.633.976 bytes) arm64 estático
RAM do servidor: ~12,4 MB RSS (medição da linhagem privada)
```

> O aparelho de referência é usado apenas para documentar o ambiente. O PocketServe não é afiliado ao fabricante do celular, ao Termux, ao Android ou a qualquer operadora.
