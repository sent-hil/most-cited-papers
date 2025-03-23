package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Paper represents a scientific paper with its title and citation count
type Paper struct {
	Title            string
	URL              string
	ArxivAbsURL      string
	GoogleScholarURL string
	ArxivSummary     string
	Citations        int
}

// CacheDB handles interactions with the SQLite cache
type CacheDB struct {
	db *sql.DB
}

func main() {
	// Define command-line flags
	dbPath := flag.String("db", "paper_cache.db", "Path to SQLite database file")
	runServer := flag.Bool("server", false, "Run the web UI server instead of processing papers")
	serverAddr := flag.String("addr", ":8080", "HTTP server address for web UI")
	flag.Parse()

	// Check if we should run the web UI server
	if *runServer {
		runWebServer(*dbPath, *serverAddr)
		return
	}

	// Check if file path is provided
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run *.go [options] <path-to-markdown-file>")
		fmt.Println("Options:")
		fmt.Println("  -db=<path>      Path to SQLite database file (default: paper_cache.db)")
		fmt.Println("  -server         Run the web UI server instead of processing papers")
		fmt.Println("  -addr=<addr>    HTTP server address for web UI (default: :8080)")
		os.Exit(1)
	}

	filePath := args[0]

	// Initialize cache
	cache, err := initCache(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.close()

	papers := parseMarkdownPapers(filePath)

	// Process each paper to get citation count
	for i := range papers {
		fmt.Printf("Processing: %s\n", papers[i].Title)

		// Check if we have this paper in cache
		cached, err := cache.getCitation(papers[i].URL)
		if err != nil {
			log.Printf("Error checking cache for '%s': %v\n", papers[i].URL, err)
		}

		if cached != nil {
			// Use cached data
			fmt.Printf("  Using cached data for %s\n", papers[i].URL)
			papers[i].Citations = cached.Citations
			papers[i].ArxivAbsURL = cached.ArxivAbsURL
			papers[i].GoogleScholarURL = cached.GoogleScholarURL
			papers[i].ArxivSummary = cached.ArxivSummary
		} else {
			// Add a delay to avoid being rate-limited
			time.Sleep(2 * time.Second)

			// Fetch new data
			if IsArxivURL(papers[i].URL) && !IsArxivPDF(papers[i].URL) {
				// For arXiv abstract pages, try to follow links
				err := processArxivPaper(&papers[i])
				if err != nil {
					log.Printf("Error processing arXiv paper '%s': %v\n", papers[i].Title, err)
					// Fall back to direct search if following links fails
					err = processNonArxivPaper(&papers[i])
					if err != nil {
						log.Printf("Fallback search also failed for '%s': %v\n", papers[i].Title, err)
					}
				}
			} else {
				// For non-arXiv papers or arXiv PDF links, directly search by title
				log.Printf("Using title search for '%s'\n", papers[i].Title)
				err := processNonArxivPaper(&papers[i])
				if err != nil {
					log.Printf("Error searching Google Scholar for '%s': %v\n", papers[i].Title, err)
				}
			}

			// Cache the result
			err = cache.saveCitation(papers[i])
			if err != nil {
				log.Printf("Error caching data for '%s': %v\n", papers[i].URL, err)
			}
		}
	}

	// Sort papers by citation count (descending)
	sort.Slice(papers, func(i, j int) bool {
		// Handle nil citations (place them at the end)
		if papers[i].Citations == 0 && papers[j].Citations == 0 {
			return false // Keep original order
		}
		if papers[i].Citations == 0 {
			return false // i goes after j
		}
		if papers[j].Citations == 0 {
			return true // i goes before j
		}
		return papers[i].Citations > papers[j].Citations
	})

	// Print results
	fmt.Println("\nResults sorted by citation count:")
	fmt.Println("----------------------------------")
	for i, paper := range papers {
		fmt.Printf("%d. Title: %s\n   URL: %s\n   Citations: ",
			i+1, paper.Title, paper.URL)

		if paper.Citations != 0 {
			fmt.Printf("%d\n", paper.Citations)
		} else {
			fmt.Printf("N/A\n")
		}

		if paper.ArxivAbsURL != "" {
			fmt.Printf("   arXiv: %s\n", paper.ArxivAbsURL)
		}

		if paper.GoogleScholarURL != "" {
			fmt.Printf("   Scholar: %s\n", paper.GoogleScholarURL)
		}

		fmt.Println()
	}
}

// UIServer handles the web interface
type UIServer struct {
	db         *sql.DB
	tmpl       *template.Template
	dbFilePath string
}

// PaperView represents a paper for display in the UI
type PaperView struct {
	Title            string
	URL              string
	ArxivAbsURL      string
	GoogleScholarURL string
	ArxivSummary     string
	Citations        int
	LastUpdate       string
}

// runWebServer starts the web UI server
func runWebServer(dbPath, addr string) {
	// Create a new UI server
	srv, err := NewUIServer(dbPath)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	log.Printf("Starting UI server at %s", addr)
	log.Printf("Database: %s", dbPath)
	log.Printf("Open your browser at http://localhost%s", addr)

	if err := srv.Start(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// NewUIServer creates a new UI server instance
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

// Close closes the database connection
func (s *UIServer) Close() error {
	return s.db.Close()
}

// Start starts the HTTP server
func (s *UIServer) Start(addr string) error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/refresh", s.handleRefresh)
	http.HandleFunc("/tailwind.css", s.serveTailwind)

	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleIndex handles the main page request
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
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// handleRefresh handles the refresh action
func (s *UIServer) handleRefresh(w http.ResponseWriter, r *http.Request) {
	// This is a simple redirect back to the index page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// serveTailwind serves the CSS
func (s *UIServer) serveTailwind(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write([]byte("/* Using Tailwind CDN instead */"))
}

// getPapers retrieves papers from the database
func (s *UIServer) getPapers() ([]PaperView, error) {
	query := `
		SELECT title, url, citations, arxiv_abs_url, google_scholar_url, arxiv_summary, timestamp
		FROM paper_cache
		ORDER BY CASE WHEN citations IS NULL THEN 1 ELSE 0 END, citations DESC
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
		var citations sql.NullInt64
		var arxivAbsURL sql.NullString
		var googleScholarURL sql.NullString
		var arxivSummary sql.NullString

		if err := rows.Scan(&paper.Title, &paper.URL, &citations, &arxivAbsURL, &googleScholarURL, &arxivSummary, &timestamp); err != nil {
			return nil, err
		}

		if citations.Valid {
			paper.Citations = int(citations.Int64)
		}

		if arxivAbsURL.Valid {
			paper.ArxivAbsURL = arxivAbsURL.String
		}

		if googleScholarURL.Valid {
			paper.GoogleScholarURL = googleScholarURL.String
		}

		if arxivSummary.Valid {
			paper.ArxivSummary = arxivSummary.String
		}

		// Format the timestamp
		t, err := time.Parse("2006-01-02 15:04:05", timestamp)
		if err == nil {
			paper.LastUpdate = t.Format("Jan 02, 2006 15:04")
		} else {
			paper.LastUpdate = timestamp
		}

		papers = append(papers, paper)
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
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <h1 class="text-3xl font-bold mb-6 text-center">Paper Citations Database</h1>
        <div class="mb-4 flex justify-between items-center">
            <p class="text-gray-600">Found {{.Count}} papers</p>
            <a href="/refresh" class="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded">
                Refresh Data
            </a>
        </div>
        <div class="bg-white shadow-md rounded-lg overflow-hidden">
            <table class="min-w-full divide-y divide-gray-200">
                <thead class="bg-gray-50">
                    <tr>
                        <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Title</th>
                        <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Citations</th>
                        <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Links</th>
                        <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Last Updated</th>
                    </tr>
                </thead>
                <tbody class="bg-white divide-y divide-gray-200">
                    {{range .Papers}}
                    <tr>
                        <td class="px-6 py-4 whitespace-normal">
                            <div class="text-sm font-medium text-gray-900">{{.Title}}</div>
                            {{if .ArxivSummary}}
                            <details class="mt-1">
                                <summary class="text-xs text-blue-500 cursor-pointer">Show Abstract</summary>
                                <p class="text-xs text-gray-500 mt-1 max-w-2xl">{{.ArxivSummary}}</p>
                            </details>
                            {{end}}
                        </td>
                        <td class="px-6 py-4 whitespace-nowrap">
                            {{if eq .Citations 0}}
                            <span class="text-sm text-gray-500">N/A</span>
                            {{else}}
                            <span class="text-sm text-gray-900">{{.Citations}}</span>
                            {{end}}
                        </td>
                        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                            <a href="{{.URL}}" target="_blank" class="text-blue-600 hover:text-blue-900 mr-2">Paper</a>
                            {{if .ArxivAbsURL}}
                            <a href="{{.ArxivAbsURL}}" target="_blank" class="text-blue-600 hover:text-blue-900 mr-2">arXiv</a>
                            {{end}}
                            {{if .GoogleScholarURL}}
                            <a href="{{.GoogleScholarURL}}" target="_blank" class="text-blue-600 hover:text-blue-900">Scholar</a>
                            {{end}}
                        </td>
                        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                            {{.LastUpdate}}
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</body>
</html>`

// initCache initializes the SQLite database for caching
func initCache(dbPath string) (*CacheDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS paper_cache (
		url TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		citations INTEGER,
		arxiv_abs_url TEXT,
		google_scholar_url TEXT,
		arxiv_summary TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &CacheDB{db: db}, nil
}

// close closes the database connection
func (c *CacheDB) close() error {
	return c.db.Close()
}

// getCitation retrieves a paper from cache by URL
func (c *CacheDB) getCitation(url string) (*Paper, error) {
	query := `SELECT title, citations, arxiv_abs_url, google_scholar_url, arxiv_summary FROM paper_cache WHERE url = ?`

	var title string
	var citations sql.NullInt64
	var arxivAbsURL sql.NullString
	var googleScholarURL sql.NullString
	var arxivSummary sql.NullString

	err := c.db.QueryRow(query, url).Scan(&title, &citations, &arxivAbsURL, &googleScholarURL, &arxivSummary)

	if err != nil {
		if err == sql.ErrNoRows {
			// Not in cache
			return nil, nil
		}
		return nil, err
	}

	paper := &Paper{
		Title:            title,
		URL:              url,
		ArxivAbsURL:      arxivAbsURL.String,
		GoogleScholarURL: googleScholarURL.String,
		ArxivSummary:     arxivSummary.String,
	}

	if citations.Valid {
		paper.Citations = int(citations.Int64)
	}

	return paper, nil
}

// saveCitation saves a paper to the cache
func (c *CacheDB) saveCitation(paper Paper) error {
	// Insert or replace existing entry
	query := `INSERT OR REPLACE INTO paper_cache (url, title, citations, arxiv_abs_url, google_scholar_url, arxiv_summary) VALUES (?, ?, ?, ?, ?, ?)`

	var citationValue interface{}
	if paper.Citations != 0 {
		citationValue = paper.Citations
	} else {
		citationValue = nil
	}

	_, err := c.db.Exec(query, paper.URL, paper.Title, citationValue, paper.ArxivAbsURL, paper.GoogleScholarURL, paper.ArxivSummary)
	return err
}

// parseMarkdownPapers extracts paper information from a markdown file
func parseMarkdownPapers(filePath string) []Paper {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	var papers []Paper
	scanner := bufio.NewScanner(file)

	// Regular expression to extract paper title and URL
	// Matches simplified markdown format: "- Title [[paper](url)]"
	titleRegex := regexp.MustCompile(`-\s+([^\[]+)\[\[paper\]\(([^)]+)\)`)

	for scanner.Scan() {
		line := scanner.Text()

		matches := titleRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			title := strings.TrimSpace(matches[1])
			url := strings.TrimSpace(matches[2])

			papers = append(papers, Paper{
				Title: title,
				URL:   url,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	return papers
}

// processArxivPaper fetches citations for an arXiv paper by following the Google Scholar link
func processArxivPaper(paper *Paper) error {
	// Store the abstract URL
	paper.ArxivAbsURL = paper.URL

	// Get arXiv summary if available
	summary, err := GetArxivSummary(paper.ArxivAbsURL)
	if err == nil && summary != "" {
		paper.ArxivSummary = summary
	}

	// Delegate to the scholar package
	scholarURL, err := GetGoogleScholarURL(paper.ArxivAbsURL)
	if err != nil {
		// Store URLs but leave citations as nil
		paper.GoogleScholarURL = scholarURL
		return nil
	}

	if scholarURL == "" {
		// No Google Scholar link found, try to construct one from the arXiv ID
		arxivID := GetArxivID(paper.ArxivAbsURL)
		if arxivID != "" {
			scholarURL = GetDirectScholarURL(arxivID)
			paper.GoogleScholarURL = scholarURL
		} else {
			// Still couldn't get a scholar URL
			return fmt.Errorf("couldn't construct Google Scholar URL")
		}
	} else {
		// Store the Google Scholar URL
		paper.GoogleScholarURL = scholarURL
	}

	// Now get the citation count
	citationPtr, err := FetchCitationsFromScholar(paper.GoogleScholarURL)
	if err != nil || citationPtr == nil {
		// Unable to fetch citations, leave it as nil
		return nil
	}

	paper.Citations = *citationPtr
	return nil
}

// processNonArxivPaper searches Google Scholar directly using the paper title
func processNonArxivPaper(paper *Paper) error {
	// If it's an arXiv URL, store the abstract URL and get the summary
	if IsArxivURL(paper.URL) {
		if IsArxivPDF(paper.URL) {
			paper.ArxivAbsURL = ConvertPDFtoAbsURL(paper.URL)
		} else {
			paper.ArxivAbsURL = paper.URL
		}

		// Try to get the arXiv summary if we have an abs URL
		if paper.ArxivAbsURL != "" {
			summary, err := GetArxivSummary(paper.ArxivAbsURL)
			if err == nil && summary != "" {
				paper.ArxivSummary = summary
			}
		}
	}

	// Search Google Scholar by title
	scholarURL, citationPtr, _ := SearchGoogleScholar(paper.Title)

	// Store the Google Scholar URL
	paper.GoogleScholarURL = scholarURL

	// Store citation count if available
	if citationPtr != nil {
		paper.Citations = *citationPtr
	}
	// Otherwise citations will remain nil

	return nil
}
