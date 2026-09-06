-- Fake SMTP server for GoNetSim custom listeners. Accepts mail and captures
-- everything: envelope addresses, the message body and decoded AUTH
-- credentials. Replies are deferred by DELAY_MS to imitate a slow server
--
-- Limitations:
--   no STARTTLS (use tls = true for implicit-TLS 465)
--   minimal capability list
--
-- Handler state spans connections: a mail counter and per-sender memory,
-- demonstrating handler:get/set/has.
--
-- Config:
--   [[listeners]]
--   name = "smtp"
--   type = "tcp"
--   listen = ":25"
--   handler = "lua:handlers/smtp.lua"
--   tls = false

local DELAY_MS = 0
local DOMAIN = "gonetsim.invalid"

local B64_USERNAME_PROMPT = "VXNlcm5hbWU6" -- base64("Username:")
local B64_PASSWORD_PROMPT = "UGFzc3dvcmQ6" -- base64("Password:")

local b64index = {}
do
	local chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	for i = 1, #chars do
		b64index[chars:sub(i, i)] = i - 1
	end
end

local function base64decode(data)
	data = data:gsub("%s", "")
	local out, acc, bits = {}, 0, 0
	for i = 1, #data do
		local c = data:sub(i, i)
		if c ~= "=" then
			acc = acc * 64 + (b64index[c] or 0)
			bits = bits + 6
			if bits >= 8 then
				bits = bits - 8
				out[#out + 1] = string.char(math.floor(acc / 2 ^ bits) % 256)
				acc = acc % 2 ^ bits
			end
		end
	end
	return table.concat(out)
end

local function envelopeAddr(arg)
	return arg:match("<([^>]*)>") or arg
end

local NUL = string.char(0)

-- splits "a\0b\0c" into a table of fields
local function splitNul(s)
	local parts, start = {}, 1
	local i = s:find(NUL, 1, true)
	while i do
		parts[#parts + 1] = s:sub(start, i - 1)
		start = i + 1
		i = s:find(NUL, start, true)
	end
	parts[#parts + 1] = s:sub(start)
	return parts
end

local function doAuth(conn, arg)
	local mech = arg:match("^(%S+)")
	local payload

	if mech == "PLAIN" then
		local b64 = arg:match("^PLAIN%s+(%S+)")
		if not b64 then
			conn:write("334 \r\n")
			b64 = conn:read_line()
			if not b64 then return end
			b64 = b64:gsub("%s+$", "")
		end
		payload = base64decode(b64)
	elseif mech == "LOGIN" then
		conn:write("334 " .. B64_USERNAME_PROMPT .. "\r\n")
		local user = conn:read_line()
		if not user then return end
		conn:write("334 " .. B64_PASSWORD_PROMPT .. "\r\n")
		local pass = conn:read_line()
		if not pass then return end
		payload = base64decode(user) .. NUL .. base64decode(pass)
	else
		conn:write("504 5.5.4 Unsupported authentication mechanism\r\n")
		return
	end

	local parts = splitNul(payload)
	local user, pass = parts[1] or "?", parts[2] or "?"
	if mech == "PLAIN" then
		-- PLAIN payload is authzid NUL authcid NUL passwd
		user, pass = parts[2] or "?", parts[3] or "?"
	end
	capture:comment("auth: " .. user .. " / " .. pass)
	log:info("AUTH " .. mech .. " captured")
	conn:write("235 2.7.0 Authentication successful\r\n")
end

function handle(conn)
	local sender, recipients = nil, {}

	conn:write("220 " .. DOMAIN .. " ESMTP GoNetSim\r\n")

	while true do
		local line = conn:read_line()
		if not line then break end
		line = line:gsub("%s+$", "")

		if line ~= "" then
			capture:comment("smtp: " .. line)
		end

		local cmd = line:match("^(%a+)") or ""
		local arg = line:match("^%a+%s+(.+)$") or ""

		if cmd == "EHLO" then
			conn:write("250-" .. DOMAIN .. " greets " .. (arg ~= "" and arg or "client") .. "\r\n")
			conn:write("250-PIPELINING\r\n")
			conn:write("250-8BITMIME\r\n")
			conn:write("250-SIZE 10485760\r\n")
			conn:write("250 OK\r\n")
		elseif cmd == "HELO" then
			conn:write("250 " .. DOMAIN .. "\r\n")
		elseif cmd == "MAIL" then
			sender = envelopeAddr(arg)
			recipients = {}
			if handler:has("sender:" .. sender) then
				log:info("repeat sender: " .. sender)
			end
			handler:set("sender:" .. sender, "1")
			conn:sleep(DELAY_MS)
			conn:write("250 2.1.0 OK\r\n")
		elseif cmd == "RCPT" then
			recipients[#recipients + 1] = envelopeAddr(arg)
			conn:write("250 2.1.5 OK\r\n")
		elseif cmd == "DATA" then
			if not sender then
				conn:write("503 5.5.1 Error: need MAIL command\r\n")
			else
				conn:write("354 End data with <CR><LF>.<CR><LF>\r\n")
				local msg = conn:read_until("\r\n.\r\n")
				if not msg then break end
				msg = msg:gsub("\r\n%.\r\n$", "\r\n")  -- strip the terminator
				msg = msg:gsub("\r\n%.%.", "\r\n.")    -- un-dot-stuff
				capture:comment("message: " .. msg)
				local mails = tonumber(handler:get("mails")) or 0
				mails = mails + 1
				handler:set("mails", tostring(mails))
				log:info(("mail #%d from %s to %d recipient(s), %d bytes"):format(mails, sender, #recipients, #msg))
				conn:write("250 2.0.0 OK: queued\r\n")
			end
		elseif cmd == "AUTH" then
			doAuth(conn, arg)
		elseif cmd == "RSET" then
			sender, recipients = nil, {}
			conn:write("250 2.0.0 OK\r\n")
		elseif cmd == "NOOP" then
			conn:write("250 2.0.0 OK\r\n")
		elseif cmd == "VRFY" then
			conn:write("252 2.1.5 Cannot VRFY user\r\n")
		elseif cmd == "HELP" then
			conn:write("214-Commands: EHLO HELO MAIL RCPT DATA AUTH RSET NOOP VRFY HELP QUIT\r\n")
			conn:write("214 End of HELP info\r\n")
		elseif cmd == "QUIT" then
			conn:write("221 2.0.0 Bye\r\n")
			break
		else
			conn:write("500 5.5.2 Error: command not recognized\r\n")
		end
	end
end
