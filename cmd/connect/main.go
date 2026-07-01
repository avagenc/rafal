// cmd/connect is a one-off local helper to obtain a Google OAuth refresh
// token for the dev entry points (cmd/cli, cmd/web). Unlike Zee's fake Tuya
// account-linking, Google OAuth has no dev bypass — this runs the real
// consent flow once via the loopback redirect pattern and prints the
// resulting refresh token to paste into .env as DEV_GOOGLE_REFRESH_TOKEN.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"go.naturallyfunny.dev/gworkspace"
)

// noopTokenStore satisfies gworkspace.TokenStore for a client that will only
// ever call Exchange, never Connect — so no token is ever actually stored.
type noopTokenStore struct{}

func (noopTokenStore) GetRefreshToken(context.Context, string) (string, error) {
	return "", gworkspace.ErrNotConnected
}
func (noopTokenStore) SaveRefreshToken(context.Context, string, string) error { return nil }

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("FATAL: %s is required", key)
	}
	return v
}

func randomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("FATAL: generate state: %v", err)
	}
	return hex.EncodeToString(b)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Info: .env not found, using system environment")
	}

	redirectURL := mustEnv("GOOGLE_REDIRECT_URL")
	u, err := url.Parse(redirectURL)
	if err != nil {
		log.Fatalf("FATAL: invalid GOOGLE_REDIRECT_URL: %v", err)
	}

	scopes := append(append(append([]string{},
		gworkspace.CalendarRequiredScopes...),
		gworkspace.GmailRequiredScopes...),
		gworkspace.ContactsRequiredScopes...)

	cfg := &oauth2.Config{
		ClientID:     mustEnv("GOOGLE_CLIENT_ID"),
		ClientSecret: mustEnv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
	}
	client := gworkspace.NewClient(noopTokenStore{}, cfg)
	state := randomState()

	done := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc(u.Path, func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		if got := r.URL.Query().Get("state"); got != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			log.Printf("FATAL: state mismatch: got %q want %q", got, state)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			log.Println("FATAL: callback carried no code")
			return
		}

		refreshToken, err := client.Exchange(r.Context(), code)
		if err != nil {
			http.Error(w, "exchange failed", http.StatusInternalServerError)
			log.Printf("FATAL: exchange: %v", err)
			return
		}

		fmt.Fprint(w, "Connected. You can close this tab.")
		fmt.Println()
		fmt.Println("Paste this into rafal's .env as DEV_GOOGLE_REFRESH_TOKEN:")
		fmt.Println()
		fmt.Println(refreshToken)
		fmt.Println()
	})

	srv := &http.Server{Addr: u.Host, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("FATAL: local server: %v", err)
		}
	}()

	fmt.Println("Open this URL in a browser and grant access:")
	fmt.Println()
	fmt.Println(client.AuthURL(state))
	fmt.Println()

	<-done
	_ = srv.Shutdown(context.Background())
}
