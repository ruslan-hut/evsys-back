package firebase

import (
	"context"
	"evsys-back/internal/lib/sl"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"fmt"
	"google.golang.org/api/option"
	"log/slog"
)

type Firebase struct {
	app     *firebase.App
	client  *auth.Client
	context context.Context
	logger  *slog.Logger
}

func New(log *slog.Logger, key string) (*Firebase, error) {
	ctx := context.Background()
	sa := option.WithCredentialsFile(key)
	app, err := firebase.NewApp(ctx, nil, sa)
	if err != nil {
		return nil, fmt.Errorf("error initializing app: %v", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting auth client: %v\n", err)
	}
	return &Firebase{
		app:     app,
		client:  client,
		logger:  log.With(sl.Module("internal.firebase")),
		context: ctx,
	}, nil
}

func (f *Firebase) CheckToken(tokenId string) (string, error) {
	token, err := f.client.VerifyIDToken(f.context, tokenId)
	if err != nil {
		f.logger.Error("verifying token", err)
		return "", err
	}
	return token.UID, nil
}
