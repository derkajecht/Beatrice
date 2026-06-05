# Go rewrite

**Going to rewrite this in Go as a challenge to myself and to learn the language.**

Frameworks to use in place of the current Python implementation;

- **TUI**
    - https://github.com/charmbracelet/bubbletea
    - https://github.com/charmbracelet/lipgloss
    - https://github.com/charmbracelet/bubbles

- **NETWORKING**
    - https://github.com/gorilla/websocket

- **CRYPTO**
    - https://github.com/golang/crypto -> Built in cryptography library

- **DATABASE**
    - https://github.com/cznic/sqlite -> SQLite in pure go
    - https://github.com/jmoiron/sqlx -> SQL extensions for database/sql

- **UTILS**
    - https://github.com/spf13/cobra -> CLI framework for handling --host and --port args
    - https://github.com/spf13/viper -> Configuration management

┌────────────┬───────────────────┬─────────────────────────┐
│ Component  │ Python (Current)  │ Go (Recommended)        │
├────────────┼───────────────────┼─────────────────────────┤
│ TUI        │ textual           │ bubbletea + lipgloss    │
│ Async      │ asyncio           │ goroutines + channels   │
│ Web Server │ FastAPI / Uvicorn │ net/http or Gin         │
│ WebSockets │ websockets        │ gorilla/websocket       │
│ Crypto     │ cryptography      │ crypto/* (Standard Lib) │
│ DB Driver  │ aiosqlite         │ modernc.org/sqlite      │
│ CLI Flags  │ argparse          │ cobra                   │
└────────────┴───────────────────┴─────────────────────────┘

## In-Progress/TODO
- Start writing the logic to handle the type of packets received
- Started writing some tests for funcs, mostly just to understand how that would work in go.
