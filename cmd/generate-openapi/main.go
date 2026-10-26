// Generates the OpenAPI document for the Cirrus server.
//
// Run it with the Makefile:
//
//	make openapi
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/hmcalister/cirrus/internal/api"
	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/internal/filestore"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
	"github.com/hmcalister/stratus/pkg/auth"
	"github.com/hmcalister/stratus/pkg/sessionstore"
)

// A user store that is never called while building the spec.
type noOpUserStore struct{}

func (noOpUserStore) GetOrCreateUser(_ context.Context, email string) (database.UserAccount, error) {
	return database.UserAccount{Email: email, Active: true}, nil
}

func (noOpUserStore) SetActive(context.Context, database.SetActiveParams) error {
	return nil
}

func (noOpUserStore) DeleteUser(context.Context, string) error {
	return nil
}

func (noOpUserStore) CountUsers(context.Context) (int64, error) {
	return 0, nil
}

func (noOpUserStore) CountActiveUsers(context.Context) (int64, error) {
	return 0, nil
}

func main() {
	outPath := flag.String("o", "", "output file (default stdout)")
	flag.Parse()

	authService, err := auth.New(auth.AuthServiceConfig{
		Sender:     skysender.NoOpSender{},
		Sessions:   sessionstore.NewMemorySessionStore(),
		BaseURL:    "https://example.com",
		SuccessURL: "/account",
		Cookie: auth.CookieConfig{
			Name:   api.SessionCookieName(true),
			Secure: true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	humaAPI := humago.New(mux, api.NewHumaConfig())
	authService.RegisterHumaHandlers(humaAPI)

	server, err := api.NewServer(mux, humaAPI, api.ServerConfig{
		AuthService:       authService,
		Users:             noOpUserStore{},
		Files:             filestore.NoOpStore{},
		SessionCookieName: api.SessionCookieName(true),
	})
	if err != nil {
		log.Fatal(err)
	}

	spec, err := server.OpenAPI().YAML()
	if err != nil {
		log.Fatal(err)
	}

	if *outPath != "" {
		if err := os.WriteFile(*outPath, spec, 0o644); err != nil {
			log.Fatal(err)
		}
		return
	}

	if _, err := fmt.Fprint(os.Stdout, string(spec)); err != nil {
		log.Fatal(err)
	}
}
