-- Read exactly 4 bytes and report them.
function handle(conn)
    local data = conn:read(4)
    conn:write("got:" .. data)
end
