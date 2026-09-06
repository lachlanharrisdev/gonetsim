-- Minimal fake FTP server for GoNetSim custom listeners
--
-- Config:
--   [[listeners]]
--   name = "ftp"
--   type = "tcp"
--   listen = ":21"
--   handler = "lua:handlers/ftp.lua"
--
-- Try it:
--   curl ftp://127.0.0.1:21/ --user guest:guest

function handle(conn)
    local user = nil
    local logged_in = false

    conn:write("220 GoNetSim FTP Server\r\n")

    while true do
        local line = conn:read_line()
        if not line then break end
        line = line:gsub("%s+$", "")

        if line ~= "" then
            capture:comment("ftp: " .. line)

            local cmd = line:match("^(%S+)")
            local arg = line:match("^%S+%s+(.+)$")

            if cmd == "USER" then
                user = arg
                conn:write("331 Password required\r\n")
            elseif cmd == "PASS" then
                if user then
                    logged_in = true
                    conn:write("230 Login successful\r\n")
                else
                    conn:write("503 Login with USER first\r\n")
                end
            elseif cmd == "SYST" then
                conn:write("215 UNIX Type: L8\r\n")
            elseif cmd == "PWD" then
                conn:write("257 \"/\" is the current directory\r\n")
            elseif cmd == "TYPE" then
                conn:write("200 Type set\r\n")
            elseif cmd == "LIST" then
                conn:write("150 Here comes the directory listing\r\n")
                conn:write("total 0\r\n")
                conn:write("226 Transfer complete\r\n")
            elseif cmd == "RETR" then
                if logged_in then
                    conn:write("550 File not found\r\n")
                else
                    conn:write("530 Please login with USER and PASS\r\n")
                end
            elseif cmd == "QUIT" then
                conn:write("221 Goodbye\r\n")
                break
            else
                conn:write("500 Command not recognized\r\n")
            end
        end
    end
end
