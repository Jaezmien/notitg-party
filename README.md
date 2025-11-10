<div align="center">
  
# NotITG Party

<a href="#">
  <img src="https://img.shields.io/badge/version-alpha-orange">  
</a>

> forcefully shoving online multiplayer into a single player game

</div>

Detailed instructions [here!](https://github.com/Jaezmien/notitg-party/wiki/Instructions)

> [!warning]
> 1. **This code is alpha, do not be surprised if things break!**
> 2. **This code is not yet suitable for public instances!**

# Server

## Usage

```bash
# Requires Go 1.24.0+ 

$ cd server
$ go mod tidy
$ go run .
```
## Flags

| Name | Required | Default | Description |
| --- | --- | --- | --- |
| `port` | No | `8080` | Sets the server port |
| `verbose` | No | `false` | Enable debug messages |
| `version` | No | `false` | Print version and exit |

# Client

## Usage

```bash
# Requires Go 1.24.0+ 

$ cd client
$ go mod tidy

# Scan your song folder
# In your first run, it will create an empty blacklist.ini for you before exiting
$ go run . --hash="Path to your Songs/ folder"

$ go run . --username="Your Username" --server="http://your.server:8080"
```

> [!note]
> If you are on Linux, you need to run the program with `sudo` priviledges.

## Flags

| Name | Required | Default | Description |
| --- | --- | --- | --- |
| `deep` | No | `false` | Scans for NotITG by forcefully reading every program's memory |
| `pid` | No | `0` | Use a specific process |
| `verbose` | No | `false` | Enable debug messages |
| `hash` | Once | `""` | When provided with the directory to 'Songs/', will scan every song in the folder |
| `server` | Maybe | `http://localhost:8080` | The server to connect to |
| `username` | Yes | `""` | Your username |
| `version` | No | `false` | Print version and exit |

# Theme

Install [theme/](./theme) into your `Themes/` folder as is. Feel free to rename the folder. (e.g. The path should now look like `Themes/simply-party`)

> [!note]
> If your NotITG default theme folder isn't `simply-love-notitg`, please go to Party's [metrics.ini](./theme/metrics.ini) and change `Global.FallbackTheme` to your folder's name.
> 
> This theme contains only the necessary files, and falls back on the default theme for a majority of its content.

# Testers
- UltraAmeise
