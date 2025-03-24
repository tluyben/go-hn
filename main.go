package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tluyben/go-hn/hn"
)

// Embed static files into the binary
//
//go:embed static templates
var content embed.FS

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

	// Serve static files
	http.Handle("/static/", http.FileServer(http.FS(content)))

	// Home route - show top stories
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Handling request for: %s", r.URL.Path)

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

		log.Printf("Fetching stories (page: %d, perPage: %d)", page, perPage)
		stories, err := client.GetStoriesPage("topstories", page, perPage)
		if err != nil {
			log.Printf("Error fetching stories: %v", err)
			http.Error(w, "Failed to load stories", http.StatusInternalServerError)
			return
		}

		log.Printf("Retrieved %d stories", len(stories))
		if len(stories) == 0 {
			log.Printf("Warning: No stories returned for page %d", page)
		}

		// Debug log the first story if available
		if len(stories) > 0 {
			log.Printf("First story: Title=%s, By=%s, Score=%d", stories[0].Title, stories[0].By, stories[0].Score)
		}

		data := map[string]interface{}{
			"Title":    "Hacker News",
			"Stories":  stories,
			"Page":     page,
			"NextPage": page + 1,
			"MoreLink": len(stories) == perPage,
		}

		var templateErr error
		if r.Header.Get("HX-Request") == "true" {
			log.Printf("HTMX request detected, executing content template")
			templateErr = tmpl.ExecuteTemplate(w, "content", data)
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

		item, err := client.GetItem(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Title":   item.Title,
			"Item":    item,
			"Content": "comments",
		}

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

		data := map[string]interface{}{
			"Title":   username + " - Profile",
			"User":    user,
			"Content": "user",
		}

		tmpl.ExecuteTemplate(w, "base", data)
	})

	// Login handler
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			data := map[string]interface{}{
				"Title":   "Login",
				"Content": "login-content",
			}
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		if err := client.Login(username, password); err != nil {
			data := map[string]interface{}{
				"Title":   "Login",
				"Error":   err.Error(),
				"Content": "login-content",
			}
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Submit story handler
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			data := map[string]interface{}{
				"Title":   "Submit",
				"Content": "submit-content",
			}
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		title := r.FormValue("title")
		url := r.FormValue("url")

		id, err := client.SubmitStory(title, url)
		if err != nil {
			data := map[string]interface{}{
				"Title":   "Submit",
				"Error":   err.Error(),
				"Content": "submit-content",
			}
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/item/%d", id), http.StatusSeeOther)
	})

	// Start server
	log.Println("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
