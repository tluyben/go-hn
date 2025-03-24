package types

// Item represents a Hacker News item (story, comment, job, etc.)
type Item struct {
	ID          int    `json:"id"`
	Deleted     bool   `json:"deleted,omitempty"`
	Type        string `json:"type"`
	By          string `json:"by,omitempty"`
	Time        int    `json:"time"`
	Text        string `json:"text,omitempty"`
	Dead        bool   `json:"dead,omitempty"`
	Parent      int    `json:"parent,omitempty"`
	Poll        int    `json:"poll,omitempty"`
	Kids        []int  `json:"kids,omitempty"`
	URL         string `json:"url,omitempty"`
	Score       int    `json:"score,omitempty"`
	Title       string `json:"title,omitempty"`
	Parts       []int  `json:"parts,omitempty"`
	Descendants int    `json:"descendants,omitempty"`
	Rank        int    `json:"rank,omitempty"`
	VoteDir     *int   `json:"vote_dir,omitempty"` // 1 for upvote, -1 for downvote, nil for no vote
}
