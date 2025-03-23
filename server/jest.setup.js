// Mock window.location
const url = new URL('http://localhost');
delete window.location;
window.location = url;

// Mock window.history
window.history.pushState = jest.fn();
window.history.replaceState = jest.fn();

// Mock fetch
global.fetch = jest.fn();

// Mock debounce to immediately invoke the function
global.debounce = (func) => func;

// Add jest-dom matchers
require('@testing-library/jest-dom');
