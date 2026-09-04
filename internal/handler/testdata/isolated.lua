-- Errors on "boom", serves "ok" otherwise. Used to verify per-connection
-- error isolation.
function handle(conn)
    local line = conn:read_line()
    if line == "boom\n" then
        error("intentional test failure")
    end
    conn:write("ok\n")
end
