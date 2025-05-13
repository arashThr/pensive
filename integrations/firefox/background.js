// For cross-browser compatibility (Chrome uses 'chrome', Firefox supports it but prefers 'browser')
const browserAPI = typeof browser !== "undefined" ? browser : chrome;

// Action item for the options
const action = browserAPI.browserAction || browserAPI.action
browserAPI.browserAction.onClicked.addListener(() => {
  browserAPI.runtime.openOptionsPage();
});

browserAPI.bookmarks.onCreated.addListener((id, bookmark) => {
  browserAPI.bookmarks.get(bookmark.parentId, (parent) => {
    // Get the configured folder name (defaults to "Archive" if not set)
    browserAPI.storage.sync.get({ folderName: "Archive" }, (data) => {
      if (parent[0].title === data.folderName) {
        sendToApi(bookmark.url, "POST");
      }
    });
  });
});

browserAPI.bookmarks.onRemoved.addListener((id, removeInfo) => {
  // Get the configured folder name (defaults to "Archive" if not set)
  browserAPI.storage.sync.get({ folderName: "Archive" }, (data) => {
    // Check if the removed bookmark's parent folder is the Archive folder
    browserAPI.bookmarks.get(removeInfo.parentId, (parent) => {
      if (parent[0].title === data.folderName) {
        // Assuming you have the URL or a unique identifier available
        // You might need to store the URL elsewhere or fetch it differently
        sendToApi(removeInfo.node.url, "DELETE"); // Call your delete endpoint with the bookmark ID or URL
      }
    });
  });
});

async function sendToApi(link, method) {
  const { endpoint, apiToken } = await browserAPI.storage.sync.get(["endpoint", "apiToken"]);
  if (!endpoint || !apiToken) {
    console.error("Endpoint or API token not configured.");
    return;
  }
  const createEndpoint = new URL("/api/v1/bookmarks", endpoint).href
  try {
    const response = await fetch(createEndpoint, {
      method,
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${apiToken}`,
      },
      body: JSON.stringify({ link }),
    });
    if (response.status !== 200) {
      // If response is JSON, decode it
      // Otherwise, parse it as text
      const contentType = response.headers.get("Content-Type");
      let error
      if (contentType?.includes("application/json")) {
        const responseBody = await response.json();
        error = parseToError(responseBody);
        console.log("Request failed:", error);
      } else {
        error = await response.text();
        console.error("Unexpected response:", error);
      }
      return;
    }
    const result = await response.json();
    console.log("Request succeeded:", result);
  } catch (error) {
    console.error("Network error:", error);
  }
}

function parseToError(result) {
  const errorCode = result.errorCode || "UNKNOWN_ERROR";
  const errorMessage = result.errorMessage || "Unknown error occurred";
  return { errorCode, errorMessage };
}