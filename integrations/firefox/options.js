// For cross-browser compatibility (Firefox prefers 'browser', Chrome uses 'chrome')
const browserAPI = typeof browser !== "undefined" ? browser : chrome;

document.addEventListener('DOMContentLoaded', function() {
  const connectButton = document.getElementById('connect-button');
  const statusDiv = document.getElementById('status');
  const authSection = document.getElementById('auth-section');

  // Set default values
  let defaultEndpoint = "http://localhost:8000"

  // Function to normalize endpoint URL
  function normalizeEndpoint(endpoint) {
    if (!endpoint) return defaultEndpoint;
    
    // If it's localhost, keep HTTP
    if (endpoint.includes('localhost') || endpoint.includes('127.0.0.1')) {
      // Ensure localhost has http:// prefix if no protocol specified
      if (!endpoint.startsWith('http://') && !endpoint.startsWith('https://')) {
        return `http://${endpoint}`;
      }
      return endpoint;
    }
    
    // For all other hosts, ensure HTTPS
    if (endpoint.startsWith('http://')) {
      // Convert HTTP to HTTPS for non-localhost
      return endpoint.replace('http://', 'https://');
    } else if (endpoint.startsWith('https://')) {
      // Already HTTPS, keep as is
      return endpoint;
    } else {
      // No protocol specified, add HTTPS
      return `https://${endpoint}`;
    }
  }

  // Load saved settings or use defaults
  browserAPI.storage.local.get(["endpoint"]).then((data) => {
    defaultEndpoint = normalizeEndpoint(data.endpoint || defaultEndpoint);
    document.getElementById("endpoint").value = defaultEndpoint;

    browserAPI.storage.local.set({ endpoint: defaultEndpoint }).then(() => {
      console.log("Default settings saved");
    })
  });

  // Save settings
  document.getElementById("save").addEventListener("click", () => {
    let inputValue = document.getElementById("endpoint").value || defaultEndpoint;
    defaultEndpoint = normalizeEndpoint(inputValue);
    
    // Update the input field to show the normalized URL
    document.getElementById("endpoint").value = defaultEndpoint;
    
    browserAPI.storage.local.set({ endpoint: defaultEndpoint }).then(() => {
      const status = document.getElementById("status");
      const originalText = status.textContent;
      status.textContent = "Settings saved!";
      setTimeout(() => {
        status.textContent = originalText;
      }, 2000);
    });
  });

  // Check if we already have a token and validate it
  browserAPI.storage.local.get(['apiToken']).then((result) => {
    if (result.apiToken) {
      validateToken(result.apiToken);
    }
  });

  // Add sign out button functionality
  function addSignOutButton() {
    const signOutButton = document.getElementById('signout-button');
    signOutButton.style.display = 'inline-block';
    
    // Add click event listener if not already added
    if (!signOutButton.hasAttribute('data-listener-added')) {
      signOutButton.addEventListener('click', function() {
        // Clear the token
        browserAPI.storage.local.remove(['apiToken']).then(() => {
          showDisconnectedState();
          console.log('Token cleared, user signed out');
        }).catch((error) => {
          console.error('Failed to clear token:', error);
          showError('Failed to sign out');
        });
      });
      signOutButton.setAttribute('data-listener-added', 'true');
    }
  }

  // Function to validate token by calling health check endpoint
  async function validateToken(token) {
    try {
      const response = await fetch(new URL('/api/ping', defaultEndpoint).href, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        }
      });

      if (response.ok) {
        showConnectedState();
        addSignOutButton();
      } else {
        // Token is invalid, clear it and show disconnected state
        await browserAPI.storage.local.remove(['apiToken']);
        showDisconnectedState();
        showError('Authentication token is invalid. Please reconnect.');
      }
    } catch (error) {
      console.error('Failed to validate token:', error);
      // On network error, assume token might still be valid but show warning
      showConnectedState();
      addSignOutButton();
      statusDiv.textContent = 'Connected to Pensive (offline)';
    }
  }

  connectButton.addEventListener('click', function() {
    connectButton.disabled = true;
    connectButton.textContent = 'Connecting...';

    // Track authentication state
    let authCompleted = false;

    // Open a new tab with the auth URL
    browserAPI.tabs.create({
      url: new URL('/extension/auth', defaultEndpoint).toString(),
      active: true
    }).then((tab) => {
      if (browserAPI.runtime.lastError) {
        console.error('Failed to create tab:', browserAPI.runtime.lastError);
        showError('Failed to open authentication page');
        return;
      }

      // Listen for messages from the content script
      const messageListener = function(request, sender, sendResponse) {
        console.log("Received message", request)
        if (request.type === 'AUTH_TOKEN') {
          authCompleted = true;
          // Store the token
          browserAPI.storage.local.set({ apiToken: request.token }).then(() => {
            validateToken(request.token);
            // Close the auth tab
            browserAPI.tabs.remove(tab.id);
            // Remove the message listener
            browserAPI.runtime.onMessage.removeListener(messageListener);
          }).catch((error) => {
            console.error('Failed to store token:', error);
            showError('Failed to store authentication token');
          });
        }
      };

      browserAPI.runtime.onMessage.addListener(messageListener);

      // Set a timeout to handle cases where the auth page doesn't respond
      setTimeout(() => {
        browserAPI.runtime.onMessage.removeListener(messageListener);
        if (!authCompleted) {
          showError('Authentication timed out. Please try again.');
        }
      }, 10000); // 10 seconds timeout
    }).catch((error) => {
      console.error('Failed to create tab:', error);
      showError('Failed to open authentication page');
    });
  });

  function showConnectedState() {
    statusDiv.className = 'status connected';
    statusDiv.textContent = 'Connected to Pensive';
    authSection.style.display = 'block';
    
    // Hide the connect button when connected
    connectButton.style.display = 'none';
  }

  function showDisconnectedState() {
    statusDiv.className = 'status disconnected';
    statusDiv.textContent = 'Not connected to Pensive';
    authSection.style.display = 'block';
    
    // Show the connect button when disconnected
    connectButton.style.display = 'inline-block';
    connectButton.disabled = false;
    connectButton.textContent = 'Connect to Pensive';
    
    // Hide the sign out button
    const signOutButton = document.getElementById('signout-button');
    if (signOutButton) {
      signOutButton.style.display = 'none';
    }
  }

  function showError(message) {
    statusDiv.className = 'status disconnected';
    statusDiv.textContent = message;
    connectButton.style.display = 'inline-block';
    connectButton.disabled = false;
    connectButton.textContent = 'Connect to Pensive';
  }
});
