package mcpserver

import (
	"context"
	"evsys-back/entity"
	"sort"
	"strings"
)

type searchInput struct {
	Search string `json:"search,omitempty" jsonschema:"case-insensitive substring of username, name, email, RFID tag or note"`
}

func containsFold(search string, fields ...string) bool {
	if search == "" {
		return true
	}
	search = strings.ToLower(search)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), search) {
			return true
		}
	}
	return false
}

func (t *tools) listUsers(ctx context.Context, user *entity.User, in searchInput) (any, error) {
	users, err := t.core.GetUsers(ctx, user)
	if err != nil {
		return nil, err
	}
	search := strings.TrimSpace(in.Search)
	views := make([]userView, 0, len(users))
	for _, u := range users {
		if u == nil || !containsFold(search, u.Username, u.Name, u.Email) {
			continue
		}
		views = append(views, userFromEntity(u))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Username < views[j].Username })
	return map[string]any{"count": len(views), "users": views}, nil
}

func (t *tools) listUserTags(ctx context.Context, user *entity.User, in searchInput) (any, error) {
	tags, err := t.core.ListUserTags(ctx, user)
	if err != nil {
		return nil, err
	}
	search := strings.TrimSpace(in.Search)
	views := make([]userTagView, 0, len(tags))
	for _, tag := range tags {
		if tag == nil || !containsFold(search, tag.IdTag, tag.Username, tag.Note) {
			continue
		}
		views = append(views, userTagFrom(tag))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].IdTag < views[j].IdTag })
	return map[string]any{"count": len(views), "user_tags": views}, nil
}

func (t *tools) paymentRetryQueue(ctx context.Context, user *entity.User, _ noInput) (any, error) {
	queue, err := t.core.GetPaymentRetryQueue(ctx, user)
	if err != nil {
		return nil, err
	}
	if queue == nil {
		queue = make([]*entity.PaymentRetryView, 0)
	}
	return map[string]any{
		"count":   len(queue),
		"retries": queue,
		"note":    "failed payments waiting for an automatic retry; amounts in cents",
	}, nil
}

func (t *tools) webhookStatus(ctx context.Context, user *entity.User, _ noInput) (any, error) {
	health, err := t.core.GetWebhookHealth(ctx, user)
	if err != nil {
		return nil, err
	}
	failures, err := t.core.ListWebhookFailures(ctx, user)
	if err != nil {
		return nil, err
	}
	if health == nil {
		health = make([]*entity.WebhookHealthView, 0)
	}
	if failures == nil {
		failures = make([]*entity.WebhookDeliveryView, 0)
	}
	return map[string]any{
		"subscribers":     health,
		"recent_failures": failures,
		"note":            "evsys delivers events to webhook subscribers through an outbox; recent_failures lists up to 100 failed or retrying deliveries",
	}, nil
}
