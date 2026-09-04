-- Minimal fake IRC server for GoNetSim custom listeners
--
-- Config:
--   [[listeners]]
--   name = "irc"
--   type = "tcp"
--   listen = ":6667"
--   handler = "lua:handlers/irc.lua"
--
-- Try it:
--   irssi -c 127.0.0.1 -p 6667

function handle(conn)
    local nick = "guest"

    conn:write(":gonetsim 001 " .. nick .. " :Welcome to GoNetSim IRC\r\n")
    conn:write(":gonetsim 002 " .. nick .. " :Your host is gonetsim\r\n")

    while true do
        local line = conn:read_line()
        if not line then break end
        line = line:gsub("%s+$", "")

        if line ~= "" then
            capture:write("irc", line)
            log:info(line)

            local cmd = line:match("^(%S+)")
            if cmd == "NICK" then
                nick = line:match("^NICK%s+(%S+)") or nick
                conn:write(":gonetsim 001 " .. nick .. " :Nickname set\r\n")
            elseif cmd == "USER" then
                conn:write(":gonetsim 001 " .. nick .. " :Welcome\r\n")
            elseif cmd == "PING" then
                conn:write(":gonetsim PONG gonetsim :" .. (line:match("^PING%s+(.+)$") or "gonetsim") .. "\r\n")
            elseif cmd == "JOIN" then
                conn:write(":gonetsim 001 " .. nick .. " :Joined\r\n")
            elseif cmd == "PRIVMSG" then
                conn:write(":gonetsim 001 " .. nick .. " :Message received\r\n")
            elseif cmd == "QUIT" then
                conn:write("ERROR :Closing link\r\n")
                break
            end
        end
    end
end
