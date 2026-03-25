# ask-postgres

ask-postgres is a small terminal app that sits between you and your PostgreSQL database. You connect it to a database you already have, type questions in normal language, and it reads your data and answers in plain English—without drowning you in database jargon. It never writes to your database: lookups are read-only, with sensible limits so you don’t accidentally pull huge result sets.

It’s a comfortable fit when you’re exploring an unfamiliar schema, sanity-checking counts, or wanting a quick, human-sounding read on what the data suggests—all from the terminal.

---

## What you need

- Go 1.24 or newer, if you’re building from source  
- A Postgres connection string for the database you want to ask about (your own server, a cloud instance, or the optional Docker demo below)  
- An API key for at least one LLM provider—OpenAI, Anthropic, or Google—matching whichever model you use in the app

---

## Setup (step by step)

### 1. Get the code

```bash
git clone <your-repo-url>
cd ask-postgres   # or whatever you named the folder
```

### 2. Configure environment

Copy the example env file and fill in what applies to you:

```bash
cp .env.example .env
```

Edit `.env`:

- Set one provider key: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GOOGLE_API_KEY`.
- Set `DATABASE_URL` to your Postgres URL, or skip it and pass `--db` when you run (see below).
- Optional: set `MODEL` to your default model id; you can also change this inside the app with `/model`.

The app loads `.env` automatically when you start it.

### 3. (Optional) Run the bundled demo database

If you don’t have Postgres handy, Docker can start a tiny demo with sample data:

```bash
docker compose up -d
```

By default Postgres listens on port 5000. If that port is busy:

```bash
export ASK_POSTGRES_PG_PORT=5001
docker compose up -d
```

Point the app at it (adjust host or port if you changed them):

```bash
export DATABASE_URL="postgres://askpostgres:askpostgres@localhost:${ASK_POSTGRES_PG_PORT:-5000}/analytics?sslmode=disable"
```

### 4. Run the app

From the project directory:

```bash
go run .
```

Or build a binary once:

```bash
go build -o ask-postgres .
./ask-postgres
```

If `DATABASE_URL` isn’t set, pass the database explicitly:

```bash
go run . --db "postgres://user:pass@localhost:5432/mydb?sslmode=disable"
```

Optional flags:

| Flag | What it does |
|------|----------------|
| `--db` | Postgres URL (overrides `DATABASE_URL`) |
| `--model` | LLM model name |
| `--max-rows` | Max rows per data query (default 10; raise if you need bigger samples) |
| `--query-timeout` | How long each SQL tool call may run |
| `--openai-base-url` | Use a local OpenAI-compatible API (e.g. Ollama, LM Studio) |

### 5. First time in the TUI

After it launches:

- Type a question and press Enter.
- Run `/help` for commands (sessions, themes, model picker, API keys under `/settings`, and more).
- Run `/model` to pick OpenAI, Claude, or Gemini models from the curated list, depending on which key you configured.

---

## Local-first: what lives where

This project is oriented around keeping your workflow on your machine.

- Chats and sessions are stored as JSON under your home directory, usually `~/.ask-postgres/sessions/`. There is no separate “ask-postgres cloud” that stores your conversations; only the LLM provider you choose receives the prompts it needs to respond.
- Preferences (model, theme, keys you save in `/settings`) live beside that on disk, in `config.json` in the same app data folder.
- Your database is whatever you point at—often localhost or a host you reach over VPN—using a normal Postgres URL. This app does not host your data.
- The LLM is the main component that may use the public internet (OpenAI, Anthropic, Google). If you want that step local too, use `--openai-base-url` with a local OpenAI-compatible server and a matching model name.

To put all app data somewhere else:

```bash
export ASK_POSTGRES_HOME="/custom/path"
```

---

## Behaviour you should know

- Read-only: queries run in a read-only transaction. The assistant is guided not to attempt writes or schema changes.
- Small samples: each data query is capped (default 10 rows) so answers stay manageable. For more rows, e.g. `go run . --max-rows 200`.
- Providers: OpenAI-style, Anthropic (Claude), and Google (Gemini) models are supported via `/model` and the env keys above.

---

## Requirements reference

- Go 1.24.4 or newer (see `go.mod`)
- Network access to your LLM provider, unless you use a local OpenAI-compatible endpoint

If startup fails, the terminal usually says why: for example missing `DATABASE_URL` / `--db`, or a missing API key for the model you selected.
