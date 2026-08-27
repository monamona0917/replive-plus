package dal

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestWithWriteDBUnlocksAfterError(t *testing.T) {
	wantErr := errors.New("boom")
	if err := WithWriteDB(func(db *gorm.DB) error {
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("WithWriteDB() error = %v, want %v", err, wantErr)
	}

	done := make(chan struct{})
	go func() {
		_ = WithWriteDB(func(db *gorm.DB) error {
			close(done)
			return nil
		})
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WithWriteDB did not release the lock after returning an error")
	}
}
