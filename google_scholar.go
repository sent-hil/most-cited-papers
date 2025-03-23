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

	// If we couldn't find a direct link, try to construct one from the title
	title := doc.Find("title").Text()
	if title != "" {
		return fmt.Sprintf("https://scholar.google.com/scholar?q=%s", url.QueryEscape(title)), nil
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

	// If we found a "Cite" button but no citations, return 0
	if citationPtr == nil {
		doc.Find(".gs_or_cit").Each(func(_ int, s *goquery.Selection) {
			if s.Find("span").Text() == "Cite" {
				zero := 0
				citationPtr = &zero
			}
		})
	}

	return citationPtr, nil
}

// SearchGoogleScholar searches Google Scholar directly for a paper title
// Returns the Google Scholar URL, citation count, and abstract (if available)
func SearchGoogleScholar(title string, authors []string) (string, *int, string, error) {
	// Create Google Scholar search URL with title in quotes and authors
	searchQuery := fmt.Sprintf("\"%s\"", title)
	if len(authors) > 0 {
		searchQuery = fmt.Sprintf("%s author:\"%s\"", searchQuery, authors[0])
	}
	searchURL := fmt.Sprintf("https://scholar.google.com/scholar?q=%s", url.QueryEscape(searchQuery))
	debugf("GET %s", searchURL)

	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return searchURL, nil, "", nil
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return searchURL, nil, "", nil
	}
	defer resp.Body.Close()

	debugf("Response: %s", resp.Status)
	if resp.StatusCode == 429 {
		log.Fatalf("Rate limited by Google Scholar. Please wait a few minutes before trying again.")
	}
	if resp.StatusCode != 200 {
		return searchURL, nil, "", nil
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return searchURL, nil, "", nil
	}

	// Look for citation count and abstract in the search results
	var citationPtr *int
	var abstract string
	var bestMatchURL string
	var foundMatch bool

	// Find all search results
	results := doc.Find(".gs_ri")

	results.Each(func(_ int, s *goquery.Selection) {
		if foundMatch {
			return
		}

		// Get the paper title from the result
		resultTitle := strings.TrimSpace(s.Find(".gs_rt").Text())
		cleanResultTitle := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(resultTitle, "\"", "")))
		cleanSearchTitle := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(title, "\"", "")))

		// Check if titles match (allowing for some flexibility)
		if strings.Contains(cleanResultTitle, cleanSearchTitle) || strings.Contains(cleanSearchTitle, cleanResultTitle) {
			// Look for "Cited by X" link
			s.Find(".gs_fl a").Each(func(_ int, link *goquery.Selection) {
				text := link.Text()
				if strings.HasPrefix(text, "Cited by") {
					citationText := strings.TrimPrefix(text, "Cited by ")
					count, err := strconv.Atoi(citationText)
					if err == nil {
						citationPtr = &count
					}
				}
			})

			// If no citations found, check for "Cite" button
			if citationPtr == nil {
				s.Find(".gs_or_cit").Each(func(_ int, link *goquery.Selection) {
					if link.Find("span").Text() == "Cite" {
						zero := 0
						citationPtr = &zero
					}
				})
			}

			// Look for abstract in the snippet
			s.Find(".gs_fma_snp").Each(func(_ int, abs *goquery.Selection) {
				abstract = strings.TrimSpace(abs.Text())
			})

			// Get the paper URL
			s.Find(".gs_rt a").Each(func(_ int, link *goquery.Selection) {
				if href, exists := link.Attr("href"); exists {
					bestMatchURL = href
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
		citationPtr, _ = FetchCitationsFromScholar(bestMatchURL)
	}

	return searchURL, citationPtr, abstract, nil
}

// GetACLInfo fetches both the abstract and authors from an ACL Anthology page in a single request
func GetACLInfo(aclURL string) (string, []string, error) {
	debugf("GET %s", aclURL)

	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", aclURL, nil)
	if err != nil {
		return "", nil, err
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	debugf("Response: %s", resp.Status)
	if resp.StatusCode == 429 {
		log.Fatalf("Rate limited by ACL Anthology. Please wait a few minutes before trying again.")
	}
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("failed to fetch ACL page: %s", resp.Status)
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", nil, err
	}

	// Get abstract
	var abstract string
	abstractBlock := doc.Find("div.card-body.acl-abstract span")
	if abstractBlock.Length() > 0 {
		abstract = strings.TrimSpace(abstractBlock.Text())
	}

	// Get authors
	var authors []string
	doc.Find("p.lead a").Each(func(_ int, s *goquery.Selection) {
		author := strings.TrimSpace(s.Text())
		if author != "" {
			authors = append(authors, author)
		}
	})

	if len(authors) == 0 {
		return abstract, nil, fmt.Errorf("no authors found on ACL page")
	}

	return abstract, authors, nil
}

// GetACLAbstract fetches the abstract from an ACL Anthology page
func GetACLAbstract(aclURL string) (string, error) {
	abstract, _, err := GetACLInfo(aclURL)
	return abstract, err
}

// GetACLAuthors fetches the authors from an ACL Anthology page
func GetACLAuthors(aclURL string) ([]string, error) {
	_, authors, err := GetACLInfo(aclURL)
	return authors, err
}

// IsACLURL checks if a URL is from ACL Anthology
func IsACLURL(url string) bool {
	return strings.Contains(url, "aclanthology.org")
}
