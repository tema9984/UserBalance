package store

import (
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/tema9984/AvitoInter/config"
)

type Store struct {
	config *config.Config
	db     *sql.DB
	evRep  *EventRepository
}

func New(conf *config.Config) *Store {
	return &Store{config: conf}
}

func (s *Store) Open() error {
	connStr := s.config.DBcon
	db, err := sql.Open("postgres", connStr)
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(5)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return nil
	}
	s.db = db
	return nil
}
func (s *Store) Close() {
	s.db.Close()
}

func (s *Store) Ev() *EventRepository {
	if s.evRep != nil {
		return s.evRep
	}
	s.evRep = &EventRepository{
		store: s,
	}
	return s.evRep
}
