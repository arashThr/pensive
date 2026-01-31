#!/usr/bin/env bash

set -euo pipefail

# Conda isn't automatically initialized in non-interactive shells.
# Source conda's shell helpers so `conda activate` works without `conda init`.
source /opt/conda/etc/profile.d/conda.sh

conda activate kokoro

exec python app.py