package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserContext = errors.New("invalid user context")
)

type userContextKey string

const usrCtxKey userContextKey = "user"

type User struct {
	ID        int
	Name      string
	SessionID string
	Expiry    time.Time
}

const expiryTime = 24 * 30 * 6 * time.Hour

func New(name string) (User, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return User{}, err
	}

	return User{
		SessionID: uuid.String(),
		Name:      strings.TrimSpace(name),
		Expiry:    time.Now().Add(expiryTime),
	}, nil
}

func AddToContext(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, usrCtxKey, user)
}

func FromContext(ctx context.Context) (User, error) {
	u, ok := ctx.Value(usrCtxKey).(User)
	if !ok {
		return User{}, ErrUserContext
	}

	return u, nil
}
