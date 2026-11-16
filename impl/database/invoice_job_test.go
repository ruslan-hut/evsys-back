package database

import (
	"context"
	"errors"
	"evsys-back/entity"
	"testing"
	"time"
)

// Run as described in concurrency_test.go.

func TestInvoiceJobs(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	if err := m.ensureInvoiceIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	job := func(id int, status string, next time.Time) *entity.InvoiceJob {
		return &entity.InvoiceJob{TransactionId: id, UserId: "ana", Amount: 100, Status: status,
			NextAttemptTime: next, CreatedAt: now, UpdatedAt: now}
	}

	created, err := m.CreateInvoiceJob(ctx, job(1, entity.InvoiceJobPending, now.Add(-time.Minute)))
	if err != nil || !created {
		t.Fatalf("first create: %v %v", created, err)
	}
	// the same transaction is never queued twice
	created, err = m.CreateInvoiceJob(ctx, job(1, entity.InvoiceJobPending, now))
	if err != nil || created {
		t.Fatalf("duplicate: created=%v err=%v, want false, nil", created, err)
	}
	for _, j := range []*entity.InvoiceJob{
		job(2, entity.InvoiceJobPending, now.Add(time.Hour)), // not due
		job(3, entity.InvoiceJobFailed, now.Add(-time.Hour)),
		job(4, entity.InvoiceJobNeedsCheck, now.Add(-time.Hour)),
	} {
		if _, err := m.CreateInvoiceJob(ctx, j); err != nil {
			t.Fatal(err)
		}
	}

	pending, err := m.GetPendingInvoiceJobs(ctx, now)
	if err != nil || len(pending) != 1 || pending[0].TransactionId != 1 {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	problems, err := m.ListInvoiceJobs(ctx, entity.InvoiceJobFailed, entity.InvoiceJobNeedsCheck)
	if err != nil || len(problems) != 2 {
		t.Fatalf("problems: %+v %v", problems, err)
	}
	if all, _ := m.ListInvoiceJobs(ctx); len(all) != 4 {
		t.Fatalf("all: %d, want 4", len(all))
	}

	pending[0].Step = entity.InvoiceStepCreated
	pending[0].HoldedInvoiceId = "h-9"
	pending[0].Status = entity.InvoiceJobDone
	if err := m.UpdateInvoiceJob(ctx, pending[0]); err != nil {
		t.Fatal(err)
	}
	if pending, _ = m.GetPendingInvoiceJobs(ctx, now); len(pending) != 0 {
		t.Fatalf("done job still pending: %+v", pending)
	}
	if err := m.UpdateInvoiceJob(ctx, job(99, entity.InvoiceJobPending, now)); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("update of a missing job: %v, want ErrNotFound", err)
	}

	// the contact id goes to the profile; a missing profile is reported
	if err := m.SetInvoiceProfileContact(ctx, "ana", "c-1"); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("contact without profile: %v, want ErrNotFound", err)
	}
	if err := m.SaveInvoiceProfile(ctx, &entity.InvoiceProfile{UserId: "ana", Name: "Ana", UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := m.SetInvoiceProfileContact(ctx, "ana", "c-1"); err != nil {
		t.Fatal(err)
	}
	if p, _ := m.GetInvoiceProfile(ctx, "ana"); p == nil || p.HoldedContactId != "c-1" {
		t.Fatalf("profile after contact: %+v", p)
	}
}
