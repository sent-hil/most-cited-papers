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
	Title      string
	URL        string
	Citations  int
	LastUpdate string
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
		SELECT title, url, citations, timestamp
		FROM paper_cache
		ORDER BY citations DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var papers []PaperView
	for rows.Next() {
		var paper PaperView
		var timestamp string

		if err := rows.Scan(&paper.Title, &paper.URL, &paper.Citations, &timestamp); err != nil {
			return nil, err
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
		return nil, err
	}

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

        <div class="bg-white shadow-md rounded-lg overflow-hidden">
            <table class="min-w-full divide-y divide-gray-200">
                <thead class="bg-gray-50">
                    <tr>
                        <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Title
                        </th>
                        <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Citations
                        </th>
                        <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Last Updated
                        </th>
                        <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Actions
                        </th>
                    </tr>
                </thead>
                <tbody class="bg-white divide-y divide-gray-200">
                    {{range .Papers}}
                    <tr class="hover:bg-gray-50">
                        <td class="px-6 py-4 whitespace-normal">
                            <div class="text-sm font-medium text-gray-900">{{.Title}}</div>
                        </td>
                        <td class="px-6 py-4 whitespace-nowrap">
                            <div class="text-lg font-bold text-gray-900">{{.Citations}}</div>
                        </td>
                        <td class="px-6 py-4 whitespace-nowrap">
                            <div class="text-sm text-gray-500">{{.LastUpdate}}</div>
                        </td>
                        <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                            <a href="{{.URL}}" target="_blank" class="text-blue-600 hover:text-blue-900">View Paper</a>
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</body>
</html>`

// No longer needed as we're using the Tailwind CDN
