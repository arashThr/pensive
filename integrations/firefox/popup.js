// For cross-browser compatibility (Firefox prefers 'browser', Chrome uses 'chrome')
const browserAPI = typeof browser !== "undefined" ? browser : chrome;

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
      
      // Use the new check endpoint to see if bookmark exists
      const checkUrl = new URL("/api/v1/bookmarks/check", endpoint);
      checkUrl.searchParams.set('url', currentTab.url);
      
      const response = await fetch(checkUrl.href, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${apiToken}`
        }
      });
      
      if (response.ok) {
        const data = await response.json();
        isBookmarked = data.exists;
        updateBookmarkStatus();
      } else {
        console.error('Error checking bookmark status:', response.status);
        // Default to not bookmarked on error
        isBookmarked = false;
        updateBookmarkStatus();
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
      updateStatus('loading', 'Extracting page content...');
      
      const { endpoint, apiToken } = await browserAPI.storage.sync.get(['endpoint', 'apiToken']);
      
      if (!endpoint || !apiToken) {
        updateStatus('error', 'Extension not configured');
        saveBtn.disabled = false;
        return;
      }

      // Extract page content from the active tab
      let pageContent = null;
      try {
        updateStatus('loading', 'Extracting page content...');
        
        // Use browser.tabs.executeScript to inject and execute content extraction
        const results = await browserAPI.tabs.executeScript(currentTab.id, {
          code: `
            (() => {
              try {
                // Get the full HTML of the page
                const htmlContent = document.documentElement.outerHTML;
                
                // Get the text content of the page
                const textContent = document.body ? document.body.innerText || document.body.textContent : '';
                
                // Clean up text content by removing excessive whitespace
                const cleanTextContent = textContent.replace(/\\s+/g, ' ').trim();
                
                return {
                  success: true,
                  content: {
                    htmlContent: htmlContent,
                    textContent: cleanTextContent
                  }
                };
              } catch (error) {
                return {
                  success: false,
                  error: error.message
                };
              }
            })();
          `
        });
        
        if (results && results[0]) {
          const result = results[0];
          if (result.success) {
            console.log("Content extracted successfully", result);
            pageContent = result.content;
          } else {
            throw new Error(result.error || 'Failed to extract content');
          }
        } else {
          throw new Error('No result from content script');
        }
      } catch (contentError) {
        console.warn('Failed to extract page content, saving without content:', contentError);
        // Continue without content if extraction fails
      }

      updateStatus('loading', 'Saving...');
      
      // Prepare the request body
      const requestBody = { link: currentTab.url };
      if (pageContent) {
        requestBody.htmlContent = pageContent.htmlContent;
        requestBody.textContent = pageContent.textContent;
      }
      
      const response = await fetch(new URL("/api/v1/bookmarks", endpoint).href, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${apiToken}`
        },
        body: JSON.stringify(requestBody)
      });
      
      if (response.ok) {
        isBookmarked = true;
        const successMessage = pageContent ? 'Page saved with content!' : 'Page saved successfully!';
        updateStatus('saved', successMessage);
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