# How to setup Gemini TTS

On local, we can use Google Cloud CLI for authentication.
To authenticate on the server, you will need service acocunt key.

## Create Service Account

1. Go to **Google Cloud Console**
2. IAM & Admin → Service Accounts
3. Click **Create Service Account**

## Add Required Roles

Grant:

* **Vertex AI Service Agent**
* **Vertex AI User**

(Optional but safe alternative: `Editor` if testing only)

## Create JSON Key

1. Open the service account
2. Go to **Keys**
3. Click **Add Key**
4. Select **JSON**
5. Download the file
6. Upload it to your server as:

```bash
service-account.json
```

Secure it:

```bash
chmod 600 service-account.json
```

## Enable Required APIs

Make sure these are enabled in your project:

* Vertex AI API
* Cloud Text-to-Speech API
