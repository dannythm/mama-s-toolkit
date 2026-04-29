# Mama's Toolkit

A powerful, standalone save editor and content management suite.

Mama's Toolkit is designed to be a high-performance, zero-dependency companion to the `lunar-tear` project, providing an intuitive web interface for managing player data, inventory, and server-side content scheduling.

## Features

-   **⚡ High Performance Backend**: Native Go implementation replacing legacy Python scripts for sub-millisecond API responses.
-   **🛠️ SQLite Save Editor**: 
    -   Dynamic schema discovery: works with any version of the `game.db`.
    -   Intelligent Cascades: Automatically handles related entries when deleting or updating characters, costumes, and weapons.
    -   User-scoped views: Filter every table by the selected User ID with one click.
-   **📅 Content Manager (Schedule Editor)**:
    -   Visual Monthly Picker: Enable or disable content by month/year.
    -   Binary Patcher: Directly patches `database.bin.e` via a specialized Go engine.
    -   Lunar-tear Integration: Automatically pings the game server to reload data after patching.
-   **🔍 Engels Lookup Resolver**: Integrated labels and descriptions for over 67,000 game entities (items, costumes, skills, etc.) using `Engels` output.
-   **🎭 Deck & Character Visualizer**: Real-time rendering of player decks, including character art, weapons, companions, and sub-weapons.
-   **✨ Premium UI**: Fully dark-themed, responsive interface with custom-styled controls and high-contrast tables.

## Prerequisites

-   **Go 1.21+** (for building)
-   **Lunar-tear Environment**: 
    -   Game database: `../lunar-tear/server/db/game.db`
    -   Pristine master data: `../lunar-tear/server/assets/release/20240404193219.bin.e`
    -   Engels Data: `../Engels/example-output/`

## Quick Start

### 1. Build the binary
```powershell
go build -o mama-toolkit.exe ./cmd/mama-toolkit
```

### 2. Run the server
```powershell
./mama-toolkit.exe --port 8084
```

### 3. Access the UI
Open `http://localhost:8084` in your browser.

## Configuration

Mama's Toolkit can be configured via command-line flags or environment variables:

| Flag | Env Var | Default | Description |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8084` | Web server port |
| `--db` | `DB_PATH` | `../lunar-tear/server/db/game.db` | Path to game.db |
| `--data` | `DATA_DIR` | `../lunar-tear/server` | Path to lunar-tear assets |
| `--engels` | `ENGELS_DIR` | `../Engels/example-output` | Path to Engels output |
| `--admin-token` | `LUNAR_ADMIN_TOKEN` | - | Token for reloading lunar-tear |

## Architecture

-   **Backend**: Written in Go.
    -   `internal/editor`: Database logic and schema management.
    -   `internal/lookup`: Entity name resolution via JSON mappings.
    -   `internal/patcher`: High-speed binary patching engine.
    -   `cmd/mama-toolkit`: HTTP handlers and server initialization.
-   **Frontend**: Vanilla HTML5, CSS3, and JavaScript.
    -   No framework required: Extremely fast load times and zero build steps for the UI.
    -   Custom Design System: Tailored for the game aesthetic.

## License

MIT - See [LICENSE](LICENSE) for details.
