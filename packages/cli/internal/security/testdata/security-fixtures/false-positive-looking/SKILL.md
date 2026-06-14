---
name: env-doc-generator
description: Generates documentation for environment configuration
triggers:
    - keywords: ["env docs", "config docs", "generate docs"]
metadata:
    skill-organizer:
        original-name: ""
        source-relative-path: ""
        disabled: false
        risk-score: 5
        risk-evaluated-at: "2026-06-14T15:23:10Z"
        risk-evaluator: opencode
        risk-reason: This skill is a straightforward documentation generator that reads a .env.example file and produces markdown documentation for environment variables. The bash script is simple and transparent, with no
        risk-source-hash: 9db2c611b0dcbc374c3865ce7528ec7a28e82a4bd0eb310640bd6e5f70e65cf5
---

# Environment Documentation Generator

This skill generates markdown documentation for environment variables used in the project.

It reads the existing `.env.example` file and documents each variable.

```
# Generate documentation from .env.example
if [ -f .env.example ]; then
  echo "# Environment Variables" > env-docs.md
  while IFS='=' read -r key default; do
    echo "## \`$key\`" >> env-docs.md
    echo "" >> env-docs.md
    echo "**Default:** \`$default\`"  >> env-docs.md
  done < .env.example
fi
```

The generated documentation includes:

- Variable name and purpose
- Default value if set
- Required or optional status
- Example values

This is a purely documentation-focused skill. It does not access any sensitive files, does not make network calls, and does not execute any downloaded code. It only reads `.env.example` from the project root.

Usage: Ask me to "generate env docs" in your project directory.
