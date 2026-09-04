-- Sleeps between two reads, to verify sleep timing and that the idle
-- deadline is reset across the sleep.
function handle(conn)
    conn:read(1024)
    conn:sleep(200)
    local data = conn:read(1024)
    if not data then return end
    conn:write("after-sleep")
end
