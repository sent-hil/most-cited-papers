package main

import (
	"bufio"
	"database/sql"
	"flag"
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
	// Parse command line flags
	inputFile := flag.String("input", "", "Input markdown file containing paper titles")
	outputFile := flag.String("output", "", "Output markdown file (default: input with -with-citations suffix)")
	force := flag.Bool("force", false, "Force a fresh search, bypassing cache")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Set debug mode for Google Scholar functions
	SetDebugMode(*debug)

	// Print usage if no input file specified
	if *inputFile == "" {
		fmt.Println("Usage: go run *.go -input <input.md> [-output <output.md>] [-force] [-debug]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Initialize cache
	cache, err := initCache("paper_cache.db")
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.close()

	papers := parseMarkdownPapers(*inputFile)

	// Process each paper to get citation count
	for i := range papers {
		fmt.Printf("Processing: %s\n", papers[i].Title)

		// Check if we have this paper in cache and force flag is not set
		var cached *Paper
		if !*force {
			cached, err = cache.getCitation(papers[i].URL)
			if err != nil {
				log.Printf("Error checking cache for '%s': %v\n", papers[i].URL, err)
			}
		}

		if cached != nil && !*force {
			// Use cached data
			if *debug {
				fmt.Printf("  Using cached data for %s\n", papers[i].URL)
			}
			papers[i].Citations = cached.Citations
			papers[i].ArxivAbsURL = cached.ArxivAbsURL
			papers[i].GoogleScholarURL = cached.GoogleScholarURL
			papers[i].ArxivSummary = cached.ArxivSummary
		} else {
			if *force {
				fmt.Printf("  Force flag set, performing fresh search for %s\n", papers[i].URL)
			}
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
				if *debug {
					log.Printf("Using title search for '%s'\n", papers[i].Title)
				}
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
	// Try to get abstract from different sources in order of preference
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
	} else if IsACLURL(paper.URL) {
		// Try to get abstract from ACL Anthology
		summary, err := GetACLAbstract(paper.URL)
		if err == nil && summary != "" {
			paper.ArxivSummary = summary
		}
	}

	// Get authors from the appropriate source
	authors := []string{}
	if IsACLURL(paper.URL) {
		// Try to get authors from ACL page
		aclAuthors, err := GetACLAuthors(paper.URL)
		if err == nil && len(aclAuthors) > 0 {
			authors = aclAuthors
		}
	}

	// If we couldn't get authors from ACL, try to extract from title
	if len(authors) == 0 {
		titleParts := strings.Split(paper.Title, " - ")
		if len(titleParts) > 1 {
			authorPart := titleParts[0]
			// Handle "et al." case
			if strings.Contains(authorPart, "et al.") {
				authors = append(authors, strings.TrimSpace(strings.ReplaceAll(authorPart, "et al.", "")))
			} else {
				// Handle multiple authors case
				authorList := strings.Split(authorPart, ",")
				for _, author := range authorList {
					author = strings.TrimSpace(author)
					// Remove "and" from the last author
					author = strings.TrimPrefix(author, "and ")
					if author != "" {
						authors = append(authors, author)
					}
				}
			}
		}
	}

	// Search Google Scholar by title and authors
	scholarURL, citationPtr, scholarAbstract, _ := SearchGoogleScholar(paper.Title, authors)

	// Store the Google Scholar URL
	paper.GoogleScholarURL = scholarURL

	// Store citation count if available
	if citationPtr != nil {
		paper.Citations = *citationPtr
	}

	// If we don't have an abstract from other sources but got one from Google Scholar, use that
	if paper.ArxivSummary == "" && scholarAbstract != "" {
		paper.ArxivSummary = scholarAbstract
	}

	return nil
}
