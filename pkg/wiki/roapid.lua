-- 0.0.14
-- https://github.com/paradoxum-wikis/RobloxAPID
local roapid = {}

local function getByPath(tbl, parts)
    local cur = tbl
    for i = 1, #parts do
        if type(cur) ~= "table" then return nil end
        local key = parts[i]
        local nextValue = cur[key]
        if nextValue == nil then
            local numeric = tonumber(key)
            if numeric then
                nextValue = cur[numeric]
            end
        end
        if nextValue == nil then
            return nil
        end
        cur = nextValue
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

local function stringify(value)
    if type(value) == "table" then
        local ok, json = pcall(mw.text.jsonEncode, value)
        if ok then
            return json
        end
        return mw.text.jsonEncode(value)
    end
    if value == nil then
        return ""
    end
    return tostring(value)
end

local function buildPathError(resource, id, path)
    local segments = {}
    if resource and resource ~= "" then
        if id and id ~= "" then
            segments[#segments + 1] = string.format("%s/%s", resource, id)
        else
            segments[#segments + 1] = resource
        end
    end
    if #path > 0 then
        segments[#segments + 1] = table.concat(path, " → ")
    end
    local moduleName = buildTitle(resource, id or "")
    return string.format(
        "Field path not found (%s), [[%s|see fields]].",
        table.concat(segments, " | "),
        moduleName
    )
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
        return stringify(data)
    end

    local value = getByPath(data, path)
    if value == nil then
        return buildPathError(resource, id, path)
    end

    return stringify(value)
end

local function makeGetter(resource, needsId)
    return function(frame)
        return roapid._get(frame, resource, needsId)
    end
end

roapid.badges = makeGetter("badges", true)
roapid.users = makeGetter("users", true)
roapid.groups = makeGetter("groups", true)
roapid.universes = makeGetter("universes", true)
roapid.places = makeGetter("places", true)
roapid.games = makeGetter("games", true)
roapid.about = makeGetter("about", false)

return roapid
