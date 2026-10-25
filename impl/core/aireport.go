package core

import (
	"context"
	"evsys-back/entity"
	"fmt"
	"time"
)

// AIReportService is the optional dependency that runs the AI analysis.
type AIReportService interface {
	Settings(ctx context.Context) (*entity.AIReportSettingsView, error)
	UpdateSettings(ctx context.Context, author string, u *entity.AIReportSettingsUpdate) (*entity.AIReportSettingsView, error)
	Test(ctx context.Context) (*entity.AIUsage, error)
	Run(ctx context.Context, author string, req *entity.AIReportRunRequest) (*entity.AIReport, error)
	Report(ctx context.Context, id string) (*entity.AIReport, error)
	Reports(ctx context.Context, limit int) ([]*entity.AIReport, error)
	Preview(ctx context.Context, id, language string) (*entity.AIReportPreview, error)
	Usage(ctx context.Context, from, to time.Time) (*entity.AIUsageReport, error)
	SendTo(ctx context.Context, author string, sub *entity.MailSubscription) error
}

// SetAIReportService plugs in the AI analysis; without it the AI endpoints
// answer that the service is not configured.
func (c *Core) SetAIReportService(s AIReportService) {
	c.aiReport = s
}

// requireAIReport admits admins only: the AI settings decide what is sent to
// a paid external service.
func (c *Core) requireAIReport(author *entity.User) error {
	if err := c.requireAdmin(author); err != nil {
		return err
	}
	if c.aiReport == nil {
		return fmt.Errorf("ai report service not configured")
	}
	return nil
}

func (c *Core) requireAdmin(author *entity.User) error {
	if c.auth == nil {
		return fmt.Errorf("authenticator not set")
	}
	if author == nil || !author.IsAdmin() {
		return fmt.Errorf("access denied: admin only")
	}
	return nil
}

// GetAIReportSettings returns the AI analysis settings (admin only).
func (c *Core) GetAIReportSettings(ctx context.Context, author *entity.User) (*entity.AIReportSettingsView, error) {
	if err := c.requireAIReport(author); err != nil {
		return nil, err
	}
	return c.aiReport.Settings(ctx)
}

// UpdateAIReportSettings changes the AI analysis settings (admin only).
func (c *Core) UpdateAIReportSettings(ctx context.Context, author *entity.User, u *entity.AIReportSettingsUpdate) (*entity.AIReportSettingsView, error) {
	if err := c.requireAIReport(author); err != nil {
		return nil, err
	}
	return c.aiReport.UpdateSettings(ctx, author.Username, u)
}

// TestAIReport checks the OpenAI key and model with a minimal request (admin only).
func (c *Core) TestAIReport(ctx context.Context, author *entity.User) (*entity.AIUsage, error) {
	if err := c.requireAIReport(author); err != nil {
		return nil, err
	}
	return c.aiReport.Test(ctx)
}

// RunAIReport starts an analysis by hand (admin only).
func (c *Core) RunAIReport(ctx context.Context, author *entity.User, req *entity.AIReportRunRequest) (*entity.AIReport, error) {
	if err := c.requireAIReport(author); err != nil {
		return nil, err
	}
	return c.aiReport.Run(ctx, author.Username, req)
}

// ListAIReports returns the latest analyses (admin only).
func (c *Core) ListAIReports(ctx context.Context, author *entity.User, limit int) ([]*entity.AIReport, error) {
	if err := c.requireAIReport(author); err != nil {
		return nil, err
	}
	return c.aiReport.Reports(ctx, limit)
}

// GetAIReport returns one analysis with the data it read (admin only).
func (c *Core) GetAIReport(ctx context.Context, author *entity.User, id string) (*entity.AIReport, error) {
	if err := c.requireAIReport(author); err != nil {
		return nil, err
	}
	return c.aiReport.Report(ctx, id)
}

// PreviewAIReport renders the email of an analysis (admin only).
func (c *Core) PreviewAIReport(ctx context.Context, author *entity.User, id, language string) (*entity.AIReportPreview, error) {
	if err := c.requireAIReport(author); err != nil {
		return nil, err
	}
	return c.aiReport.Preview(ctx, id, language)
}

// GetAIUsage reports the OpenAI requests and their cost (admin only).
func (c *Core) GetAIUsage(ctx context.Context, author *entity.User, from, to time.Time) (*entity.AIUsageReport, error) {
	if err := c.requireAIReport(author); err != nil {
		return nil, err
	}
	return c.aiReport.Usage(ctx, from, to)
}
