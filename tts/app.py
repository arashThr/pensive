"""
Kokoro TTS Flask Service

Exposes a simple HTTP endpoint for text-to-speech generation using Kokoro.
"""

import os
import time
import threading
import requests

import numpy as np
from flask import Flask, request, jsonify
from kokoro import KPipeline
import soundfile as sf

def log(msg):
    """Print to stderr with flush for visibility in Docker logs"""
    print(f"[TTS] {msg}", flush=True)


app = Flask(__name__)

# Configuration
OUTPUT_DIR = '/app/uploads/podcasts'
VOICE = 'af_heart'
SAMPLE_RATE = 24000

# Ensure output directory exists
os.makedirs(OUTPUT_DIR, exist_ok=True)

# Initialize the Kokoro pipeline at startup
log("Initializing Kokoro TTS pipeline...")
start_time = time.time()
try:
    pipeline = KPipeline(lang_code='a')  # American English
    log(f"Pipeline ready in {time.time() - start_time:.2f}s")
except Exception as e:
    log(f"Failed to initialize pipeline: {e}")
    pipeline = None


def send_callback(callback_url: str, filename: str, user_email: str, user_id: int, success: bool, error: str = None):
    """Send completion callback to the server."""
    if not callback_url:
        log("No callback URL provided, skipping callback")
        return
    
    try:
        payload = {
            "filename": filename,
            "user_email": user_email,
            "user_id": user_id,
            "success": success,
        }
        if error:
            payload["error"] = error
        
        log(f"Sending callback to {callback_url}")
        resp = requests.post(callback_url, json=payload, timeout=30)
        log(f"Callback response: {resp.status_code}")
    except Exception as e:
        log(f"Failed to send callback: {e}")


def generate_audio_async(text: str, filename: str, user_email: str, user_id: int, callback_url: str):
    """Generate audio in background thread and save to disk."""
    try:
        output_path = os.path.join(OUTPUT_DIR, filename)
        log(f"Generating audio: {filename} ({len(text)} chars)")
        
        if pipeline is None:
            log("Error: Pipeline not initialized")
            send_callback(callback_url, filename, user_email, user_id, False, "Pipeline not initialized")
            return
        
        start_time = time.time()
        
        # Generate audio using Kokoro
        generator = pipeline(text, voice=VOICE, speed=1, split_pattern=r'\n+')
        
        # Collect all audio segments
        audio_segments = []
        for i, (gs, ps, audio) in enumerate(generator):
            log(f"  Segment {i}: {len(audio)} samples")
            audio_segments.append(audio)
        
        if not audio_segments:
            log(f"Error: No audio generated for {filename}")
            send_callback(callback_url, filename, user_email, user_id, False, "No audio segments generated")
            return
        
        # Concatenate and save
        full_audio = np.concatenate(audio_segments)
        sf.write(output_path, full_audio, SAMPLE_RATE, format='WAV')
        
        duration = len(full_audio) / SAMPLE_RATE
        generation_time = time.time() - start_time
        file_size = os.path.getsize(output_path)
        
        log(f"Saved {filename}: {duration:.1f}s audio, {file_size} bytes, {duration/generation_time:.1f}x realtime")
        
        # Send success callback
        send_callback(callback_url, filename, user_email, user_id, True)
        
    except Exception as e:
        log(f"Error generating {filename}: {e}")
        send_callback(callback_url, filename, user_email, user_id, False, str(e))


@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint"""
    return jsonify({
        "status": "healthy" if pipeline else "unhealthy",
        "pipeline_ready": pipeline is not None
    })


@app.route('/generate', methods=['POST'])
def generate():
    """
    Accept text and start audio generation in background.
    
    Request body:
        {
            "text": "Your text here",
            "filename": "output.wav",
            "user_email": "user@example.com",
            "callback_url": "http://server:8000/internal/podcast/complete"
        }
    
    Returns immediately with acknowledgment.
    """
    try:
        data = request.get_json()
        
        if not data or 'text' not in data:
            return jsonify({"error": "Missing 'text' field"}), 400
        
        text = data['text'].strip()
        if not text:
            return jsonify({"error": "Text cannot be empty"}), 400
        
        filename = data.get('filename', f"{int(time.time())}.wav")
        if not filename.endswith('.wav'):
            filename += '.wav'
        
        user_email = data.get('user_email', '')
        user_id = data.get('user_id', 0)
        callback_url = data.get('callback_url', '')
        
        if pipeline is None:
            return jsonify({"error": "TTS pipeline not initialized"}), 500
        
        log(f"Request received: {filename} ({len(text)} chars, email: {user_email}, user_id: {user_id})")
        
        # Start generation in background thread
        thread = threading.Thread(
            target=generate_audio_async,
            args=(text, filename, user_email, user_id, callback_url),
            daemon=True
        )
        thread.start()
        
        return jsonify({
            "success": True,
            "message": "Podcast generation started",
            "filename": filename
        })
        
    except Exception as e:
        log(f"Error: {e}")
        return jsonify({"error": str(e)}), 500


if __name__ == '__main__':
    log("Starting TTS service on port 5000")
    app.run(host='0.0.0.0', port=5000)
