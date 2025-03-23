package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// GetACLInfo fetches both the abstract and authors from an ACL Anthology page
func GetACLInfo(aclURL string) (string, []string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the request
	resp, err := client.Get(aclURL)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch ACL page: %v", err)
	}
	defer resp.Body.Close()

	// Check for rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", nil, fmt.Errorf("Rate limited by ACL Anthology. Please wait a few minutes before trying again.")
	}

	// Check for other errors
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("failed to fetch ACL page: status code %d", resp.StatusCode)
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	// Extract abstract
	var abstract string
	doc.Find(".acl-abstract").Each(func(i int, s *goquery.Selection) {
		abstract = strings.TrimSpace(s.Text())
	})

	// Extract authors
	var authors []string
	doc.Find(".acl-authors span").Each(func(i int, s *goquery.Selection) {
		author := strings.TrimSpace(s.Text())
		if author != "" {
			authors = append(authors, author)
		}
	})

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
