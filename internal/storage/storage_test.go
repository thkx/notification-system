package storage

import (
	"testing"
	"time"

	"github.com/thkx/notification-system/pkg/model"
)

func TestMemoryStoreSaveAndGetByIDReturnsCopy(t *testing.T) {
	store := NewMemoryStore()
	notification := &model.Notification{
		ID:       "notif-1",
		UserID:   "user-1",
		Status:   "pending",
		Channels: []string{"email"},
	}

	if err := store.Save(notification); err != nil {
		t.Fatalf("save notification: %v", err)
	}

	saved, err := store.GetByID(notification.ID)
	if err != nil {
		t.Fatalf("get notification: %v", err)
	}

	if saved == nil {
		t.Fatal("expected notification to be stored")
	}

	if saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be populated")
	}

	saved.Status = "mutated"

	storedAgain, err := store.GetByID(notification.ID)
	if err != nil {
		t.Fatalf("get notification again: %v", err)
	}

	if storedAgain.Status != "pending" {
		t.Fatalf("expected stored notification to remain unchanged, got %q", storedAgain.Status)
	}

	saved.Channels[0] = "sms"
	storedThird, err := store.GetByID(notification.ID)
	if err != nil {
		t.Fatalf("get notification third time: %v", err)
	}

	if storedThird.Channels[0] != "email" {
		t.Fatalf("expected channels to remain unchanged, got %v", storedThird.Channels)
	}
}

func TestMemoryStoreListFiltersByUserIDAndStatus(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	notifications := []*model.Notification{
		{ID: "1", UserID: "user-1", Status: "sent", Channels: []string{"email"}, CreatedAt: now.Add(-time.Minute)},
		{ID: "2", UserID: "user-1", Status: "failed", Channels: []string{"sms"}, CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "3", UserID: "user-2", Status: "sent", Channels: []string{"inapp"}, CreatedAt: now},
	}

	for _, notification := range notifications {
		if err := store.Save(notification); err != nil {
			t.Fatalf("save notification %s: %v", notification.ID, err)
		}
	}

	filtered, err := store.List(NotificationFilter{
		UserID: "user-1",
		Status: "sent",
	})
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}

	if len(filtered) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(filtered))
	}

	if filtered[0].ID != "1" {
		t.Fatalf("expected notification ID 1, got %s", filtered[0].ID)
	}
}

func TestMemoryStoreListReturnsSortedCopies(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()

	input := []*model.Notification{
		{ID: "older", UserID: "user-1", Status: "sent", Channels: []string{"email"}, CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "newer", UserID: "user-1", Status: "sent", Channels: []string{"sms"}, CreatedAt: now},
	}

	for _, notification := range input {
		if err := store.Save(notification); err != nil {
			t.Fatalf("save notification %s: %v", notification.ID, err)
		}
	}

	list, err := store.List(NotificationFilter{UserID: "user-1"})
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(list))
	}

	if list[0].ID != "newer" || list[1].ID != "older" {
		t.Fatalf("expected sorted notifications [newer older], got [%s %s]", list[0].ID, list[1].ID)
	}

	list[0].Channels[0] = "mutated"

	stored, err := store.GetByID("newer")
	if err != nil {
		t.Fatalf("get notification: %v", err)
	}
	if stored.Channels[0] != "sms" {
		t.Fatalf("expected stored channels unchanged, got %v", stored.Channels)
	}
}

func TestMemoryStoreUpdateStatusRejectsMissingNotification(t *testing.T) {
	store := NewMemoryStore()

	if err := store.UpdateStatus("missing", "sent"); err == nil {
		t.Fatal("expected error when updating a missing notification")
	}
}
