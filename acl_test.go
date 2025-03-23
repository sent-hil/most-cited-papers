package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsACLURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Valid ACL URL",
			url:      "https://aclanthology.org/2023.acl-long.123",
			expected: true,
		},
		{
			name:     "Valid ACL URL with www",
			url:      "https://www.aclanthology.org/2023.acl-long.123",
			expected: true,
		},
		{
			name:     "Non-ACL URL",
			url:      "https://arxiv.org/abs/2301.12345",
			expected: false,
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsACLURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsACLURL(%q) = %v; want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestGetACLInfo(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a mock ACL Anthology page
		html := `
		<html>
			<head>
				<title>Test Paper - ACL Anthology</title>
			</head>
			<body>
				<div class="acl-abstract">
					<p>This is a test abstract for the paper.</p>
				</div>
				<div class="acl-authors">
					<span>John Doe</span>
					<span>Jane Smith</span>
				</div>
			</body>
		</html>`
		w.Write([]byte(html))
	}))
	defer server.Close()

	// Test successful case
	abstract, authors, err := GetACLInfo(server.URL)
	if err != nil {
		t.Fatalf("GetACLInfo failed: %v", err)
	}
	if abstract != "This is a test abstract for the paper." {
		t.Errorf("Expected abstract 'This is a test abstract for the paper.', got %q", abstract)
	}
	if len(authors) != 2 {
		t.Fatalf("Expected 2 authors, got %d", len(authors))
	}
	if authors[0] != "John Doe" {
		t.Errorf("Expected first author 'John Doe', got %q", authors[0])
	}
	if authors[1] != "Jane Smith" {
		t.Errorf("Expected second author 'Jane Smith', got %q", authors[1])
	}

	// Test rate limiting case
	rateLimitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("Rate limited"))
	}))
	defer rateLimitServer.Close()

	_, _, err = GetACLInfo(rateLimitServer.URL)
	if err == nil {
		t.Error("Expected error for rate limiting, got nil")
	}
	if !strings.Contains(err.Error(), "Rate limited") {
		t.Errorf("Expected error containing 'Rate limited', got %v", err)
	}
}

func TestGetACLAbstract(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<body>
					<div class="card-body acl-abstract">
						<span>Test abstract</span>
					</div>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	abstract, err := GetACLAbstract(server.URL)
	if err != nil {
		t.Fatalf("GetACLAbstract failed: %v", err)
	}

	expectedAbstract := "Test abstract"
	if abstract != expectedAbstract {
		t.Errorf("GetACLAbstract = %q; want %q", abstract, expectedAbstract)
	}
}

func TestGetACLAuthors(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<body>
					<div class="acl-authors">
						<span>Author 1</span>
						<span>Author 2</span>
					</div>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	authors, err := GetACLAuthors(server.URL)
	if err != nil {
		t.Fatalf("GetACLAuthors failed: %v", err)
	}

	expectedAuthors := []string{"Author 1", "Author 2"}
	if len(authors) != len(expectedAuthors) {
		t.Fatalf("GetACLAuthors length = %d; want %d", len(authors), len(expectedAuthors))
	}
	for i, author := range authors {
		if author != expectedAuthors[i] {
			t.Errorf("GetACLAuthors[%d] = %q; want %q", i, author, expectedAuthors[i])
		}
	}
}
