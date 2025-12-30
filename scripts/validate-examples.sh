#!/bin/bash
# Validate example YAML files

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Validating example YAML files..."

EXAMPLES_DIR="$PROJECT_ROOT/examples"
ERRORS=0

for yaml_file in "$EXAMPLES_DIR"/*.yaml; do
    if [ -f "$yaml_file" ]; then
        echo "Validating: $(basename "$yaml_file")"
        if ! kubectl apply --dry-run=client -f "$yaml_file" > /dev/null 2>&1; then
            echo "  ❌ Validation failed"
            ERRORS=$((ERRORS + 1))
        else
            echo "  ✅ Valid"
        fi
    fi
done

if [ $ERRORS -eq 0 ]; then
    echo "✅ All examples are valid"
    exit 0
else
    echo "❌ $ERRORS example(s) failed validation"
    exit 1
fi

