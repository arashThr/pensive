document.getElementById("save").addEventListener("click", () => {
    const endpoint = document.getElementById("endpoint").value;
    const apiKey = document.getElementById("apiKey").value;
    chrome.storage.sync.set({ endpoint, apiKey }, () => {
      const status = document.getElementById("status");
      status.textContent = "Settings saved!";
      setTimeout(() => (status.textContent = ""), 2000);
    });
  });
  
  // Load saved settings
  chrome.storage.sync.get(["endpoint", "apiKey"], (data) => {
    document.getElementById("endpoint").value = data.endpoint || "";
    document.getElementById("apiKey").value = data.apiKey || "";
  });