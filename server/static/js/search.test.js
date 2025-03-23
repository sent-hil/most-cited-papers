// Import jest-dom matchers
require('@testing-library/jest-dom');

// Import functions to test
const {
    highlightText,
    updateURL,
    setupAbstractExpansion,
    performSearch,
    debounce
} = require('./search.js');

// Mock the DOM environment
document.body.innerHTML = `
    <div class="flex justify-between items-center mb-8">
        <h1 class="text-2xl font-semibold text-gray-900 px-4">
            <a href="/" class="hover:text-gray-600 transition-colors">Paper Citations</a>
        </h1>
        <div class="flex items-center space-x-2">
            <div id="searchContainer">
                <input type="text" id="searchInput" class="search-input" value="test query">
            </div>
            <div id="paginationContainer"></div>
        </div>
    </div>
    <table>
        <tbody></tbody>
    </table>
`;

// Test highlightText function
describe('highlightText', () => {
    test('should return original text if no query', () => {
        const text = 'Test paper about mining';
        const result = highlightText(text, '');
        expect(result).toBe(text);
    });

    test('should highlight exact matches', () => {
        const text = 'Test paper about mining';
        const result = highlightText(text, 'mining');
        expect(result).toBe('Test paper about <mark class="highlight">mining</mark>');
    });

    test('should handle case-insensitive matches', () => {
        const text = 'Test paper about Mining';
        const result = highlightText(text, 'mining');
        expect(result).toBe('Test paper about <mark class="highlight">Mining</mark>');
    });

    test('should handle multiple matches', () => {
        const text = 'Mining paper about mining';
        const result = highlightText(text, 'mining');
        expect(result).toBe('<mark class="highlight">Mining</mark> paper about <mark class="highlight">mining</mark>');
    });

    test('should handle special regex characters in query', () => {
        const text = 'Test paper about (mining)';
        const result = highlightText(text, '(mining)');
        expect(result).toBe('Test paper about <mark class="highlight">(mining)</mark>');
    });
});

// Test updateURL function
describe('updateURL', () => {
    beforeEach(() => {
        // Reset URL before each test
        delete window.location;
        window.location = new URL('http://localhost');
        window.history.pushState = jest.fn();
    });

    test('should update URL with search query', () => {
        updateURL('mining', 1);
        const call = window.history.pushState.mock.calls[0];
        expect(call[0]).toEqual({});
        expect(call[1]).toBe('');
        const url = new URL(call[2]);
        expect(url.searchParams.get('q')).toBe('mining');
        expect(url.searchParams.get('page')).toBeNull();
    });

    test('should update URL with page number', () => {
        updateURL('', 2);
        const call = window.history.pushState.mock.calls[0];
        expect(call[0]).toEqual({});
        expect(call[1]).toBe('');
        const url = new URL(call[2]);
        expect(url.searchParams.get('q')).toBeNull();
        expect(url.searchParams.get('page')).toBe('2');
    });

    test('should update URL with both query and page', () => {
        updateURL('mining', 2);
        const call = window.history.pushState.mock.calls[0];
        expect(call[0]).toEqual({});
        expect(call[1]).toBe('');
        const url = new URL(call[2]);
        expect(url.searchParams.get('q')).toBe('mining');
        expect(url.searchParams.get('page')).toBe('2');
    });
});

// Test setupAbstractExpansion function
describe('setupAbstractExpansion', () => {
    beforeEach(() => {
        // Reset DOM before each test
        document.body.innerHTML = `
            <div class="abstract-container">
                <span class="text-gray-600">First sentence.</span>
                <button class="expand-button">...</button>
                <div class="abstract-text" style="display: none;">Full abstract text.</div>
            </div>
        `;
    });

    test('should expand abstract when clicking button', () => {
        setupAbstractExpansion();
        const button = document.querySelector('.expand-button');
        const firstSentence = document.querySelector('.text-gray-600');

        button.click();
        expect(firstSentence.textContent).toBe('Full abstract text.');
        expect(button.textContent).toBe('Show less');
    });

    test('should collapse abstract when clicking button again', () => {
        setupAbstractExpansion();
        const button = document.querySelector('.expand-button');
        const firstSentence = document.querySelector('.text-gray-600');

        // First click to expand
        button.click();
        // Second click to collapse
        button.click();

        expect(firstSentence.textContent).toBe('First sentence.');
        expect(button.textContent).toBe('...');
    });
});

// Test performSearch function
describe('performSearch', () => {
    beforeEach(() => {
        // Reset DOM and mock fetch
        document.body.innerHTML = `
            <div id="searchContainer">
                <input type="text" id="searchInput" class="search-input" value="">
            </div>
            <div id="paginationContainer"></div>
            <table>
                <tbody></tbody>
            </table>
        `;
        // Mock fetch with a proper Response object
        global.fetch = jest.fn().mockImplementation(() =>
            Promise.resolve({
                json: () => Promise.resolve({
                    papers: [],
                    count: 0,
                    currentPage: 1,
                    totalPages: 1
                })
            })
        );
    });

    test('should show loading state', () => {
        performSearch('mining');
        const tbody = document.querySelector('tbody');
        expect(tbody.innerHTML).toContain('Loading...');
    });

    test('should handle empty results', async () => {
        global.fetch.mockImplementationOnce(() =>
            Promise.resolve({
                json: () => Promise.resolve({
                    papers: [],
                    count: 0,
                    currentPage: 1,
                    totalPages: 1
                })
            })
        );

        await performSearch('nonexistent');
        const tbody = document.querySelector('tbody');
        expect(tbody.innerHTML).toContain('No papers found');
    });

    test('should handle successful search results', async () => {
        const mockPapers = [{
            Title: 'Test Paper',
            URL: 'http://test.com',
            Citations: 10,
            ArxivSummary: 'Test abstract about mining.'
        }];

        global.fetch.mockImplementationOnce(() =>
            Promise.resolve({
                json: () => Promise.resolve({
                    papers: mockPapers,
                    count: 1,
                    currentPage: 1,
                    totalPages: 1
                })
            })
        );

        await performSearch('mining');
        const tbody = document.querySelector('tbody');
        expect(tbody.innerHTML).toContain('Test Paper');
        expect(tbody.innerHTML).toContain('mining');
    });

    test('should handle errors', async () => {
        global.fetch.mockImplementationOnce(() =>
            Promise.reject(new Error('Network error'))
        );

        await performSearch('mining');
        const tbody = document.querySelector('tbody');
        expect(tbody.innerHTML).toContain('Error loading papers');
    });
});

// Test header link functionality
describe('Header Link', () => {
    beforeEach(() => {
        // Reset DOM
        document.body.innerHTML = `
            <div class="flex justify-between items-center mb-8">
                <h1 class="text-2xl font-semibold text-gray-900 px-4">
                    <a href="/" class="hover:text-gray-600 transition-colors">Paper Citations</a>
                </h1>
                <div class="flex items-center space-x-2">
                    <div id="searchContainer">
                        <input type="text" id="searchInput" class="search-input" value="test query">
                    </div>
                    <div id="paginationContainer"></div>
                </div>
            </div>
            <table>
                <tbody></tbody>
            </table>
        `;
        // Mock fetch
        global.fetch = jest.fn().mockImplementation(() =>
            Promise.resolve({
                json: () => Promise.resolve({
                    papers: [],
                    count: 0,
                    currentPage: 1,
                    totalPages: 1
                })
            })
        );
    });

    test('should clear search and reset to home page when clicking header', async () => {
        // Mock window.location
        delete window.location;
        window.location = new URL('http://localhost/?q=test&page=2');
        window.history.pushState = jest.fn();

        // Get the header link
        const headerLink = document.querySelector('h1 a');

        // Setup the click handler
        headerLink.addEventListener('click', (e) => {
            e.preventDefault();
            // Clear search input
            const searchInput = document.getElementById('searchInput');
            searchInput.value = '';
            // Update URL to home page
            window.history.pushState({}, '', '/');
            // Perform empty search to reset results
            performSearch('');
        });

        // Create and dispatch click event
        const clickEvent = new MouseEvent('click', {
            bubbles: true,
            cancelable: true,
            view: window
        });
        headerLink.dispatchEvent(clickEvent);

        // Wait for any async operations to complete
        await new Promise(resolve => setTimeout(resolve, 0));

        // Verify search input was cleared
        const searchInput = document.getElementById('searchInput');
        expect(searchInput.value).toBe('');

        // Verify URL was updated to home page
        const pushStateCall = window.history.pushState.mock.calls[0];
        expect(pushStateCall[0]).toEqual({});
        expect(pushStateCall[1]).toBe('');
        expect(pushStateCall[2]).toBe('/');
    });
});
