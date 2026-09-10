# Capture

Every run writes a single pcapng file. GoNetSim synthesises Ethernet, IP and TCP
or UDP frames from the bytes your handler reads and writes. Nothing is sniffed
from a real interface. The file opens in Wireshark, tshark, or any pcapng 
reader.

## Capture Storage

Without `--pcap`, the capture is written to `./<id>.pcapng` in the current
directory, where `<id>` is a timestamp. Pass `--pcap path` to choose the
location, or `--no-pcap` to write no capture. The terminal prints the path and 
the packet count when you stop the simulator.

## Annotating packets

A handler can attach a note to the packet it just read or wrote.

```lua
capture:comment("C2 beacon key exchange")
```

The note appears on that packet in Wireshark. Comments longer than 256 bytes are
truncated.

## Decrypting TLS

TLS listeners are captured as ciphertext. To make the capture readable:

1. Save the session keys and point Wireshark at them.

```sh
gonetsim handlers/irc.lua@:6667 --tls --tls-keylog keys.log
```

2. In Wireshark, open Edit, Preferences, Protocols, TLS and set 
(Pre)-Master-Secret log filename to `keys.log`, then reopen the capture

`keys.log` is the standard SSLKEYLOGFILE format written by Go's TLS library.
Without `--tls-keylog`, Wireshark cannot decrypt a TLS capture and GoNetSim
will print a warning at startup

## Troubleshooting

If the capture still shows garbled application data after you set the keylog
file, check these before you reopen:

- The keylog file is not empty and holds four secrets per TLS 1.3 session:
  `CLIENT_HANDSHAKE_TRAFFIC_SECRET`, `SERVER_HANDSHAKE_TRAFFIC_SECRET`,
  `CLIENT_TRAFFIC_SECRET_0` and `SERVER_TRAFFIC_SECRET_0`.
- Wireshark 3.2 or newer can decrypt TLS 1.3 captures. Older versions only
  understand the TLS 1.2 `CLIENT_RANDOM` line.
- The (Pre)-Master-Secret log filename preference is applied and the capture is
  reopened after you set it. Wireshark reads the keylog when it sees the
  session, so loading the file after opening the capture is not enough.