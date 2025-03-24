package main

import (
	"embed"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tluyben/go-hn/hn"
	"github.com/tluyben/go-hn/search"
	"github.com/tluyben/go-hn/types"
)

// Embed static files into the binary
//
//go:embed static templates
var content embed.FS

var (
	searchIndex *search.Index
	client      *hn.Client
)

// Settings struct for user preferences
type Settings struct {
	Theme string `json:"theme"`
}

// Template functions
var funcMap = template.FuncMap{
	"add": func(a, b int) int {
		return a + b
	},
	"timeAgo": func(unixTime int) string {
		t := time.Unix(int64(unixTime), 0)
		duration := time.Since(t)
		if duration.Hours() > 24*365 {
			years := int(duration.Hours() / (24 * 365))
			return fmt.Sprintf("%d years ago", years)
		}
		if duration.Hours() > 24*30 {
			months := int(duration.Hours() / (24 * 30))
			return fmt.Sprintf("%d months ago", months)
		}
		if duration.Hours() > 24 {
			days := int(duration.Hours() / 24)
			return fmt.Sprintf("%d days ago", days)
		}
		if duration.Hours() >= 1 {
			return fmt.Sprintf("%d hours ago", int(duration.Hours()))
		}
		if duration.Minutes() >= 1 {
			return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
		}
		return "just now"
	},
	"getDomain": func(urlStr string) string {
		if urlStr == "" {
			return ""
		}
		u, err := url.Parse(urlStr)
		if err != nil {
			return urlStr
		}
		return u.Host
	},
	"unescape": func(s string) template.HTML {
		return template.HTML(html.UnescapeString(s))
	},
	"hasVoted": func(dir *int, val int) bool {
		if dir == nil {
			return false
		}
		return *dir == val
	},
	"dict": func(values ...interface{}) (map[string]interface{}, error) {
		if len(values)%2 != 0 {
			return nil, fmt.Errorf("invalid dict call")
		}
		dict := make(map[string]interface{}, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict keys must be strings")
			}
			dict[key] = values[i+1]
		}
		return dict, nil
	},
}

// Initialize search index and HN client
func init() {
	var err error
	searchIndex, err = search.GetIndex()
	if err != nil {
		log.Fatalf("Failed to initialize search index: %v", err)
	}

	client, err = hn.NewClient()
	if err != nil {
		log.Fatalf("Failed to initialize HN client: %v", err)
	}

	// Start background jobs for fetching stories and comments
	client.StartBackgroundJobs()
	log.Println("Background jobs started successfully")
}

// Get item from search index or fetch from HN API
func getItem(id int) (*types.Item, error) {
	// Try to get from search index first
	searchableItem, err := searchIndex.GetItem(id)
	if err == nil {
		// Convert SearchableItem back to types.Item
		return &types.Item{
			ID:          searchableItem.ID,
			Type:        searchableItem.Type,
			By:          searchableItem.By,
			Time:        searchableItem.Time,
			Text:        searchableItem.Text,
			Parent:      searchableItem.Parent,
			URL:         searchableItem.URL,
			Score:       searchableItem.Score,
			Title:       searchableItem.Title,
			Descendants: searchableItem.Descendants,
			Rank:        searchableItem.Rank,
			VoteDir:     searchableItem.VoteDir,
			Kids:        searchableItem.Kids,
		}, nil
	}

	// If not found in index, fetch from HN API
	item, err := client.GetItem(id)
	if err != nil {
		return nil, err
	}

	return item, nil
}

// Get theme from cookie or default to system
func getTheme(r *http.Request) string {
	cookie, err := r.Cookie("theme")
	if err != nil {
		return "system"
	}

	theme := cookie.Value

	// Validate theme
	validThemes := map[string]bool{
		"light":  true,
		"dark":   true,
		"system": true,
	}

	if !validThemes[theme] {
		return "system"
	}

	return theme
}

// Set theme in cookie
func setTheme(w http.ResponseWriter, theme string) {
	// Validate theme
	validThemes := map[string]bool{
		"light":  true,
		"dark":   true,
		"system": true,
	}

	if !validThemes[theme] {
		theme = "system" // Default to system if theme is invalid
	}

	// Set the cookie directly with the theme value, no JSON encoding
	http.SetCookie(w, &http.Cookie{
		Name:     "theme",
		Value:    theme,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60, // 1 year
		HttpOnly: false,              // Allow JavaScript to access this cookie for theme sync
		SameSite: http.SameSiteLaxMode,
	})
}

// Get menu state from cookie
func getMenuState(r *http.Request) string {
	cookie, err := r.Cookie("menu_state")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// Helper function to create template data with common fields
func createTemplateData(title string, content string, r *http.Request) map[string]interface{} {
	return map[string]interface{}{
		"Title":     title,
		"Content":   content,
		"Theme":     getTheme(r),
		"MenuState": getMenuState(r),
	}
}

func main() {
	// Set up logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting Hacker News frontend...")

	// Parse templates with functions
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(content, "templates/*.html")
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}
	log.Println("Templates parsed successfully")

	// Create HN client
	client, err := hn.NewClient()
	if err != nil {
		log.Fatalf("Error creating HN client: %v", err)
	}
	log.Println("HN client created successfully")

	// Create a custom server with timeouts
	server := &http.Server{
		Addr:           ":8080",
		Handler:        nil, // Will be set to http.DefaultServeMux
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
		IdleTimeout:    120 * time.Second,
	}

	// Serve static files
	http.Handle("/static/", http.FileServer(http.FS(content)))

	// Home route - show stories based on section
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Handling request for: %s", r.URL.Path)

		// Get the section from the URL path
		section := r.URL.Path[1:] // Remove leading slash
		if section == "" {
			section = "topstories" // Default to topstories for root path
		}

		// Validate section
		validSections := map[string]bool{
			"topstories":  true,
			"newstories":  true,
			"paststories": true,
			"askstories":  true,
			"showstories": true,
			"jobstories":  true,
		}

		if section == "newcomments" {
			page := 1
			perPage := 30

			if pageStr := r.URL.Query().Get("p"); pageStr != "" {
				if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
					page = p
					log.Printf("Using page number: %d", page)
				} else if err != nil {
					log.Printf("Invalid page number '%s': %v", pageStr, err)
				}
			}

			log.Printf("Fetching new comments (page: %d, perPage: %d)", page, perPage)

			// Get comments from HN API with caching
			comments, err := client.GetNewComments(perPage*5, false) // Fetch more than needed for pagination
			if err != nil {
				log.Printf("Error fetching comments: %v", err)
				http.Error(w, "Failed to load comments", http.StatusInternalServerError)
				return
			}

			log.Printf("Retrieved %d total comments", len(comments))

			// Calculate pagination
			start := (page - 1) * perPage
			end := start + perPage
			if end > len(comments) {
				end = len(comments)
			}
			pageComments := comments[start:end]

			log.Printf("Retrieved %d comments for page %d", len(pageComments), page)
			if len(pageComments) == 0 {
				log.Printf("Warning: No comments returned for page %d", page)
			}

			data := createTemplateData("New Comments", "comments-list", r)
			data["Comments"] = pageComments
			data["Page"] = page
			data["NextPage"] = page + 1
			data["MoreLink"] = end < len(comments)

			var templateErr error
			if r.Header.Get("HX-Request") == "true" {
				log.Printf("HTMX request detected, executing content template")
				templateErr = tmpl.ExecuteTemplate(w, "comments-list", data)
			} else {
				log.Printf("Regular request, executing base template")
				templateErr = tmpl.ExecuteTemplate(w, "base", data)
			}

			if templateErr != nil {
				log.Printf("Template error: %v", templateErr)
				http.Error(w, "Failed to render page", http.StatusInternalServerError)
				return
			}

			log.Printf("Successfully rendered page with %d comments", len(pageComments))
			return
		}

		if !validSections[section] {
			http.Error(w, "Invalid section", http.StatusBadRequest)
			return
		}

		page := 1
		perPage := 30

		// Get page parameter from URL query
		pageStr := r.URL.Query().Get("p")
		if pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
				log.Printf("Using page number: %d", page)
			} else if err != nil {
				log.Printf("Invalid page number '%s': %v", pageStr, err)
			}
		}

		log.Printf("Fetching %s (page: %d, perPage: %d)", section, page, perPage)

		// Get stories for this page
		stories, err := client.GetStoriesPage(section, page, perPage, false)
		if err != nil {
			log.Printf("Error fetching stories: %v", err)
			http.Error(w, "Failed to load stories", http.StatusInternalServerError)
			return
		}

		log.Printf("Retrieved %d total stories", len(stories))

		data := createTemplateData("Hacker News", "stories-list", r)
		data["Stories"] = stories
		data["Page"] = page
		data["NextPage"] = page + 1
		data["MoreLink"] = len(stories) == perPage
		data["Section"] = section

		var templateErr error
		if r.Header.Get("HX-Request") == "true" {
			log.Printf("HTMX request detected, executing content template")
			// For pagination, render just the story items
			templateErr = tmpl.ExecuteTemplate(w, "stories-content", data)

		} else {
			log.Printf("Regular request, executing base template")
			templateErr = tmpl.ExecuteTemplate(w, "base", data)
		}

		if templateErr != nil {
			log.Printf("Template error: %v", templateErr)
			http.Error(w, "Failed to render page", http.StatusInternalServerError)
			return
		}

		log.Printf("Successfully rendered page with %d stories", len(stories))
	})

	// Item/Comments page
	http.HandleFunc("/item/", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.URL.Path[6:])
		if err != nil {
			http.Error(w, "Invalid item ID", http.StatusBadRequest)
			return
		}

		item, err := getItem(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create a map to store all comments for O(1) lookup
		commentMap := make(map[int]*types.Item)

		// Fetch all comments recursively
		comments := make([]*types.Item, 0)

		if item.Kids != nil && len(item.Kids) > 0 {
			for _, kidID := range item.Kids {

				comment, err := getItem(kidID)
				if err != nil {
					log.Printf("Error fetching comment %d: %v", kidID, err)
					continue
				}
				if comment != nil && !comment.Dead && !comment.Deleted {
					comments = append(comments, comment)
					commentMap[comment.ID] = comment
					// Recursively fetch child comments
					fetchChildComments(comment, &comments, commentMap)
				}
			}
		}

		// Sort comments to ensure parent comments come before their children
		sortedComments := make([]*types.Item, 0, len(comments))
		addedComments := make(map[int]bool)

		// First add all top-level comments (those whose parent is the item)
		for _, comment := range comments {
			if comment.Parent == item.ID {
				sortedComments = append(sortedComments, comment)
				addedComments[comment.ID] = true
			}
		}

		// Then add remaining comments in parent-child order
		for len(sortedComments) < len(comments) {
			for _, comment := range comments {
				if !addedComments[comment.ID] && addedComments[comment.Parent] {
					sortedComments = append(sortedComments, comment)
					addedComments[comment.ID] = true
				}
			}
		}

		data := createTemplateData(item.Title, "comments-content", r)
		data["Item"] = item
		data["Comments"] = sortedComments
		data["LoggedIn"] = client.IsLoggedIn()

		tmpl.ExecuteTemplate(w, "base", data)
	})

	// User profile page
	http.HandleFunc("/user/", func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Path[6:]
		user, err := client.GetUser(username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := createTemplateData(username+" - Profile", "user-content", r)
		data["User"] = user

		tmpl.ExecuteTemplate(w, "base", data)
	})

	// Login handler
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			data := createTemplateData("Login", "login-content", r)
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		if err := client.Login(username, password); err != nil {
			data := createTemplateData("Login", err.Error(), r)
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Submit story handler
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			data := createTemplateData("Submit", "submit-content", r)
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		title := r.FormValue("title")
		url := r.FormValue("url")

		id, err := client.SubmitStory(title, url)
		if err != nil {
			data := createTemplateData("Submit", err.Error(), r)
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/item/%d", id), http.StatusSeeOther)
	})

	// Theme toggle handler
	http.HandleFunc("/toggle-theme", func(w http.ResponseWriter, r *http.Request) {
		currentTheme := getTheme(r)
		var newTheme string

		switch currentTheme {
		case "light":
			newTheme = "dark"
		case "dark":
			newTheme = "system"
		default:
			newTheme = "light"
		}

		log.Printf("Toggling theme from %s to %s", currentTheme, newTheme)
		setTheme(w, newTheme)

		// For HTMX requests, return a script to update the theme
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/javascript")
			w.Write([]byte(fmt.Sprintf(`
				// Update theme
				document.documentElement.setAttribute('data-theme', '%s');
				
				// For system theme, apply based on OS preference
				if ('%s' === 'system') {
					const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
					document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
				}
				
				// Dispatch event to notify theme has changed
				document.dispatchEvent(new CustomEvent('themeChanged', { detail: { theme: '%s' } }));
			`, newTheme, newTheme, newTheme)))
		} else {
			// For non-HTMX requests, redirect back
			referer := r.Header.Get("Referer")
			if referer == "" {
				referer = "/"
			}
			http.Redirect(w, r, referer, http.StatusSeeOther)
		}
	})

	// Menu toggle handler
	http.HandleFunc("/toggle-menu", func(w http.ResponseWriter, r *http.Request) {
		// Get current state from cookie
		cookie, err := r.Cookie("menu_state")
		isOpen := false
		if err == nil {
			isOpen = cookie.Value == "open"
		}

		// Toggle state
		isOpen = !isOpen
		state := "closed"
		if isOpen {
			state = "open"
		}

		// Set new state in cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "menu_state",
			Value:    state,
			Path:     "/",
			MaxAge:   365 * 24 * 60 * 60, // 1 year
			HttpOnly: true,
		})

		// Return menu HTML with appropriate class
		w.Header().Set("Content-Type", "text/html")
		menuHTML := `<div id="mobile-menu" class="nav-links %s">
			<a href="/newest">new</a>
			<a href="/front">past</a>
			<a href="/newcomments">comments</a>
			<a href="/ask">ask</a>
			<a href="/show">show</a>
			<a href="/jobs">jobs</a>
			<a href="/submit" class="submit-link">submit</a>
		</div>`

		class := ""
		if isOpen {
			class = "show"
		}
		w.Write([]byte(fmt.Sprintf(menuHTML, class)))

		// Add script to handle body scroll
		if isOpen {
			w.Write([]byte(`<script>document.body.classList.add('menu-open');</script>`))
		} else {
			w.Write([]byte(`<script>document.body.classList.remove('menu-open');</script>`))
		}
	})

	// Comment reply handler
	http.HandleFunc("/reply/", func(w http.ResponseWriter, r *http.Request) {
		if !client.IsLoggedIn() {
			http.Error(w, "Must be logged in to reply", http.StatusUnauthorized)
			return
		}

		id, err := strconv.Atoi(r.URL.Path[7:])
		if err != nil {
			http.Error(w, "Invalid comment ID", http.StatusBadRequest)
			return
		}

		// Get the parent comment
		parent, err := getItem(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"ParentID": id,
			"Parent":   parent,
		}

		tmpl.ExecuteTemplate(w, "reply-form", data)
	})

	// Comment submit handler
	http.HandleFunc("/comment", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !client.IsLoggedIn() {
			http.Error(w, "Must be logged in to comment", http.StatusUnauthorized)
			return
		}

		parentID, err := strconv.Atoi(r.FormValue("parent_id"))
		if err != nil {
			http.Error(w, "Invalid parent ID", http.StatusBadRequest)
			return
		}

		text := r.FormValue("text")
		if text == "" {
			http.Error(w, "Comment text cannot be empty", http.StatusBadRequest)
			return
		}

		err = client.Comment(parentID, text)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Redirect to the item page
		http.Redirect(w, r, fmt.Sprintf("/item/%d", parentID), http.StatusSeeOther)
	})

	// Start server
	log.Println("Server starting on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// Helper function to recursively fetch child comments
func fetchChildComments(parent *types.Item, allComments *[]*types.Item, commentMap map[int]*types.Item) {
	if parent.Kids == nil || len(parent.Kids) == 0 {
		return
	}

	for _, kidID := range parent.Kids {
		comment, err := getItem(kidID)
		if err != nil {
			log.Printf("Error fetching child comment %d: %v", kidID, err)
			continue
		}
		if comment != nil && !comment.Dead && !comment.Deleted {
			*allComments = append(*allComments, comment)
			commentMap[comment.ID] = comment
			// Recursively fetch this comment's children
			fetchChildComments(comment, allComments, commentMap)
		}
	}
}
