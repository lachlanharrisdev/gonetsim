# Writing a handler

A handler is a plain Lua file that speaks the server side of a protocol. It runs
sandboxed, so it cannot read files or touch the host. This page walks through a
complete TCP handler. For the full API and the sandbox rules, see
`gonetsim docs lua-api`.

## Pick an entry point

Define `handle(conn)` for a TCP listener. The function is called once per
connection and returns when the connection is done.

Define `handle_packet(data, peer)` for a UDP listener. It returns the reply
string, or nil to send nothing.

## Read, write, and log

The example below opens with a banner, reads lines until the client hangs up,
and logs every line it received.

```lua
function handle(conn)
    conn:write("220 example ready\r\n")
    while true do
        local line = conn:read_line()
        if line == nil then
            break
        end
        log:info("client said", line)
        conn:write("250 ok\r\n")
    end
end
```

`conn:read_line` returns nil on a clean end of stream, which ends the loop. Send
bytes with `conn:write`, or end a connection early with `conn:close`.

## Keep state

Handlers remember values through the `conn`, `handler`, and `global` stores. The
`handler` store is shared by every connection of one listener, so it fits
counters.

```lua
local count = tonumber(handler:get("count") or "0") + 1
handler:set("count", tostring(count))
conn:write("you are visitor " .. count .. "\r\n")
```

## Annotate the capture

Mark an interesting packet with `capture:comment`. The note appears on that
packet in Wireshark.

```lua
capture:comment("client finished the handshake")
```

## Test the handler

Put the file in the current directory and run it on a port.

```sh
gonetsim example.lua@:12345
```

Connect with a terminal client.

```sh
nc 127.0.0.1 12345
```

Send a line and read the reply. Stop the simulator with Ctrl-C and open the
capture in Wireshark. The terminal prints the capture path and packet count.