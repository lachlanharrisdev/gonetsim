# TLS

GoNetSim needs no network. The binary embeds its documentation and can generate
its own certificates, so a static binary plus your handler files is everything an
isolated lab needs.

Samples will attempt to hamper dynamic analysis by checking how "real" a server
seems, and a common way to check this through TLS is by validating the 
server certificate. 

A persistent certificate lets you trust the CA on the analyst host once and have
every later run present an identical server.

```sh
gonetsim tls
gonetsim handlers/irc.lua@:6667 --tls \
  --tls-cert ./tls/gonetsim-cert.pem --tls-key ./tls/gonetsim-key.pem
```

The first command writes `gonetsim-cert.pem`, `gonetsim-key.pem` and
`gonetsim-ca.pem` under `./tls/`. Trust `gonetsim-ca.pem` once; the certificate
is signed by it.

- `gonetsim tls --force` regenerates the pair.
- `gonetsim tls --verify-only` checks the existing files without changing them.

## Reading TLS traffic

TLS is captured as ciphertext, exactly as it would appear on a real network,
while the handler still sees plaintext. To read the traffic in Wireshark, add
`--tls-keylog ./keys.log` and set the TLS (Pre)-Master-Secret log filename to
that file. See `gonetsim docs capture`.

## Moving between machines

Copy the static binary, the `handlers/` directory, and `./tls/`. On Windows,
`gonetsim.exe` behaves the same, including capture and key-log file paths.
