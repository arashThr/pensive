// For cross-browser compatibility (Chrome uses 'chrome', Firefox supports it but prefers 'browser')
const isChrome = !(window.browser && browser.runtime)
const browserAPI = isChrome ? chrome : browser;
const devMode = false

document.addEventListener('DOMContentLoaded', async function () {
  const connectButton = document.getElementById('connect-button');
  const statusDiv = document.getElementById('status');
  const authSection = document.getElementById('auth-section');
  const fullPageCaptureCheckbox = document.getElementById('fullPageCapture');
  const fullPageWarning = document.getElementById('fullPageWarning');
  const saveContentSettingsButton = document.getElementById('save-content-settings');

  let grantOrigins = ['https://getpensive.com/*'];
  if (devMode) {
    grantOrigins.push('http://localhost:8000/*');
  }

  // Use fixed endpoint based on dev mode
  const endpoint = devMode ? "http://localhost:8000" : "https://getpensive.com";

  const result = await browserAPI.storage.local.get(['endpoint', 'extractionMethod', 'fullPageCapture']);
  if (result.endpoint !== endpoint) {
    browserAPI.storage.local.set({ endpoint: endpoint });
  }

  // Check if we already have a token and validate it
  if (result.apiToken) {
    validateToken(result.apiToken);
  }

  const enabled = result.fullPageCapture || false;
  fullPageCaptureCheckbox.checked = enabled;
  toggleWarningVisibility(enabled);

  // Handle checkbox toggle
  fullPageCaptureCheckbox.addEventListener('change', function() {
    toggleWarningVisibility(checked);
  });

  // Show/hide warning based on checkbox state
  function toggleWarningVisibility(show) {
    fullPageWarning.style.display = show ? 'block' : 'none';
  }

  // Save content processing settings
  saveContentSettingsButton.addEventListener('click', async () => {
    const enabled = fullPageCaptureCheckbox.checked;
    await browserAPI.storage.local.set({ fullPageCapture: enabled })
    const originalText = saveContentSettingsButton.textContent;
    saveContentSettingsButton.textContent = 'Saved!';
    setTimeout(() => {
      saveContentSettingsButton.textContent = originalText;
    }, 2000);
  });

  // Add sign out button functionality
  function addSignOutButton() {
    const signOutButton = document.getElementById('signout-button');
    signOutButton.style.display = 'inline-block';

    // Add click event listener if not already added
    if (!signOutButton.hasAttribute('data-listener-added')) {
      signOutButton.addEventListener('click', async function () {
        try {
          // Get the current token
          const result = await browserAPI.storage.local.get(['apiToken']);
          const token = result.apiToken;
          
          if (token) {
            // Delete token from server first
            try {
              await fetch(new URL('/api/v1/tokens/current', endpoint).href, {
                method: 'DELETE',
                headers: {
                  'Authorization': `Bearer ${token}`,
                  'Content-Type': 'application/json'
                }
              });
              console.log('Token deleted from server');
            } catch (error) {
              console.error('Failed to delete token from server:', error);
              // Continue with local deletion even if server deletion fails
            }
          }
          
          // Clear the token locally
          await browserAPI.storage.local.remove(['apiToken']);
          showDisconnectedState();
          console.log('Token cleared locally, user signed out');
        } catch (error) {
          console.error('Failed to sign out:', error);
          showError('Failed to sign out');
        }
      });
      signOutButton.setAttribute('data-listener-added', 'true');
    }
  }

  // Function to validate token by calling health check endpoint
  async function validateToken(token) {
    try {
      const response = await fetch(new URL('/api/v1/ping', endpoint).href, {
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
        showError('Connect to Pensive to continue');
      }
    } catch (error) {
      console.error('Failed to validate token:', error);
      // On network error, assume token might still be valid but show warning
      showConnectedState();
      addSignOutButton();
      statusDiv.textContent = 'Connected to Pensive (offline)';
    }
  }

  connectButton.addEventListener('click', async function () {
    connectButton.disabled = true;
    connectButton.textContent = 'Connecting...';

    if (isChrome) {
      const granted = await browserAPI.permissions.contains({
        origins: grantOrigins,
      });

      if (!granted) {
        const granted = await browserAPI.permissions.request({
          origins: grantOrigins,
        });

        if (!granted) {
          showError('Permissions not granted. Please allow access to Pensive in the extension settings.');
          return
        }
      }
    }

    // Track authentication state
    let authCompleted = false;

    try {
      // Open a new tab with the auth URL
      const tab = await browserAPI.tabs.create({
        url: new URL('/extension/auth', endpoint).toString(),
        active: true
      });

      if (browserAPI.runtime.lastError) {
        console.error('Failed to create tab:', browserAPI.runtime.lastError);
        showError('Failed to open authentication page');
        return;
      }

      // Listen for messages from the content script
      const messageListener = function (request, sender, sendResponse) {
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
    } catch (error) {
      console.error('Failed to create tab:', error);
      showError('Failed to open authentication page');
    }
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
