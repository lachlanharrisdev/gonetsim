# Quick start

GoNetSim runs a simulated network service that malware can phone home to. You
point a handler at a TCP or UDP port. Every connection is logged to the terminal
and written to a pcapng file that opens in Wireshark.

## Run a built-in handler

The echo and sink handlers need no script file.

```sh
gonetsim echo@:7777
gonetsim sink@:9999/udp
```

## Run a Lua handler

Handlers are plain Lua files. The `handlers/` directory in the repository holds
example scripts.

```sh
gonetsim handlers/irc.lua@:6667
```

## Run several listeners at once

Each positional argument is one listener. All of them share a single capture
file.

```sh
gonetsim handlers/irc.lua@:6667 sink@:9999/udp --pcap case.pcapng
```

## Read the capture

Stop the simulator with Ctrl-C. The terminal prints the file path and the packet
count. Open the `.pcapng` file in Wireshark. Comments a handler writes with
`capture:comment()` appear on their packets. If a listener uses `--tls`, add
`--tls-keylog` so the capture is decryptable.

## More documentation

```sh
gonetsim docs flags
gonetsim docs capture
gonetsim docs lua-api
```
