# Creating a Service Account JSON Key File for Vertex AI

How to get [Service account page](https://console.cloud.google.com/iam-admin/serviceaccounts) for TTS.

## Set Up Your Google Cloud Project:

Sign in to your Google Account and go to the Google Cloud console.
Select or create a Google Cloud project.
Ensure that billing is enabled for your project and enable the Vertex AI API.

## Create a Service Account:

Navigate to the "Create service account" page in the Google Cloud console.
Provide a name for your service account and an optional description, then click "Create".

## Grant Necessary IAM Roles:

On the "Service account details" page, go to the "Permissions" tab and click "Grant access."
Add the service account's email address in the "New principals" box.
Select the "Vertex AI User" (roles/aiplatform.user) IAM role. This role allows your service account to access Vertex AI resources.
I have also enabled "Cloud speech to text" and "Vertex AI service agent" roles.
(Optional) If you plan to use service account impersonation for creating JSON Web Tokens (JWTs), you might also need the "Service Account Token Creator" (roles/iam.serviceAccountTokenCreator) role.
Click "Save".

## Download the JSON Key File:

In the Google Cloud console, click on the email address of the service account you just created.
Go to the "Keys" tab.
Click "Add key", then "Create new key".
Select "JSON" as the key type and click "Create". Your JSON key file will be downloaded automatically to your computer.

## Configure for Local Development (Optional but Recommended):

To use the key file in your local development environment, set the GOOGLE_APPLICATION_CREDENTIALS environment variable to the path of the downloaded JSON key file.
For Linux/macOS: export GOOGLE_APPLICATION_CREDENTIALS="/path/to/your/key-file.json"
For Windows PowerShell: $env:GOOGLE_APPLICATION_CREDENTIALS="C:\path\to\your\key-file.json"

Your application can now use this JSON key file for authentication with Vertex AI services.
