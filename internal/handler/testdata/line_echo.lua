-- Echo every line back with a prefix; exit cleanly on EOF.
function handle(conn)
    while true do
        local line = conn:read_line()
        if not line then break end
        conn:write("echo: " .. line)
    end
end
