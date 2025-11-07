-- 0.0.11
-- https://github.com/paradoxum-wikis/RobloxAPID
local roapid = {}

local function getByPath(tbl, parts)
    local cur = tbl
    for i = 1, #parts do
        if type(cur) ~= "table" then return nil end
        cur = cur[parts[i]]
    end
    return cur
end

local function buildTitle(resource, id)
    return (id == "" or not id)
        and string.format("Module:roapid/%s.json", resource)
         or string.format("Module:roapid/%s-%s.json", resource, id)
end

local function getQueueCategory(resource, id)
    if id and id ~= "" then
        return string.format("[[Category:robloxapid-queue-%s-%s]]", resource, id)
    end
    return ""
end

local function getQueueNotice(resource, id)
    local cat = getQueueCategory(resource, id)
    local note = "Publish this page and wait at least a minute for data to be fetched."
    return cat ~= "" and (cat .. note) or note
end

function roapid._get(frame, resource, needsId)
    local args = frame.args
    local id = needsId and (args[1] or "") or ""

    local path = {}
    local i = needsId and 2 or 1
    while args[tostring(i)] do
        local v = args[tostring(i)]
        if v and v ~= "" then path[#path + 1] = v end
        i = i + 1
    end

    if needsId and id == "" then
        return ""
    end

    local moduleName = buildTitle(resource, id)
    local ok, data = pcall(mw.loadJsonData, moduleName)
    if not ok or type(data) ~= "table" then
        return getQueueNotice(resource, id)
    end

    if #path == 0 then
        local okJson, json = pcall(mw.text.jsonEncode, data)
        return okJson and json or mw.text.jsonEncode(data)
    end

    local value = getByPath(data, path)
    if value == nil then
        return getQueueNotice(resource, id)
    end

    if type(value) == "table" then
        local okJson, json = pcall(mw.text.jsonEncode, value)
        return okJson and json or mw.text.jsonEncode(value)
    else
        return tostring(value)
    end
end

function roapid.badges(frame)
    local id = frame.args[1]
    return roapid._get(frame, "badges", id and id ~= "")
end

function roapid.users(frame)
    local id = frame.args[1]
    return roapid._get(frame, "users", id and id ~= "")
end

function roapid.groups(frame)
    local id = frame.args[1]
    return roapid._get(frame, "groups", id and id ~= "")
end

function roapid.universes(frame)
    local id = frame.args[1]
    return roapid._get(frame, "universes", id and id ~= "")
end

function roapid.places(frame)
    local id = frame.args[1]
    return roapid._get(frame, "places", id and id ~= "")
end

function roapid.about(frame)
    return roapid._get(frame, "about", false)
end

return roapid
