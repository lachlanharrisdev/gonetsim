-- comment on the next frame the client sends
function handle(conn)
    local data = conn:read(1024)
    capture:comment("client sent " .. #data .. " bytes")
end