-- Proves the sandbox: io and os are unavailable at runtime.
function handle(conn)
    conn:write("io=" .. tostring(io) .. " os=" .. tostring(os) .. " require=" .. tostring(require))
end
