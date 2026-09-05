-- Exercises the conn/handler/global state scopes; handler visits persist
-- across connections, global is shared process-wide.
function handle(conn)
	local visits = tonumber(handler:get("visits")) or 0
	visits = visits + 1
	handler:set("visits", tostring(visits))

	conn:set("mark", "conn")
	global:set("seen", "yes")

	conn:write(visits .. "|" .. tostring(conn:get("mark")) .. "|" .. tostring(global:get("seen")))
end
