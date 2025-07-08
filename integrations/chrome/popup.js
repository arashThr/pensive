// For cross-browser compatibility
const browserAPI = typeof chrome !== "undefined" ? chrome : browser;

document.addEventListener('DOMContentLoaded', async () => {
  const statusElement = document.getElementById('status');
  const pageTitleElement = document.getElementById('pageTitle');
  const pageUrlElement = document.getElementById('pageUrl');
  const saveBtn = document.getElementById('saveBtn');
  const removeBtn = document.getElementById('removeBtn');
  const settingsLink = document.getElementById('settingsLink');
  const searchLink = document.getElementById('searchLink');
  
  let currentTab = null;
  let isBookmarked = false;
  
  // Get current tab information
  try {
    const tabs = await browserAPI.tabs.query({active: true, currentWindow: true});
    currentTab = tabs[0];
    
    // Update page info
    pageTitleElement.textContent = currentTab.title;
    pageUrlElement.textContent = currentTab.url;
  } catch (error) {
    console.error('Error getting current tab:', error);
    updateStatus('error', 'Error loading page information');
    return;
  }
  
  // Check if current page is bookmarked
  await checkBookmarkStatus();
  
  // Event listeners
  saveBtn.addEventListener('click', saveBookmark);
  removeBtn.addEventListener('click', removeBookmark);
  settingsLink.addEventListener('click', openSettings);
  searchLink.addEventListener('click', openSearch);
  
  async function checkBookmarkStatus() {
    if (!currentTab) return;
    
    try {
      updateStatus('loading', 'Checking status...');
      
      const { endpoint, apiToken } = await browserAPI.storage.sync.get(['endpoint', 'apiToken']);
      
      if (!endpoint || !apiToken) {
        updateStatus('error', 'Extension not configured');
        saveBtn.disabled = true;
        removeBtn.disabled = true;
        return;
      }
      
      // Check if bookmark exists by trying to create it
      // If it already exists, the API should return an error or the existing bookmark
      const createEndpoint = new URL("/api/v1/bookmarks", endpoint).href;
      const response = await fetch(createEndpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${apiToken}`
        },
        body: JSON.stringify({ link: currentTab.url })
      });
      
      if (response.ok) {
        // Bookmark was created (or already existed), so it's now bookmarked
        isBookmarked = true;
        updateBookmarkStatus();
      } else {
        // Check if it failed because it already exists
        const contentType = response.headers.get('Content-Type');
        if (contentType?.includes('application/json')) {
          const errorData = await response.json();
          // If it's a duplicate error, it means the bookmark already exists
          if (errorData.errorCode === 'DUPLICATE_BOOKMARK' || response.status === 409) {
            isBookmarked = true;
            updateBookmarkStatus();
          } else {
            // Other error, assume not bookmarked
            isBookmarked = false;
            updateBookmarkStatus();
          }
        } else {
          // Non-JSON error, assume not bookmarked
          isBookmarked = false;
          updateBookmarkStatus();
        }
      }
      
    } catch (error) {
      console.error('Error checking bookmark status:', error);
      // Default to not bookmarked on error
      isBookmarked = false;
      updateBookmarkStatus();
    }
  }
  
  function updateBookmarkStatus() {
    if (isBookmarked) {
      updateStatus('saved', 'Page is saved');
      saveBtn.disabled = true;
      removeBtn.disabled = false;
    } else {
      updateStatus('not-saved', 'Page not saved');
      saveBtn.disabled = false;
      removeBtn.disabled = true;
    }
  }
  
  async function saveBookmark() {
    if (!currentTab) return;
    
    try {
      saveBtn.disabled = true;
      updateStatus('loading', 'Saving...');
      
      const { endpoint, apiToken } = await browserAPI.storage.sync.get(['endpoint', 'apiToken']);
      
      if (!endpoint || !apiToken) {
        updateStatus('error', 'Extension not configured');
        saveBtn.disabled = false;
        return;
      }
      
      const response = await fetch(new URL("/api/v1/bookmarks", endpoint).href, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${apiToken}`
        },
        body: JSON.stringify({ link: currentTab.url })
      });
      
      if (response.ok) {
        isBookmarked = true;
        updateStatus('saved', 'Page saved successfully!');
        saveBtn.disabled = true;
        removeBtn.disabled = false;
        
        // Auto-hide success message after 2 seconds
        setTimeout(() => {
          if (isBookmarked) {
            updateStatus('saved', 'Page is saved');
          }
        }, 2000);
      } else {
        const contentType = response.headers.get('Content-Type');
        let errorMessage = 'Failed to save page';
        
        if (contentType?.includes('application/json')) {
          const errorData = await response.json();
          // If it's already bookmarked, treat it as success
          if (errorData.errorCode === 'DUPLICATE_BOOKMARK' || response.status === 409) {
            isBookmarked = true;
            updateStatus('saved', 'Page already saved');
            saveBtn.disabled = true;
            removeBtn.disabled = false;
            return;
          }
          errorMessage = errorData.errorMessage || errorMessage;
        } else {
          errorMessage = await response.text() || errorMessage;
        }
        
        updateStatus('error', errorMessage);
        saveBtn.disabled = false;
      }
      
    } catch (error) {
      console.error('Error saving bookmark:', error);
      updateStatus('error', 'Network error occurred');
      saveBtn.disabled = false;
    }
  }
  
  async function removeBookmark() {
    if (!currentTab) return;
    
    try {
      removeBtn.disabled = true;
      updateStatus('loading', 'Removing...');
      
      const { endpoint, apiToken } = await browserAPI.storage.sync.get(['endpoint', 'apiToken']);
      
      if (!endpoint || !apiToken) {
        updateStatus('error', 'Extension not configured');
        removeBtn.disabled = false;
        return;
      }
      
      const response = await fetch(new URL("/api/v1/bookmarks", endpoint).href, {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${apiToken}`
        },
        body: JSON.stringify({ link: currentTab.url })
      });
      
      if (response.ok) {
        isBookmarked = false;
        updateStatus('not-saved', 'Page removed successfully!');
        saveBtn.disabled = false;
        removeBtn.disabled = true;
        
        // Auto-hide success message after 2 seconds
        setTimeout(() => {
          if (!isBookmarked) {
            updateStatus('not-saved', 'Page not saved');
          }
        }, 2000);
      } else if (response.status === 404) {
        // Bookmark doesn't exist, which is fine for removal
        isBookmarked = false;
        updateStatus('not-saved', 'Page not saved');
        saveBtn.disabled = false;
        removeBtn.disabled = true;
      } else {
        const contentType = response.headers.get('Content-Type');
        let errorMessage = 'Failed to remove page';
        
        if (contentType?.includes('application/json')) {
          const errorData = await response.json();
          errorMessage = errorData.errorMessage || errorMessage;
        } else {
          errorMessage = await response.text() || errorMessage;
        }
        
        updateStatus('error', errorMessage);
        removeBtn.disabled = false;
      }
      
    } catch (error) {
      console.error('Error removing bookmark:', error);
      updateStatus('error', 'Network error occurred');
      removeBtn.disabled = false;
    }
  }
  
  function updateStatus(type, message) {
    statusElement.className = `status ${type}`;
    
    if (type === 'loading') {
      statusElement.innerHTML = `
        <div class="spinner"></div>
        <span>${message}</span>
      `;
    } else {
      statusElement.innerHTML = `<span>${message}</span>`;
    }
  }
  
  function openSettings() {
    browserAPI.runtime.openOptionsPage();
  }
  
  async function openSearch() {
    try {
      const { endpoint } = await browserAPI.storage.sync.get(['endpoint']);
      
      if (!endpoint) {
        // If no endpoint is configured, show an error or open options
        browserAPI.runtime.openOptionsPage();
        return;
      }
      
      // Open the search page in a new tab
      const searchUrl = new URL('/home', endpoint).href;
      await browserAPI.tabs.create({ url: searchUrl });
      
      // Close the popup
      window.close();
    } catch (error) {
      console.error('Error opening search page:', error);
      // Fallback to opening options page
      browserAPI.runtime.openOptionsPage();
    }
  }
}); 