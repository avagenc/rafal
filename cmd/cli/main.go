package main

import (
	"context"
	"log"
	"os"

	_ "time/tzdata"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/genai"

	"go.naturallyfunny.dev/gworkspace"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/console"
	"google.golang.org/adk/cmd/launcher/universal"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/session"

	rafal "go.avagenc.com/rafal"
)

// staticTokenStore hands out a single fixed refresh token to any owner,
// obtained once via `go run ./cmd/connect`. No postgres needed for local dev.
type staticTokenStore struct {
	refreshToken string
}

func (s *staticTokenStore) GetRefreshToken(_ context.Context, _ string) (string, error) {
	if s.refreshToken == "" {
		return "", gworkspace.ErrNotConnected
	}
	return s.refreshToken, nil
}
func (s *staticTokenStore) SaveRefreshToken(_ context.Context, _, _ string) error { return nil }

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("FATAL: %s is required", key)
	}
	return v
}

func main() {
	ctx := context.Background()

	if err := godotenv.Load(); err != nil {
		log.Println("Info: .env not found, using system environment")
	}

	scopes := append(append(append([]string{},
		gworkspace.CalendarRequiredScopes...),
		gworkspace.GmailRequiredScopes...),
		gworkspace.ContactsRequiredScopes...)

	oauthCfg := &oauth2.Config{
		ClientID:     mustEnv("GOOGLE_CLIENT_ID"),
		ClientSecret: mustEnv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  mustEnv("GOOGLE_REDIRECT_URL"),
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
	}
	workspaceClient := gworkspace.NewClient(&staticTokenStore{
		refreshToken: mustEnv("DEV_GOOGLE_REFRESH_TOKEN"),
	}, oauthCfg)

	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: mustEnv("GEMINI_API_KEY"),
	})
	if err != nil {
		log.Fatalf("FATAL: gemini model: %v", err)
	}

	rafalAgent, err := rafal.New(rafal.Config{
		Model:           model,
		WorkspaceClient: workspaceClient,
	})
	if err != nil {
		log.Fatalf("FATAL: rafal agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(rafalAgent),
		SessionService: session.InMemoryService(),
	}

	// console is ADK's CLI run mode (equivalent to Python's `adk run`).
	// Prepend the "console" keyword so any extra flags (e.g. -streaming_mode)
	// are forwarded to it.
	l := universal.NewLauncher(console.NewLauncher())
	args := append([]string{"console"}, os.Args[1:]...)
	if err = l.Execute(ctx, config, args); err != nil {
		log.Fatalf("FATAL: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
