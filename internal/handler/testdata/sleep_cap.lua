-- Sleeps far beyond the cap; the call must fail.
function handle(conn)
    conn:read(1024)
    conn:sleep(3600001)
end
