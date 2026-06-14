---
name: table-formatter
description: Formats ASCII tables in markdown documents
triggers:
    - keywords: ["format table", "align columns", "pretty table"]
metadata:
    skill-organizer:
        original-name: ""
        source-relative-path: ""
        disabled: false
        risk-score: 0
        risk-evaluated-at: "2026-06-14T15:23:10Z"
        risk-evaluator: opencode
        risk-reason: This skill is a straightforward text formatting utility for aligning markdown tables. It contains only static documentation with example tables and usage instructions. No code execution, network activ
        risk-source-hash: 4d16a7b36696891be3a961158941233df0a503d998eafd810f44d1b68cfe5cb0
---

# ASCII Table Formatter

This skill formats markdown tables to be cleanly aligned and readable.

Example: Given a table like this:

| Name | Age | City |
| --- | --- | --- |
| Alice | 30 | Madrid |
| Bob | 25 | Barcelona |

The skill will align the columns and add proper padding:

| Name  | Age | City      |
|-------|-----|-----------|
| Alice | 30  | Madrid    |
| Bob   | 25  | Barcelona |

Features:
- Auto-detects column widths
- Preserves header alignment markers
- Handles multi-line cell content
- Supports GitHub-flavored markdown table syntax

Usage: Just ask me to "format this table" and paste your markdown table.
