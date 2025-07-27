// For cross-browser compatibility (Chrome uses 'chrome', Firefox supports it but prefers 'browser')
const isChrome = !(window.browser && browser.runtime)
const browserAPI = isChrome ? chrome : browser;

document.addEventListener('DOMContentLoaded', function () {
  const consentCheckbox = document.getElementById('consent-checkbox');
  const continueBtn = document.getElementById('continue-btn');
  const serverOnlyBtn = document.getElementById('server-only-btn');

  // Check if user has already given consent
  checkConsentStatus();

  // Handle checkbox change
  consentCheckbox.addEventListener('change', function() {
    continueBtn.disabled = !this.checked;
  });

  // Handle continue button click
  continueBtn.addEventListener('click', async function() {
    if (consentCheckbox.checked) {
      try {
        // Store consent in extension storage (default to enhanced capture)
        await browserAPI.storage.local.set({ 
          consentGiven: true,
          consentDate: new Date().toISOString(),
          fullPageCapture: true
        });
        
        // Navigate to options page
        window.location.href = 'options.html';
      } catch (error) {
        console.error('Failed to save consent:', error);
        alert('Failed to save consent. Please try again.');
      }
    }
  });

  // Handle server-only button click
  serverOnlyBtn.addEventListener('click', async function() {
    try {
      // Store preference for server-side only capture
      await browserAPI.storage.local.set({ 
        consentGiven: true,
        consentDate: new Date().toISOString(),
        fullPageCapture: false
      });
      
      // Navigate to options page
      window.location.href = 'options.html';
    } catch (error) {
      console.error('Failed to save server-only preference:', error);
      alert('Failed to save preference. Please try again.');
    }
  });

  async function checkConsentStatus() {
    try {
      const result = await browserAPI.storage.local.get(['consentGiven']);
      
      // If consent is already given, redirect to options page
      if (result.consentGiven) {
        window.location.href = 'options.html';
      }
    } catch (error) {
      console.error('Failed to check consent status:', error);
    }
  }
}); 