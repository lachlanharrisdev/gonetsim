<div align="center">

  <img src=".github/logo_transparent.png" width="100" height="135"/>

  <h1 align="center">GoNetSim</h1>

  <p align="center" width="100">
    Go Network Simulator. A programmable network simulator for malware analysis that lets you simulate any network protocol with small, sandboxed, shareable Lua handlers.
    <a href="https://gonetsim.lachlanharris.au"><strong>Explore the docs »</strong></a>
    <br />
  </p>
  <p align="center" width="50">
    
  [![GitHub Repo stars](https://img.shields.io/github/stars/lachlanharrisdev/gonetsim?style=social)](https://github.com/lachlanharrisdev/gonetsim/stargazers)
  [![GitHub](https://img.shields.io/github/license/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim?tab=Apache-2.0-1-ov-file)
  [![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim/)<br/>
  [![GitHub CI Status](https://img.shields.io/github/actions/workflow/status/lachlanharrisdev/gonetsim/ci.yaml?branch=main&label=CI)](https://github.com/lachlanharrisdev/gonetsim/actions)
  [![GitHub Release Status](https://img.shields.io/github/v/release/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim/releases/latest)

  </p>
  <div align="center">
    <a href="https://github.com/lachlanharrisdev/gonetsim/blob/main/.github/CONTRIBUTING.md">Contribute</a>
    &middot;
    <a href="https://github.com/lachlanharrisdev/gonetsim/issues/new?template=bug_report.md">Report a Bug</a>
    &middot;
    <a href="https://github.com/lachlanharrisdev/gonetsim/issues/new?template=feature_request.md">Request a Feature</a>
  </div>
</div>

<br/>

## Usage

### Installation

Installation instructions can be found [here](https://gonetsim.lachlanharris.au/guides/installation).

### Quick Start

Running `gonetsim` starts all services enabled in the configuration file:

```sh
gonetsim
```

Individual services and listeners can be selected as targets, as preset names, listener names from config, or inline `handler@addr` listeners:

```sh
gonetsim run http               # just the HTTP service
gonetsim run http dns           # multiple presets
gonetsim run irc                # a named [[listeners]] entry from config
gonetsim run echo@:7777         # inline echo listener, no config needed
gonetsim run sink@:9999/udp     # inline UDP sink
gonetsim run c2.lua@:8080       # inline Lua handler from a local script
```

Targets named explicitly run regardless of their `enabled` setting in config. Common settings are also available as flags which override the config file:

```sh
gonetsim run http --listen 127.0.0.1:8080
gonetsim run c2.lua@:8080 --tls --no-capture
gonetsim run http -s http.mode=real -s http.root_dir=/srv/www
```

A more detailed usage guide can be found [here](https://gonetsim.lachlanharris.au/guides/usage).

<br/>

## Configuration

GoNetSim uses a TOML configuration file for most configuration, rather than forcing the memorisation of many flags.

On first run, if no config file is found, GoNetSim generates a default commented config file in `$XDG_CONFIG_HOME/gonetsim/config.toml` and uses it.

Default search locations:

- `./gonetsim.toml`
- `$XDG_CONFIG_HOME/gonetsim/config.toml` (usually `~/.config/gonetsim/config.toml`)
- `/etc/gonetsim/gonetsim.toml`

To use a specific config file:

```yaml
gonetsim --config /path/to/gonetsim.toml
```

For more information on configuration, please see the [configuration reference](https://gonetsim.lachlanharris.au/references/configuration)

<br/>

## Custom Listeners

Beyond the built-in services, GoNetSim can simulate arbitrary TCP/UDP protocols through custom listeners. Listeners can either use basic builtins or fully custom Lua scripts:

```toml
[[listeners]]
name = "irc"
type = "tcp"
listen = ":6667"
handler = "lua:handlers/irc.lua" 
capture = true
```

Run it with `gonetsim run irc`, or skip using a pre-defined config entirely with `gonetsim run lua:handlers/irc.lua@:6667`.

The [`examples/`](examples/) directory has a full sample config plus example IRC and FTP handlers.

<br/>

## Captures

Every run saves everything it handles to a single pcapng file, typically `~/.local/share/gonetsim/runs/<run-id>.pcapng`. GoNetSim prints the path on startup and a packet count on shutdown. Lua handlers can annotate interesting packets with `capture:comment("...")`, which shows up as a packet comment in Wireshark.

```sh
gonetsim run http --output ./case.pcapng   # choose the capture location
gonetsim pcap ./case.pcapng                # summarize a capture
gonetsim check                             # also verifies captures can be written
```

Two things to know when reading captures: handshakes are synthesized (sequence numbers start at 0, Ethernet MACs are fake, timestamps mark when GoNetSim wrote the frame), and TLS services capture ciphertext, not plaintext.

<br/>

## Docker

A lightweight distroless container setup lives in `docker/` and is built/published with `ko`. This is the recommended installation method if you require long periods of uptime, or if your system is incompatible with the provided binaries.

For a full reference guide please see the [Docker guide](https://gonetsim.lachlanharris.au/guides/docker)

<br/>

## Contributing

GoNetSim follows most standard conventions for contributing, and accepts any contributions from documentation improvements, bug triage / fixes, small features or any updates for [issues in the backlog](https://github.com/lachlanharrisdev/gonetsim/issues?q=is%3Aissue). For more information on contributing please see [CONTRIBUTING.md](https://github.com/lachlanharrisdev/gonetsim/blob/main/.github/CONTRIBUTING.md) and [AI_USAGE.md](https://github.com/lachlanharrisdev/gonetsim/blob/main/.github/AI_USAGE.md)

### Codespaces

GoNetSim has full support for Github Codespaces. These are recommended for small changes or devices with no access to a development environment. You can use the buttons below to open the repository in a web-based editor and get started.

[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/lachlanharrisdev/gonetsim?quickstart=1)

### Dev Containers

We also have full support for Dev Containers. These provide a reproducible development environment that automatically isolates the project and installs the officially supported toolchain. 

Clicking the below button will open up VS Code on your local machine, clone this repository and open it automatically inside a development container.

[![Open in Dev Containers](https://img.shields.io/badge/Open%20In%20Dev%20Container-0078D4?style=for-the-badge&logo=visual%20studio%20code&logoColor=white)](https://vscode.dev/redirect?url=vscode://ms-vscode-remote.remote-containers/cloneInVolume?url=https://github.com/lachlanharrisdev/gonetsim)

### Local Development

For local development, please refer to [CONTRIBUTING.md](https://github.com/lachlanharrisdev/gonetsim/blob/main/.github/CONTRIBUTING.md). Again, we follow most conventions so local development involves the standard flow of `fork-PR-merge`.

<br/>

---

<br/>

> This project is licensed under the Apache 2.0 License. Please see [LICENSE](https://github.com/lachlanharrisdev/gonetsim?tab=Apache-2.0-1-ov-file) for more info.
>
> Copyright (c) 2026 Lachlan Harris. All Rights Reserved.
