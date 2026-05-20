package database_mock

import (
	"context"
	"testing"

	"evsys-back/entity"
)

func TestGetWarningEmailRecipients(t *testing.T) {
	ctx := context.Background()
	db := NewMockDB()

	users := []*entity.User{
		{Username: "a", WarningEmailsEnabled: true, WarningEmail: "a@example.com"},
		{Username: "b", WarningEmailsEnabled: false, WarningEmail: "b@example.com"},
		{Username: "c", WarningEmailsEnabled: true, WarningEmail: ""},
	}
	for _, u := range users {
		if err := db.AddUser(ctx, u); err != nil {
			t.Fatalf("AddUser: %v", err)
		}
	}

	got, err := db.GetWarningEmailRecipients(ctx)
	if err != nil {
		t.Fatalf("GetWarningEmailRecipients: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d recipients, want 2", len(got))
	}
	for _, u := range got {
		if !u.WarningEmailsEnabled {
			t.Errorf("recipient %q has the flag disabled", u.Username)
		}
	}
}
