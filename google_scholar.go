package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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

// GetGoogleScholarURL extracts the Google Scholar URL from a paper's page
func GetGoogleScholarURL(url string) (string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the request
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch page: %v", err)
	}
	defer resp.Body.Close()

	// Check for rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("Rate limited by %s. Please wait a few minutes before trying again.", url)
	}

	// Check for other errors
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch page: status code %d", resp.StatusCode)
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %v", err)
	}

	// Try to find the Google Scholar URL in different formats
	var scholarURL string

	// Try ACL Anthology format
	doc.Find("div.card-body a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists && strings.Contains(href, "scholar.google.com") {
			scholarURL = href
		}
	})

	// Try arXiv format
	if scholarURL == "" {
		doc.Find("div.gs_r div.gs_a a").Each(func(i int, s *goquery.Selection) {
			if href, exists := s.Attr("href"); exists && strings.Contains(href, "scholar.google.com") {
				scholarURL = href
			}
		})
	}

	if scholarURL == "" {
		return "", fmt.Errorf("no Google Scholar URL found on page")
	}

	return scholarURL, nil
}

// FetchCitationsFromScholar gets the citation count from a Google Scholar page
// Returns nil for citations if not found or error
func FetchCitationsFromScholar(scholarURL string) (*int, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the request
	resp, err := client.Get(scholarURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %v", err)
	}
	defer resp.Body.Close()

	// Check for rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("Rate limited by Google Scholar. Please wait a few minutes before trying again.")
	}

	// Check for other errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch page: status code %d", resp.StatusCode)
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
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
func SearchGoogleScholar(title string, authors []string, baseURL string) (string, *int, string, error) {
	// Create Google Scholar search URL with title in quotes and authors
	searchQuery := fmt.Sprintf("\"%s\"", title)
	if len(authors) > 0 {
		searchQuery = fmt.Sprintf("%s author:\"%s\"", searchQuery, authors[0])
	}
	requestURL := fmt.Sprintf("%s?q=%s", baseURL, url.QueryEscape(searchQuery))
	debugf("GET %s", requestURL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the request
	resp, err := client.Get(requestURL)
	if err != nil {
		return requestURL, nil, "", fmt.Errorf("failed to fetch page: %v", err)
	}
	defer resp.Body.Close()

	debugf("Response: %s", resp.Status)
	if resp.StatusCode == http.StatusTooManyRequests {
		return requestURL, nil, "", fmt.Errorf("Rate limited by Google Scholar. Please wait a few minutes before trying again.")
	}
	if resp.StatusCode != http.StatusOK {
		return requestURL, nil, "", fmt.Errorf("failed to fetch page: status code %d", resp.StatusCode)
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return requestURL, nil, "", fmt.Errorf("failed to parse HTML: %v", err)
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
		requestURL = bestMatchURL
	}

	// If we found a match but no citation count, try to get it from the paper's page
	if foundMatch && citationPtr == nil && bestMatchURL != "" {
		citationPtr, _ = FetchCitationsFromScholar(bestMatchURL)
	}

	return requestURL, citationPtr, abstract, nil
}
