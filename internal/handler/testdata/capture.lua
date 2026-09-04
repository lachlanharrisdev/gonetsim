-- Captures whatever the client sends under a named section.
function handle(conn)
    local data = conn:read(1024)
    capture:write("section", data)
    log:info("captured " .. #data .. " bytes")
end
