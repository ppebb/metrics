package main

import (
	"bytes"
	"encoding/gob"
	"os"
	"sync"
)

const SAVEFILE = "./state.gob"

type ConcData struct {
	mu    sync.Mutex
	v     map[string]*LineBytePair
	l     map[string][]LineBytePairForLang
	f     int
	repos []Repo
}

type SerializedRepo struct {
	CommitHashes    []string
	LangCounts      map[string]*LineBytePair
	UniqueFileCount int
}

// Serialized state
type State struct {
	Repos map[string]SerializedRepo
}

func (s *State) toGOB64() ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(s)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func stateFromGOB64(s *State, by []byte) error {
	buf := bytes.Buffer{}
	buf.Write(by)

	dec := gob.NewDecoder(&buf)
	err := dec.Decode(&s)

	return err
}

func (s *State) read() error {
	gob.Register(State{})

	_, err := os.Stat(SAVEFILE)

	if err != nil || os.IsNotExist(err) {
		return err
	}

	by, err := os.ReadFile(SAVEFILE)
	if err != nil {
		return err
	}

	err = stateFromGOB64(s, by)
	return err
}

func (d *ConcData) writeState() error {
	s := State{}
	s.Repos = map[string]SerializedRepo{}

	for _, repo := range d.repos {
		s.Repos[repo.Identifier] = SerializedRepo{
			CommitHashes:    repo.CommitHashesOrdered,
			LangCounts:      repo.LangCounts,
			UniqueFileCount: repo.UniqueFileCount,
		}
	}

	by, err := s.toGOB64()
	if err != nil {
		return err
	}

	return os.WriteFile(SAVEFILE, by, 0644)
}
