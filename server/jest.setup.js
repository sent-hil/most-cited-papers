// Mock window.location
const location = new URL('http://localhost');
delete window.location;
window.location = location;

// Mock window.history
window.history.pushState = jest.fn();
window.history.replaceState = jest.fn();

// Mock fetch
global.fetch = jest.fn();

// Mock debounce function
global.debounce = (fn, wait) => fn;
