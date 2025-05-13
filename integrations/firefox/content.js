// Alternative approach that doesn't require waiting for DOMContentLoaded
// and works even if the script runs after page load
(function() {
  // Try to find the token immediately
  let tokenField = document.getElementById('token');
  
  // If not found, set up a small delay to try again
  // (in case our script runs before the element is available)
  if (!tokenField) {
    setTimeout(function() {
      tokenField = document.getElementById('token');
      if (tokenField && tokenField.value) {
        chrome.runtime.sendMessage({
          type: "AUTH_TOKEN",
          token: tokenField.value,
        });
        console.log("Token found and sent (delayed)");
      }
    }, 500);
  } else if (tokenField && tokenField.value) {
    chrome.runtime.sendMessage({
      type: "AUTH_TOKEN",
      token: tokenField.value,
    });
    console.log("Token found and sent (immediate)");
  }
})();
