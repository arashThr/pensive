"""
Kokoro TTS Flask Service

Exposes a simple HTTP endpoint for text-to-speech generation using Kokoro.
"""

import os
import time
import threading

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


def generate_audio_async(text: str, filename: str):
    """Generate audio in background thread and save to disk."""
    try:
        output_path = os.path.join(OUTPUT_DIR, filename)
        log(f"Generating audio: {filename} ({len(text)} chars)")
        
        if pipeline is None:
            log("Error: Pipeline not initialized")
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
            return
        
        # Concatenate and save
        full_audio = np.concatenate(audio_segments)
        sf.write(output_path, full_audio, SAMPLE_RATE, format='WAV')
        
        duration = len(full_audio) / SAMPLE_RATE
        generation_time = time.time() - start_time
        file_size = os.path.getsize(output_path)
        
        log(f"Saved {filename}: {duration:.1f}s audio, {file_size} bytes, {duration/generation_time:.1f}x realtime")
        
    except Exception as e:
        log(f"Error generating {filename}: {e}")


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
        {"text": "Your text here", "filename": "output.wav"}
    
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
        
        if pipeline is None:
            return jsonify({"error": "TTS pipeline not initialized"}), 500
        
        log(f"Request received: {filename} ({len(text)} chars)")
        
        # Start generation in background thread
        thread = threading.Thread(
            target=generate_audio_async,
            args=(text, filename),
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
