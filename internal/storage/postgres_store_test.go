package storage

import (
	"database/sql"
	"reflect"
	"slices"
	"testing"
)

func TestPGXDriverRegistered(t *testing.T) {
	if !slices.Contains(sql.Drivers(), "pgx") {
		t.Fatalf("expected pgx driver to be registered, got %v", sql.Drivers())
	}
}

func TestBuildNotificationListQuery(t *testing.T) {
	tests := []struct {
		name      string
		filter    NotificationFilter
		wantQuery string
		wantArgs  []interface{}
	}{
		{
			name:      "no filters",
			filter:    NotificationFilter{},
			wantQuery: "SELECT id, user_id, type, content, channels, priority, scheduled, status, created_at, updated_at FROM notifications ORDER BY created_at DESC, updated_at DESC, id ASC",
			wantArgs:  []interface{}{},
		},
		{
			name:      "user filter",
			filter:    NotificationFilter{UserID: "user-1"},
			wantQuery: "SELECT id, user_id, type, content, channels, priority, scheduled, status, created_at, updated_at FROM notifications WHERE user_id = $1 ORDER BY created_at DESC, updated_at DESC, id ASC",
			wantArgs:  []interface{}{"user-1"},
		},
		{
			name:      "user and status filters",
			filter:    NotificationFilter{UserID: "user-1", Status: "sent"},
			wantQuery: "SELECT id, user_id, type, content, channels, priority, scheduled, status, created_at, updated_at FROM notifications WHERE user_id = $1 AND status = $2 ORDER BY created_at DESC, updated_at DESC, id ASC",
			wantArgs:  []interface{}{"user-1", "sent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := buildNotificationListQuery(tt.filter)
			if query != tt.wantQuery {
				t.Fatalf("unexpected query\nwant: %s\ngot:  %s", tt.wantQuery, query)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", tt.wantArgs, args)
			}
		})
	}
}

func TestSerializeAndDeserializeChannels(t *testing.T) {
	channels := []string{"email", "sms", "inapp"}
	serialized := serializeChannels(channels)
	if serialized != "email,sms,inapp" {
		t.Fatalf("unexpected serialized channels: %s", serialized)
	}

	deserialized := deserializeChannels(sql.NullString{String: serialized, Valid: true})
	if !reflect.DeepEqual(deserialized, channels) {
		t.Fatalf("unexpected deserialized channels: %#v", deserialized)
	}

	if deserializeChannels(sql.NullString{}) != nil {
		t.Fatal("expected nil channels for invalid/null input")
	}
}
