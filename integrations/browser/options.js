document.getElementById("save").addEventListener("click", () => {
    const endpoint = document.getElementById("endpoint").value;
    const apiKey = document.getElementById("apiKey").value;
    const folderName = document.getElementById("folderName").value || "Archive"; // Default to "Archive" if empty
    chrome.storage.sync.set({ endpoint, apiKey, folderName }, () => {
      const status = document.getElementById("status");
      status.textContent = "Settings saved!";
      setTimeout(() => (status.textContent = ""), 2000);
    });
  });
  
  // Load saved settings
  chrome.storage.sync.get(["endpoint", "apiKey", "folderName"], (data) => {
    document.getElementById("endpoint").value = data.endpoint || "";
    document.getElementById("apiKey").value = data.apiKey || "";
    document.getElementById("folderName").value = data.folderName || "Archive";
  });