# Spacetop

A CLI tool to analyze spacestore databases and identify the top trees by number of changes across all spaces.

## Overview

This tool iterates through directories containing spacestore databases, analyzes all trees within each space, and displays statistics about change counts and last modification times. It's useful for debugging, monitoring, and understanding space activity.

## Features

- Analyze multiple spacestore databases in a single run
- Show top N trees by change count across all spaces
- Filter trees by last modification time
- Filter by specific space ID
- Display results in a human-readable table format

## Installation

Build the tool from the repository root:

```bash
cd /Users/roman/anytype/any-sync
go build -o spacetree-analyzer ./cmd/spacetree-analyzer
```

Or run directly with `go run`:

```bash
go run ./cmd/spacetree-analyzer --path /path/to/spaces
```

## Usage

### Basic Usage

Analyze all spaces and show top 20 trees:

```bash
./spacetree-analyzer --path /Users/roman/Library/Application\ Support/anytype/data/A9Jq6EX56tR2LorgkKwkNUq24G1Zk2GLQcRohvz6CBdhjjVY/spaceStoreNew
```

### Options

- `--path, -p` (required): Root path containing space database directories
- `--top, -n`: Number of top trees to show (default: 20)
- `--since, -s`: Filter by last change time (e.g., "10m", "1h", "24h")
- `--space-id`: Filter by specific space ID

### Examples

Show top 10 trees with changes in the last 10 minutes:

```bash
./spacetree-analyzer --path /path/to/spaces --top 10 --since 10m
```

Show top 50 trees with changes in the last hour:

```bash
./spacetree-analyzer --path /path/to/spaces --top 50 --since 1h
```

Analyze only a specific space:

```bash
./spacetree-analyzer --path /path/to/spaces --space-id bafyreib...
```

Show all trees (no limit) with changes in the last 24 hours:

```bash
./spacetree-analyzer --path /path/to/spaces --top 0 --since 24h
```

## Output Format

The tool displays results in a table with the following columns:

### Without --since filter:
- **RANK**: Position in the sorted list (by total change count)
- **SPACE ID**: Space identifier (full ID shown)
- **TREE ID**: Tree identifier (full ID shown)
- **CHANGES**: Total number of changes in the tree
- **LAST MODIFIED**: Relative time or absolute timestamp of last change

### With --since filter:
- **RANK**: Position in the sorted list (by recent change count)
- **SPACE ID**: Space identifier (full ID shown)
- **TREE ID**: Tree identifier (full ID shown)
- **TOTAL CHANGES**: Total number of changes in the tree (all time)
- **RECENT CHANGES**: Number of changes within the time window
- **LAST MODIFIED**: Relative time or absolute timestamp of last change

When using `--since`, only trees with at least one change in the time window are shown, and they are sorted by the number of recent changes.

## Error Handling

- Databases that cannot be opened are skipped with a warning message
- The tool continues processing even if individual spaces fail
- A summary shows how many spaces were processed vs. skipped

## Performance Considerations

- The tool opens databases with `synchronous: off` for better read performance
- Resources (databases, collections, iterators) are properly closed after use
- For large datasets, consider using filters to reduce the result set

## Troubleshooting

**No trees found:**
- Check that the path is correct and contains space databases
- Verify that the time filter (--since) is not too restrictive
- Try running without filters first to see if any data exists