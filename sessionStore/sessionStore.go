package sessionstore

import (
	"goft/postgres"
	"goft/user"
	"net/http"
	"sync"
)

type Store struct {
	store map[string]user.User
	lock  sync.RWMutex
	pg    postgres.Postgres
}

func New(pg postgres.Postgres) *Store {
	return &Store{
		store: make(map[string]user.User),
		pg:    pg,
	}
}

func (s *Store) Set(r *http.Request, sessionID string, u user.User) {
	s.lock.Lock()
	s.store[sessionID] = u
	s.lock.Unlock()
}

func (s *Store) Get(r *http.Request, sessionID string) (user.User, error) {
	// get user data from session store if there's any
	s.lock.RLock()
	data, found := s.store[sessionID]
	if found {
		s.lock.RUnlock()
		return data, nil
	}
	s.lock.RUnlock()

	data, err := s.pg.GetUserIDFromSession(sessionID, r.Context())
	if err != nil {
		return user.User{}, err
	}

	s.Set(r, sessionID, data)

	return data, nil
}
