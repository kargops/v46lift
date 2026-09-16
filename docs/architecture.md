# Architecture

## Goals

- No modification or injection into the legacy application.
- Support arbitrary application executables.
- Support IPv4-listen to IPv6-target forwarding.
- Support TCP and UDP.
- Keep application and network lifecycle coupled.
- Keep the game process unprivileged.
- Make the dataplane replaceable.

## Backend model

`v46lift` deliberately separates orchestration from translation.

```text
                    v46lift launcher
                         |
       +-----------------+------------------+
       |                 |                  |
 local network      backend engine      process supervisor
 preparation          interface               |
       |                 |                    v
       |          +------+------+          game.exe
       |          |             |
       |        GOST          Jool/SIIT
       |      TCP/UDP       address-level
       |
 synthetic IPv4s
```

### GOST backend

Best fit when the compatibility contract is explicit:

```text
IPv4:port/protocol -> IPv6:port/protocol
```

Advantages:

- Cross-platform GOST binary.
- TCP and UDP already implemented.
- No application protocol knowledge.
- Backend uses the host's normal IPv6 connectivity.

Tradeoff:

- Every listening port/protocol must be declared.

### Jool/SIIT backend

Best fit when the compatibility contract is:

```text
synthetic IPv4 host -> native IPv6 host
```

Advantages:

- Translation happens at L3.
- Ports remain untouched.
- TCP, UDP, ICMP and other translatable traffic do not need separate mappings.

Tradeoffs:

- Linux-specific.
- Requires privileged network setup.
- Node-based translation requires namespace/routing orchestration and a usable IPv6 source mapping.

## Privilege separation

The current scaffold is single-process for speed of iteration.

Production should become:

```text
unprivileged launcher
       |
       | local authenticated IPC
       v
privileged v46liftd
       |
       +-- install/remove synthetic addresses
       +-- install/remove routes
       +-- own Wintun/TUN/Jool
       +-- start/stop backend
```

The launcher owns the application lifecycle; the daemon owns only networking.

Minted installers currently approximate this without a daemon. The player-facing
file is the original client path, replaced with a `v46lift` shim that already
contains the mapping. The shim may hold `CAP_NET_ADMIN` on Linux so synthetic
addresses can be added without sudo, then drops privilege before execing the
preserved original client (`*.v46lift-real`). GOST is bundled beside the shim
and is started only while the client runs.

## Packaging

Operators mint a single installer with `v46lift pack`:

```text
operator machine
  mapping JSON + gost + vendor setup + v46lift
                    |
                    v
            minted installer
                    |
                    v
player machine (one elevation prompt)
  +-- install bundled lift + gost
  +-- run vendor client installer
  +-- replace client binary with baked launcher
                    |
                    v
player starts the game as usual
  +-- baked launcher starts gost
  +-- original client runs unprivileged
  +-- teardown when the client exits
```

The mapping is a trailer on the launcher binary, not a file the player points at.

## Process lifetime

A future Windows implementation should use Job Objects so launchers/updaters that spawn a second process do not cause premature teardown.

Linux should use a process group or systemd transient scope for equivalent process-tree semantics.
