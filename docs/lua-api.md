# Lua handler API

A handler is a plain Lua 5.1 file. The file runs once at load time to define its
entry points. Each connection or datagram then runs in a fresh sandboxed state,
so one listener cannot leak state into another except through the stores below.

## Entry points

Define at least one of these two functions.

```lua
function handle(conn)                -- TCP: called once per connection
function handle_packet(data, peer)   -- UDP: return a reply, or nil for no reply
```

`handle` returns when the connection is finished. `handle_packet` returns the
bytes to send back, or nil to stay silent.

## TCP: the conn object

| Call | Meaning |
|---|---|
| `conn:read(n)` | read up to n bytes, or nil on a clean end of stream |
| `conn:read_line()` | read through the next newline |
| `conn:read_until(delim)` | read until delim |
| `conn:write(s)` | write bytes |
| `conn:sleep(ms)` | pause the handler; cancellable and capped at one hour |
| `conn:close()` | close the connection |
| `conn:remote()`, `conn:local()` | full address strings |
| `conn:remote_ip()`, `conn:remote_port()`, `conn:local_port()` | address parts |
| `conn:sni()` | the TLS Server Name Indication, or nil |
| `conn:tls()` | a table `{version=, cipher=}`, or nil |

Reads are capped at 1 MiB per call. `conn:get`, `set`, `has` and `delete` act on
the per-connection store.

## UDP: the peer argument

`handle_packet` receives `peer`, a table with `peer.addr`, `peer.ip` and
`peer.port`.

## State stores

A budgeted key/value store with `get`, `set`, `has` and `delete`, in three
scopes.

- `conn` holds values for one connection.
- `handler` is shared by every connection of one handler.
- `global` is shared by every listener in the process.

Keys are limited to 4 KiB and values to 1 MiB, with a 64 MiB total budget. `set`
returns false and an error message when a limit is reached.

## Logging and comments

```lua
log:info("got a line")         -- log:warn and log:error also exist
log:info("from", conn:local()) -- several arguments join with a space
capture:comment("flagged")     -- annotates the packet just read or written
```

`print` writes at info level, the same as `log:info`. A comment written before
any packet exists attaches to the next packet.

## The sandbox

Only the `base` (pruned), `string`, `table` and `math` libraries are available.
The filesystem (`io`, `os`, `dofile`, `loadfile`, `require`, `load`,
`loadstring`) and reflection (`debug`, `coroutine`, `rawget`, `getfenv`) are
removed. A handler cannot read files, run code it did not define, or reach the
host.

## A complete UDP handler

```lua
function handle_packet(data, peer)
    log:info("packet from", peer.ip, peer.port)
    if data:find("who") then
        return "gonetsim"
    end
    return nil
end
```

## Reserved namespaces

`http.respond` and `dns.answer` are reserved for future high-level HTTP and DNS
simulation. They currently raise an error. Protocol simulation today is written
directly against `conn`, as in `handlers/irc.lua`.
