document.addEventListener('DOMContentLoaded', function() {
  const connectButton = document.getElementById('connect-button');
  const statusDiv = document.getElementById('status');
  const authSection = document.getElementById('auth-section');
  const tokenDisplaySection = document.getElementById('token-display-section');
  const tokenDisplay = document.getElementById('token-display');

  // Set default values
  let defaultEndpoint = "http://localhost:8000"
  let defaultFolderName = "Archive"

  // Load saved settings or use defaults
  chrome.storage.sync.get(["endpoint", "folderName"], (data) => {
    defaultEndpoint = data.endpoint || defaultEndpoint;
    defaultFolderName = data.folderName || defaultFolderName;
    document.getElementById("endpoint").value = defaultEndpoint;
    document.getElementById("folderName").value = defaultFolderName;
  });

  chrome.storage.sync.set({ endpoint: defaultEndpoint, folderName: defaultFolderName }, () => {
    console.log("Default settings saved");
  })

  // Save settings
  document.getElementById("save").addEventListener("click", () => {
    defaultEndpoint = document.getElementById("endpoint").value || defaultEndpoint;
    defaultFolderName = document.getElementById("folderName").value || defaultFolderName;
    
    chrome.storage.sync.set({ endpoint: defaultEndpoint, folderName: defaultFolderName }, () => {
      const status = document.getElementById("status");
      status.textContent = "Settings saved!";
      setTimeout(() => {
        status.textContent = "";
      }, 2000);
    });
  });

  // Check if we already have a token
  chrome.storage.sync.get(['apiToken'], function(result) {
    if (result.apiToken) {
      showConnectedState(result.apiToken);
    }
  });

  connectButton.addEventListener('click', function() {
    connectButton.disabled = true;
    connectButton.textContent = 'Connecting...';

    // Open a new tab with the auth URL
    chrome.tabs.create({
      url: defaultEndpoint + '/extension/auth',
      active: true
    }, function(tab) {
      if (chrome.runtime.lastError) {
        console.error('Failed to create tab:', chrome.runtime.lastError);
        showError('Failed to open authentication page');
        return;
      }

      // Listen for messages from the content script
      const messageListener = function(request, sender, sendResponse) {
        console.log("Received message", request)
        if (request.type === 'AUTH_TOKEN') {
          // Store the token
          chrome.storage.sync.set({ apiToken: request.token }, function() {
            if (chrome.runtime.lastError) {
              console.error('Failed to store token:', chrome.runtime.lastError);
              showError('Failed to store authentication token');
              return;
            }
            showConnectedState(request.token);
            // Close the auth tab
            chrome.tabs.remove(tab.id);
            // Remove the message listener
            chrome.runtime.onMessage.removeListener(messageListener);
          });
        }
      };

      chrome.runtime.onMessage.addListener(messageListener);

      // Set a timeout to handle cases where the auth page doesn't respond
      setTimeout(() => {
        chrome.runtime.onMessage.removeListener(messageListener);
        if (!tokenDisplaySection.style.display || tokenDisplaySection.style.display === 'none') {
          showError('Authentication timed out. Please try again.');
        }
      }, 10000); // 10 seconds timeout
    });
  });

  function showConnectedState(token) {
    statusDiv.className = 'status connected';
    statusDiv.textContent = 'Connected to Pensieve';
    authSection.style.display = 'none';
    tokenDisplaySection.style.display = 'block';
    
    // Mask the token except for last 4 characters
    const maskedToken = '*'.repeat(10) + token.slice(-6);
    tokenDisplay.textContent = maskedToken;
    
    connectButton.disabled = false;
    connectButton.textContent = 'Connect to Pensieve';
  }

  function showError(message) {
    statusDiv.className = 'status disconnected';
    statusDiv.textContent = message;
    connectButton.disabled = false;
    connectButton.textContent = 'Connect to Pensieve';
  }
});
