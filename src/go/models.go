package handlers

type Discussion struct {
	ID        int
	Title     string
	Author    string
	Content   string
	AvatarURL string
	Score     int
	UserVote  int
	Replies   []Reply
	CreatedAt string
}

type Reply struct {
	ID           int
	DiscussionID int
	Author       string
	Content      string
	AvatarURL    string
	CreatedAt    string
}

type HomePageData struct {
	Discussions []Discussion
	Message     string
}

type ProfilePageData struct {
	Email       string
	ID          string
	Username    string
	AvatarURL   string
	AvatarError string
	Error       string
}

type CreatePageData struct {
	Error  string
	Author string
}

type voteResponse struct {
	Score    int `json:"score"`
	UserVote int `json:"user_vote"`
}
