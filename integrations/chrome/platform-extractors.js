/**
 * Content extraction logic for specific platforms (Twitter, Reddit, YouTube)
 * Falls back to Readability for general web pages
 */

/**
 * Waits for an element to appear in the DOM (useful for SPAs)
 * @param {string} selector - CSS selector to wait for
 * @param {number} timeout - Maximum time to wait in milliseconds (default: 3000)
 * @returns {Promise<Element>} - Promise that resolves with the element or rejects on timeout
 */
function waitForElement(selector, timeout = 3000) {
  return new Promise((resolve, reject) => {
    const el = document.querySelector(selector);
    if (el) return resolve(el);

    const observer = new MutationObserver(() => {
      const el = document.querySelector(selector);
      if (el) {
        observer.disconnect();
        resolve(el);
      }
    });

    observer.observe(document.body, { childList: true, subtree: true });
    setTimeout(() => {
      observer.disconnect();
      reject(new Error(`Timeout waiting for element: ${selector}`));
    }, timeout);
  });
}

/**
 * Waits for any selector in a list to appear in the DOM
 * @param {string[]} selectors - CSS selectors to wait for
 * @param {number} timeout - Maximum time to wait in milliseconds (default: 3000)
 * @returns {Promise<Element>} - Promise that resolves with the first matching element
 */
function waitForAnyElement(selectors, timeout = 3000) {
  return new Promise((resolve, reject) => {
    for (const selector of selectors) {
      const el = document.querySelector(selector);
      if (el) return resolve(el);
    }

    const observer = new MutationObserver(() => {
      for (const selector of selectors) {
        const el = document.querySelector(selector);
        if (el) {
          observer.disconnect();
          resolve(el);
          return;
        }
      }
    });

    observer.observe(document.body, { childList: true, subtree: true });
    setTimeout(() => {
      observer.disconnect();
      reject(new Error(`Timeout waiting for selectors: ${selectors.join(', ')}`));
    }, timeout);
  });
}

/**
 * Extracts a tweet from X/Twitter /status/ page
 * @returns {Promise<Object|null>} - Tweet object or null if extraction fails
 */
async function extractTweet() {
  try {
    // Wait for the tweet article element (since Twitter is an SPA)
    await waitForElement('article[data-testid="tweet"]', 3000);
    
    const article = document.querySelector('article[data-testid="tweet"]');
    if (!article) return null;

    const text = article.querySelector('[data-testid="tweetText"]')?.innerText;
    const author = article.querySelector('[data-testid="User-Name"]')?.innerText;
    const timestamp = article.querySelector('time')?.getAttribute('datetime');

    return {
      text,
      author,
      timestamp,
      platform: 'twitter',
      type: 'status'
    };
  } catch (error) {
    console.error('[Pensive] Failed to extract tweet:', error);
    return null;
  }
}

/**
 * Extracts a Reddit post from new Reddit (shreddit) or old Reddit
 * @returns {Promise<Object|null>} - Post object or null if extraction fails
 */
async function extractRedditPost() {
  try {
    // Try new Reddit first (shreddit)
    let post = document.querySelector('shreddit-post');
    if (post) {
      return {
        title: post.getAttribute('post-title'),
        author: post.getAttribute('author'),
        score: post.getAttribute('score'),
        body: post.querySelector('[slot="text-body"]')?.innerText,
        platform: 'reddit',
        type: 'post'
      };
    }

    // Fallback to old Reddit
    const title = document.querySelector('.top-matter .title a')?.innerText;
    const body = document.querySelector('.expando .md')?.innerText;
    
    if (title) {
      return {
        title,
        body,
        platform: 'reddit',
        type: 'post'
      };
    }

    return null;
  } catch (error) {
    console.error('[Pensive] Failed to extract Reddit post:', error);
    return null;
  }
}

/**
 * Extracts a YouTube video's metadata and description from watch pages
 * @returns {Promise<Object|null>} - Video object or null if extraction fails
 */
async function extractYouTubeVideo() {
  try {
    const descriptionSelectors = ['#description-inline-expander', '#snippet'];
    let descriptionElement = null;

    for (const selector of descriptionSelectors) {
      const el = document.getElementById(selector.replace('#', ''));
      if (el) {
        descriptionElement = el;
        break;
      }
    }

    if (!descriptionElement) {
      try {
        descriptionElement = await waitForAnyElement(descriptionSelectors, 4000);
      } catch (error) {
        // Description can be lazily rendered; continue with other metadata.
      }
    }

    const title = document.querySelector('h1.ytd-watch-metadata yt-formatted-string')?.innerText
      || document.querySelector('h1.title')?.innerText
      || document.title?.replace(/\s*-\s*YouTube\s*$/, '').trim()
      || 'YouTube Video';
    const author = document.querySelector('ytd-channel-name a')?.innerText
      || document.querySelector('#owner #channel-name a')?.innerText;
    const description = descriptionElement?.innerText?.trim() || '';
    const publishedTime = document.querySelector('meta[itemprop="datePublished"]')?.content
      || document.querySelector('meta[property="og:video:release_date"]')?.content;

    if (!title && !description) return null;

    return {
      title,
      author,
      body: description,
      publishedTime,
      platform: 'youtube',
      type: 'video'
    };
  } catch (error) {
    console.error('[Pensive] Failed to extract YouTube video:', error);
    return null;
  }
}

/**
 * Checks if the current page is a valid Twitter/X status page
 * @returns {boolean}
 */
function isTwitterStatusPage() {
  const url = window.location.href;
  return url.match(/twitter\.com|x\.com/) && url.includes('/status/');
}

/**
 * Checks if the current page is a valid Reddit post page
 * @returns {boolean}
 */
function isRedditPostPage() {
  const url = window.location.href;
  return url.match(/reddit\.com\/r\/.+\/comments\//);
}

/**
 * Checks if the current page is a valid YouTube watch page
 * @returns {boolean}
 */
function isYouTubeWatchPage() {
  try {
    const parsed = new URL(window.location.href);
    const hostname = parsed.hostname.toLowerCase();

    if (hostname === 'youtu.be') {
      return parsed.pathname.length > 1;
    }

    return hostname.endsWith('youtube.com')
      && parsed.pathname === '/watch'
      && parsed.searchParams.has('v');
  } catch (error) {
    return false;
  }
}

/**
 * Checks if the current page is a Twitter/X home page (not a specific post)
 * @returns {boolean}
 */
function isTwitterHomePage() {
  const url = window.location.href;
  return url.match(/twitter\.com|x\.com/) && (!url.includes('/status/'));
}

/**
 * Checks if the current page is a Reddit home/subreddit page (not a specific post)
 * @returns {boolean}
 */
function isRedditHomePage() {
  const url = window.location.href;
  if (!url.includes('reddit.com')) return false;
  // Home pages are those without /comments/ in the path
  return !url.match(/reddit\.com\/r\/.+\/comments\//);
}

/**
 * Main extraction function that routes to appropriate extractor
 * Handles both platform-specific extraction and fallback to Readability
 * @returns {Promise<Object>} - Extracted content object
 */
async function extractPageContent() {
  // Check for invalid home pages first
  if (isTwitterHomePage()) {
    return {
      success: false,
      error: 'Please navigate to a specific tweet to save it. Go to a tweet\'s page and try again.',
    };
  }

  if (isRedditHomePage()) {
    return {
      success: false,
      error: 'Please navigate to a specific post to save it. Go to a post\'s page and try again.',
    };
  }

  // Try platform-specific extraction
  if (isTwitterStatusPage()) {
    const tweet = await extractTweet();
    if (tweet) {
      return {
        success: true,
        title: tweet.author ? `Tweet by ${tweet.author}` : 'Tweet',
        excerpt: tweet.text ? tweet.text.substring(0, 200) : '',
        textContent: tweet.text || '',
        platform: 'twitter',
        timestamp: tweet.timestamp,
        extractionMethod: 'twitter-api'
      };
    }
  }

  if (isRedditPostPage()) {
    const post = await extractRedditPost();
    if (post) {
      return {
        success: true,
        title: post.title || 'Reddit Post',
        excerpt: post.body ? post.body.substring(0, 200) : '',
        textContent: post.body || '',
        platform: 'reddit',
        score: post.score,
        extractionMethod: 'reddit-dom'
      };
    }
  }

  if (isYouTubeWatchPage()) {
    const video = await extractYouTubeVideo();
    if (video) {
      return {
        success: true,
        title: video.title || 'YouTube Video',
        excerpt: video.body ? video.body.substring(0, 200) : (video.title || ''),
        textContent: video.body || video.title || '',
        platform: 'youtube',
        timestamp: video.publishedTime,
        extractionMethod: 'youtube-dom'
      };
    }
  }

  // Fallback to default behavior (return null to indicate use of Readability)
  return null;
}

/**
 * Execute extraction and return results along with validation status
 * @returns {Promise<Object>} - Extraction result with platform info
 */
async function performPlatformExtraction() {
  const result = await extractPageContent();
  
  // If we got a home page error, return it as-is
  if (result && result.isHomePageError) {
    return result;
  }

  // If we got platform-specific content, return it
  if (result && result.success) {
    return result;
  }

  // Otherwise, return null to signal that Readability should be used
  return null;
}
