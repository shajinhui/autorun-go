package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalStoreSaveAndLoad(t *testing.T) {
	store := &Store{path: filepath.Join(t.TempDir(), defaultStoreName)}
	ctx := context.Background()

	sessionKey, err := store.Save(ctx, "13900000000", Session{
		Token:     "token-1",
		UserID:    11,
		StudentID: 22,
		SchoolID:  33,
		UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if sessionKey == "" {
		t.Fatal("Save() returned empty session key")
	}

	byStudent, source, err := store.LoadByStudentID(ctx, 22)
	if err != nil {
		t.Fatalf("LoadByStudentID() error = %v", err)
	}
	if source != "local" || byStudent == nil || byStudent.Token != "token-1" {
		t.Fatalf("LoadByStudentID() = (%+v, %q), want local token-1", byStudent, source)
	}

	byPhone, source, err := store.LoadByPhone(ctx, "13900000000")
	if err != nil {
		t.Fatalf("LoadByPhone() error = %v", err)
	}
	if source != "local" || byPhone == nil || byPhone.StudentID != 22 {
		t.Fatalf("LoadByPhone() = (%+v, %q), want local student 22", byPhone, source)
	}

	byKey, source, err := store.LoadBySessionKey(ctx, sessionKey)
	if err != nil {
		t.Fatalf("LoadBySessionKey() error = %v", err)
	}
	if source != "local" || byKey == nil || byKey.UserID != 11 {
		t.Fatalf("LoadBySessionKey() = (%+v, %q), want local user 11", byKey, source)
	}
}
