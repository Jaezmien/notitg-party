<div align="center">
  
# NotITG Party

> forcefully shoving online multiplayer into a single player game

</div>

# Server

## Usage

```bash
# Requires Go 1.24.0+ 

$ go mod tidy
$ go run .
```
## Flags

| Name | Required | Default | Description |
| --- | --- | --- | --- |
| `port` | No | `8080` | Sets the server port |

# Client

## Usage

```bash
# Requires Go 1.24.0+ 

$ go mod tidy

# Scan your song folder
# In your first run, it will create an empty blacklist.ini for you before exiting
$ go run . --hash="Path to your Songs/ folder"

$ go run . --username="Your Username" --server="http://your.server:8080"
```

## Flags

| Name | Required | Default | Description |
| --- | --- | --- | --- |
| `deep` | No | `false` | Scans for NotITG by forcefully reading every program's memory |
| `pid` | No | `0` | Use a specific process |
| `verbose` | No | `false` | Enable debug messages |
| `hash` | Once | `""` | When provided with the directory to 'Songs/', will scan every song in the folder |
| `server` | Maybe | `https://localhost:8080` | The server to connect to |
| `username` | Yes | `""` | Your username |

## Notes

- If you are on Linux, you need to run the program with `sudo` priviledges.

# Theme

## Note

If your NotITG default theme folder isn't `simply-love-notitg`, please go to [metrics.ini](./metrics.ini) and change `Global.FallbackTheme` to your folder's name.

This theme contains only the necessary files, and falls back on the default theme for a majority of its content.
