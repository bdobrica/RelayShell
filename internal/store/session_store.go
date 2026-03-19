package store

import (
	"sync"

	"github.com/bdobrica/RelayShell/internal/sessions"
)

type SessionStore struct {
	mu     sync.RWMutex
	byID   map[string]*sessions.Session
	byRoom map[string]*sessions.Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		byID:   map[string]*sessions.Session{},
		byRoom: map[string]*sessions.Session{},
	}
}

func (s *SessionStore) Add(session *sessions.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[session.ID] = session
	s.byRoom[session.RoomID] = session
}

func (s *SessionStore) GetByID(id string) (*sessions.Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.byID[id]
	return session, ok
}

func (s *SessionStore) GetByRoomID(roomID string) (*sessions.Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.byRoom[roomID]
	return session, ok
}

func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.byID[id]
	if !ok {
		return
	}
	delete(s.byID, id)
	delete(s.byRoom, session.RoomID)
}
