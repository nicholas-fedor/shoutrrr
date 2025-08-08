#!/usr/bin/env bash

# Enable strict error handling: exit on any error to prevent unexpected behavior.
set -e

# Function: generate_docs
# Purpose: Generates Markdown documentation for a given service and saves it to the docs/services/<service> directory.
# Arguments:
#   $1: The name of the service to generate documentation for.
function generate_docs() {
  # Store the service name from the first argument.
  SERVICE=$1
  # Define the output path for the service's documentation file (docs/services/<service>/config.md).
  DOCSPATH="$(dirname "$(dirname "$0")")/docs/services/$SERVICE"
  # Print a status message indicating which service is being processed, using ANSI color for visibility.
  echo -en "Creating docs for \e[96m$SERVICE\e[0m... "
  # Create the service's documentation directory if it doesn't exist, ensuring the output path is ready.
  mkdir -p "$DOCSPATH"
  # Run the shoutrrr CLI's 'docs' command to generate Markdown documentation for the service.
  # The command uses 'go run' to execute the main package in ./shoutrrr, passing the service name and Markdown format flag.
  # Output is redirected to config.md in the service's docs directory.
  go run "$(dirname "$(dirname "$0")")/shoutrrr" docs -f markdown "$SERVICE" > "$DOCSPATH"/config.md
  # Check the exit status of the previous command to confirm success.
  if [ $? -eq 0 ]; then
    # Print success message if the documentation was generated successfully.
    echo -e "Done!"
  fi
}

# Check if a specific service name was provided as a command-line argument.
if [[ -n "$1" ]]; then
  # If an argument is provided, generate documentation only for that service and exit.
  generate_docs "$1"
  exit 0
fi

# Define the path to the services directory, relative to the repository root.
# Use dirname to get the repository root from the script's location ($0 is the script path).
SERVICES_PATH="$(dirname "$(dirname "$0")")/pkg/services"

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

# Iterate over all entries in the pkg/services/ directory to generate documentation for each valid service.
for S in "$SERVICES_PATH"/*; do
  # Skip any entry that is not a directory (e.g., files like .gitkeep or .DS_Store).
  # This ensures only valid service directories are processed.
  if [[ ! -d "$S" ]]; then
    continue
  fi
  # Extract the service name from the directory path using basename.
  SERVICE=$(basename "$S")
  # Skip specific services ('standard' and 'xmpp') as they are not meant to have documentation generated.
  # This is likely due to their special status or incomplete implementation.
  if [[ "$SERVICE" == "standard" ]] || [[ "$SERVICE" == "xmpp" ]]; then
    continue
  fi
  # Debug: Print the service being processed to track progress.
  echo "Debug: Processing service: $SERVICE"
  # Call the generate_docs function to create documentation for the valid service.
  generate_docs "$SERVICE"
done
