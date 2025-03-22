// For cross-browser compatibility (Chrome uses 'chrome', Firefox supports it but prefers 'browser')
const browserAPI = typeof chrome !== "undefined" ? chrome : browser;

browserAPI.bookmarks.onCreated.addListener((id, bookmark) => {
  browserAPI.bookmarks.get(bookmark.parentId, (parent) => {
    console.log("parent", parent[0].title);
    // Get the configured folder name (defaults to "archive" if not set)
    browserAPI.storage.sync.get({ folderName: "archive" }, (data) => {
      if (parent[0].title === data.folderName) {
        sendToApi(bookmark.url, bookmark.title);
      }
    });
  });
});

async function sendToApi(link) {
  const { endpoint, apiKey } = await browserAPI.storage.sync.get(["endpoint", "apiKey"]);
  if (!endpoint || !apiKey) {
    console.error("Endpoint or API key not configured.");
    return;
  }

  try {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${apiKey}`,
      },
      body: JSON.stringify({ link }),
    });
    const result = await response.json();
    if (response.status !== 200) {
      const error = parseToError(result);
      console.error("Error:", error);
      return;
    }
    console.log("Success:", result);
  } catch (error) {
    console.error("Error:", error);
  }
}

function parseToError(result) {
  let errorCode = result.errorCode || "UNKNOWN_ERROR";
  let errorMessage = result.errorMessage || "Unknown error occurred";
  return { errorCode, errorMessage };
}