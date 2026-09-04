-- Reads HTTP-style headers and reports their byte length.
function handle(conn)
    local data = conn:read_until("\r\n\r\n")
    conn:write("len:" .. #data)
end
