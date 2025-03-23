package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessMarkdownFile(t *testing.T) {
	// Create a temporary test markdown file
	content := `# Test Papers

[Test Paper 1](https://arxiv.org/abs/2301.12345)
[Test Paper 2](https://aclanthology.org/2023.acl-long.123)`

	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test.md")
	err := os.WriteFile(inputFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test processing the file
	papers, err := processMarkdownFile(inputFile)
	if err != nil {
		t.Fatalf("processMarkdownFile failed: %v", err)
	}

	// Check number of papers
	if len(papers) != 2 {
		t.Fatalf("Expected 2 papers, got %d", len(papers))
	}

	// Check first paper
	if papers[0].Title != "Test Paper 1" {
		t.Errorf("Expected title 'Test Paper 1', got %q", papers[0].Title)
	}
	if papers[0].URL != "https://arxiv.org/abs/2301.12345" {
		t.Errorf("Expected URL 'https://arxiv.org/abs/2301.12345', got %q", papers[0].URL)
	}

	// Check second paper
	if papers[1].Title != "Test Paper 2" {
		t.Errorf("Expected title 'Test Paper 2', got %q", papers[1].Title)
	}
	if papers[1].URL != "https://aclanthology.org/2023.acl-long.123" {
		t.Errorf("Expected URL 'https://aclanthology.org/2023.acl-long.123', got %q", papers[1].URL)
	}
}

func TestProcessArxivPaper(t *testing.T) {
	// Create a test paper
	paper := &Paper{
		Title: "Test Paper",
		URL:   "https://arxiv.org/abs/2301.12345",
	}

	// Test processing the paper
	err := processArxivPaper(paper)
	if err != nil {
		t.Fatalf("processArxivPaper failed: %v", err)
	}

	// Check that the paper was processed
	if paper.Processed {
		t.Error("Paper should not be marked as processed without actual processing")
	}
}

func TestProcessNonArxivPaper(t *testing.T) {
	// Create a test paper
	paper := &Paper{
		Title: "Test Paper",
		URL:   "https://aclanthology.org/2023.acl-long.123",
	}

	// Test processing the paper
	err := processNonArxivPaper(paper)
	if err != nil {
		t.Fatalf("processNonArxivPaper failed: %v", err)
	}

	// Check that the paper was processed
	if paper.Processed {
		t.Error("Paper should not be marked as processed without actual processing")
	}
}

func TestGetCitation(t *testing.T) {
	// Create a temporary database file
	tmpfile, err := os.CreateTemp("", "testdb-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Initialize cache with temporary database
	cacheDB, err = sql.Open("sqlite3", tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cacheDB.Close()

	// Create the table
	_, err = cacheDB.Exec(`
		CREATE TABLE IF NOT EXISTS citations (
			url TEXT PRIMARY KEY,
			citations INTEGER,
			abstract TEXT,
			last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Test getting citation for non-existent paper
	citations, err := getCitation("https://example.com/paper")
	if err != nil {
		t.Fatalf("getCitation failed: %v", err)
	}
	if citations != nil {
		t.Error("Expected nil citations for non-existent paper")
	}

	// Test getting citation for existing paper
	err = saveCitation("https://example.com/paper", 42, "Test abstract")
	if err != nil {
		t.Fatalf("saveCitation failed: %v", err)
	}

	citations, err = getCitation("https://example.com/paper")
	if err != nil {
		t.Fatalf("getCitation failed: %v", err)
	}
	if citations == nil {
		t.Fatal("Expected non-nil citations for existing paper")
	}
	if *citations != 42 {
		t.Errorf("Expected 42 citations, got %d", *citations)
	}
}

func TestSaveCitation(t *testing.T) {
	// Initialize cache
	err := initCache()
	if err != nil {
		t.Fatalf("initCache failed: %v", err)
	}
	defer closeCache()

	// Test saving citation
	err = saveCitation("https://example.com/paper", 42, "Test abstract")
	if err != nil {
		t.Fatalf("saveCitation failed: %v", err)
	}

	// Verify citation was saved
	citations, err := getCitation("https://example.com/paper")
	if err != nil {
		t.Fatalf("getCitation failed: %v", err)
	}
	if citations == nil {
		t.Fatal("Expected non-nil citations")
	}
	if *citations != 42 {
		t.Errorf("Expected 42 citations, got %d", *citations)
	}
}

func TestSortPapersByCitations(t *testing.T) {
	// Create test papers
	papers := []Paper{
		{Title: "Paper 1", Citations: nil},
		{Title: "Paper 2", Citations: intPtr(10)},
		{Title: "Paper 3", Citations: intPtr(5)},
		{Title: "Paper 4", Citations: intPtr(20)},
	}

	// Sort papers
	sortPapersByCitations(papers)

	// Check sorting order
	expected := []string{"Paper 4", "Paper 2", "Paper 3", "Paper 1"}
	for i, paper := range papers {
		if paper.Title != expected[i] {
			t.Errorf("Expected paper %q at position %d, got %q", expected[i], i, paper.Title)
		}
	}
}

// Helper function to create pointer to int
func intPtr(i int) *int {
	return &i
}
