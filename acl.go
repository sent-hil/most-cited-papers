package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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
