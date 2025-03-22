chrome.bookmarks.onCreated.addListener((id, bookmark) => {
  // Check if bookmark is in the "archive" folder (you’ll need its ID)
  chrome.bookmarks.get(bookmark.parentId, (parent) => {
    console.log("parent", parent[0].title);
    if (parent[0].title === "archive") {
      sendToApi(bookmark.url, bookmark.title);
    }
  });
});

async function sendToApi(link) {
  const { endpoint, apiKey } = await chrome.storage.sync.get([
    "endpoint",
    "apiKey",
  ]);
  if (!endpoint || !apiKey) {
    console.error("Endpoint or API key not configured.");
    return;
  }

  try {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${apiKey}`,
      },
      body: JSON.stringify({ link }),
    });
    const result = await response.json();
    if (response.status !== 200) {
      const error = parseToError(result);
      console.error("Error:", result);
      return;
    }
  } catch (error) {
    console.error("Error:", error);
  }
}

function parseToError(result) {
  let errorCode = result.errorCode;
  let errorMessage = result.errorMessage;
  if (!errorCode) {
    errorCode = "UNKNOWN_ERROR";
  }
  if (!errorMessage) {
    errorMessage = "Unknown error occurred";
  }
  return { errorCode, errorMessage };
}
