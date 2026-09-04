-- Reply to a ping, otherwise stay silent.
function handle_packet(data)
    if data == "ping" then
        return "pong"
    end
    return nil
end
