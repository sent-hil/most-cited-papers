package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// UIServer represents the web server for the citations UI
type UIServer struct {
	db         *sql.DB
	tmpl       *template.Template
	dbFilePath string
}

// PaperView represents a paper for view in the UI
type PaperView struct {
	Title            string
	URL              string
	ArxivAbsURL      string
	GoogleScholarURL string
	ArxivSummary     string
	Citations        int
	LastUpdate       string
	FirstSentence    string
}

// getFirstSentence returns the first sentence of a text
func getFirstSentence(text string) string {
	sentences := strings.Split(text, ".")
	if len(sentences) > 0 {
		return strings.TrimSpace(sentences[0]) + "."
	}
	return text
}

// NewUIServer creates a new UI server
func NewUIServer(dbFilePath string) (*UIServer, error) {
	// Connect to the database
	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return nil, err
	}

	// Create template with custom functions
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"subtract": func(a, b int) int {
			return a - b
		},
	}

	// Parse template with custom functions
	tmpl, err := template.New("index").Funcs(funcMap).Parse(indexTemplate)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &UIServer{
		db:         db,
		tmpl:       tmpl,
		dbFilePath: dbFilePath,
	}, nil
}

// Start starts the UI server
func (s *UIServer) Start(addr string) error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/api/papers", s.handlePapersAPI)
	http.HandleFunc("/refresh", s.handleRefresh)
	http.HandleFunc("/tailwind.css", s.serveTailwind)
	http.HandleFunc("/static/js/", s.serveStaticJS)

	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// Close closes the UI server
func (s *UIServer) Close() error {
	return s.db.Close()
}

// handleIndex handles the index page
func (s *UIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Parse page parameter from query string
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Parse search query
	searchQuery := r.URL.Query().Get("q")

	const pageSize = 25
	papers, total, err := s.getPapers(page, pageSize, searchQuery)
	if err != nil {
		http.Error(w, "Failed to fetch papers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate pagination info
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	data := struct {
		Papers      []PaperView
		Count       int
		CurrentPage int
		TotalPages  int
		PageSize    int
		SearchQuery string
	}{
		Papers:      papers,
		Count:       total,
		CurrentPage: page,
		TotalPages:  totalPages,
		PageSize:    pageSize,
		SearchQuery: searchQuery,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := s.tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution failed: "+err.Error(), http.StatusInternalServerError)
	}
}

// handleRefresh handles the refresh action
func (s *UIServer) handleRefresh(w http.ResponseWriter, r *http.Request) {
	// This is a simple redirect back to the index page
	// In a more advanced version, you could add cache invalidation logic here
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// serveTailwind serves a minimal CSS file
// This is no longer used since we're using the CDN, but keeping the handler for compatibility
func (s *UIServer) serveTailwind(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write([]byte("/* Using Tailwind CDN instead */"))
}

// handlePapersAPI handles AJAX requests for paper data
func (s *UIServer) handlePapersAPI(w http.ResponseWriter, r *http.Request) {
	// Parse page parameter from query string
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Parse search query
	searchQuery := r.URL.Query().Get("q")

	const pageSize = 25
	papers, total, err := s.getPapers(page, pageSize, searchQuery)
	if err != nil {
		http.Error(w, "Failed to fetch papers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate pagination info
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	// If no results found, ensure papers is an empty array
	if total == 0 {
		papers = []PaperView{}
	}

	response := struct {
		Papers      []PaperView `json:"papers"`
		Count       int         `json:"count"`
		CurrentPage int         `json:"currentPage"`
		TotalPages  int         `json:"totalPages"`
		PageSize    int         `json:"pageSize"`
	}{
		Papers:      papers,
		Count:       total,
		CurrentPage: page,
		TotalPages:  totalPages,
		PageSize:    pageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response: "+err.Error(), http.StatusInternalServerError)
	}
}

// getPapers fetches all papers from the database with pagination and search
func (s *UIServer) getPapers(page, pageSize int, searchQuery string) ([]PaperView, int, error) {
	// Build the base query
	baseQuery := `SELECT title, url, citations, arxiv_abs_url, google_scholar_url, timestamp, arxiv_summary FROM paper_cache`
	countQuery := `SELECT COUNT(*) FROM paper_cache`

	var args []interface{}
	var whereClause string

	if searchQuery != "" {
		whereClause = ` WHERE title LIKE ? OR arxiv_summary LIKE ?`
		searchPattern := "%" + searchQuery + "%"
		args = append(args, searchPattern, searchPattern)
	}

	// Get total count
	var total int
	if whereClause != "" {
		countQuery += whereClause
	}
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		log.Printf("Error getting total count: %v", err)
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get paginated results
	query := baseQuery + whereClause + ` ORDER BY CASE WHEN citations IS NULL THEN 1 ELSE 0 END, citations DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Printf("Error querying papers: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	var papers []PaperView
	for rows.Next() {
		var paper PaperView
		var timestamp string
		var arxivAbsURL sql.NullString
		var googleScholarURL sql.NullString
		var arxivSummary sql.NullString
		var citations sql.NullInt64

		if err := rows.Scan(&paper.Title, &paper.URL, &citations, &arxivAbsURL, &googleScholarURL, &timestamp, &arxivSummary); err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, 0, err
		}

		if arxivAbsURL.Valid {
			paper.ArxivAbsURL = arxivAbsURL.String
		}

		if googleScholarURL.Valid {
			paper.GoogleScholarURL = googleScholarURL.String
		}

		if arxivSummary.Valid {
			paper.ArxivSummary = arxivSummary.String
			paper.FirstSentence = getFirstSentence(arxivSummary.String)
		}

		if citations.Valid {
			paper.Citations = int(citations.Int64)
		}

		// Parse timestamp and format it for display
		t, err := time.Parse("2006-01-02 15:04:05", timestamp)
		if err == nil {
			paper.LastUpdate = t.Format("Jan 02, 2006 15:04")
		} else {
			paper.LastUpdate = timestamp
		}

		papers = append(papers, paper)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v", err)
		return nil, 0, err
	}

	log.Printf("Loaded %d papers (page %d, total %d, search: %q)", len(papers), page, total, searchQuery)
	return papers, total, nil
}

// serveStaticJS serves static JavaScript files
func (s *UIServer) serveStaticJS(w http.ResponseWriter, r *http.Request) {
	// Get the file path from the URL
	filePath := r.URL.Path[len("/static/js/"):]

	// Read the file from the current directory
	content, err := os.ReadFile("static/js/" + filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Set the content type
	w.Header().Set("Content-Type", "application/javascript")
	w.Write(content)
}

// No longer needed as we're using the Tailwind CDN
