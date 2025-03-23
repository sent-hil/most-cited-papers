package main

const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Paper Citations</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        .citation-count {
            font-variant-numeric: tabular-nums;
            font-feature-settings: "tnum";
        }
        .abstract-text {
            font-size: 0.875rem;
            line-height: 1.6;
        }
        .abstract-container {
            margin-right: 60px;
        }
        .expand-button {
            color: #4b5563;
            cursor: pointer;
            font-size: 0.875rem;
            padding: 0.25rem 0.5rem;
            border-radius: 0.25rem;
            transition: background-color 0.2s;
        }
        .expand-button:hover {
            background-color: #f3f4f6;
        }
        .search-input {
            width: 300px;
        }
    </style>
    <script>
        // Debounce function
        function debounce(func, wait) {
            let timeout;
            return function executedFunction(...args) {
                const later = () => {
                    clearTimeout(timeout);
                    func(...args);
                };
                clearTimeout(timeout);
                timeout = setTimeout(later, wait);
            };
        }

        // Update URL with search parameters
        function updateURL(searchQuery, page) {
            const url = new URL(window.location);
            if (searchQuery) {
                url.searchParams.set('q', searchQuery);
            } else {
                url.searchParams.delete('q');
            }
            if (page && page > 1) {
                url.searchParams.set('page', page);
            } else {
                url.searchParams.delete('page');
            }
            window.history.pushState({}, '', url);
        }

        // Setup abstract expansion functionality
        function setupAbstractExpansion() {
            document.querySelectorAll('.abstract-container').forEach(container => {
                const button = container.querySelector('.expand-button');
                const firstSentence = container.querySelector('.text-gray-600');
                const fullText = container.querySelector('.abstract-text').textContent;

                if (button) {
                    button.addEventListener('click', function() {
                        if (firstSentence.textContent === fullText) {
                            // Collapse - show first sentence and ...
                            firstSentence.textContent = firstSentence.textContent.split('.')[0] + '.';
                            button.textContent = '...';
                        } else {
                            // Expand - show full text
                            firstSentence.textContent = fullText;
                            button.textContent = 'Show less';
                        }
                    });
                }
            });
        }

        // Perform search
        function performSearch(query) {
            const currentPage = new URLSearchParams(window.location.search).get('page') || '1';
            const url = new URL('/api/papers', window.location.origin);
            url.searchParams.set('q', query);
            url.searchParams.set('page', '1'); // Reset to first page on new search

            fetch(url.toString())
                .then(response => response.json())
                .then(data => {
                    // Update the table body
                    const tbody = document.querySelector('tbody');
                    if (data.papers.length === 0) {
                        tbody.innerHTML = '<tr>' +
                            '<td colspan="3" class="px-4 py-3 text-center text-gray-500">' +
                                'No papers found matching "' + query + '"' +
                            '</td>' +
                        '</tr>';
                    } else {
                        tbody.innerHTML = data.papers.map(paper =>
                            '<tr>' +
                                '<td class="px-4 py-3">' +
                                    '<div class="text-lg font-medium text-gray-900">' + paper.Title + '</div>' +
                                    (paper.ArxivSummary ?
                                    '<div class="mt-2 abstract-container">' +
                                        '<span class="text-sm text-gray-600">' + paper.FirstSentence + '</span>' +
                                        '<button class="expand-button ml-2">...</button>' +
                                        '<div class="abstract-text" style="display: none;">' + paper.ArxivSummary + '</div>' +
                                    '</div>' : '') +
                                '</td>' +
                                '<td class="px-4 py-3">' +
                                    '<div class="citation-count text-sm text-gray-900">' + (paper.Citations || 0) + '</div>' +
                                '</td>' +
                                '<td class="px-4 py-3">' +
                                    '<div class="flex flex-col gap-1">' +
                                        '<a href="' + paper.URL + '" target="_blank" class="text-sm text-gray-600 hover:text-gray-900">Paper</a>' +
                                        (paper.ArxivAbsURL ? '<a href="' + paper.ArxivAbsURL + '" target="_blank" class="text-sm text-gray-600 hover:text-gray-900">arXiv</a>' : '') +
                                        (paper.GoogleScholarURL ? '<a href="' + paper.GoogleScholarURL + '" target="_blank" class="text-sm text-gray-600 hover:text-gray-900">Scholar</a>' : '') +
                                    '</div>' +
                                '</td>' +
                            '</tr>'
                        ).join('');
                    }

                    // Update pagination
                    const paginationContainer = document.getElementById('paginationContainer');
                    const paginationHtml =
                        (data.currentPage > 1 ?
                            '<a href="?page=' + (data.currentPage - 1) + (query ? '&q=' + query : '') + '" class="px-3 py-1 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50">Previous</a>' : '') +
                        '<span class="px-3 py-1 text-sm font-medium text-gray-700 whitespace-nowrap">Page ' + data.currentPage + ' of ' + data.totalPages + '</span>' +
                        (data.currentPage < data.totalPages ?
                            '<a href="?page=' + (data.currentPage + 1) + (query ? '&q=' + query : '') + '" class="px-3 py-1 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50">Next</a>' : '');
                    paginationContainer.innerHTML = paginationHtml;

                    // Update URL
                    updateURL(query, 1);

                    // Reattach event listeners for abstract expansion
                    setupAbstractExpansion();
                })
                .catch(error => console.error('Error:', error));
        }

        document.addEventListener('DOMContentLoaded', function() {
            // Setup abstract expansion functionality
            setupAbstractExpansion();

            // Add sorting functionality
            const table = document.querySelector('table');
            const headers = table.querySelectorAll('th');

            headers.forEach(header => {
                if (header.textContent === 'Citations') {
                    header.style.cursor = 'pointer';
                    header.addEventListener('click', function() {
                        const tbody = table.querySelector('tbody');
                        const rows = Array.from(tbody.querySelectorAll('tr'));

                        const isAscending = header.getAttribute('data-sort') === 'asc';
                        header.setAttribute('data-sort', isAscending ? 'desc' : 'asc');

                        rows.sort((a, b) => {
                            const aCitations = parseInt(a.querySelector('.citation-count').textContent) || 0;
                            const bCitations = parseInt(b.querySelector('.citation-count').textContent) || 0;
                            return isAscending ? aCitations - bCitations : bCitations - aCitations;
                        });

                        rows.forEach(row => tbody.appendChild(row));
                    });
                }
            });

            // Setup search functionality
            const searchInput = document.getElementById('searchInput');
            const debouncedSearch = debounce((query) => {
                performSearch(query);
            }, 100);

            searchInput.addEventListener('input', (e) => {
                debouncedSearch(e.target.value);
            });

            // Handle browser back/forward buttons
            window.addEventListener('popstate', () => {
                const url = new URL(window.location);
                const query = url.searchParams.get('q') || '';
                searchInput.value = query;
                performSearch(query);
            });
        });
    </script>
</head>
<body class="bg-white">
    <div class="max-w-6xl mx-auto px-4 py-6">
        <div class="flex justify-between items-center mb-8">
            <h1 class="text-2xl font-semibold text-gray-900 px-4">Paper Citations</h1>
            <div class="flex items-center space-x-2" style="margin-right: 18px;">
                <div id="searchContainer">
                    <input type="text"
                           id="searchInput"
                           class="search-input px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                           placeholder="Search papers..."
                           value="{{.SearchQuery}}">
                </div>
                <div id="paginationContainer" class="flex items-center space-x-2">
                    {{if gt .CurrentPage 1}}
                    <a href="?page={{subtract .CurrentPage 1}}{{if .SearchQuery}}&q={{.SearchQuery}}{{end}}" class="px-3 py-1 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50">
                        Previous
                    </a>
                    {{end}}
                    <span class="px-3 py-1 text-sm font-medium text-gray-700 whitespace-nowrap">
                        Page {{.CurrentPage}} of {{.TotalPages}}
                    </span>
                    {{if lt .CurrentPage .TotalPages}}
                    <a href="?page={{add .CurrentPage 1}}{{if .SearchQuery}}&q={{.SearchQuery}}{{end}}" class="px-3 py-1 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50">
                        Next
                    </a>
                    {{end}}
                </div>
            </div>
        </div>

        <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200">
                <thead>
                    <tr>
                        <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Title</th>
                        <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Citations</th>
                        <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Links</th>
                    </tr>
                </thead>
                <tbody class="bg-white divide-y divide-gray-200">
                    {{range .Papers}}
                    <tr>
                        <td class="px-4 py-3">
                            <div class="text-lg font-medium text-gray-900">{{.Title}}</div>
                            {{if .ArxivSummary}}
                            <div class="mt-2 abstract-container">
                                <span class="text-sm text-gray-600">{{.FirstSentence}}</span>
                                <button class="expand-button ml-2">...</button>
                                <div class="abstract-text" style="display: none;">{{.ArxivSummary}}</div>
                            </div>
                            {{end}}
                        </td>
                        <td class="px-4 py-3">
                            <div class="citation-count text-sm text-gray-900">{{if .Citations}}{{.Citations}}{{else}}0{{end}}</div>
                        </td>
                        <td class="px-4 py-3">
                            <div class="flex flex-col gap-1">
                                <a href="{{.URL}}" target="_blank" class="text-sm text-gray-600 hover:text-gray-900">Paper</a>
                                {{if .ArxivAbsURL}}
                                <a href="{{.ArxivAbsURL}}" target="_blank" class="text-sm text-gray-600 hover:text-gray-900">arXiv</a>
                                {{end}}
                                {{if .GoogleScholarURL}}
                                <a href="{{.GoogleScholarURL}}" target="_blank" class="text-sm text-gray-600 hover:text-gray-900">Scholar</a>
                                {{end}}
                            </div>
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</body>
</html>`
