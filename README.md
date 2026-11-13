<div align="center">
  <img src="web/icon.svg" width="88" alt="PocketServe icon" />
  <h1>PocketServe</h1>
  <p><strong>Your pocket file server.</strong><br />
  A single Go binary that turns an old Android phone into a private file server.</p>

  <p>
    <img alt="Go 1.24" src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&amp;logoColor=white" />
    <img alt="Platform: Android / Termux arm64" src="https://img.shields.io/badge/platform-Android%20%C2%B7%20Termux%20arm64-3DDC84?logo=android&amp;logoColor=white" />
    <img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-2EA043" />
    <img alt="Dependencies: 1" src="https://img.shields.io/badge/dependencies-1-6E7681" />
  </p>

  <sub><a href="README.pt-BR.md">Português</a> · <strong>English</strong></sub>
</div>

> [!NOTE]
> The web interface and the in-depth documentation are written in Portuguese (pt-BR).
> The code, the JSON API and this page are language-neutral.

---

## Why this exists

Most people have a spare Android phone in a drawer: enough CPU, enough storage, a battery
that keeps it alive during a power cut, and no use. PocketServe gives it a job.

- **The hardware is already paid for.** No NAS, no VPS, no monthly bill.
- **One binary, nothing to install.** The HTTP server, the JSON API and the web panel are the
  same static executable. No database, no runtime, no frontend build step.
- **Your files stay on your phone.** No cloud account, no telemetry, no outbound connection
  required. Access is over your Wi-Fi or a private VPN.
- **It is a real file manager, not a demo.** Trash, versioned snapshots, duplicate detection
  and expiring share links are part of the first release.

## Features

| Area | What you get |
|---|---|
| **Files** | Browse, upload, download, rename, copy, move, create folders, edit text files and download a whole folder as ZIP |
| **Roots** | Up to 8 root folders at once, such as internal storage and a microSD card, each with a short alias (`card/...`) |
| **Trash** | Deleting moves to a per-volume trash with configurable retention (30 days by default) |
| **Snapshots** | Content-addressed (SHA-256) and deduplicated backups; 14 are kept, one per day when enabled |
| **File versions** | Per-file history taken from those snapshots, ready to download or restore |
| **Cleanup** | Duplicate report grouped by SHA-256, with a safe suggestion of which copy to keep |
| **Sharing** | Read-only public link with token, expiry date and optional password |
| **Access control** | PBKDF2 password hashing (210,000 iterations), TOTP two-factor, recovery codes and trusted devices |
| **Sessions** | Signed `HttpOnly` cookie, CSRF protection, login throttling and server-side session revocation |
| **Automation** | API tokens with scopes (`files:read`, `files:write`, `files:delete`, `shares:manage`, `admin`), expiry and individual revocation |
| **Operations** | Supervisor with automatic restart, start on boot, local activity log and update with rollback |
| **Dashboard** | CPU, memory, storage per volume, battery, network, access addresses and a QR code to open the panel |

## What it does not do

Stated up front so nobody expects the wrong thing:

- it is not multi-user;
- it has no WebDAV;
- it has no native mobile app;
- it does not sync by itself (use Syncthing or scripts against the API);
- it restores files individually, not a whole snapshot through the UI;
- it is not a public cloud service and must not be exposed without TLS, a strong password and 2FA;
- it does not replace a NAS or a desktop server in throughput, redundancy or durability.

The full list is in [docs/LIMITACOES.md](docs/LIMITACOES.md) (Portuguese).

## How it works

```
 Browser (LAN / Tailscale / HTTPS proxy)
        │  HTTP :8080  (or HTTPS :8443)
        ▼
┌──────────────────────────────────────────────────┐
│  PocketServe — one Go process, static binary     │
│  routes · auth · files · shares · backups        │
│  web panel embedded through go:embed             │
└──────────────────────────────────────────────────┘
        │
        ▼
  /sdcard  →  microSD card (optional)
  ~/.pocketserve/config.json · shares.json
                 tokens.json · activity.jsonl
```

One process, no database: state lives in JSON files under `~/.pocketserve`. The only external
dependency is [`skip2/go-qrcode`](https://github.com/skip2/go-qrcode), used to draw QR codes
locally. Details in [docs/ARQUITETURA.md](docs/ARQUITETURA.md) (Portuguese).

## Quick start

Requirements: an Android phone with **arm64-v8a**, [Termux](https://termux.dev/), and **Go 1.24+**
on a computer to cross-compile. No root, no Android SDK, no NDK, no C compiler.

**1. Build the static binary** (on your computer):

```bash
git clone https://github.com/ReiThomato/pocketserve.git
cd pocketserve

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags "-s -w" -o pocketserve-arm64 .
```

**2. Prepare the phone** (in Termux):

```bash
termux-setup-storage
termux-wake-lock
pkg update && pkg upgrade
```

Also set Termux to **unrestricted** battery usage and enable autostart, otherwise Android will
kill the process when the screen turns off.

**3. Install and open it:**

```bash
adb shell mkdir -p /sdcard/Download/PocketServe
adb push pocketserve-arm64 /sdcard/Download/PocketServe/
adb push deploy/install.sh deploy/ctl.sh deploy/syncthing-rc.sh \
  deploy/rollback-guardar.sh deploy/backup-estado.sh \
  /sdcard/Download/PocketServe/

# then, in Termux:
bash /sdcard/Download/PocketServe/install.sh
```

The installer prints the LAN address. Open `http://YOUR_PHONE_IP:8080` from any device on the
same network and set the administrator password on first access.

Step-by-step with expected output, updates and rollback:
[docs/INSTALACAO.md](docs/INSTALACAO.md) and [docs/OPERACAO.md](docs/OPERACAO.md) (Portuguese).

## Security in one sentence

The model assumes **a single user, access over a local network or private VPN, and no port
forwarded on the router**. If you decide to expose it to the internet, read
[docs/SEGURANCA.md](docs/SEGURANCA.md) and [docs/ACESSO-REMOTO.md](docs/ACESSO-REMOTO.md)
first and use HTTPS, a strong password and 2FA.

## Tests

```bash
make test     # go test ./...
make race     # go test -race ./...
make vet      # go vet ./...
make cover    # coverage
make arm64    # arm64 binary for Termux
```

`deploy/verify-local.sh` starts a disposable instance, runs 80 end-to-end HTTP checks and shuts
it down. Results and coverage notes: [docs/VERIFICACAO.md](docs/VERIFICACAO.md) (Portuguese).

## Documentation

The complete manual is in Portuguese, under [`docs/`](docs/):

| Document | For |
|---|---|
| [Instalação](docs/INSTALACAO.md) | from zero to the first login, with commands and expected output |
| [Configuração](docs/CONFIGURACAO.md) | flags, `config.json`, environment variables |
| [Uso](docs/USO.md) | every panel screen and operation |
| [Arquitetura](docs/ARQUITETURA.md) | modules, request flow, decisions and limits |
| [Armazenamento](docs/ARMAZENAMENTO.md) | roots, aliases, SD card, trash and disk reserve |
| [Backup e recuperação](docs/BACKUP-E-RECUPERACAO.md) | snapshots, versions, restore and disasters |
| [Operação](docs/OPERACAO.md) | supervisor, logs, update, rollback and diagnostics |
| [Segurança](docs/SEGURANCA.md) | threat model, authentication and residual risks |
| [Acesso remoto](docs/ACESSO-REMOTO.md) | LAN, Tailscale, Cloudflare Tunnel and HTTPS |
| [Sincronização](docs/SINCRONIZACAO.md) | Syncthing, rsync and API automation |
| [API](docs/API.md) | routes, scopes, `curl` examples and error codes |
| [Hardware](docs/HARDWARE.md) | measurements, thermal limits, what is and is not verified |
| [Limitações](docs/LIMITACOES.md) | what does not exist or is not guaranteed |
| [FAQ](docs/FAQ.md) | common questions and problems |

## License

Distributed under the **MIT** license. See [LICENSE](LICENSE).

PocketServe started as a personal file-server project on a phone. This public version was
reorganised, renamed and documented independently. Library and tool credits are in
[docs/TERCEIROS.md](docs/TERCEIROS.md).

**No affiliation with phone manufacturers, Termux, Tailscale or Cloudflare.** Trademarks belong
to their respective owners.
