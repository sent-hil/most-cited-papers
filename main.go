package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Paper represents a scientific paper with its title and citation count
type Paper struct {
	Title     string
	URL       string
	Citations int
}

func main() {
	// Check if file path is provided
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <path-to-markdown-file>")
		os.Exit(1)
	}

	filePath := os.Args[1]
	papers := parseMarkdownPapers(filePath)

	// Process each paper to get citation count
	for i := range papers {
		fmt.Printf("Processing: %s\n", papers[i].Title)

		// Add a delay to avoid being rate-limited
		time.Sleep(2 * time.Second)

		err := getScholarCitations(&papers[i])
		if err != nil {
			log.Printf("Error processing '%s': %v\n", papers[i].Title, err)
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
	// Matches markdown list items with title and paper link
	titleRegex := regexp.MustCompile(`-\s+\([^)]+\)\s+([^\[]+)\[\[paper\]\(([^)]+)\)`)

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

// getScholarCitations fetches the citation count for a paper
func getScholarCitations(paper *Paper) error {
	// Step 1: Fetch the paper page
	scholarURL, err := getGoogleScholarURL(paper.URL)
	if err != nil {
		return fmt.Errorf("failed to get Google Scholar URL: %v", err)
	}

	if scholarURL == "" {
		return fmt.Errorf("Google Scholar link not found")
	}

	// Step 2: Fetch the Google Scholar page
	citations, err := fetchCitationsFromScholar(scholarURL)
	if err != nil {
		return err
	}

	paper.Citations = citations
	return nil
}

// getGoogleScholarURL fetches the paper page and extracts the Google Scholar URL
func getGoogleScholarURL(paperURL string) (string, error) {
	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", paperURL, nil)
	if err != nil {
		return "", err
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch paper page: %s", resp.Status)
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Different sites have different ways to link to Google Scholar
	// Try different selector patterns

	// ACL Anthology pattern
	scholarURL, exists := doc.Find("a[href*='scholar.google.com']").Attr("href")
	if exists {
		return scholarURL, nil
	}

	// arXiv pattern
	scholarURL, exists = doc.Find("a.gs").Attr("href")
	if exists {
		return scholarURL, nil
	}

	// Some arXiv pages have a different pattern
	scholarURL, exists = doc.Find("a[href*='scholar.google']").Attr("href")
	if exists {
		return scholarURL, nil
	}

	// If direct link isn't found, we can try to construct it for arXiv
	if strings.Contains(paperURL, "arxiv.org") {
		// Extract arXiv ID
		idRegex := regexp.MustCompile(`arxiv\.org/abs/([0-9v.]+)`)
		matches := idRegex.FindStringSubmatch(paperURL)
		if len(matches) >= 2 {
			arxivID := matches[1]
			// Construct Scholar URL with arXiv ID
			return fmt.Sprintf("https://scholar.google.com/scholar?q=arxiv:%s", arxivID), nil
		}
	}

	return "", nil
}

// fetchCitationsFromScholar gets the citation count from a Google Scholar page
func fetchCitationsFromScholar(scholarURL string) (int, error) {
	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", scholarURL, nil)
	if err != nil {
		return 0, err
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("failed to fetch Google Scholar page: %s", resp.Status)
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, err
	}

	// Look for citation count on the page
	var citations int

	// Pattern: "Cited by X" link
	doc.Find(".gs_fl a").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()
		if strings.HasPrefix(text, "Cited by") {
			citationText := strings.TrimPrefix(text, "Cited by ")
			count, err := strconv.Atoi(citationText)
			if err == nil {
				citations = count
			}
		}
	})

	return citations, nil
}

