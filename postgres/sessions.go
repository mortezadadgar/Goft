package postgres

import (
	"fmt"
	"goft/user"
	"net/http"
)

func (p Postgres) InsertSession(r *http.Request, u user.User, ID int) error {
	query := `
	INSERT INTO sessions(uuid, user_id, expiry) VALUES($1, $2, $3)
	`

	_, err := p.DB.Exec(r.Context(), query, u.SessionID, ID, u.Expiry)
	if err != nil {
		return fmt.Errorf("failed to insert session, %v", err)
	}

	return nil
}
