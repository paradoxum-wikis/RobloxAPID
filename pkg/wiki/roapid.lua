-- 0.0.9
-- https://github.com/paradoxum-wikis/RobloxAPID
local roapid = {}
local function get_by_path(tbl, parts)
    local cur = tbl
    for i = 1, #parts do
        if type(cur) ~= "table" then
            return nil
        end
        cur = cur[parts[i]]
    end
    return cur
end

local function build_title(resource, id)
    if id == "" then
        return string.format("Module:roapid/%s.json", resource)
    else
        return string.format("Module:roapid/%s-%s.json", resource, id)
    end
end

local function get_queue_category(resource, id)
    if resource == "badges" and id ~= "" then
        return string.format("[[Category:robloxapid-queue-badges-%s]]", id)
    end
    if resource == "users" and id ~= "" then
        return string.format("[[Category:robloxapid-queue-users-%s]]", id)
    end
    return ""
end

function roapid._get(frame, resource, needs_id)
    local args = frame.args
    local id = needs_id and (args[1] or "") or ""
    local path = {}
    local start_idx = needs_id and 2 or 1
    local i = start_idx
    while args[tostring(i)] do
        local v = args[tostring(i)]
        if v and v ~= "" then
            path[#path + 1] = v
        end
        i = i + 1
    end

    if needs_id and id == "" then
        return ""
    end

    local module_name = build_title(resource, id)
    local ok, data = pcall(mw.loadJsonData, module_name)

    if not ok or type(data) ~= "table" then
        return get_queue_category(resource, id)
    end

    if #path == 0 then
        local json_ok, json = pcall(mw.text.jsonEncode, data)
        return json_ok and json or mw.text.jsonEncode(data)
    end

    local value = get_by_path(data, path)
    if value == nil then
        return ""
    end

    if type(value) == "table" then
        local json_ok, json = pcall(mw.text.jsonEncode, value)
        return json_ok and json or mw.text.jsonEncode(value)
    else
        return tostring(value)
    end
end

function roapid.badges(frame)
    local id = frame.args[1]
    if not id or id == "" then
        return roapid._get(frame, "badges", false)
    end
    return roapid._get(frame, "badges", true)
end

function roapid.users(frame)
    local id = frame.args[1]
    -- serve root Module:roapid/users.json when no id provided
    if not id or id == "" then
        return roapid._get(frame, "users", false)
    end
    return roapid._get(frame, "users", true)
end

function roapid.about(frame)
    return roapid._get(frame, "about", false)
end

return roapid