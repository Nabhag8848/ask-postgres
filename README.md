# ask-postgres

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev/) [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?style=flat&logo=postgresql&logoColor=white)](https://www.postgresql.org/) [![Bubble Tea](https://img.shields.io/badge/Bubble%20Tea-TUI-FF67C7?style=flat)](https://github.com/charmbracelet/bubbletea) [![pgx](https://img.shields.io/badge/pgx-v5-003B57?style=flat&logo=postgresql&logoColor=white)](https://github.com/jackc/pgx) [![langchaingo](https://img.shields.io/badge/langchaingo-agents-7C3AED?style=flat)](https://github.com/tmc/langchaingo) [![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker&logoColor=white)](https://docs.docker.com/compose/)

![Screen recording of the ask-postgres TUI](assets/ask-postgres.gif)

---

**Ask questions about your own PostgreSQL database in plain English—without leaving the terminal.**

ask-postgres is a small, focused CLI: you point it at a database you already run or own, type what you want to know in natural language, and get answers you can act on. Behind the scenes an LLM proposes read-only SQL, runs it through your connection, and explains results in normal language. No separate web app, no hosted “data warehouse” product—just your shell, your Postgres URL, and the model you choose.

**Good moments for it:** onboarding to a new schema, sanity-checking row counts or distributions, drafting the SQL you’d later put in a report, or getting a quick narrative read on what a table actually contains—without hand-writing every query first.

---

## What you get

| | |
| --- | --- |
| **Your database** | Connects only to the Postgres URL you provide (local, VPN, cloud—your choice). |
| **Natural language in, answers out** | Describe intent; the agent explores the schema and data with tools, then replies in prose. |
| **Read-only by design** | Queries run in a read-only transaction; the assistant is steered away from writes and DDL. |
| **Sensible sampling** | Each data pull is row-capped by default so answers stay small and fast (tune with `--max-rows`). |
| **Real TUI** | Bubble Tea interface: sessions, themes, `/model` picker, `/settings` for keys, scrollable transcript for long replies. |
| **Several LLM backends** | OpenAI-style, Anthropic (Claude), and Google (Gemini)—pick in-app or via env. |

---

## Local-first: your machine, your data

ask-postgres is built so **your workflow stays on your hardware** unless you explicitly use a cloud model.

- **Sessions and chat history** are stored as JSON under your home directory (typically `~/.ask-postgres/sessions/`). There is **no ask-postgres cloud** and no vendor-hosted copy of your conversations.
- **Your Postgres** is whatever host and database you configure; this project does not host or mirror your data.
- **Configuration** (default model, theme, API keys you save in `/settings`) lives next to that data in `config.json` in the same app directory.
- **The LLM** is the main component that may call the public internet when you use OpenAI, Anthropic, or Google. Want the whole stack local? Point the app at a **local OpenAI-compatible server** with `--openai-base-url` (e.g. Ollama, LM Studio) and a matching model name.

To relocate all app data:

```bash
export ASK_POSTGRES_HOME="/custom/path"
```

---

## Requirements

- **Go 1.24.4+** (see `go.mod`) if you build from source  
- A **PostgreSQL connection string** for the database you want to query  
- An **API key** for at least one provider—**OpenAI**, **Anthropic**, or **Google**—matching the model you select (unless you use a fully local OpenAI-compatible endpoint)

---

## Setup

### 1. Clone the repository

```bash
git clone <your-repo-url>
cd ask-postgres
```

### 2. Environment variables

Copy the example file and edit values for your setup:

```bash
cp .env.example .env
```

In `.env`:

- Set **one** provider key: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GOOGLE_API_KEY`.
- Set `DATABASE_URL` to your Postgres URL, *or* omit it and pass `--db` when launching (see below).
- Optionally set `MODEL`; you can always change the model inside the app with `/model`.

The app loads `.env` automatically on startup.

### 3. (Optional) Demo database with Docker

If you don’t have Postgres ready, Compose can start a small sample database:

```bash
docker compose up -d
```

Default port is **5000**. If it’s taken:

```bash
export ASK_POSTGRES_PG_PORT=5001
docker compose up -d
```

Then point the app at it:

```bash
export DATABASE_URL="postgres://askpostgres:askpostgres@localhost:${ASK_POSTGRES_PG_PORT:-5000}/analytics?sslmode=disable"
```

### 4. Run

From the project root:

```bash
go run .
```

Or build once and run the binary:

```bash
go build -o ask-postgres .
./ask-postgres
```

If `DATABASE_URL` is not set:

```bash
go run . --db "postgres://user:pass@localhost:5432/mydb?sslmode=disable"
```

**Useful flags**

| Flag | Purpose |
| --- | --- |
| `--db` | Postgres URL (overrides `DATABASE_URL`) |
| `--model` | Default LLM id |
| `--max-rows` | Max rows per data query (default `10`) |
| `--query-timeout` | Timeout for each SQL tool invocation |
| `--openai-base-url` | Local or custom OpenAI-compatible API base URL |

---

## Using the TUI

- **Welcome screen** appears when the transcript is empty; it’s UI-only and **not** sent to the model as chat.
- **Layout:** transcript → **input** (directly under the conversation) → **status** line (hints, streaming state, model).
- **Long answers:** scroll the transcript with **PgUp** / **PgDn** or the **mouse wheel** where your terminal supports it. **↑** / **↓** navigate **input history**, not the chat log.
- **?** opens the shortcuts sheet.
- **Enter** sends your question.
- **`/help`** lists slash commands (sessions, themes, `/model`, `/settings`, and more).
- **`/model`** opens the curated model list for the providers you’ve configured.
- **Ctrl+L** clears the on-screen transcript. **`/clear`** also clears persisted session history and the assistant’s in-memory context for this chat; use **`/help`** for session-related commands.

---

## Behaviour and trust

- **Read-only:** database work runs read-only; the assistant is instructed not to perform writes or schema changes.
- **Row caps:** default sample size keeps responses tight; increase when you need more context, e.g. `go run . --max-rows 200`.
- **Providers:** OpenAI-compatible, Anthropic, and Google models are supported via `/model` and the env keys above.
- **Transcript:** older messages stay reachable when replies are long; if you’ve scrolled up, streaming usually won’t snap the view to the bottom unless you were already following the end.

---

## Troubleshooting

Startup errors are usually explicit: missing `DATABASE_URL` / `--db`, a missing API key for the selected model, or an unreachable database URL. Check the message in the terminal header and fix env or flags accordingly.
