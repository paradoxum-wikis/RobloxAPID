# RobloxAPID
**RobloxAPID**, also known as **roapid**, is a lightweight Go daemon that bridges the Roblox API to Fandom (and other MediaWiki) wikis.

<p align="center"><img src="https://bin.t7ru.link/fol/roapid.gif" alt="RobloxAPID in action"></p>

As the name suggests, it continuously runs in the background, monitoring updates from Roblox's **Open Cloud** and **Legacy** APIs, then automatically mirrors the latest data to your wiki. This allows readers to see up-to-date Roblox information across both **FandomDesktop** and **FandomMobile** skins.

RobloxAPID requires an account with [**bot userrights**](https://community.fandom.com/wiki/Help:Bots), meaning you will need to have the flag enabled.

> [!TIP]
> **Don't want to self-host?** RobloxAPID is also available as a **free opt-in service** for reputable Fandom wikis. Simply contact **t7ru on Discord** to request access!

<table>
  <tr>
    <th colspan="3" align="left">Wikis using RobloxAPID</th>
  </tr>
  <tr>
    <td align="center">
      <a href="https://alterego.wiki/">
        <img src="https://static.wikia.nocookie.net/alter-ego/images/e/e6/Site-logo.png/revision/latest" width="200">
      </a>
    </td>
    <td align="center">
      <a href="https://tds.wiki/">
        <img src="https://static.wikia.nocookie.net/tower-defense-sim/images/e/e6/Site-logo.png/revision/latest" width="220">
      </a>
    </td>
    <td align="center">
      <a href="https://rex-reincarnated.fandom.com/">
        <img src="https://static.wikia.nocookie.net/rex-3/images/e/e6/Site-logo.png/revision/latest" width="100">
      </a>
    </td>
  </tr>
  <tr>
    <td align="center">ALTER EGO Wiki</td>
    <td align="center">Tower Defense Simulator Wiki</td>
    <td align="center">REx: Reincarnated Wiki</td>
  </tr>
</table>

## Why use it?
- Lightweight: a single Go binary with very efficient resource usage.
- Low-maintenance: configurable intervals and automated refreshes.
- Fast: periodically writes JSON pages that are consumed by a tiny Lua module (`Module:Roapid`) so editors can embed data with `{{#invoke:roapid|...}}`.
- Instant: as data are cached natively on the wiki, pulling data are instant.
- Reliable updates: detects data changes and purges caches after updates so pages stay fresh.
- Fandom-friendly: works on both FandomDesktop and FandomMobile skins.
- Open source: audit and extend it on GitHub.

...and of course, you don't need to host it at all if you choose to opt-in with us!

## Currently supported endpoints
- **Open Cloud**
  - Users
  - Groups
  - Universes
  - Places
- **Legacy**
  - Badges
  - Games

## Installation

1. **Prerequisites**:
   - Go 1.22.3 or later.
   - A server, obviously.

2. **Clone the Repository**:
   ```bash
   git clone https://github.com/paradoxum-wikis/RobloxAPID.git
   cd RobloxAPID
   ```

3. **Build**:
   ```bash
   go build -o robloxapid .
   ```

## Configuration

### config.json
Main configuration file:
```json
{
    "server": {
        "categoryCheckInterval": "1m",
        "dataRefreshInterval": "30m"
    },
    "wiki": {
        "apiUrl": "https://your-wiki.com/api.php",
        "username": "YourWikiUsername@YourBotName",
        "password": "your_bot_password_here",
        "namespace": "Module"
    },
    "dynamicEndpoints": {
        "categoryPrefix": "robloxapid-queue",
        "apiMap": {
            "badges": "https://badges.roblox.com/v1/badges/%s",
            "users": "https://apis.roblox.com/cloud/v2/users/%s",
            "groups": "https://apis.roblox.com/cloud/v2/groups/%s",
            "universes": "https://apis.roblox.com/cloud/v2/universes/%s",
            "places": "https://apis.roblox.com/cloud/v2/%s"
        },
        "refreshIntervals": {
            "badges": "30m",
            "about": "168h",
            "users": "1h",
            "groups": "1h",
            "universes": "1h",
            "places": "1h",
            "games": "1h"
        }
    }
}
```
- `categoryCheckInterval`: How often to check for new categories (this is how we know what to fetch).
- `dataRefreshInterval`: Default refresh interval for endpoints.
- `apiMap`: Maps endpoint types to API URLs (use `%s` for ID placeholder).
- `refreshIntervals`: Endpoint refresh intervals (overrides default).

### about.json
Static about information, if you're hosting publicly, do not change it:
```json
{
	"description": "A daemon that bridges the Roblox API to Fandom wikis.",
	"license": "GNU Affero General Public License v3.0",
	"name": "RobloxAPID",
	"source": "https://github.com/paradoxum-wikis/RobloxAPID"
}
```

### badges.json / users.json / groups.json / universes.json / places.json / games.json
Static usage guides for the API endpoints. Each file documents relevant information such as usage, description, fields, and examples.

They all sync to `Module:roapid/<endpoint>.json` so editors can surface instructions directly on the wiki.

## Usage

1. **Run Locally**:
   ```bash
   ./robloxapid
   ```
   The daemon will start and vomit out the logs for you to debug and whatnot.

2. **On the Wiki**:
   - The Lua module `Module:Roapid` is automatically set up.
   - Use invokes to access data:
     - `{{#invoke:roapid|badges|123456}}`: Gets the description field for badge ID 123456.
   - When you're accessing an ID that isn't mirrored yet, wait for the daemon to fetch it and it will be up in less than a minute.
   - The page will have missing data for a while, but that is intentional.
   - We also recommend making a template wrapper to abstract the invokes.

## License

[AGPLv3](LICENSE)
