package services

import (
	"testing"
	"time"

	"reel/internal/clients/notifications"
	"reel/internal/database/models"
)

func TestNotify_NoNotifiers_Skips(t *testing.T) {
	stage := NewNotifyStage(testLogger(), nil)
	ctx := testContext(t, models.MediaTypeMovie)

	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("should skip with no notifiers: %v", err)
	}
}

func TestNotify_SendsToAllNotifiers(t *testing.T) {
	n1 := &mockNotifier{}
	n2 := &mockNotifier{}

	notifiers := []notifications.Notifier{n1, n2}
	stage := NewNotifyStage(testLogger(), notifiers)

	ctx := testContext(t, models.MediaTypeMovie)
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for goroutines to complete
	time.Sleep(200 * time.Millisecond)

	if len(n1.getCalls()) != 1 {
		t.Errorf("notifier 1: expected 1 call, got %d", len(n1.getCalls()))
	}
	if len(n2.getCalls()) != 1 {
		t.Errorf("notifier 2: expected 1 call, got %d", len(n2.getCalls()))
	}
}

func TestNotify_Rollback_IsNoop(t *testing.T) {
	stage := NewNotifyStage(testLogger(), nil)
	ctx := testContext(t, models.MediaTypeMovie)
	if err := stage.Rollback(ctx); err != nil {
		t.Fatalf("rollback should be noop: %v", err)
	}
}
