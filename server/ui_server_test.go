package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupTestDB(t *testing.T) (*sql.DB, string) {
	// Disable logging during tests
	log.SetOutput(ioutil.Discard)

	// Create a temporary database file
	tmpfile, err := os.CreateTemp("", "testdb-*.db")
	if err != nil {
		t.Fatal(err)
	}

	// Connect to the temporary database
	db, err := sql.Open("sqlite3", tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Create the paper_cache table
	_, err = db.Exec(`
		CREATE TABLE paper_cache (
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			citations INTEGER,
			arxiv_abs_url TEXT,
			google_scholar_url TEXT,
			timestamp TEXT NOT NULL,
			arxiv_summary TEXT
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert some test data
	_, err = db.Exec(`
		INSERT INTO paper_cache (title, url, citations, arxiv_abs_url, google_scholar_url, timestamp, arxiv_summary)
		VALUES
			('Test Paper 1', 'http://test1.com', 10, 'http://arxiv1.com', 'http://scholar1.com', '2024-03-23 10:00:00', 'This is a test paper about mining.'),
			('Test Paper 2', 'http://test2.com', 20, 'http://arxiv2.com', 'http://scholar2.com', '2024-03-23 10:00:00', 'This is another test paper about data mining.'),
			('Test Paper 3', 'http://test3.com', 30, 'http://arxiv3.com', 'http://scholar3.com', '2024-03-23 10:00:00', 'This is a third test paper about machine learning.')
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db, tmpfile.Name()
}

func TestHandleIndex(t *testing.T) {
	db, dbPath := setupTestDB(t)
	defer os.Remove(dbPath)
	defer db.Close()

	server, err := NewUIServer(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		checkContent   func(*testing.T, string)
	}{
		{
			name:           "basic page load",
			url:            "/",
			expectedStatus: http.StatusOK,
			checkContent: func(t *testing.T, content string) {
				if len(content) == 0 {
					t.Error("Expected non-empty content")
				}
			},
		},
		{
			name:           "search query",
			url:            "/?q=mining",
			expectedStatus: http.StatusOK,
			checkContent: func(t *testing.T, content string) {
				if len(content) == 0 {
					t.Error("Expected non-empty content")
				}
			},
		},
		{
			name:           "pagination",
			url:            "/?page=1",
			expectedStatus: http.StatusOK,
			checkContent: func(t *testing.T, content string) {
				if len(content) == 0 {
					t.Error("Expected non-empty content")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			server.handleIndex(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d; got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkContent != nil {
				tt.checkContent(t, w.Body.String())
			}
		})
	}
}

func TestHandlePapersAPI(t *testing.T) {
	db, dbPath := setupTestDB(t)
	defer os.Remove(dbPath)
	defer db.Close()

	server, err := NewUIServer(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		checkResponse  func(*testing.T, []byte)
	}{
		{
			name:           "basic papers request",
			url:            "/api/papers",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response struct {
					Papers []PaperView `json:"papers"`
					Count  int         `json:"count"`
				}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatal(err)
				}
				if len(response.Papers) == 0 {
					t.Error("Expected non-empty papers array")
				}
				if response.Count == 0 {
					t.Error("Expected non-zero count")
				}
			},
		},
		{
			name:           "search papers",
			url:            "/api/papers?q=mining",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response struct {
					Papers []PaperView `json:"papers"`
					Count  int         `json:"count"`
				}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatal(err)
				}
				if len(response.Papers) == 0 {
					t.Error("Expected non-empty papers array")
				}
				if response.Count == 0 {
					t.Error("Expected non-zero count")
				}
			},
		},
		{
			name:           "pagination",
			url:            "/api/papers?page=1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response struct {
					Papers      []PaperView `json:"papers"`
					CurrentPage int         `json:"currentPage"`
					TotalPages  int         `json:"totalPages"`
				}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatal(err)
				}
				if response.CurrentPage != 1 {
					t.Errorf("expected current page 1; got %d", response.CurrentPage)
				}
				if response.TotalPages == 0 {
					t.Error("Expected non-zero total pages")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			server.handlePapersAPI(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d; got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.Bytes())
			}
		})
	}
}

func TestGetPapers(t *testing.T) {
	db, dbPath := setupTestDB(t)
	defer os.Remove(dbPath)
	defer db.Close()

	server, err := NewUIServer(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		page          int
		pageSize      int
		searchQuery   string
		checkResults  func(*testing.T, []PaperView, int, error)
	}{
		{
			name:         "get all papers",
			page:         1,
			pageSize:     10,
			searchQuery:  "",
			checkResults: func(t *testing.T, papers []PaperView, total int, err error) {
				if err != nil {
					t.Fatal(err)
				}
				if len(papers) == 0 {
					t.Error("Expected non-empty papers array")
				}
				if total == 0 {
					t.Error("Expected non-zero total")
				}
			},
		},
		{
			name:         "search papers",
			page:         1,
			pageSize:     10,
			searchQuery:  "mining",
			checkResults: func(t *testing.T, papers []PaperView, total int, err error) {
				if err != nil {
					t.Fatal(err)
				}
				if len(papers) == 0 {
					t.Error("Expected non-empty papers array")
				}
				if total == 0 {
					t.Error("Expected non-zero total")
				}
			},
		},
		{
			name:         "pagination",
			page:         1,
			pageSize:     2,
			searchQuery:  "",
			checkResults: func(t *testing.T, papers []PaperView, total int, err error) {
				if err != nil {
					t.Fatal(err)
				}
				if len(papers) != 2 {
					t.Errorf("expected 2 papers; got %d", len(papers))
				}
				if total != 3 {
					t.Errorf("expected total 3; got %d", total)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			papers, total, err := server.getPapers(tt.page, tt.pageSize, tt.searchQuery)
			tt.checkResults(t, papers, total, err)
		})
	}
}
