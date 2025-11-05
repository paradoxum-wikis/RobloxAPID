# RobloxAPID

Also known as **roapid**, is a daemon that bridges the Roblox API to Fandom wikis (well technically it works everywhere using MediaWiki).

As its name suggests, RobloxAPID runs in the background, monitors updates from the Roblox API, and mirrors them back to the wiki. This allows readers to see up-to-date Roblox data on both desktop and mobile Fandom skins.

RobloxAPID requires an account with [bot userrights](https://community.fandom.com/wiki/Help:Bots) to be used, meaning you will need to have the flag enabled.

*RobloxAPID is also available as an opt-in service at no cost. All that's required is that your wiki is considered reputable. You can DM t7ru on Discord to opt in.*

| Wikis using RobloxAPID | |
| :------------------: | :--------------------: |
| <a href="https://alterego.wiki/"><img src="https://static.wikia.nocookie.net/alter-ego/images/e/e6/Site-logo.png/revision/latest" width="200"></a><br>ALTER EGO Wiki | <a href="https://rex-reincarnated.fandom.com/"><img src="https://static.wikia.nocookie.net/rex-3/images/e/e6/Site-logo.png/revision/latest" width="100"></a><br>REx: Reincarnated Wiki |

## Why use it?
- Lightweight: a single Go binary with very efficient resource usage.
- Low-maintenance: configurable intervals and automated refreshes.
- Fast: periodically writes JSON pages that are consumed by a tiny Lua module (`Module:Roapid`) so editors can embed data with `{{#invoke:roapid|...}}`.
- Instant: as data are cached natively on the wiki, pulling data are instant.
- Reliable updates: detects data changes and purges caches after updates so pages stay fresh.
- Fandom-friendly: works on both FandomDesktop and FandomMobile skins.
- Open source: audit and extend it on GitHub.

...and of course, you don't need to host it at all if you choose to opt-in with us!

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
            "badges": "https://badges.roblox.com/v1/badges/%s"
        },
        "refreshIntervals": {
            "badges": "30m",
            "about": "168h"
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

### badges.json
Static badges usage guide:
```json
{
  "description": "Roblox badges API integration yippee!",
  "usage": {
    "specific_badge": "{{#invoke:roapid|badges|badge_id}} - Replace 'badge_id' with the numeric Roblox badge ID to get full badge data.",
    "index": "{{#invoke:roapid|badges}} - Returns this usage guide.",
    "subfields": "{{#invoke:roapid|badges|badge_id|field}} - Access a specific field (e.g., 'description', 'statistics').",
    "nested": "{{#invoke:roapid|badges|badge_id|field|subfield}} - Drill deeper, dehehe (e.g., 'statistics|awardedCount')."
  },
  "examples": [
    "{{#invoke:roapid|badges|3964419828587997}}",
    "{{#invoke:roapid|badges|3964419828587997|description}}",
    "{{#invoke:roapid|badges|3964419828587997|statistics|awardedCount}}"
  ]
}
```

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