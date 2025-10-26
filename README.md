# Song Battle CLI

A terminal-based application for ranking your Spotify songs using the Elo rating system through 1v1 song battles.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Features

- **Elo-based ranking** - Adaptive rating system (K-factor: 32→24→16)
- **Spotify integration** - OAuth2 PKCE authentication, playback control
- **Smart matchmaking** - Balanced pairing based on Elo scores (±100 range)
- **Auto-import** - Fetch your top tracks automatically on first launch
- **Leaderboard view** - Browse and play ranked songs
- **Playlist export** - Create Spotify playlists from top-ranked tracks
- **Cross-platform** - Linux, macOS, Windows support

## Quick Start

```bash
# Install
go install github.com/leukalm/songbattle-cli/cmd/song-battle@latest

# Or build from source
git clone https://github.com/leukalm/songbattle-cli.git
cd songbattle-cli
go build ./cmd/song-battle

# Run (imports tracks on first launch)
./song-battle
```

## Prerequisites

- **Spotify Premium** account (required for playback)
- **Spotify Developer App** - Create at [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard)
  - Set Redirect URI: `http://127.0.0.1:8080/callback`
  - Enable scopes: `user-read-playback-state`, `user-modify-playback-state`, `user-top-read`, `playlist-modify-private`

## Usage

### First Launch

The app auto-imports your top tracks on first run. Provide your Spotify Client ID:

```bash
# Via flag
./song-battle -client-id=YOUR_CLIENT_ID

# Via environment variable
export SPOTIFY_CLIENT_ID=YOUR_CLIENT_ID
./song-battle
```

The Client ID is saved locally after first authentication.

### Controls

| Key | Action |
|-----|--------|
| `←` `→` | Select track |
| `Enter` | Vote for selected track |
| `Space` | Play selected track |
| `C` | View leaderboard |
| `S` | Skip battle |
| `G` | Open in Spotify |
| `Q` | Quit |

## Configuration

### CLI Options

```bash
song-battle [OPTIONS]

  -client-id string      Spotify Client ID
  -db-path string        Database path (default: ~/.songbattle/songbattle.db)
  -import                Force reimport of Spotify data
  -redirect-uri string   Custom OAuth redirect URI
  -version               Show version
  -help                  Show help
```

### Environment Variables

```bash
SPOTIFY_CLIENT_ID       # Your Spotify app Client ID
SONGBATTLE_DEBUG        # Enable debug logging (true/false)
```

## Architecture

```
cmd/song-battle/        # Main entry point
internal/
├── auth/               # OAuth2 PKCE flow
├── elo/                # Elo rating algorithm
├── export/             # Playlist export
├── matchmaker/         # Battle pairing logic
├── models/             # Data structures
├── spotify/            # Spotify API client
├── store/              # SQLite persistence
└── ui/                 # Bubble Tea TUI
```

## Elo System

- **Initial rating**: 1200
- **K-factor**: Adaptive based on battle count
  - New tracks (<10 battles): K=32
  - Medium (<30 battles): K=24
  - Experienced (≥30 battles): K=16

Formula:
```
Expected_A = 1 / (1 + 10^((Elo_B - Elo_A) / 400))
Elo_A_new = Elo_A + K × (Actual_score - Expected_A)
```

## Matchmaking

- 85% balanced matches (Elo difference ≤100)
- 15% exploration matches (include underplayed tracks)
- Avoids recent opponents

## Build from Source

```bash
# All platforms
./build-releases.sh 1.0.0

# Specific platform
GOOS=linux GOARCH=amd64 go build -o songbattle ./cmd/song-battle
```

## Development

```bash
# Dependencies
go mod download

# Build
go build ./cmd/song-battle

# Run tests (when available)
go test ./...

# Format code
go fmt ./...
```

## Troubleshooting

### Authentication Issues

**"Invalid redirect URI"**
- Ensure `http://127.0.0.1:8080/callback` is set in your Spotify app settings
- Note: Use `127.0.0.1`, not `localhost` (Spotify requirement)

**"Client ID required"**
```bash
# Set via environment variable
export SPOTIFY_CLIENT_ID=your_client_id
./song-battle
```

### Playback Issues

**"No active device found"**
- Open Spotify desktop/mobile app
- Start playing any track to activate device
- Retry playback in Song Battle

**"Premium required"**
- Spotify Premium is mandatory for playback control via API

### Data Issues

**"No tracks available"**
```bash
# Force reimport
./song-battle -import

# Check Spotify app scopes include user-top-read
```

**Debug mode**
```bash
export SONGBATTLE_DEBUG=true
./song-battle
```

## Tech Stack

- **Language**: Go 1.22+
- **TUI**: [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Database**: SQLite ([modernc.org/sqlite](https://modernc.org/sqlite))
- **API**: [Spotify Web API](https://developer.spotify.com/documentation/web-api)
- **Auth**: OAuth2 with PKCE

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Credits

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Spotify Web API](https://developer.spotify.com/) - Music data and playback
