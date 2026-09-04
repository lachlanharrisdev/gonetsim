-- Replies with the client's TLS SNI, or "no-sni" on plain connections.
function handle(conn)
    local sni = conn:sni()
    if sni then
        conn:write("sni:" .. sni)
    else
        conn:write("no-sni")
    end
end
