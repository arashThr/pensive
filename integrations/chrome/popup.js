// For cross-browser compatibility
const browserAPI = !(window.browser && browser.runtime) ? chrome : browser;

document.addEventListener('DOMContentLoaded', async () => {
  const statusElement = document.getElementById('status');
  const notConfiguredStatus = document.getElementById('notConfiguredStatus');
  const pageTitleElement = document.getElementById('pageTitle');
  const pageUrlElement = document.getElementById('pageUrl');
  const saveBtn = document.getElementById('saveBtn');
  const removeBtn = document.getElementById('removeBtn');
  const actionsSaved = document.getElementById('actions-saved');
  const viewSavedBtn = document.getElementById('viewSavedBtn');
  const removeSavedBtn = document.getElementById('removeSavedBtn');
  const settingsLink = document.getElementById('settingsLink');
  const searchLink = document.getElementById('searchLink');
  const createAccountBtn = document.getElementById('createAccountBtn');
  const connectAccountBtn = document.getElementById('connectAccountBtn');

  let currentTab = null;
  let isBookmarked = false;
  let bookmarkId = null;

  // Get current tab information
  try {
    const tabs = await browserAPI.tabs.query({ active: true, currentWindow: true });
    currentTab = tabs[0];

    // Update page info
    pageTitleElement.textContent = currentTab.title;
    pageUrlElement.textContent = currentTab.url;
  } catch (error) {
    console.error('[Pensive] Error getting current tab:', error);
    updateStatus('error', 'Error loading page information');
    return;
  }

  // Check if current page is bookmarked
  await checkBookmarkStatus();

  // Event listeners
  saveBtn.addEventListener('click', saveBookmark);
  removeBtn.addEventListener('click', removeBookmark);
  viewSavedBtn.addEventListener('click', viewSavedPage);
  removeSavedBtn.addEventListener('click', removeBookmark);
  settingsLink.addEventListener('click', openSettings);
  searchLink.addEventListener('click', openSearch);
  createAccountBtn.addEventListener('click', (e) => {
    e.preventDefault();
    browserAPI.tabs.create({ url: 'https://getpensive.com/signup' });
  });
  connectAccountBtn.addEventListener('click', (e) => {
    e.preventDefault();
    browserAPI.runtime.openOptionsPage();
  });

  async function checkBookmarkStatus() {
    if (!currentTab) return;

    // if page is not http, return
    if (!currentTab.url.startsWith('http')) {
      updateStatus('error', 'Page is not a valid URL');
      saveBtn.disabled = true;
      removeBtn.disabled = true;
      return;
    }

    // Don't save getpensive.com pages
    if (currentTab.url.includes('getpensive.com')) {
      updateStatus('error', 'Cannot save Pensive pages');
      saveBtn.disabled = true;
      removeBtn.disabled = true;
      return;
    }

    try {
      updateStatus('loading', 'Checking status...');

      const { endpoint, apiToken } = await browserAPI.storage.local.get(['endpoint', 'apiToken']);

      if (!endpoint || !apiToken) {
        updateStatus('not-configured', 'Account not connected');
        // Hide action buttons when not configured
        document.getElementById('actions').style.display = 'none';
        actionsSaved.style.display = 'none';
        // Show auto-save notice
        const autoSaveNotice = document.getElementById('autoSaveNotice');
        if (autoSaveNotice) {
          autoSaveNotice.style.display = 'block';
        }
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
        if (data.exists && data.bookmark && data.bookmark.Id) {
          bookmarkId = data.bookmark.Id;
        } else {
          bookmarkId = null;
        }
        updateBookmarkStatus();
      } else {
        // Default to not bookmarked on error
        isBookmarked = false;
        bookmarkId = null;
        updateBookmarkStatus();
      }

    } catch (error) {
      // Default to not bookmarked on error
      isBookmarked = false;
      bookmarkId = null;
      updateBookmarkStatus();
    }
  }

  function updateBookmarkStatus() {
    // Hide auto-save notice
    const autoSaveNotice = document.getElementById('autoSaveNotice');
    if (autoSaveNotice) {
      autoSaveNotice.style.display = 'none';
    }
    
    if (isBookmarked) {
      updateStatus('saved', 'Saved to your library');
      // Show saved actions, hide regular actions
      document.getElementById('actions').style.display = 'none';
      actionsSaved.style.display = 'flex';
    } else {
      updateStatus('not-saved', 'Click Save button to bookmark');
      // Show regular actions, hide saved actions
      document.getElementById('actions').style.display = 'flex';
      actionsSaved.style.display = 'none';
      saveBtn.disabled = false;
      removeBtn.disabled = true;
    }
  }

  async function saveBookmark() {
    if (!currentTab) return;

    saveBtn.disabled = true;
    updateStatus('loading', 'Extracting page content...');

    const { endpoint, apiToken, fullPageCapture } = await browserAPI.storage.local.get(['endpoint', 'apiToken', 'fullPageCapture']);

    if (!endpoint || !apiToken) {
      updateStatus('not-configured', 'Account not connected');
      // Hide action buttons when not configured
      document.getElementById('actions').style.display = 'none';
      actionsSaved.style.display = 'none';
      // Show auto-save notice
      const autoSaveNotice = document.getElementById('autoSaveNotice');
      if (autoSaveNotice) {
        autoSaveNotice.style.display = 'block';
      }
      return;
    }

    let pageContent = {
      link: currentTab.url,
      extractionMethod: 'server-side'
    };

    // If server-side only is enabled, skip client-side extraction
    if (fullPageCapture) {
      let readabilitySucceeded = false;

      try {
        updateStatus('loading', 'Extracting page content...');

        await browserAPI.scripting.executeScript({
          target: { tabId: currentTab.id },
          files: ['Readability-readerable.js', 'Readability.js'],
        });
        
        const readabilityResults = await browserAPI.scripting.executeScript({
          target: { tabId: currentTab.id },
          func: parseWithReadability
        });

        const result = readabilityResults?.[0]?.result;
        if (result?.success) {
          readabilitySucceeded = true;
          let publishedDate = Date.parse(result.content.publishedTime || document.querySelector('meta[property="article:published_time"]')?.content)
          if (isNaN(publishedDate)) {
            publishedDate = Date.now()
          }
          pageContent.title = result.content.title || currentTab.title;
          pageContent.excerpt = result.content.excerpt || document.querySelector('meta[name="description"]')?.content || '';
          pageContent.lang = result.content.lang || document.documentElement.lang;
          pageContent.siteName = result.content.siteName || document.querySelector('meta[property="og:site_name"]')?.content || document.title;
          pageContent.publishedTime = new Date(publishedDate).toISOString()
          pageContent.textContent = result.content.textContent || document.body.textContent;
          pageContent.extractionMethod = 'client-readability';
        }

        // Use chrome.scripting to inject and execute content extraction
        const contentResults = await browserAPI.scripting.executeScript({
          target: { tabId: currentTab.id },
          func: extractContent
        });

        const htmlResult = contentResults?.[0]?.result;
        if (htmlResult?.success) {
          pageContent.htmlContent = htmlResult.htmlContent;
          updateStatus('loading', 'Content cleaned and processed...');
          pageContent.extractionMethod = pageContent.extractionMethod === 'client-readability'
            ? 'client-readability-html'
            : 'client-html';
        }
      } catch (contentError) {
        updateStatus('error', 'Failed to extract page content. Continuing with server-side extraction...');
      }
    }

    let response = null;
    try {
      updateStatus('loading', 'Saving...');

      // Prepare the request body
      let requestBody = pageContent;
      response = await fetch(new URL("/api/v1/bookmarks", endpoint).href, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${apiToken}`
        },
        body: JSON.stringify(requestBody)
      });
    } catch (error) {
      if (response && response.status === 401) {
        updateStatus('error', 'Unauthorized - Go to setting and re-connect');
      } else {
        updateStatus('error', 'Network error occurred: ' + error.message);
      }
      saveBtn.disabled = false;
      return
    }

    try {
      if (response.ok) {
        isBookmarked = true;
        const responseData = await response.json();
        if (responseData.Id) {
          bookmarkId = responseData.Id;
        }
        const successMessage = pageContent.htmlContent ? 'Page saved with content!' : 'Page saved successfully!';
        updateStatus('saved', successMessage);
        // Switch to saved state UI
        document.getElementById('actions').style.display = 'none';
        actionsSaved.style.display = 'flex';

        // Auto-hide success message after 2 seconds
        setTimeout(() => {
          if (isBookmarked) {
            updateStatus('saved', 'Saved to your library');
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
            // The bookmark ID should have been retrieved during the initial status check
            // so we don't need to extract it from the error response
            updateStatus('saved', 'Page already saved');
            // Switch to saved state UI
            document.getElementById('actions').style.display = 'none';
            actionsSaved.style.display = 'flex';
            return;
          }
          errorMessage = errorData.errorMessage || errorMessage;
        } else {
          errorMessage = await response.text() || errorMessage;
        }

        if (response.status === 401) {
          updateStatus('error', 'Unauthorized - Go to setting and re-connect');
        } else {
          updateStatus('error', errorMessage);
        }
        saveBtn.disabled = false;
      }

    } catch (error) {
      updateStatus('error', 'Network error occurred: ' + error.message + ' ' + error.stack);
      saveBtn.disabled = false;
    }
  }

  async function removeBookmark() {
    if (!currentTab) return;

    try {
      removeBtn.disabled = true;
      updateStatus('loading', 'Removing...');

      const { endpoint, apiToken } = await browserAPI.storage.local.get(['endpoint', 'apiToken']);

      if (!endpoint || !apiToken) {
        updateStatus('not-configured', 'Account not connected');
        // Hide action buttons when not configured
        document.getElementById('actions').style.display = 'none';
        actionsSaved.style.display = 'none';
        // Show auto-save notice
        const autoSaveNotice = document.getElementById('autoSaveNotice');
        if (autoSaveNotice) {
          autoSaveNotice.style.display = 'block';
        }
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
        bookmarkId = null;
        updateStatus('not-saved', 'Page removed successfully!');
        // Switch to not-saved state UI
        document.getElementById('actions').style.display = 'flex';
        actionsSaved.style.display = 'none';
        saveBtn.disabled = false;
        removeBtn.disabled = true;

        // Auto-hide success message after 2 seconds
        setTimeout(() => {
          if (!isBookmarked) {
            updateStatus('not-saved', 'Click Save button to bookmark');
          }
        }, 2000);
      } else if (response.status === 404) {
        // Bookmark doesn't exist, which is fine for removal
        isBookmarked = false;
        bookmarkId = null;
        updateStatus('not-saved', 'Click Save button to bookmark');
        // Switch to not-saved state UI
        document.getElementById('actions').style.display = 'flex';
        actionsSaved.style.display = 'none';
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
      console.error('[Pensive] Error removing bookmark:', error);
      updateStatus('error', 'Network error occurred');
      removeBtn.disabled = false;
    }
  }

  function updateStatus(type, message) {
    if (type === 'not-configured') {
      // Hide the regular status and show the not-configured status
      statusElement.style.display = 'none';
      notConfiguredStatus.style.display = 'block';
      return;
    }

    // Show the regular status and hide the not-configured status
    statusElement.style.display = 'block';
    notConfiguredStatus.style.display = 'none';
    
    statusElement.className = `status ${type}`;

    // Clear existing content
    statusElement.innerHTML = '';

    if (type === 'loading') {
      const spinner = document.createElement('div');
      spinner.className = 'spinner';
      
      const span = document.createElement('span');
      span.textContent = message;
      
      statusElement.appendChild(spinner);
      statusElement.appendChild(span);
    } else {
      const span = document.createElement('span');
      span.textContent = message;
      statusElement.appendChild(span);
    }
  }

  function openSettings() {
    browserAPI.runtime.openOptionsPage();
  }

  async function viewSavedPage() {
    if (!currentTab || !bookmarkId) return;

    try {
      const { endpoint } = await browserAPI.storage.local.get(['endpoint']);

      if (!endpoint) {
        // If no endpoint is configured, show an error or open options
        browserAPI.runtime.openOptionsPage();
        return;
      }

      // Open the specific bookmark page using the bookmark ID
      const bookmarkUrl = new URL(`/bookmarks/${bookmarkId}/edit`, endpoint);
      await browserAPI.tabs.create({ url: bookmarkUrl.href });

      // Close the popup
      window.close();
    } catch (error) {
      // Fallback to opening options page
      browserAPI.runtime.openOptionsPage();
    }
  }

  async function openSearch() {
    try {
      const { endpoint } = await browserAPI.storage.local.get(['endpoint']);

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
      // Fallback to opening options page
      browserAPI.runtime.openOptionsPage();
    }
  }
});

function extractContent() {
  /**
   * Cleans HTML by removing noise elements and optionally extracting main content.
   * @param {string|Element} input - HTML string or DOM element to clean.
   * @param {Object} [options] - Options object.
   * @param {number|null} [options.maxChars=null] - Truncate output text to this many characters.
   * @returns {string} - Cleaned HTML string.
   */
  function extractMainHtmlContent(input, { maxChars = null } = {}) {
    // Convert input to DOM if it's a string
    let doc;
    if (typeof input === 'string') {
      const parser = new DOMParser();
      doc = parser.parseFromString(input, 'text/html');
    } else if (input instanceof Element || input instanceof Document) {
      doc = input;
    } else {
      throw new Error('Input must be an HTML string or DOM element');
    }

    let workingDoc = doc;

    // 1. Try to find main content area
    const contentSelectors = [
      'main',
      'article',
      '.post-content',
      '.entry-content',
      '.content',
      '[role="main"]',
      '.post',
      '.hrecipe',
      '[itemtype*="Recipe"]'
    ];

    let mainContent = null;
    let bestElement = null;
    let maxTextLength = 0;

    for (const selector of contentSelectors) {
      const elements = doc.querySelectorAll(selector);
      if (elements.length) {
        for (const element of elements) {
          const textContent = element.textContent.trim();
          if (textContent.length > maxTextLength) {
            maxTextLength = textContent.length;
            bestElement = element;
          }
        }
        if (maxTextLength > 500) {
          mainContent = bestElement;
          break;
        }
      }
    }

    // 2. Clean the HTML
    if (mainContent) {
      // Work with main content area
      const tempDiv = document.createElement('div');
      tempDiv.appendChild(mainContent.cloneNode(true));
      workingDoc = new DOMParser().parseFromString(tempDiv.innerHTML, 'text/html');

      // Remove minimal noise
      const minimalNoiseSelectors = [
        'script',
        'style',
        'noscript',
        '.advertisement',
        '.ads',
        '.ad-container',
        '.social-share',
        '.share-buttons',
        '[aria-hidden="true"]'
      ];
      for (const sel of minimalNoiseSelectors) {
        const nodes = workingDoc.querySelectorAll(sel);
        for (const node of nodes) {
          node.remove();
        }
      }
    } else {
      // Fallback: clean the whole page conservatively
      const conservativeNoiseSelectors = [
        'script',
        'style',
        'noscript',
        'iframe',
        'embed',
        'object',
        'nav',
        'header',
        'footer',
        '[role="navigation"]',
        '[role="banner"]',
        '[role="contentinfo"]',
        '[aria-hidden="true"]',
        '.advertisement',
        '.ads',
        '.ad-container',
        '.sidebar-ads',
        '.social-share',
        '.share-buttons',
        '.social-media',
        '.cookie-notice',
        '.newsletter-signup'
      ];
      for (const sel of conservativeNoiseSelectors) {
        const nodes = workingDoc.querySelectorAll(sel);
        for (const node of nodes) {
          node.remove();
        }
      }
    }

    // 3. Serialize cleaned HTML
    let cleanedHtml = workingDoc.body.innerHTML;

    // Optional text truncation (based on text content length)
    if (maxChars !== null) {
      const tempDoc = new DOMParser().parseFromString(cleanedHtml, 'text/html');
      const textContent = tempDoc.body.textContent.trim();
      if (textContent.length > maxChars) {
        // Truncate by removing elements from the end until text content is under maxChars
        const nodes = tempDoc.body.childNodes;
        while (tempDoc.body.textContent.trim().length > maxChars && nodes.length) {
          nodes[nodes.length - 1].remove();
        }
        cleanedHtml = tempDoc.body.innerHTML + '<!-- ...(truncated) -->';
      }
    }

    return cleanedHtml;
  }

  /**
   * Cleans HTML content string by removing noise elements.
   * @param {string} htmlContent - HTML content string to clean.
   * @returns {string} - Cleaned HTML content string.
   */
  function cleanHtmlContentString(htmlContent) {
    // Ensure input is a string
    if (typeof htmlContent !== 'string') {
      throw new Error('Input must be an HTML string');
    }

    // Regular expressions with fixed patterns (replace /s with [\s\S]*?)
    const scriptRe = /<script[^>]*>[\s\S]*?<\/script>/gi;
    const styleRe = /<style[^>]*>[\s\S]*?<\/style>/gi;
    const commentRe = /<!--[\s\S]*?-->/g;
    const trackingRe = /<[^>]*?(?:data-track|data-analytics|onclick|onload|onerror)[^>]*>/gi;
    const attrRe = /\s+(?:class|id|style|data-[^=]*|onclick|onload|onerror|width|height|align|bgcolor|border|cellpadding|cellspacing|valign)="[^"]*"/gi;
    const whitespaceRe = /\s+/g;

    // Apply cleaning steps
    let cleaned = htmlContent;
    cleaned = cleaned.replace(scriptRe, ' ');
    cleaned = cleaned.replace(styleRe, ' ');
    cleaned = cleaned.replace(commentRe, ' ');
    cleaned = cleaned.replace(trackingRe, ' ');
    cleaned = cleaned.replace(attrRe, ' ');
    cleaned = cleaned.replace(whitespaceRe, ' ');

    // Trim leading/trailing whitespace
    return cleaned.trim();
  }

  /**
   * Only keep the main content of the page and throw away all the meta content.
   * @param {Document} doc - Document to extract clean HTML content from.
   * @returns {string} - Cleaned HTML content string.
   */
  function extractCleanHtmlWhitelist(doc) {
    // Allowed tags
    const allowedTags = [
      'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
      'title', 'meta',
      'header', 'nav', 'main', 'article', 'section', 'aside', 'footer',
      'table', 'tr', 'td', 'th', 'thead', 'tbody', 'tfoot', 'caption', 'colgroup', 'col',
      'ul', 'ol', 'li', 'dl', 'dd', 'dt'
    ].map(tag => tag.toLowerCase());

    // Select main if present, otherwise body
    const container = doc.querySelector('main') || doc.body;

    // Remove non-allowed tags while preserving their content
    container.querySelectorAll('*').forEach(el => {
      const tagName = el.tagName.toLowerCase();
      if (!allowedTags.includes(tagName)) {
        const parent = el.parentNode;
        while (el.firstChild) {
          parent.insertBefore(el.firstChild, el);
        }
        parent.insertBefore(document.createTextNode(' '), el);
        el.remove();
      }
    });

    // Remove hidden elements
    container.querySelectorAll('*').forEach(el => {
      const style = window.getComputedStyle(el);
      if (style.display === 'none' || style.visibility === 'hidden') {
        el.remove();
      }
    });

    // Create valid HTML document
    const htmlDoc = document.implementation.createHTMLDocument('');
    const newBody = htmlDoc.createElement('body');
    newBody.appendChild(container.cloneNode(true));

    // Serialize to valid HTML
    const serializer = new XMLSerializer();
    const cleanedHtml = `<!DOCTYPE html><html><head></head><body>${serializer.serializeToString(newBody)}</body></html>`;

    return cleanedHtml;
  }

  try {
    // Extract clean HTML content
    const originalHtml = document.documentElement.outerHTML;
    const originalSize = originalHtml.length;
    // console.log(`Original HTML size: ${originalSize} characters`);

    const htmlContent = cleanHtmlContentString(originalHtml);
    // const afterLLMCleanSize = htmlContent.length;
    // const llmReduction = ((originalSize - afterLLMCleanSize) / originalSize * 100).toFixed(1);
    // console.log(`After LLM clean: ${afterLLMCleanSize} characters (${llmReduction}% reduction)`);

    // turn htmlContent to document
    const cleanedHtml = extractMainHtmlContent(htmlContent, { maxChars: 100_000 });
    // const afterGeneralCleanSize = cleanedHtml.length;
    // const generalReduction = ((afterLLMCleanSize - afterGeneralCleanSize) / afterLLMCleanSize * 100).toFixed(1);
    // console.log(`After general clean: ${afterGeneralCleanSize} characters (${generalReduction}% reduction from previous step)`);

    const doc = new DOMParser().parseFromString(cleanedHtml, 'text/html');
    const htmlContentWhitelist = extractCleanHtmlWhitelist(doc);
    // const finalSize = htmlContentWhitelist.length;
    // const whitelistReduction = ((afterGeneralCleanSize - finalSize) / afterGeneralCleanSize * 100).toFixed(1);
    // const totalReduction = ((originalSize - finalSize) / originalSize * 100).toFixed(1);
    // console.log(`After whitelist clean: ${finalSize} characters (${whitelistReduction}% reduction from previous step)`);
    // console.log(`Total reduction: ${totalReduction}% (${originalSize} → ${finalSize} characters)`);

    // Get page title

    return {
      success: true,
      htmlContent: htmlContentWhitelist,
    };
  } catch (error) {
    console.error("[Pensive] Capturing content failed: ", error)
    return {
      success: false,
      error: error.message
    };
  }
}

function parseWithReadability() {
  let article;
  try {
    // Check if the page is probably readable
    const isReadable = isProbablyReaderable(document);
    if (!isReadable) {
      return {
        success: false,
        error: 'Page is not readable'
      };
    }

    // Use Readability to extract the article content
    // clone the document
    const documentClone = document.cloneNode(true);
    const reader = new Readability(documentClone);
    article = reader.parse();
  } catch (error) {
    return {
      success: false,
      error: error.message
    };
  }

  if (article && article.content) {
    result = {
      success: true,
      isReadable: true,
      content: {
        title: article.title,
        lang: article.lang,
        siteName: article.siteName,
        publishedTime: article.publishedTime,
        textContent: article.textContent,
        excerpt: article.excerpt,
      }
    }
    return result;
  }
}