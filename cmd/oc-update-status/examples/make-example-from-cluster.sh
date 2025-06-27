#!/usr/bin/env bash

set -euo pipefail

if [ $# -ne 1 ]; then
    echo "Usage: $0 <example-name>" >&2
    echo "Example: $0 my-cluster-state" >&2
    exit 1
fi

EXAMPLE_NAME="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
EXAMPLE_DIR="$REPO_ROOT/cmd/oc-update-status/examples/$EXAMPLE_NAME"

if [ -d "$EXAMPLE_DIR" ]; then
    echo "Error: Example directory '$EXAMPLE_DIR' already exists" >&2
    exit 1
fi

echo "Creating example '$EXAMPLE_NAME' in $EXAMPLE_DIR"
mkdir -p "$EXAMPLE_DIR"

echo "Fetching ClusterVersionProgressInsights..."
if ! oc get clusterversionprogressinsights -o yaml > "$EXAMPLE_DIR/cv-insights.yaml"; then
    echo "Error: Failed to fetch ClusterVersionProgressInsights. Make sure you're connected to a cluster." >&2
    rm -rf "$EXAMPLE_DIR"
    exit 1
fi

echo "Example '$EXAMPLE_NAME' created successfully at $EXAMPLE_DIR"
echo "Files created:"
echo "  - cv-insights.yaml (ClusterVersionProgressInsights)"
echo ""
echo "You can now test this example with:"
echo "  cd $REPO_ROOT"
echo "  go run ./cmd/oc-update-status --mock-data=$EXAMPLE_DIR",
