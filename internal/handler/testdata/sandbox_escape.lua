-- Fails to load: the sandbox removes io, so indexing it errors at load time.
local escape = io.write("sandbox escape attempt")

function handle(conn)
    conn:write("never runs")
end
