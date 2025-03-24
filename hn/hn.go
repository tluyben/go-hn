package hn

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

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
	Voted       bool   `json:"voted,omitempty"`
}

// User represents a Hacker News user
type User struct {
	ID        string `json:"id"`
	Created   int    `json:"created"`
	Karma     int    `json:"karma"`
	About     string `json:"about,omitempty"`
	Submitted []int  `json:"submitted,omitempty"`
}

// SearchResult represents a result from the Algolia HN search API
type SearchResult struct {
	Hits             []map[string]interface{} `json:"hits"`
	Page             int                      `json:"page"`
	NbHits           int                      `json:"nbHits"`
	NbPages          int                      `json:"nbPages"`
	HitsPerPage      int                      `json:"hitsPerPage"`
	ProcessingTimeMS int                      `json:"processingTimeMS"`
}

// Client represents a Hacker News client
type Client struct {
	httpClient *http.Client
	apiBase    string
	webBase    string
	searchBase string
	loggedIn   bool
	username   string
	csrf       string
}

// NewClient creates a new Hacker News client
func NewClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	return &Client{
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
		apiBase:    "https://hacker-news.firebaseio.com/v0",
		webBase:    "https://news.ycombinator.com",
		searchBase: "https://hn.algolia.com/api/v1",
		loggedIn:   false,
	}, nil
}

// GetItem fetches an item by ID
func (c *Client) GetItem(id int) (*Item, error) {
	url := fmt.Sprintf("%s/item/%d.json", c.apiBase, id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var item Item
	err = c.doRequest(req, &item)
	if err != nil {
		return nil, err
	}

	return &item, nil
}

// GetUser fetches a user by username
func (c *Client) GetUser(username string) (*User, error) {
	url := fmt.Sprintf("%s/user/%s.json", c.apiBase, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var user User
	err = c.doRequest(req, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetMaxItem returns the current largest item ID
func (c *Client) GetMaxItem() (int, error) {
	url := fmt.Sprintf("%s/maxitem.json", c.apiBase)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	var maxID int
	err = c.doRequest(req, &maxID)
	if err != nil {
		return 0, err
	}

	return maxID, nil
}

// getStories is a helper function to fetch stories by type
func (c *Client) getStories(storyType string, limit int) ([]Item, error) {
	url := fmt.Sprintf("%s/%s.json", c.apiBase, storyType)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	var ids []int
	err = c.doRequest(req, &ids)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch story IDs: %v", err)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no stories found for type: %s", storyType)
	}

	// Limit the number of stories if specified
	if limit > 0 && limit < len(ids) {
		ids = ids[:limit]
	}

	// Create a channel to receive items and errors
	type result struct {
		item *Item
		err  error
	}
	results := make(chan result, len(ids))

	// Fetch items concurrently
	for _, id := range ids {
		go func(id int) {
			item, err := c.GetItem(id)
			results <- result{item: item, err: err}
		}(id)
	}

	// Collect results
	items := make([]Item, 0, len(ids))
	var lastErr error
	for i := 0; i < len(ids); i++ {
		res := <-results
		if res.err != nil {
			lastErr = res.err
			continue
		}
		if res.item != nil && (res.item.Type == "story" || res.item.Type == "job") {
			items = append(items, *res.item)
		}
	}

	// If we got no items but had errors, return the last error
	if len(items) == 0 && lastErr != nil {
		return nil, fmt.Errorf("failed to fetch any valid stories: %v", lastErr)
	}

	return items, nil
}

// GetTopStories fetches up to 500 top stories
func (c *Client) GetTopStories(limit int) ([]Item, error) {
	return c.getStories("topstories", limit)
}

// GetNewStories fetches up to 500 newest stories
func (c *Client) GetNewStories(limit int) ([]Item, error) {
	return c.getStories("newstories", limit)
}

// GetBestStories fetches the best stories
func (c *Client) GetBestStories(limit int) ([]Item, error) {
	return c.getStories("beststories", limit)
}

// GetAskStories fetches up to 200 latest Ask HN stories
func (c *Client) GetAskStories(limit int) ([]Item, error) {
	return c.getStories("askstories", limit)
}

// GetShowStories fetches up to 200 latest Show HN stories
func (c *Client) GetShowStories(limit int) ([]Item, error) {
	return c.getStories("showstories", limit)
}

// GetJobStories fetches up to 200 latest Job stories
func (c *Client) GetJobStories(limit int) ([]Item, error) {
	return c.getStories("jobstories", limit)
}

// GetUpdates fetches items and profiles that have been changed
func (c *Client) GetUpdates() (map[string][]interface{}, error) {
	url := fmt.Sprintf("%s/updates.json", c.apiBase)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var updates map[string][]interface{}
	err = c.doRequest(req, &updates)
	if err != nil {
		return nil, err
	}

	return updates, nil
}

// Search searches for stories using the Algolia HN API
func (c *Client) Search(query string) (*SearchResult, error) {
	url := fmt.Sprintf("%s/search?query=%s", c.searchBase, url.QueryEscape(query))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	err = c.doRequest(req, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Login logs in to Hacker News
func (c *Client) Login(username, password string) error {
	loginURL := fmt.Sprintf("%s/login", c.webBase)

	// First, get the login page to extract any potential CSRF token
	req, err := http.NewRequest("GET", loginURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Extract CSRF token if present - HN doesn't use CSRF tokens for login,
	// but we'll keep this code in case they add it in the future
	csrfRe := regexp.MustCompile(`name="csrf" value="([^"]+)"`)
	csrfMatches := csrfRe.FindSubmatch(body)
	if len(csrfMatches) > 1 {
		c.csrf = string(csrfMatches[1])
	}

	// Now perform the login
	data := make(url.Values)
	data.Set("acct", username)
	data.Set("pw", password)
	if c.csrf != "" {
		data.Set("csrf", c.csrf)
	}
	data.Set("goto", "news")

	req, err = http.NewRequest("POST", loginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check if login was successful by looking for a user-specific element
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// If we find "Bad login", login failed
	if strings.Contains(string(body), "Bad login") {
		return errors.New("login failed: bad username or password")
	}

	// If we find a logout link or the username, login succeeded
	if strings.Contains(string(body), fmt.Sprintf("user?id=%s", username)) ||
		strings.Contains(string(body), ">logout<") {
		c.loggedIn = true
		c.username = username
		return nil
	}

	return errors.New("login failed: unknown reason")
}

// SubmitStory submits a new story to Hacker News
func (c *Client) SubmitStory(title, urlStr string) (int, error) {
	if !c.loggedIn {
		return 0, errors.New("you must be logged in to submit a story")
	}

	// Get the submit page to extract any potential CSRF token
	submitURL := fmt.Sprintf("%s/submit", c.webBase)
	req, err := http.NewRequest("GET", submitURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Extract CSRF token if present
	csrfRe := regexp.MustCompile(`name="csrf" value="([^"]+)"`)
	csrfMatches := csrfRe.FindSubmatch(body)
	if len(csrfMatches) > 1 {
		c.csrf = string(csrfMatches[1])
	}

	// Now submit the story
	formData := make(url.Values)
	formData.Set("title", title)
	formData.Set("url", urlStr)
	if c.csrf != "" {
		formData.Set("csrf", c.csrf)
	}

	var submitReq *http.Request
	submitReq, err = http.NewRequest("POST", submitURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return 0, err
	}

	submitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = c.httpClient.Do(submitReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Check if the submission was successful and get the new item ID
	if resp.StatusCode == http.StatusFound {
		// Get the redirect URL which should contain the item ID
		if location := resp.Header.Get("Location"); location != "" {
			re := regexp.MustCompile(`item\?id=(\d+)`)
			matches := re.FindStringSubmatch(location)
			if len(matches) > 1 {
				id, err := strconv.Atoi(matches[1])
				if err != nil {
					return 0, err
				}
				return id, nil
			}
		}
	}

	return 0, errors.New("failed to submit story")
}

// Upvote upvotes an item
func (c *Client) Upvote(itemID int) error {
	if !c.loggedIn {
		return errors.New("you must be logged in to upvote")
	}

	// First, visit the item page to extract any potential CSRF token
	itemURL := fmt.Sprintf("%s/item?id=%d", c.webBase, itemID)
	req, err := http.NewRequest("GET", itemURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Check if there's an upvote link
	upvoteRe := regexp.MustCompile(fmt.Sprintf(`href="vote\?id=(%d)&amp;how=up&amp;goto=`, itemID))
	upvoteMatches := upvoteRe.FindSubmatch(body)
	if len(upvoteMatches) < 2 {
		return errors.New("upvote link not found or you may have already voted")
	}

	// Extract auth parameter
	authRe := regexp.MustCompile(fmt.Sprintf(`href="vote\?id=%d&amp;how=up&amp;goto=.+&amp;auth=([^"]+)"`, itemID))
	authMatches := authRe.FindSubmatch(body)
	if len(authMatches) < 2 {
		return errors.New("auth parameter not found")
	}
	auth := string(authMatches[1])

	// Now upvote the item
	voteURL := fmt.Sprintf("%s/vote", c.webBase)
	data := make(url.Values)
	data.Set("id", strconv.Itoa(itemID))
	data.Set("how", "up")
	data.Set("auth", auth)
	data.Set("goto", fmt.Sprintf("item?id=%d", itemID))

	req, err = http.NewRequest("POST", voteURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		return errors.New("failed to upvote")
	}

	return nil
}

// Comment adds a comment to an item
func (c *Client) Comment(itemID int, text string) error {
	if !c.loggedIn {
		return errors.New("you must be logged in to comment")
	}

	// First, visit the item page to extract any potential CSRF token
	itemURL := fmt.Sprintf("%s/item?id=%d", c.webBase, itemID)
	req, err := http.NewRequest("GET", itemURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Extract CSRF token if present
	csrfRe := regexp.MustCompile(`name="csrf" value="([^"]+)"`)
	csrfMatches := csrfRe.FindSubmatch(body)
	if len(csrfMatches) > 1 {
		c.csrf = string(csrfMatches[1])
	}

	// Extract form action URL
	formRe := regexp.MustCompile(`<form action="([^"]+)" method="post"`)
	formMatches := formRe.FindSubmatch(body)
	if len(formMatches) < 2 {
		return errors.New("comment form not found")
	}
	commentURL := fmt.Sprintf("%s/%s", c.webBase, string(formMatches[1]))

	// Now post the comment
	data := make(url.Values)
	data.Set("text", text)
	data.Set("parent", strconv.Itoa(itemID))
	data.Set("goto", fmt.Sprintf("item?id=%d", itemID))
	if c.csrf != "" {
		data.Set("csrf", c.csrf)
	}

	req, err = http.NewRequest("POST", commentURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		return errors.New("failed to post comment")
	}

	return nil
}

// GetStoriesPage fetches a specific page of stories
func (c *Client) GetStoriesPage(storyType string, page, perPage int) ([]Item, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 30
	}

	// Calculate start and end indices for the page
	start := (page - 1) * perPage
	end := start + perPage

	// Get the full list of story IDs
	url := fmt.Sprintf("%s/%s.json", c.apiBase, storyType)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	var ids []int
	err = c.doRequest(req, &ids)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch story IDs: %v", err)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no stories found for type: %s", storyType)
	}

	// Adjust end index if it exceeds the number of stories
	if end > len(ids) {
		end = len(ids)
	}
	if start >= len(ids) {
		return nil, fmt.Errorf("page %d exceeds available stories", page)
	}

	// Get the IDs for this page
	pageIDs := ids[start:end]

	// Create a channel to receive items and errors
	type result struct {
		item *Item
		err  error
		idx  int // Add index to track original position
	}
	results := make(chan result, len(pageIDs))

	// Fetch items concurrently
	for i, id := range pageIDs {
		go func(id, idx int) {
			item, err := c.GetItem(id)
			results <- result{item: item, err: err, idx: idx}
		}(id, i)
	}

	// Collect results
	collected := make([]result, len(pageIDs))
	var lastErr error
	for i := 0; i < len(pageIDs); i++ {
		res := <-results
		if res.err != nil {
			log.Printf("Error fetching item: %v", res.err)
			lastErr = res.err
			continue
		}
		collected[res.idx] = res
	}

	// Build final items slice in correct order
	items := make([]Item, 0, len(pageIDs))
	for _, res := range collected {
		if res.item != nil && (res.item.Type == "story" || res.item.Type == "job") {
			items = append(items, *res.item)
		}
	}

	// If we got no items but had errors, return the last error
	if len(items) == 0 && lastErr != nil {
		return nil, fmt.Errorf("failed to fetch any valid stories: %v", lastErr)
	}

	// taking into account page/perpage, add a Rank field to each item
	for i := range items {
		items[i].Rank = start + i + 1
	}

	return items, nil
}

// doRequest performs an HTTP request and unmarshals the response
func (c *Client) doRequest(req *http.Request, v interface{}) error {
	log.Printf("Making request to: %s", req.URL.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Request failed with status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if len(body) == 0 {
		log.Printf("Empty response body received")
		return fmt.Errorf("empty response body")
	}

	log.Printf("Response body length: %d bytes", len(body))
	log.Printf("Response body preview: %.200s", string(body))

	err = json.Unmarshal(body, v)
	if err != nil {
		log.Printf("Failed to unmarshal response: %v", err)
		return fmt.Errorf("failed to unmarshal response: %v, body: %s", err, string(body))
	}

	return nil
}
