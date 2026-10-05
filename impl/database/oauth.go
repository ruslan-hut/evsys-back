package database

import (
	"context"
	"evsys-back/entity"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// The OAuth collections belong to this service, not to evsys: they hold the
// MCP clients registered here and the tokens issued to them.

// EnsureOAuthIndexes creates the indexes the OAuth collections rely on. The
// TTL index lets MongoDB drop expired tokens on its own. Safe to call on every
// start: creating an existing index is a no-op.
func (m *MongoDB) EnsureOAuthIndexes(ctx context.Context) error {
	_, err := m.col(collectionOAuthClients).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "client_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("oauth clients index: %w", err)
	}
	_, err = m.col(collectionOAuthTokens).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "hash", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "grant_id", Value: 1}}},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	if err != nil {
		return fmt.Errorf("oauth tokens indexes: %w", err)
	}
	return nil
}

func (m *MongoDB) SaveOAuthClient(ctx context.Context, client *entity.OAuthClient) error {
	_, err := m.col(collectionOAuthClients).InsertOne(ctx, client)
	return err
}

// GetOAuthClient returns nil, nil when no client has the id.
func (m *MongoDB) GetOAuthClient(ctx context.Context, clientId string) (*entity.OAuthClient, error) {
	return findOne[entity.OAuthClient](m, ctx, collectionOAuthClients, bson.D{{Key: "client_id", Value: clientId}})
}

func (m *MongoDB) SaveOAuthToken(ctx context.Context, token *entity.OAuthToken) error {
	_, err := m.col(collectionOAuthTokens).InsertOne(ctx, token)
	return err
}

// GetOAuthToken returns nil, nil when no token has the hash.
func (m *MongoDB) GetOAuthToken(ctx context.Context, hash string) (*entity.OAuthToken, error) {
	return findOne[entity.OAuthToken](m, ctx, collectionOAuthTokens, bson.D{{Key: "hash", Value: hash}})
}

// DeleteOAuthToken removes a token and reports whether it was there. Callers
// use the answer to claim a refresh token: of two concurrent requests
// presenting the same one, only the request whose delete matched may proceed.
func (m *MongoDB) DeleteOAuthToken(ctx context.Context, hash string) (bool, error) {
	result, err := m.col(collectionOAuthTokens).DeleteOne(ctx, bson.D{{Key: "hash", Value: hash}})
	if err != nil {
		return false, err
	}
	return result.DeletedCount > 0, nil
}

// DeleteOAuthGrant removes every token issued from one consent.
func (m *MongoDB) DeleteOAuthGrant(ctx context.Context, grantId string) error {
	_, err := m.col(collectionOAuthTokens).DeleteMany(ctx, bson.D{{Key: "grant_id", Value: grantId}})
	return err
}
