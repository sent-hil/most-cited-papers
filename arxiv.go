package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// IsArxivURL checks if a URL is from arXiv
func IsArxivURL(url string) bool {
	return strings.Contains(url, "arxiv.org")
}

// IsArxivPDF checks if a URL is from arXiv and points to a PDF
func IsArxivPDF(url string) bool {
	return strings.Contains(url, "arxiv.org/pdf")
}

// ConvertPDFtoAbsURL converts an arXiv PDF URL to an abstract URL
func ConvertPDFtoAbsURL(pdfURL string) string {
	// Replace /pdf/ with /abs/
	absURL := strings.Replace(pdfURL, "/pdf/", "/abs/", 1)

	// Remove .pdf extension if present
	absURL = strings.TrimSuffix(absURL, ".pdf")

	return absURL
}

// GetArxivID extracts the arXiv ID from a URL
func GetArxivID(arxivURL string) string {
	// Extract arXiv ID
	idRegex := regexp.MustCompile(`arxiv\.org/(?:abs|pdf)/([0-9v.]+)`)
	matches := idRegex.FindStringSubmatch(arxivURL)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// GetArxivSummary fetches the abstract/summary from an arXiv page
func GetArxivSummary(arxivURL string) (string, error) {
	fmt.Printf("Fetching abstract from: %s\n", arxivURL)

	// Create HTTP client
	client := &http.Client{}
	req, err := http.NewRequest("GET", arxivURL, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return "", err
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("Bad status code: %d\n", resp.StatusCode)
		return "", fmt.Errorf("failed to fetch arXiv page: %s", resp.Status)
	}

	// Parse the HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Printf("Error parsing HTML: %v\n", err)
		return "", err
	}

	// Find the abstract using the correct selector
	abstractBlock := doc.Find("blockquote.abstract.mathjax")
	if abstractBlock.Length() > 0 {
		// Remove the descriptor span first
		abstractBlock.Find("span.descriptor").Remove()

		// Get the text content and clean it
		summary := abstractBlock.Text()
		summary = strings.TrimSpace(summary)
		fmt.Printf("Successfully found abstract, length: %d\n", len(summary))
		return summary, nil
	}

	fmt.Printf("No abstract found with selector 'blockquote.abstract.mathjax'\n")
	// If we reach here, we couldn't find the abstract
	return "", fmt.Errorf("abstract not found on page")
}

// GetDirectScholarURL constructs a direct Google Scholar URL for an arXiv paper
func GetDirectScholarURL(arxivID string) string {
	return fmt.Sprintf("https://scholar.google.com/scholar?q=arxiv:%s", arxivID)
}
