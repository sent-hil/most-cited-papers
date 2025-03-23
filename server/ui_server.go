package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
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
}

// NewUIServer creates a new UI server
func NewUIServer(dbFilePath string) (*UIServer, error) {
	// Connect to the database
	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return nil, err
	}

	// Parse templates
	tmpl, err := template.New("index").Parse(indexTemplate)
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
	http.HandleFunc("/refresh", s.handleRefresh)
	http.HandleFunc("/tailwind.css", s.serveTailwind)

	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// Close closes the UI server
func (s *UIServer) Close() error {
	return s.db.Close()
}

// handleIndex handles the index page
func (s *UIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	papers, err := s.getPapers()
	if err != nil {
		http.Error(w, "Failed to fetch papers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Papers []PaperView
		Count  int
	}{
		Papers: papers,
		Count:  len(papers),
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

// getPapers fetches all papers from the database
func (s *UIServer) getPapers() ([]PaperView, error) {
	query := `
		SELECT title, url, citations, arxiv_abs_url, google_scholar_url, timestamp, arxiv_summary
		FROM paper_cache
		ORDER BY CASE WHEN citations IS NULL THEN 1 ELSE 0 END, citations DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		log.Printf("Error querying papers: %v", err)
		return nil, err
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
			return nil, err
		}

		if arxivAbsURL.Valid {
			paper.ArxivAbsURL = arxivAbsURL.String
		}

		if googleScholarURL.Valid {
			paper.GoogleScholarURL = googleScholarURL.String
		}

		if arxivSummary.Valid {
			paper.ArxivSummary = arxivSummary.String
			log.Printf("Found abstract for paper: %s (length: %d)", paper.Title, len(paper.ArxivSummary))
		} else {
			log.Printf("No abstract found for paper: %s", paper.Title)
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
		return nil, err
	}

	log.Printf("Total papers loaded: %d", len(papers))
	return papers, nil
}

// indexTemplate is the HTML template for the index page
const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Paper Citations Database</title>
    <script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <header class="mb-8">
            <h1 class="text-3xl font-bold text-gray-800">Paper Citations Database</h1>
            <p class="text-gray-600">Showing {{.Count}} papers sorted by citation count</p>
            <div class="mt-4">
                <a href="/refresh" class="bg-blue-500 hover:bg-blue-600 text-white font-semibold py-2 px-4 rounded">
                    Refresh Data
                </a>
            </div>
        </header>

        <div class="space-y-4">
            {{range .Papers}}
            <div class="bg-white shadow-md rounded-lg overflow-hidden">
                <div class="p-6">
                    <div class="flex justify-between items-start">
                        <div class="flex-1">
                            <h2 class="text-xl font-semibold text-gray-900 mb-2">{{.Title}}</h2>
                            <div class="flex items-center space-x-4 text-sm text-gray-600">
                                <span>Citations: {{if .Citations}}{{.Citations}}{{else}}N/A{{end}}</span>
                                <span>Last Updated: {{.LastUpdate}}</span>
                            </div>
                        </div>
                        <div class="flex space-x-4">
                            <a href="{{.URL}}" target="_blank" class="text-gray-600 hover:text-gray-900">Paper</a>
                            {{if .ArxivAbsURL}}
                            <a href="{{.ArxivAbsURL}}" target="_blank" class="text-gray-600 hover:text-gray-900">arXiv</a>
                            {{end}}
                            {{if .GoogleScholarURL}}
                            <a href="{{.GoogleScholarURL}}" target="_blank" class="text-gray-600 hover:text-gray-900">Scholar</a>
                            {{end}}
                        </div>
                    </div>
                </div>
                {{if .ArxivSummary}}
                <div class="border-t border-gray-200">
                    <div class="p-6">
                        <h3 class="text-sm font-medium text-gray-500 mb-2">Abstract</h3>
                        <p class="text-gray-700">{{.ArxivSummary}}</p>
                    </div>
                </div>
                {{end}}
            </div>
            {{end}}
        </div>
    </div>
</body>
</html>`

// No longer needed as we're using the Tailwind CDN
