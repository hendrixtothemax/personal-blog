#!/bin/bash

# Exit immediately if a command fails
set -e

# Print each command before executing (optional for debugging)
#set -x

# Function to print an error and exit
fail() {
  echo "❌ $1" >&2
  exit 1
}

# Check if conda is installed
command -v conda >/dev/null 2>&1 || fail "Conda is not installed or not in PATH."

# Initialize conda for the current shell
eval "$(conda shell.bash hook)" || fail "Failed to initialize conda shell."

# Activate the environment
conda activate pb || fail "Failed to activate conda environment 'pb'."

# Export environment variables
export CC=/usr/bin/gcc
export CXX=/usr/bin/gcc
export CGO_ENABLED=1

# Run Go program
go run . || fail "Go program failed to run."
