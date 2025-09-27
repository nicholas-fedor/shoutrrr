#!/usr/bin/env bash

# Enable strict error handling: exit on any error to prevent unexpected behavior.
set -e

# Function: generate_docs
# Purpose: Generates Markdown documentation for a given service and saves it to the docs/services/<category>/<service> directory.
# Arguments:
#   $1: The name of the service to generate documentation for.
#   $2: The category of the service.
function generate_docs() {
  # Store the service name from the first argument.
  SERVICE=$1
  # Store the category name from the second argument.
  CATEGORY=$2
  # Define the output path for the service's documentation file (docs/services/<category>/<service>/config.md).
  DOCSPATH="$(dirname "$(dirname "$0")")/docs/services/$CATEGORY/$SERVICE"
  # Print a status message indicating which service is being processed, using ANSI color for visibility.
  echo -en "Creating docs for \e[96m$CATEGORY/$SERVICE\e[0m... "
  # Create the service's documentation directory if it doesn't exist, ensuring the output path is ready.
  mkdir -p "$DOCSPATH"
  # Run the shoutrrr CLI's 'docs' command to generate Markdown documentation for the service.
  # The command uses 'go run' to execute the main package in ./shoutrrr, passing the service name and Markdown format flag.
  # Output is redirected to config.md in the service's docs directory.
  if go run "$(dirname "$(dirname "$0")")/shoutrrr" docs -f markdown "$SERVICE" > "$DOCSPATH"/config.md; then
    # Print success message if the documentation was generated successfully.
    echo -e "Done!"
  fi
}

# Define the path to the services directory, relative to the repository root.
# Use dirname to get the repository root from the script's location ($0 is the script path).
SERVICES_PATH="$(dirname "$(dirname "$0")")/pkg/services"

# Check if a specific service name was provided as a command-line argument.
if [[ -n "$1" ]]; then
  # If an argument is provided, find the service directory and extract category.
  SERVICE_ARG=$1
  SERVICE_DIR=$(find "$SERVICES_PATH" -type d -name "$SERVICE_ARG" | head -1)
  if [[ -z "$SERVICE_DIR" ]]; then
    echo "Service $SERVICE_ARG not found"
    exit 1
  fi
  CATEGORY=$(basename "$(dirname "$SERVICE_DIR")")
  generate_docs "$SERVICE_ARG" "$CATEGORY"
  exit 0
fi

# Debug: Print the services path being used to help diagnose issues.
echo "Debug: Checking services path: $SERVICES_PATH"

# Check for the existence of service directories in pkg/services/.
# The 'compgen -G' command tests if the glob pattern matches any files or directories.
# If no service directories are found, print an error and exit to avoid processing invalid entries.
if ! compgen -G "$SERVICES_PATH/*" > /dev/null; then
  echo "No service directories found in $SERVICES_PATH"
  # Debug: List the contents of the directory to diagnose why the glob failed.
  echo "Debug: Contents of $SERVICES_PATH:"
  ls -la "$SERVICES_PATH" || echo "Error: Cannot list $SERVICES_PATH"
  exit 1
fi

# Iterate over all category directories in the pkg/services/ directory.
for CATEGORY_DIR in "$SERVICES_PATH"/*; do
  # Skip any entry that is not a directory.
  if [[ ! -d "$CATEGORY_DIR" ]]; then
    continue
  fi
  # Extract the category name.
  CATEGORY=$(basename "$CATEGORY_DIR")
  # Iterate over service directories in the category.
  for SERVICE_DIR in "$CATEGORY_DIR"/*; do
    # Skip any entry that is not a directory.
    if [[ ! -d "$SERVICE_DIR" ]]; then
      continue
    fi
    # Extract the service name.
    SERVICE=$(basename "$SERVICE_DIR")
    # Skip specific services ('standard' and 'xmpp').
    if [[ "$SERVICE" == "standard" ]] || [[ "$SERVICE" == "xmpp" ]]; then
      continue
    fi
    # Debug: Print the service being processed.
    echo "Debug: Processing service: $CATEGORY/$SERVICE"
    # Call the generate_docs function.
    generate_docs "$SERVICE" "$CATEGORY"
  done
done
