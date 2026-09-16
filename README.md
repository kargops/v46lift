# v46lift

`v46lift` lifts legacy IPv4-only applications onto IPv6 infrastructure.

The name is read “v4-to-6 lift.”

The application is left unmodified. `v46lift` prepares any synthetic IPv4 addresses needed locally, starts a transport backend, launches the application, and tears the compatibility layer down when the application exits.

## Status

Early scaffold / proof of architecture.

The first backend is [GOST v3](https://v3.gost.run/) for explicit TCP/UDP port mappings. A Linux SIIT/Jool backend is planned for address-level translation that preserves ports and protocols without per-port configuration.

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

See [`examples/legacy-game.json`](examples/legacy-game.json).

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

## Usage

```bash
go build -o ./bin/v46lift ./cmd/v46lift

./bin/v46lift validate --config examples/legacy-game.json
./bin/v46lift print-gost --config examples/legacy-game.json
sudo ./bin/v46lift launch --config examples/legacy-game.json
```

`sudo` is currently needed on Linux only when `network.manage_synthetic_ips` is enabled, because adding/removing addresses on loopback requires `CAP_NET_ADMIN`.

## Why GOST first?

GOST already implements the TCP/UDP forwarding dataplane and supports IPv6 targets. `v46lift` therefore focuses on the missing product layer: lifecycle, configuration, packaging, process supervision, and OS integration.

For UDP, GOST's keepalive option is enabled per mapping when requested. This matters for game protocols that exchange multiple datagrams over a logical session.

## Roadmap

1. **MVP — explicit TCP/UDP mappings**
   - [x] Config schema
   - [x] GOST command generation
   - [x] GOST child-process lifecycle
   - [x] Game process lifecycle
   - [x] Linux synthetic IPv4 address lifecycle
   - [x] Signal-aware cleanup
   - [ ] Integration tests with real GOST
   - [ ] Release packaging

2. **Native UX**
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
   - [ ] `v46lift pack`
   - [ ] MSI/EXE installer integration
   - [ ] Linux package generation
   - [ ] Optional bundled GOST binary with license notices

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

The game should never run elevated.

## License

MIT.
