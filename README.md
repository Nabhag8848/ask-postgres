# pgwatch-copilot (TUI)

TUI-first Postgres “analysis agent” in Go.

## Run

Option A: use a local `.env` file (recommended for dev):

```bash
cp .env.example .env
# edit .env
```

Option B: export env vars:

```bash
export OPENAI_API_KEY="..."
export DATABASE_URL="postgres://user:pass@localhost:5432/dbname?sslmode=disable"
```

## Demo database (Docker)

Bring up a local Postgres on `localhost:5000` with a seeded `analytics` database:

```bash
docker compose up -d
```

If port `5000` is already taken, pick another port:

```bash
export PGWATCH_PG_PORT=5001
docker compose up -d
```

Connection string:

```bash
export DATABASE_URL="postgres://pgwatch:pgwatch@localhost:${PGWATCH_PG_PORT:-5000}/analytics?sslmode=disable"
```

Run the TUI:

```bash
go run .
```

Or override the DB on the command line:

```bash
go run . --db "postgres://user:pass@localhost:5432/dbname?sslmode=disable"
```

## Notes

- Default mode is **read-only**: the agent is only allowed to run safe SQL via tools.
- This is a scaffold: add more tools (vacuum/analyze advice, index health, slow queries) as you iterate.
- Multi-provider model support is built in (OpenAI, Anthropic Claude, Google Gemini).
- Use `/model` to open the model picker, or `/model <model-id>` to set the model if that id is in the curated picker list.

## Sessions (local)

Sessions are stored locally under:

- `~/.pgwatch-copilot/sessions/<sessionId>.json`

Override the base directory:

```bash
export PGWATCH_COPILOT_HOME="/custom/path"
```

