function handle(conn)
    conn:write("Welcome to GoNetSim\r\n")

    while true do
        local line = conn:read_line()
        if not line then break end -- client closed the connection
        capture:write("greet", line)
        conn:write("You said: " .. line)
    end
end
