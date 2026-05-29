package main

import "time"

type Post struct {
	ID      int
	Author  string
	Content string
	Created time.Time
}

type Thread struct {
	ID      int
	Title   string
	Posts   []Post
	Created time.Time
}
