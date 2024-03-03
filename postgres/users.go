package postgres

import (
	"context"
	"errors"
	"fmt"
	"goft/chat"
	"goft/user"
	"net/http"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotExists = errors.New("user not exits")
)

func (p Postgres) CreateUserMessages(ctx context.Context, message chat.Message) error {
	query := `
	INSERT INTO messages(user_id, text, room_id)
	VALUES($1, $2, $3)
	`

	_, err := p.DB.Exec(ctx, query, message.UserID, message.Text, message.RoomID)
	if err != nil {
		return err
	}

	return nil
}

func (p Postgres) ValidateUser(r *http.Request, u user.User, password string) (int, error) {
	query := `
	SELECT id, hashed_password
	FROM users WHERE name = $1
	`

	var ID int
	var hashedPassword []byte
	err := p.DB.QueryRow(r.Context(), query, u.Name).Scan(&ID, &hashedPassword)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrUserNotExists
	} else if err != nil {
		return 0, err
	}

	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	if err != nil {
		return 0, err
	}

	err = p.InsertSession(r, u, ID)
	if err != nil {
		return 0, err
	}

	return ID, nil
}

func (p Postgres) CreateUser(r *http.Request, u user.User, password string) error {
	query := `
	INSERT
	INTO users(name, hashed_password)VALUES($1, $2)
	RETURNING id
	`

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password, %v", err)
	}

	var ID int
	err = p.DB.QueryRow(r.Context(), query, u.Name, hashedPassword).Scan(&ID)
	if err != nil {
		return fmt.Errorf("failed to insert user, %v", err)
	}

	err = p.InsertSession(r, u, ID)
	if err != nil {
		return err
	}

	return nil
}

func (p Postgres) GetUserIDFromSession(sessionID string, ctx context.Context) (user.User, error) {
	query := `
	SELECT users.id, users.name
	FROM users
	JOIN sessions ON users.id = sessions.user_id
	WHERE sessions.uuid = $1;
	`

	var ID int
	var name string
	err := p.DB.QueryRow(ctx, query, sessionID).Scan(&ID, &name)
	if err != nil {
		return user.User{}, err
	}

	return user.User{ID: ID, Name: name, SessionID: sessionID}, nil
}
