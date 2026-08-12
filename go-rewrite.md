# Go rewrite

**Going to rewrite this in Go as a challenge to myself and to learn the language.**

Frameworks to use in place of the current Python implementation;

- **TUI**
    - https://github.com/charmbracelet/bubbletea (v1, tea.NewProgram)
    - https://github.com/charmbracelet/lipgloss (v1)
    - https://github.com/charmbracelet/bubbles (v2, key/viewport helpers)

- **NETWORKING**
    - https://github.com/coder/websocket (wsjson for JSON frames, InactivityTimeout, SetReadLimit)

- **CRYPTO**
    - stdlib `crypto/hpke` (KEM MLKEM768X25519 + HKDFSHA512 + AES256GCM)

- **DATABASE**
    - https://modernc.org/sqlite (pure-Go sqlite, `database/sql` driver)
    - plain `database/sql` + SQL strings (no sqlx)

- **CLI**
    - stdlib `flag` (no cobra/viper)

┌────────────┬───────────────────┬────────────────────────────┐
│ Component  │ Python (Current)  │ Go (Current)               │
├────────────┼───────────────────┼────────────────────────────┤
│ TUI        │ textual           │ bubbletea + lipgloss       │
│ Async      │ asyncio           │ goroutines + channels      │
│ Web Server │ FastAPI / Uvicorn │ net/http                   │
│ WebSockets │ websockets        │ coder/websocket            │
│ Crypto     │ cryptography      │ crypto/hpke (stdlib)       │
│ DB Driver  │ aiosqlite         │ modernc.org/sqlite         │
│ CLI Flags  │ argparse          │ stdlib flag                │
└────────────┴───────────────────┴────────────────────────────┘

## In-Progress/TODO
- Start writing the logic to handle the type of packets received
- Started writing some tests for funcs, mostly just to understand how that would work in go.
