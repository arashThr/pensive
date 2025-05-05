// Listen for the page to load
// The auth endpoint returns JSON, so we need to parse it
const response = document.body.textContent;
try {
  const data = JSON.parse(response);
  if (data.token) {
    // Send the token back to the options page
    chrome.runtime.sendMessage({
      type: "AUTH_TOKEN",
      token: data.token,
    });
  } else if (data.errorCode) {
    console.error("Error from auth response:", data);
  }
} catch (e) {
  console.error("Failed to parse response:", e);
}
