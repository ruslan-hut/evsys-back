package firebase

import (
	"context"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"fmt"
	"google.golang.org/api/option"
)

type Firebase struct {
	app     *firebase.App
	client  *auth.Client
	context context.Context
}

func New(key string) (*Firebase, error) {
	ctx := context.Background()
	sa := option.WithCredentialsFile(key)
	app, err := firebase.NewApp(ctx, nil, sa)
	if err != nil {
		return nil, fmt.Errorf("initializing app: %v", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting auth client: %v", err)
	}
	return &Firebase{
		app:     app,
		client:  client,
		context: ctx,
	}, nil
}

func (f *Firebase) CheckToken(tokenId string) (string, error) {
	token, err := f.client.VerifyIDToken(f.context, tokenId)
	if err != nil {
		return "", err
	}
	return token.UID, nil
}
