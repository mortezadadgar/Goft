package chat

import (
	"errors"
	"goft/user"
	"testing"

	"github.com/google/uuid"
)

func TestClient(t *testing.T) {
	roomID := 1
	sessionID := uuid.New().String()
	user := user.User{
		SessionID: sessionID,
	}

	t.Run("add", func(t *testing.T) {
		r := New()
		err := r.AddClient(user, nil, nil, roomID)
		if err != nil {
			t.Fatal(err)
		}

		got, ok := r.GetClient(sessionID)
		if !ok {
			t.Fatal("expected to get a client but got none")
		}

		if got.roomID != roomID {
			t.Errorf("mismatch\n got: %d\nwant: %d", got.roomID, roomID)
		}
	})

	t.Run("remove", func(t *testing.T) {
		r := New()
		err := r.AddClient(user, nil, nil, roomID)
		if err != nil {
			t.Fatal(err)
		}

		r.RemoveClient(sessionID)

		_, ok := r.GetClient(sessionID)
		if ok {
			t.Errorf("added client is not removed")
		}
	})

	t.Run("duplicated", func(t *testing.T) {
		r := New()
		err := r.AddClient(user, nil, nil, roomID)
		if err != nil {
			t.Fatal(err)
		}

		err = r.AddClient(user, nil, nil, roomID)
		if !errors.Is(err, ErrDuplicatedSession) {
			t.Errorf("no error returned but expected one")
		}
	})
}
