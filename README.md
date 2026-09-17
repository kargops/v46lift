# v46lift

`v46lift` lifts legacy IPv4-only applications onto IPv6 infrastructure.

The name is read “v4-to-6 lift.”

The application is left unmodified. Operators mint a normal-looking installer with the IPv4→IPv6 mapping already baked in. Players run that installer once, then start the game the way they always have. `v46lift` prepares any synthetic IPv4 addresses needed locally, starts a transport backend, launches the application, and tears the compatibility layer down when the application exits.

## Status

Early scaffold with a working GOST backend and a seamless pack/install/wrap path.

The first backend is [GOST v3](https://v3.gost.run/) for explicit TCP/UDP port mappings. A Linux SIIT/Jool backend is planned for address-level translation that preserves ports and protocols without per-port configuration.

## Player experience

Players should never see `v46lift`, GOST, or a config file.

1. Run the minted installer (it may ask for administrator permission once).
2. If the original client has its own installer, that runs as part of setup.
3. Start the game from the usual shortcut or executable.

The installed client is replaced with a shim that has the mapping prebaked. Starting the game starts GOST, and exiting the game stops it.

## Operator experience

Mint an installer from a mapping plus the vendor client installer and a GOST binary:

```bash
go build -o ./bin/v46lift ./cmd/v46lift

./bin/v46lift pack \
  --config examples/legacy-game-pack.json \
  --gost /usr/local/bin/gost \
  --installer /path/to/legacy-game-setup.sh \
  --output dist/legacy-game-setup
```

Give players `dist/legacy-game-setup` (or `legacy-game-setup.exe` on Windows). That is the only file they need.

`--installer` is only the vendor *setup* program. It is not how GOST starts. GOST is not a background watcher. After setup, `v46lift` replaces the player-facing client binary (`game.executable`, or `--wrap`) with a shim. Shortcuts keep pointing at that same path, so launching the game actually launches the shim:

1. shim starts GOST
2. shim starts the preserved original (`*.v46lift-real`)
3. when that process exits, the shim stops GOST

If pack does not know that launch path, it cannot intercept the game, and GOST will never start automatically. That path is baked into the installer; players never type it.

Pack flags:

| Flag | Purpose |
| --- | --- |
| `--config` | Mapping and game path JSON |
| `--gost` | GOST v3 binary to bundle |
| `--installer` | Optional vendor *setup* program, run once during install |
| `--name` | Short id used in install paths (`legacy-game`) |
| `--display-name` | Name shown during setup |
| `--install-dir` | Where lift + GOST are stored (default `/opt/v46lift/<name>`) |
| `--wrap` | Player-facing binary to intercept; starting it starts GOST (default `game.executable`) |
| `--output` | Path of the minted installer |
| `--no-set-caps` | Linux: do not grant `CAP_NET_ADMIN` to the shim |

On Linux, setup grants the shim `CAP_NET_ADMIN` so later game launches do not need `sudo`. Addresses are added and removed in-process via netlink so that capability is not lost by execing `ip`. The shim keeps it until teardown; GOST and the game start unprivileged and do not inherit it.

## MVP architecture

```text
legacy application
       |
       | IPv4
       v
synthetic IPv4:port
       |
       v
     v46lift
       |
       +-- local address lifecycle
       +-- GOST process lifecycle
       +-- application lifecycle
       |
       v
IPv6 backend:port
```

Example:

```text
198.18.0.10:27015/TCP -> [2001:db8:42::10]:27015/TCP
198.18.0.10:27015/UDP -> [2001:db8:42::10]:27015/UDP
```

## Configuration

See [`examples/legacy-game.json`](examples/legacy-game.json) and [`examples/legacy-game-pack.json`](examples/legacy-game-pack.json).

```json
{
  "game": {
    "executable": "/opt/legacy-game/game",
    "args": [],
    "working_directory": "/opt/legacy-game"
  },
  "engine": {
    "type": "gost",
    "binary": "/usr/local/bin/gost"
  },
  "network": {
    "manage_synthetic_ips": true,
    "synthetic_ips": ["198.18.0.10"]
  },
  "mappings": [
    {
      "protocol": "tcp",
      "listen_ip": "198.18.0.10",
      "listen_port": 27015,
      "target_host": "2001:db8:42::10",
      "target_port": 27015
    },
    {
      "protocol": "udp",
      "listen_ip": "198.18.0.10",
      "listen_port": 27015,
      "target_host": "2001:db8:42::10",
      "target_port": 27015,
      "udp_keepalive": true,
      "udp_ttl": "2m"
    }
  ]
}
```

Packed launchers load this JSON from an embedded payload. Unpackaged development still accepts `--config`, `V46LIFT_CONFIG`, or a `config.json` next to the binary.

Listen ports must be 1024 or higher. GOST runs unprivileged and cannot bind privileged ports.

## Developer usage

```bash
go build -o ./bin/v46lift ./cmd/v46lift

./bin/v46lift validate --config examples/legacy-game.json
./bin/v46lift print-gost --config examples/legacy-game.json
sudo ./bin/v46lift launch --config examples/legacy-game.json
```

`sudo` is currently needed on Linux only when `network.manage_synthetic_ips` is enabled and the binary does not have `CAP_NET_ADMIN`. Minted installers set that capability on the shim so players do not run commands.

## Why GOST first?

GOST already implements the TCP/UDP forwarding dataplane and supports IPv6 targets. `v46lift` therefore focuses on the missing product layer: lifecycle, configuration, packaging, process supervision, and OS integration.

For UDP, GOST's keepalive option is enabled per mapping when requested. This matters for game protocols that exchange multiple datagrams over a logical session.

If a minted installer bundles GOST, see [`docs/third_party/gost-NOTICE.txt`](docs/third_party/gost-NOTICE.txt). GOST remains separately licensed.

## Roadmap

1. **MVP — explicit TCP/UDP mappings**
   - [x] Config schema
   - [x] GOST command generation
   - [x] GOST child-process lifecycle
   - [x] Game process lifecycle
   - [x] Linux synthetic IPv4 address lifecycle
   - [x] Signal-aware cleanup
   - [ ] Integration tests with real GOST
   - [x] Release packaging (self-contained minted installer)

2. **Native UX**
   - [x] Prebaked mapping in the client shim
   - [x] Vendor installer chaining
   - [x] Auto-start GOST when the client starts
   - [x] Linux file capabilities so play-time needs no sudo
   - [ ] Privileged background service
   - [ ] Unprivileged launcher IPC
   - [ ] Windows service
   - [ ] Windows Job Objects for process-tree supervision
   - [ ] Linux systemd unit/socket activation

3. **Transparent interception**
   - [ ] Windows Wintun/WinDivert backend
   - [ ] Linux TUN backend

4. **Address-level translation**
   - [ ] Linux Jool SIIT backend
   - [ ] EAMT management
   - [ ] Node-based translation namespace orchestration
   - [ ] Preserve arbitrary TCP/UDP/ICMP without per-port rules

5. **Packaging**
   - [x] `v46lift pack`
   - [ ] MSI/EXE installer integration
   - [ ] Linux package generation
   - [x] Optional bundled GOST binary with license notices

## Security model

The final design should separate privileges:

```text
v46lift-launcher (user)
       |
       | authenticated local IPC
       v
v46liftd (privileged)
       |
       +-- addresses/routes/TUN
       +-- translation engine
       |
       v
game process (user)
```

The current minted installer approximates that without a daemon: the shim may carry `CAP_NET_ADMIN` to manage synthetic addresses in-process, and it keeps that capability until teardown. GOST and the game start unprivileged and do not inherit it.

## License

MIT.
