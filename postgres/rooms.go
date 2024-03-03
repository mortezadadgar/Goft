package postgres

import (
	"context"
	"goft/types"
	"strings"
)

func (p Postgres) GetRoomMessages(ctx context.Context, roomID int) ([]string, error) {
	rows, err := p.DB.Query(ctx, "SELECT text FROM messages WHERE room_id = $1", roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []string
	for rows.Next() {
		var message string
		err := rows.Scan(&message)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return messages, nil
}

func (p Postgres) SearchRooms(ctx context.Context, term string) ([]types.Room, error) {
	query := `
	SELECT id, name, description
	FROM rooms
	WHERE to_tsvector(name) @@ to_tsquery($1)
	`

	if term != "" && term[len(term)-1] != ' ' {
		term = strings.ReplaceAll(term, " ", " | ") + ":*"
	}

	rows, err := p.DB.Query(ctx, query, term)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []types.Room

	for rows.Next() {
		var name string
		var id int
		var description string
		err := rows.Scan(&id, &name, &description)
		if err != nil {
			return nil, err
		}
		results = append(results, types.Room{ID: id, Name: name, Description: description})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (p Postgres) ListRoom(ctx context.Context) ([]types.Room, error) {
	query := `
	SELECT id, name, description
	FROM rooms
	`

	var results []types.Room

	rows, err := p.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			ID          int
			name        string
			description string
		)
		if err := rows.Scan(&ID, &name, &description); err != nil {
			return nil, err
		}
		results = append(results, types.Room{ID: ID, Name: name, Description: description})
	}

	return results, nil
}

func (p Postgres) GetRoom(ctx context.Context, ID int) (types.Room, error) {
	query := `
	SELECT name, description
	FROM rooms
	WHERE id = $1
	`

	var name string
	var description string
	err := p.DB.QueryRow(ctx, query, ID).Scan(&name, &description)
	if err != nil {
		return types.Room{}, err
	}

	return types.Room{
		ID:          ID,
		Name:        name,
		Description: description,
	}, nil
}
