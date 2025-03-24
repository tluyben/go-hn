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

		data := createTemplateData("Hacker News", "stories-content", r)
		data["Stories"] = stories
		data["Page"] = page
		data["NextPage"] = page + 1
		data["MoreLink"] = len(stories) == perPage

		var templateErr error
		if r.Header.Get("HX-Request") == "true" {
			log.Printf("HTMX request detected, executing content template")
			// For pagination, render just the story items
			if r.URL.Query().Get("htmx") == "true" {
				templateErr = tmpl.ExecuteTemplate(w, "story-items", data)
			} else {
				templateErr = tmpl.ExecuteTemplate(w, "stories-content", data)
			}
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

		data := createTemplateData(item.Title, "comments-content", r)
		data["Item"] = item

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

	// Start server
	log.Println("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
