# Rafal

Rafal is the Avagenc Google Workspace agent — Gmail, Calendar, and Contacts, treated as
one connected identity — built on the [Google ADK](https://pkg.go.dev/google.golang.org/adk).
It ships as a **pure Go module** — there is no standalone service or HTTP API. You embed
Rafal into your own runner/session lifecycle and supply an LLM plus a
[`gworkspace.Client`](https://pkg.go.dev/go.naturallyfunny.dev/gworkspace); the module
gives you a configured agent wired to the Gmail, Calendar, and Contacts tools.

```
go get go.avagenc.com/rafal
```

Rafal does not cover Drive, Docs, Sheets, or any other Workspace product — only Gmail,
Calendar, and Contacts, and it reasons across them as a single surface (e.g. it resolves a
person's email via Contacts before inviting them to an event or writing to them).

---

## Usage

`rafal.New` returns a configured `agent.Agent`. Running it — the runner and session
lifecycle — is the consumer's responsibility. Rafal's identity and system instruction are
owned by the module; you supply only its dependencies.

```go
a, err := rafal.New(rafal.Config{
    Model:           model,           // a google.golang.org/adk/model.LLM
    WorkspaceClient: workspaceClient, // a go.naturallyfunny.dev/gworkspace.Client
})
```

`workspaceClient` must carry the union of `gworkspace.CalendarRequiredScopes`,
`gworkspace.GmailRequiredScopes`, and `gworkspace.ContactsRequiredScopes` — one OAuth
refresh token spans all three APIs.

### Config

| Field | Type | Description |
|---|---|---|
| `Model` | `model.LLM` | The LLM backing the agent (e.g. Gemini). |
| `WorkspaceClient` | `*gworkspace.Client` | Google Workspace client; its Gmail/Calendar/Contacts tools are exposed to the agent. |
| `AdditionalInstruction` | `string` | Appended to Rafal's base system instruction (optional). |

The system instruction is embedded from `internal/instruction.txt`.

### Per-channel / per-run instruction

Rafal ships one fixed identity and system instruction. Anything channel- or run-specific
(tone, extra context) is the **consumer's** concern, added at the runner you own — the
module exposes no hook for it. The idiomatic ADK way is a `BeforeModelCallback` that
appends to the request's system instruction per run, registered as a plugin on your
runner:

```go
p, _ := plugin.New(plugin.Config{
    Name: "channel-instruction",
    BeforeModelCallback: func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
        // append channel-specific instruction to req for this run
        return nil, nil
    },
})

r, _ := runner.New(runner.Config{
    Agent:          a, // the rafal.New agent
    SessionService: sessionService,
    PluginConfig:   runner.PluginConfig{Plugins: []*plugin.Plugin{p}},
})
```

---

## Dev entry points

Three `cmd/` programs let you run Rafal locally without any database. Unlike Tuya (Zee's
domain), Google OAuth has no dev bypass — you connect a real Google account once via
`cmd/connect`, then `cmd/cli` and `cmd/web` reuse that one refresh token for every session
`user_id` via a `staticTokenStore`.

> [!WARNING]
> These are local dev tools with **no authentication**, and any `user_id` gets full
> Gmail/Calendar/Contacts access to the Google account behind `DEV_GOOGLE_REFRESH_TOKEN`.
> Do not expose them to the public internet.

Before running any of them, add `GOOGLE_REDIRECT_URL` (see below) as an authorized
redirect URI on your Google Cloud OAuth client — Google validates it exactly, including
port.

### `cmd/connect` — one-time OAuth bootstrap

Prints a consent URL, waits for the OAuth callback on `GOOGLE_REDIRECT_URL`, and prints
the resulting refresh token. Paste it into `.env` as `DEV_GOOGLE_REFRESH_TOKEN` before
running `cmd/cli` or `cmd/web`.

```
go run ./cmd/connect
```

### `cmd/web` — ADK web UI / API

Launches the ADK `web`, `webui`, and `api` servers (chat playground + REST API) on
`http://localhost:8080`.

```
go run ./cmd/web
```

### `cmd/cli` — ADK console (`adk run`)

Launches the ADK `console` launcher — the Go equivalent of Python's `adk run`: an
interactive terminal chat loop. Extra ADK console flags pass through.

```
go run ./cmd/cli
go run ./cmd/cli -streaming_mode=none
```

---

## Environment variables (dev entry points)

Copy `.env.example` to `.env` and fill these in. All are required by the dev entry points;
the module itself reads no environment variables.

| Variable | Description |
|---|---|
| `GEMINI_API_KEY` | Google Gemini API key (model is `gemini-2.5-flash`). |
| `GOOGLE_CLIENT_ID` | Google Cloud OAuth client ID. |
| `GOOGLE_CLIENT_SECRET` | Google Cloud OAuth client secret. |
| `GOOGLE_REDIRECT_URL` | OAuth redirect URI, must match Google Cloud Console exactly (e.g. `http://localhost:8765/callback`). |
| `DEV_GOOGLE_REFRESH_TOKEN` | Refresh token printed by `cmd/connect`; every dev session links to this one Google account. |
