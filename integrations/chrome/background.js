// For cross-browser compatibility (Chrome uses 'chrome', Firefox supports it but prefers 'browser')
const browserAPI = typeof chrome !== "undefined" ? chrome : browser;

// Handle authentication token messages from content script
browserAPI.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === "AUTH_TOKEN") {
    // Store the authentication token
    browserAPI.storage.sync.set({ apiToken: message.token }, () => {
      console.log("Authentication token stored successfully");
    });
  }
});

// Handle installation and updates
browserAPI.runtime.onInstalled.addListener((details) => {
  if (details.reason === "install") {
    console.log("Pensieve extension installed");
    // Open options page on first install
    browserAPI.runtime.openOptionsPage();
  } else if (details.reason === "update") {
    console.log("Pensieve extension updated");
  }
});

// Utility function for making API requests (if needed by popup)
async function makeApiRequest(endpoint, method = 'GET', body = null) {
  const { endpoint: baseEndpoint, apiToken } = await browserAPI.storage.sync.get(['endpoint', 'apiToken']);
  
  if (!baseEndpoint || !apiToken) {
    throw new Error('Extension not configured');
  }
  
  const url = new URL(endpoint, baseEndpoint).href;
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${apiToken}`
  };
  
  const options = {
    method,
    headers
  };
  
  if (body) {
    options.body = JSON.stringify(body);
  }
  
  const response = await fetch(url, options);
  
  if (!response.ok) {
    const contentType = response.headers.get('Content-Type');
    let error;
    
    if (contentType?.includes('application/json')) {
      const errorData = await response.json();
      error = parseToError(errorData);
    } else {
      error = { errorCode: 'UNKNOWN_ERROR', errorMessage: await response.text() };
    }
    
    throw error;
  }
  
  return await response.json();
}

function parseToError(result) {
  const errorCode = result.errorCode || "UNKNOWN_ERROR";
  const errorMessage = result.errorMessage || "Unknown error occurred";
  return { errorCode, errorMessage };
}