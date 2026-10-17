# How to setup Gemini TTS
 
Gemini Text-to-Speech (TTS) is powered directly by Google AI Studio through the official `google.golang.org/genai` Go SDK.
 
## Requirements
 
1. Obtain an API key from [Google AI Studio](https://aistudio.google.com/).
2. Add your key to your `.env` file:
 
```bash
GEMINI_API_KEY=your_gemini_api_key_here
```
 
## Configuration (Optional)
 
You can customize the models and voices in your `.env` file:
 
```bash
GEMINI_FLASH_MODEL=gemini-3.5-flash-lite
GEMINI_PRO_MODEL=gemini-3.8-flash
GEMINI_TTS_MODEL=gemini-3.8-flash-lite-tts
```
 
No Google Cloud service account or OAuth credentials are required. Audio generation is handled directly via `GEMINI_API_KEY`.
