package main

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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
func FetchCitationsFromScholar(scholarURL string) (int, error) {
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

// SearchGoogleScholar searches Google Scholar directly for a paper title
func SearchGoogleScholar(title string) (int, error) {
	// Create Google Scholar search URL
	searchURL := fmt.Sprintf("https://scholar.google.com/scholar?q=%s", url.QueryEscape(title))

	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", searchURL, nil)
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
		return 0, fmt.Errorf("failed to fetch search results: %s", resp.Status)
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, err
	}

	// Look for citation count in the first search result
	var citations int

	// Find the first search result
	doc.Find(".gs_ri").First().Each(func(_ int, s *goquery.Selection) {
		// Look for "Cited by X" link
		s.Find(".gs_fl a").Each(func(_ int, link *goquery.Selection) {
			text := link.Text()
			if strings.HasPrefix(text, "Cited by") {
				citationText := strings.TrimPrefix(text, "Cited by ")
				count, err := strconv.Atoi(citationText)
				if err == nil {
					citations = count
				}
			}
		})
	})

	return citations, nil
}
