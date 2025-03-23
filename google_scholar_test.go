package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// searchURL is used to override the Google Scholar search URL in tests
var searchURL = "https://scholar.google.com/scholar"

func TestGetGoogleScholarURL(t *testing.T) {
	// Create a mock server for ACL page
	aclServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
			<body>
				<div class="card-body">
					<a href="https://scholar.google.com/scholar?cites=123456789">Cited by 42</a>
				</div>
			</body>
		</html>`
		w.Write([]byte(html))
	}))
	defer aclServer.Close()

	// Test ACL URL
	url, err := GetGoogleScholarURL(aclServer.URL)
	if err != nil {
		t.Fatalf("GetGoogleScholarURL failed for ACL URL: %v", err)
	}
	if url != "https://scholar.google.com/scholar?cites=123456789" {
		t.Errorf("Expected URL 'https://scholar.google.com/scholar?cites=123456789', got %q", url)
	}

	// Create a mock server for arXiv page
	arxivServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
			<body>
				<div class="gs_r">
					<div class="gs_a">
						<a href="https://scholar.google.com/scholar?cites=987654321">Cited by 24</a>
					</div>
				</div>
			</body>
		</html>`
		w.Write([]byte(html))
	}))
	defer arxivServer.Close()

	// Test arXiv URL
	url, err = GetGoogleScholarURL(arxivServer.URL)
	if err != nil {
		t.Fatalf("GetGoogleScholarURL failed for arXiv URL: %v", err)
	}
	if url != "https://scholar.google.com/scholar?cites=987654321" {
		t.Errorf("Expected URL 'https://scholar.google.com/scholar?cites=987654321', got %q", url)
	}

	// Test rate limiting
	rateLimitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("Rate limited"))
	}))
	defer rateLimitServer.Close()

	_, err = GetGoogleScholarURL(rateLimitServer.URL)
	if err == nil {
		t.Error("Expected error for rate limiting, got nil")
	}
	if !strings.Contains(err.Error(), "Rate limited") {
		t.Errorf("Expected error containing 'Rate limited', got %v", err)
	}
}

func TestFetchCitationsFromScholar(t *testing.T) {
	// Create test server for paper with citations
	citedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<body>
					<div class="gs_fl">
						<a href="#">Cited by 42</a>
					</div>
				</body>
			</html>
		`))
	}))
	defer citedServer.Close()

	// Test paper with citations
	citations, err := FetchCitationsFromScholar(citedServer.URL)
	if err != nil {
		t.Fatalf("FetchCitationsFromScholar failed for cited paper: %v", err)
	}
	if citations == nil {
		t.Fatal("FetchCitationsFromScholar returned nil for cited paper")
	}
	if *citations != 42 {
		t.Errorf("FetchCitationsFromScholar citations = %d; want %d", *citations, 42)
	}

	// Create test server for paper with no citations
	uncitedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<body>
					<div class="gs_or_cit">
						<span>Cite</span>
					</div>
				</body>
			</html>
		`))
	}))
	defer uncitedServer.Close()

	// Test paper with no citations
	citations, err = FetchCitationsFromScholar(uncitedServer.URL)
	if err != nil {
		t.Fatalf("FetchCitationsFromScholar failed for uncited paper: %v", err)
	}
	if citations == nil {
		t.Fatal("FetchCitationsFromScholar returned nil for uncited paper")
	}
	if *citations != 0 {
		t.Errorf("FetchCitationsFromScholar citations = %d; want %d", *citations, 0)
	}

	// Test rate limiting
	rateLimitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer rateLimitServer.Close()

	_, err = FetchCitationsFromScholar(rateLimitServer.URL)
	if err == nil {
		t.Error("FetchCitationsFromScholar should fail on rate limit")
	}
}

func TestSearchGoogleScholar(t *testing.T) {
	// Create a mock server for Google Scholar search
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
			<body>
				<div class="gs_ri">
					<div class="gs_rt">
						<a href="https://example.com/paper">Test Paper Title</a>
					</div>
					<div class="gs_fl">
						<a href="#">Cited by 42</a>
					</div>
					<div class="gs_fma_snp">
						This is a test abstract for the paper.
					</div>
				</div>
			</body>
		</html>`
		w.Write([]byte(html))
	}))
	defer server.Close()

	// Test successful case
	url, citations, abstract, err := SearchGoogleScholar("Test Paper Title", []string{"John Doe"}, server.URL)
	if err != nil {
		t.Fatalf("SearchGoogleScholar failed: %v", err)
	}
	if url != "https://example.com/paper" {
		t.Errorf("Expected URL 'https://example.com/paper', got %q", url)
	}
	if citations == nil {
		t.Error("Expected non-nil citations")
	}
	if *citations != 42 {
		t.Errorf("Expected 42 citations, got %d", *citations)
	}
	if abstract != "This is a test abstract for the paper." {
		t.Errorf("Expected abstract 'This is a test abstract for the paper.', got %q", abstract)
	}

	// Test rate limiting case
	rateLimitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("Rate limited"))
	}))
	defer rateLimitServer.Close()

	_, _, _, err = SearchGoogleScholar("Test Paper Title", []string{"John Doe"}, rateLimitServer.URL)
	if err == nil {
		t.Error("Expected error for rate limiting, got nil")
	}
}
