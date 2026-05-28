package main

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	mu         sync.Mutex
	threads    map[int]*Thread
	nextThread int
	nextPost   int
}

func NewStore() *Store {
	return &Store{threads: make(map[int]*Thread), nextThread: 1, nextPost: 1}
}

func (s *Store) CreateThread(title, author, content string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextThread
	thread := &Thread{ID: id, Title: title, Created: time.Now()}
	post := Post{ID: s.nextPost, Author: author, Content: content, Created: time.Now()}
	s.nextPost++
	thread.Posts = append(thread.Posts, post)
	s.threads[id] = thread
	s.nextThread++
	return id
}

func (s *Store) ListThreads() []*Thread {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]*Thread, 0, len(s.threads))
	for _, t := range s.threads {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID > list[j].ID })
	return list
}

func (s *Store) GetThread(id int) (*Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	thread, ok := s.threads[id]
	if !ok {
		return nil, ErrNotFound
	}
	return thread, nil
}

func (s *Store) AddPost(threadID int, author, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	thread, ok := s.threads[threadID]
	if !ok {
		return ErrNotFound
	}
	p := Post{ID: s.nextPost, Author: author, Content: content, Created: time.Now()}
	s.nextPost++
	thread.Posts = append(thread.Posts, p)
	return nil
}
