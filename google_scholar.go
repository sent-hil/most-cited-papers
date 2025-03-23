package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var debugMode bool

// SetDebugMode sets whether debug logging is enabled
func SetDebugMode(enabled bool) {
	debugMode = enabled
}

// debugf prints debug messages only when debug mode is enabled
func debugf(format string, v ...interface{}) {
	if debugMode {
		fmt.Printf(format+"\n", v...)
	}
}

// GetGoogleScholarURL fetches the paper page and extracts the Google Scholar URL
func GetGoogleScholarURL(paperURL string) (string, error) {
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

	if resp.StatusCode == 429 {
		log.Fatalf("Rate limited by %s. Please wait a few minutes before trying again.", paperURL)
	}
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

// FetchCitationsFromScholar gets the citation count from a Google Scholar page
// Returns nil for citations if not found or error
func FetchCitationsFromScholar(scholarURL string) (*int, error) {
	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", scholarURL, nil)
	if err != nil {
		return nil, nil
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		log.Fatalf("Rate limited by Google Scholar. Please wait a few minutes before trying again.")
	}
	if resp.StatusCode != 200 {
		return nil, nil
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, nil
	}

	// Look for citation count on the page
	var citationPtr *int

	// Pattern: "Cited by X" link
	doc.Find(".gs_fl a").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()
		if strings.HasPrefix(text, "Cited by") {
			citationText := strings.TrimPrefix(text, "Cited by ")
			count, err := strconv.Atoi(citationText)
			if err == nil {
				citationPtr = &count
			}
		}
	})

	return citationPtr, nil
}

// SearchGoogleScholar searches Google Scholar directly for a paper title
// Returns the Google Scholar URL, citation count, and abstract (if available)
func SearchGoogleScholar(title string, authors []string) (string, *int, string, error) {
	// Create Google Scholar search URL with title in quotes and authors
	searchQuery := fmt.Sprintf("\"%s\"", title)
	if len(authors) > 0 {
		// Add first author to the search query
		searchQuery = fmt.Sprintf("%s author:\"%s\"", searchQuery, authors[0])
	}
	searchURL := fmt.Sprintf("https://scholar.google.com/scholar?q=%s", url.QueryEscape(searchQuery))
	debugf("Debug: Searching with query: %s", searchQuery)
	debugf("Debug: Full URL: %s", searchURL)

	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		debugf("Debug: Error creating request: %v", err)
		return searchURL, nil, "", nil
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		debugf("Debug: Error making request: %v", err)
		return searchURL, nil, "", nil
	}
	defer resp.Body.Close()

	debugf("Debug: Response status: %s", resp.Status)
	if resp.StatusCode == 429 {
		log.Fatalf("Rate limited by Google Scholar. Please wait a few minutes before trying again.")
	}
	if resp.StatusCode != 200 {
		debugf("Debug: Bad response status: %d", resp.StatusCode)
		return searchURL, nil, "", nil
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		debugf("Debug: Error parsing HTML: %v", err)
		return searchURL, nil, "", nil
	}

	// Look for citation count and abstract in the search results
	var citationPtr *int
	var abstract string
	var bestMatchURL string
	var foundMatch bool

	// Find all search results
	results := doc.Find(".gs_ri")
	debugf("Debug: Found %d search results", results.Length())

	results.Each(func(_ int, s *goquery.Selection) {
		if foundMatch {
			return
		}

		// Get the paper title from the result
		resultTitle := strings.TrimSpace(s.Find(".gs_rt").Text())
		debugf("Debug: Found result title: %s", resultTitle)

		// Clean up titles for comparison
		cleanResultTitle := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(resultTitle, "\"", "")))
		cleanSearchTitle := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(title, "\"", "")))
		debugf("Debug: Clean titles - Result: %s, Search: %s", cleanResultTitle, cleanSearchTitle)

		// Check if titles match (allowing for some flexibility)
		if strings.Contains(cleanResultTitle, cleanSearchTitle) || strings.Contains(cleanSearchTitle, cleanResultTitle) {
			debugf("Debug: Found title match!")

			// Look for "Cited by X" link
			s.Find(".gs_fl a").Each(func(_ int, link *goquery.Selection) {
				text := link.Text()
				debugf("Debug: Found link text: %s", text)
				if strings.HasPrefix(text, "Cited by") {
					citationText := strings.TrimPrefix(text, "Cited by ")
					count, err := strconv.Atoi(citationText)
					if err == nil {
						citationPtr = &count
						debugf("Debug: Found citation count: %d", count)
					} else {
						debugf("Debug: Error parsing citation count: %v", err)
					}
				}
			})

			// Look for abstract in the snippet
			s.Find(".gs_fma_snp").Each(func(_ int, abs *goquery.Selection) {
				abstract = strings.TrimSpace(abs.Text())
			})

			// Get the paper URL
			s.Find(".gs_rt a").Each(func(_ int, link *goquery.Selection) {
				if href, exists := link.Attr("href"); exists {
					bestMatchURL = href
					debugf("Debug: Found paper URL: %s", bestMatchURL)
				}
			})

			foundMatch = true
		}
	})

	// If we found a good match, use its URL, otherwise use the search URL
	if bestMatchURL != "" {
		searchURL = bestMatchURL
	}

	// If we found a match but no citation count, try to get it from the paper's page
	if foundMatch && citationPtr == nil && bestMatchURL != "" {
		debugf("Debug: No citation count found in search results, trying paper page")
		citationPtr, _ = FetchCitationsFromScholar(bestMatchURL)
		if citationPtr != nil {
			debugf("Debug: Found citation count on paper page: %d", *citationPtr)
		} else {
			debugf("Debug: No citation count found on paper page either")
		}
	}

	return searchURL, citationPtr, abstract, nil
}

// GetACLAbstract fetches the abstract from an ACL Anthology page
func GetACLAbstract(aclURL string) (string, error) {
	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", aclURL, nil)
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

	if resp.StatusCode == 429 {
		log.Fatalf("Rate limited by ACL Anthology. Please wait a few minutes before trying again.")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch ACL page: %s", resp.Status)
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Find the abstract using the ACL Anthology selector
	abstractBlock := doc.Find("div.card-body.acl-abstract span")
	if abstractBlock.Length() > 0 {
		summary := strings.TrimSpace(abstractBlock.Text())
		return summary, nil
	}

	return "", fmt.Errorf("abstract not found on ACL page")
}

// IsACLURL checks if a URL is from ACL Anthology
func IsACLURL(url string) bool {
	return strings.Contains(url, "aclanthology.org")
}

// GetACLAuthors fetches the authors from an ACL Anthology page
func GetACLAuthors(aclURL string) ([]string, error) {
	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", aclURL, nil)
	if err != nil {
		return nil, err
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		log.Fatalf("Rate limited by ACL Anthology. Please wait a few minutes before trying again.")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch ACL page: %s", resp.Status)
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Find the authors using the ACL Anthology selector
	var authors []string
	doc.Find("p.lead a").Each(func(_ int, s *goquery.Selection) {
		author := strings.TrimSpace(s.Text())
		if author != "" {
			authors = append(authors, author)
		}
	})

	if len(authors) == 0 {
		return nil, fmt.Errorf("no authors found on ACL page")
	}

	return authors, nil
}
