package storage

import (
	"strconv"
	"testing"

	"github.com/thkx/notification-system/pkg/model"
)

func BenchmarkMemoryStoreSave(b *testing.B) {
	store := NewMemoryStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		notification := &model.Notification{
			ID:       "notif-" + strconv.Itoa(i),
			UserID:   "user-" + strconv.Itoa(i%100),
			Status:   "pending",
			Channels: []string{"email"},
		}
		if err := store.Save(notification); err != nil {
			b.Fatalf("save notification: %v", err)
		}
	}
}

func BenchmarkMemoryStoreListByUserAndStatus(b *testing.B) {
	store := NewMemoryStore()
	for i := 0; i < 1000; i++ {
		notification := &model.Notification{
			ID:       "seed-" + strconv.Itoa(i),
			UserID:   "user-" + strconv.Itoa(i%50),
			Status:   "sent",
			Channels: []string{"email"},
		}
		if err := store.Save(notification); err != nil {
			b.Fatalf("seed notification: %v", err)
		}
	}

	filter := NotificationFilter{
		UserID: "user-1",
		Status: "sent",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.List(filter); err != nil {
			b.Fatalf("list notifications: %v", err)
		}
	}
}
