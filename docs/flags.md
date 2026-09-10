# Flags

GoNetSim has no config file. Every option is a positional target or a flag.

## Targets

Each positional argument is `handler@addr[/tcp|/udp]`. TCP is the default
transport. The script path is resolved relative to the current directory.

| Target | Meaning |
|---|---|
| `echo@:7777` | built-in echo on TCP port 7777 |
| `sink@:9999/udp` | built-in sink on UDP port 9999 |
| `handlers/irc.lua@:6667` | the Lua file `handlers/irc.lua` on TCP port 6667 |
| `lua:handlers/irc.lua@:6667` | the same handler, written with an explicit scheme |

## Listener flags

| Flag | Meaning |
|---|---|
| `--listen addr` | override the listen address; requires exactly one target |
| `--timeout t` | connection idle timeout, default 30s; 0 disables it. Accepts seconds (`5`) or a duration (`5s`, `1m30s`) |
| `--tls` | wrap TCP listeners in TLS |
| `--tls-cert f` | TLS certificate file; used with `--tls` |
| `--tls-key f` | TLS key file; used with `--tls` |

## Capture flags

| Flag | Meaning |
|---|---|
| `--pcap f` | write the capture to f; default `./<id>.pcapng` |
| `--no-pcap` | do not write a capture |
| `--tls-keylog f` | write TLS session keys to f in SSLKEYLOGFILE format so Wireshark can decrypt the capture; requires `--tls` |

## Logging flags

| Flag | Meaning |
|---|---|
| `--log-level l` | one of debug, info, warn, error; default info |
| `--log-format f` | `text` (default) or `json`. Human output never uses key=value fields. json emits `{time, level, msg}` lines |

## Subcommands

| Command | Meaning |
|---|---|
| `gonetsim tls` | generate or verify a persisted certificate pair and CA |
| `gonetsim docs [topic]` | print built-in documentation; works offline |
| `gonetsim version` | print version information |
