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
        const query = new URLSearchParams(window.location.search).get('q');

        if (button) {
            button.addEventListener('click', function() {
                if (firstSentence.textContent === fullText) {
                    // Collapse - show first sentence and ...
                    firstSentence.textContent = firstSentence.textContent.split('.')[0] + '.';
                    button.textContent = '...';
                } else {
                    // Expand - show full text with highlighting
                    firstSentence.innerHTML = highlightText(fullText, query);
                    button.textContent = 'Show less';
                }
            });
        }
    });
}

// Function to highlight text
function highlightText(text, query) {
    if (!query || !text) return text;

    // Escape special regex characters in the query
    const escapedQuery = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const regex = new RegExp('(' + escapedQuery + ')', 'gi');

    // Log the text and query for debugging
    console.log('Highlighting text:', text);
    console.log('With query:', query);

    // Decode HTML entities before highlighting
    const decodedText = text.replace(/&amp;/g, '&')
                           .replace(/&lt;/g, '<')
                           .replace(/&gt;/g, '>')
                           .replace(/&quot;/g, '"')
                           .replace(/&#039;/g, "'");

    const result = decodedText.replace(regex, '<mark class="highlight">$1</mark>');
    console.log('Result:', result);

    return result;
}

// Function to perform search
function performSearch(query) {
    // Update URL with search query
    updateURL(query, 1);

    // Show loading state
    const tbody = document.querySelector('tbody');
    tbody.innerHTML = '<tr><td colspan="3" class="px-4 py-3 text-center">Loading...</td></tr>';

    // Fetch data from API
    fetch(`/api/papers?q=${encodeURIComponent(query)}&page=1`)
        .then(response => response.json())
        .then(data => {
            // Update table body
            if (data.papers.length === 0) {
                tbody.innerHTML = '<tr>' +
                    '<td colspan="3" class="px-4 py-3 text-center text-gray-500">' +
                        'No papers found matching "' + query + '"' +
                    '</td>' +
                '</tr>';
            } else {
                tbody.innerHTML = data.papers.map(paper => `
                    <tr>
                        <td class="px-4 py-3">
                            <div class="text-lg font-medium text-gray-900">${highlightText(paper.Title, query)}</div>
                            ${paper.ArxivSummary ? `
                            <div class="mt-2 abstract-container">
                                <span class="text-sm text-gray-600">${highlightText(paper.FirstSentence, query)}</span>
                                <button class="expand-button ml-2">...</button>
                                <div class="abstract-text" style="display: none;">${highlightText(paper.ArxivSummary, query)}</div>
                            </div>
                            ` : ''}
                        </td>
                        <td class="px-4 py-3">
                            <div class="citation-count text-sm text-gray-900">${paper.Citations || 0}</div>
                        </td>
                        <td class="px-4 py-3">
                            <div class="flex flex-col gap-1">
                                <a href="${paper.URL}" target="_blank" class="text-sm text-gray-600 hover:text-gray-900">Paper</a>
                                ${paper.ArxivAbsURL ? `<a href="${paper.ArxivAbsURL}" target="_blank" class="text-sm text-gray-600 hover:text-gray-900">arXiv</a>` : ''}
                                ${paper.GoogleScholarURL ? `<a href="${paper.GoogleScholarURL}" target="_blank" class="text-sm text-gray-600 hover:text-gray-900">Scholar</a>` : ''}
                            </div>
                        </td>
                    </tr>
                `).join('');
            }

            // Update pagination
            const paginationContainer = document.getElementById('paginationContainer');
            paginationContainer.innerHTML = `
                ${data.currentPage > 1 ? `
                <a href="?page=${data.currentPage - 1}${query ? '&q=' + encodeURIComponent(query) : ''}" class="px-3 py-1 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50">
                    Previous
                </a>
                ` : ''}
                <span class="px-3 py-1 text-sm font-medium text-gray-700 whitespace-nowrap">
                    Page ${data.currentPage} of ${data.totalPages}
                </span>
                ${data.currentPage < data.totalPages ? `
                <a href="?page=${data.currentPage + 1}${query ? '&q=' + encodeURIComponent(query) : ''}" class="px-3 py-1 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50">
                    Next
                </a>
                ` : ''}
            `;

            // Re-setup abstract expansion
            setupAbstractExpansion();
        })
        .catch(error => {
            console.error('Error:', error);
            tbody.innerHTML = '<tr><td colspan="3" class="px-4 py-3 text-center text-red-500">Error loading papers</td></tr>';
        });
}

document.addEventListener('DOMContentLoaded', function() {
    // Setup abstract expansion functionality
    setupAbstractExpansion();

    // Add highlighting to initial page load
    const url = new URL(window.location);
    const query = url.searchParams.get('q');
    if (query) {
        // Highlight text in all paper titles
        document.querySelectorAll('.text-lg.font-medium').forEach(title => {
            title.innerHTML = highlightText(title.textContent, query);
        });

        // Highlight text in all first sentences
        document.querySelectorAll('.abstract-container .text-gray-600').forEach(sentence => {
            sentence.innerHTML = highlightText(sentence.textContent, query);
        });

        // Highlight text in all full abstracts
        document.querySelectorAll('.abstract-text').forEach(abstract => {
            abstract.innerHTML = highlightText(abstract.textContent, query);
        });
    }

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
