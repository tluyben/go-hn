package search

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/tluyben/go-hn/types"
)

// SearchableItem represents an HN item with additional fields for search and user preferences
type SearchableItem struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Time        int    `json:"time"`
	Text        string `json:"text,omitempty"`
	Parent      int    `json:"parent,omitempty"`
	URL         string `json:"url,omitempty"`
	Score       int    `json:"score,omitempty"`
	Title       string `json:"title,omitempty"`
	Descendants int    `json:"descendants,omitempty"`
	Rank        int    `json:"rank,omitempty"`
	VoteDir     *int   `json:"vote_dir,omitempty"`
	Favorite    bool   `json:"favorite"`
	Hidden      bool   `json:"hidden"`
	Flagged     bool   `json:"flagged"`
	Summary     string `json:"summary,omitempty"`
}

// Index manages the Bleve search index
type Index struct {
	index bleve.Index
	mu    sync.RWMutex
}

var (
	globalIndex *Index
	once        sync.Once
)

// GetIndex returns the global search index instance
func GetIndex() (*Index, error) {
	var err error
	once.Do(func() {
		globalIndex, err = newIndex()
	})
	return globalIndex, err
}

// newIndex creates a new search index
func newIndex() (*Index, error) {
	// Create a directory for the index if it doesn't exist
	indexPath := "data/search"
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create index directory: %v", err)
	}

	// Try to open existing index
	index, err := bleve.Open(filepath.Join(indexPath, "hn.bleve"))
	if err == nil {
		return &Index{index: index}, nil
	}

	// Create new index if it doesn't exist
	mapping := bleve.NewIndexMapping()
	index, err = bleve.New(filepath.Join(indexPath, "hn.bleve"), mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %v", err)
	}

	return &Index{index: index}, nil
}

// IndexItem adds or updates an item in the search index
func (i *Index) IndexItem(item *types.Item) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Create a consistent ID format
	id := fmt.Sprintf("%d", item.ID)

	searchableItem := &SearchableItem{
		ID:          item.ID,
		Type:        item.Type,
		By:          item.By,
		Time:        item.Time,
		Text:        item.Text,
		Parent:      item.Parent,
		URL:         item.URL,
		Score:       item.Score,
		Title:       item.Title,
		Descendants: item.Descendants,
		Rank:        item.Rank,
		VoteDir:     item.VoteDir,
	}

	// Index with the same ID format
	return i.index.Index(id, searchableItem)
}

// GetItem retrieves an item from the search index by ID
func (i *Index) GetItem(id int) (*SearchableItem, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// Create a query to find the document by ID
	query := bleve.NewTermQuery(fmt.Sprintf("%d", id))
	query.SetField("id") // Search in the id field specifically
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Fields = []string{"*"} // Retrieve all fields

	// Execute the search
	searchResult, err := i.index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	if searchResult.Total == 0 {
		return nil, fmt.Errorf("document not found")
	}

	// Get the first hit
	hit := searchResult.Hits[0]

	// Convert the fields to our struct
	item := &SearchableItem{
		ID:          int(hit.Fields["id"].(float64)),
		Type:        hit.Fields["type"].(string),
		By:          hit.Fields["by"].(string),
		Time:        int(hit.Fields["time"].(float64)),
		Text:        hit.Fields["text"].(string),
		Parent:      int(hit.Fields["parent"].(float64)),
		URL:         hit.Fields["url"].(string),
		Score:       int(hit.Fields["score"].(float64)),
		Title:       hit.Fields["title"].(string),
		Descendants: int(hit.Fields["descendants"].(float64)),
		Rank:        int(hit.Fields["rank"].(float64)),
	}

	// Handle optional fields
	if voteDir, ok := hit.Fields["vote_dir"]; ok && voteDir != nil {
		val := int(voteDir.(float64))
		item.VoteDir = &val
	}
	if favorite, ok := hit.Fields["favorite"]; ok {
		item.Favorite = favorite.(bool)
	}
	if hidden, ok := hit.Fields["hidden"]; ok {
		item.Hidden = hidden.(bool)
	}
	if flagged, ok := hit.Fields["flagged"]; ok {
		item.Flagged = flagged.(bool)
	}
	if summary, ok := hit.Fields["summary"]; ok {
		item.Summary = summary.(string)
	}

	return item, nil
}

// Search performs a full-text search across all indexed items
func (i *Index) Search(query string, from, size int) (*bleve.SearchResult, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	searchQuery := bleve.NewQueryStringQuery(query)
	searchRequest := bleve.NewSearchRequest(searchQuery)
	searchRequest.From = from
	searchRequest.Size = size

	return i.index.Search(searchRequest)
}

// Close closes the search index
func (i *Index) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.index.Close()
}
