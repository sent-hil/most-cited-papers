package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Paper represents a scientific paper with its title and citation count
type Paper struct {
	Title     string
	URL       string
	Citations int
}

// CacheDB handles interactions with the SQLite cache
type CacheDB struct {
	db *sql.DB
}

func main() {
	// Check if file path is provided
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <path-to-markdown-file>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// Initialize cache
	cache, err := initCache("paper_cache.db")
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
		} else {
			// Add a delay to avoid being rate-limited
			time.Sleep(2 * time.Second)

			// Fetch new data
			if isArxivURL(papers[i].URL) {
				// For arXiv papers, try to follow links
				err := getArxivCitations(&papers[i])
				if err != nil {
					log.Printf("Error processing arXiv paper '%s': %v\n", papers[i].Title, err)
					// Fall back to direct search if following links fails
					err = searchScholarByTitle(&papers[i])
					if err != nil {
						log.Printf("Fallback search also failed for '%s': %v\n", papers[i].Title, err)
					}
				}
			} else {
				// For non-arXiv papers, directly search by title
				err := searchScholarByTitle(&papers[i])
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
		return papers[i].Citations > papers[j].Citations
	})

	// Print results
	fmt.Println("\nResults sorted by citation count:")
	fmt.Println("----------------------------------")
	for i, paper := range papers {
		fmt.Printf("%d. Title: %s\n   URL: %s\n   Citations: %d\n\n",
			i+1, paper.Title, paper.URL, paper.Citations)
	}
}

// isArxivURL checks if a URL is from arXiv
func isArxivURL(url string) bool {
	return strings.Contains(url, "arxiv.org")
}

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
		citations INTEGER NOT NULL,
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
	query := `SELECT title, citations FROM paper_cache WHERE url = ?`

	var title string
	var citations int
	err := c.db.QueryRow(query, url).Scan(&title, &citations)

	if err != nil {
		if err == sql.ErrNoRows {
			// Not in cache
			return nil, nil
		}
		return nil, err
	}

	return &Paper{
		Title:     title,
		URL:       url,
		Citations: citations,
	}, nil
}

// saveCitation saves a paper to the cache
func (c *CacheDB) saveCitation(paper Paper) error {
	// Insert or replace existing entry
	query := `INSERT OR REPLACE INTO paper_cache (url, title, citations) VALUES (?, ?, ?)`

	_, err := c.db.Exec(query, paper.URL, paper.Title, paper.Citations)
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

// getArxivCitations fetches citations by accessing arXiv and following Google Scholar link
func getArxivCitations(paper *Paper) error {
	// Delegate to the scholar package
	scholarURL, err := GetGoogleScholarURL(paper.URL)
	if err != nil {
		return fmt.Errorf("failed to get Google Scholar URL: %v", err)
	}

	if scholarURL == "" {
		return fmt.Errorf("Google Scholar link not found")
	}

	citations, err := FetchCitationsFromScholar(scholarURL)
	if err != nil {
		return err
	}

	paper.Citations = citations
	return nil
}

// searchScholarByTitle searches Google Scholar directly using the paper title
func searchScholarByTitle(paper *Paper) error {
	citations, err := SearchGoogleScholar(paper.Title)
	if err != nil {
		return err
	}

	paper.Citations = citations
	return nil
}
