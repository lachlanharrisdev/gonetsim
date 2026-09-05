-- Reply to a ping, otherwise stay silent.
function handle_packet(data, peer)
    if data == "ping" then
        log:info("ping from " .. peer.addr)
        return "pong"
    end
    return nil
end
