package main

const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Most Cited Papers</title>
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
        .highlight {
            background-color: #fef08a;
            padding: 0.1em 0.2em;
            border-radius: 0.2em;
        }
    </style>
    <script src="/static/js/search.js"></script>
</head>
<body class="bg-white">
    <div class="max-w-6xl mx-auto px-4 py-6">
        <div class="flex justify-between items-center mb-8">
            <h1 class="text-2xl font-semibold text-gray-900 px-4">
                <a href="/" class="hover:text-gray-600 transition-colors">Most Cited Papers</a>
            </h1>
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
