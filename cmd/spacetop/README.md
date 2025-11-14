# Spacetop

An interactive TUI tool to analyze spacestore databases and identify the top trees by number of changes across all spaces. Features table navigation and scrollable object detail views.

## Overview

Spacetop iterates through directories containing spacestore databases, analyzes all trees within each space, and displays statistics about change counts and last modification times. It provides an interactive Bubble Tea interface to navigate results and view detailed object information from the objectstore.

## Features

- Analyze multiple spacestore databases in a single run
- Show top N trees by change count across all spaces
- Filter trees by last modification time
- Filter by specific space ID
- **Interactive selection** with arrow key navigation
- **View object details** from objectstore in pretty JSON format
- Display results in a human-readable table format

## Installation

Build the tool from the repository root:

```bash
cd /Users/roman/anytype/any-sync
go build -o spacetop ./cmd/spacetop
```

Or run directly with `go run`:

```bash
go run ./cmd/spacetop --path /path/to/spaces
```

## Usage

### Basic Usage

Analyze all spaces and show top 20 trees with interactive selection:

```bash
./spacetop --path /Users/roman/Library/Application\ Support/anytype/data/A9Jq6EX56tR2LorgkKwkNUq24G1Zk2GLQcRohvz6CBdhjjVY/spaceStoreNew
```

### Options

- `--path, -p` (required): Root path containing space database directories
- `--top, -n`: Number of top trees to show (default: 20)
- `--since, -s`: Filter by last change time (e.g., "10m", "1h", "24h")
- `--space-id`: Filter by specific space ID

### Examples

Show top 10 trees with changes in the last 10 minutes:

```bash
./spacetop --path /path/to/spaces --top 10 --since 10m
```

Show top 50 trees with changes in the last hour:

```bash
./spacetop --path /path/to/spaces --top 50 --since 1h
```

Analyze only a specific space:

```bash
./spacetop --path /path/to/spaces --space-id bafyreib...
```

Show all trees (no limit) with changes in the last 24 hours:

```bash
./spacetop --path /path/to/spaces --top 0 --since 24h
```

## Interactive Mode

The tool uses Bubble Tea to provide a rich interactive terminal UI:

1. **Interactive Table**: The results are displayed as an interactive table where you can navigate rows with arrow keys
2. **Row Selection**: The currently selected row is highlighted
3. **Navigation**: Use arrow keys (↑/↓) to navigate through trees, Page Up/Down for fast scrolling
4. **View Object**: Press `Enter` to view the selected tree's object details
5. **Object Display**: The corresponding object from `../objectstore/<spaceid>/objects.db` is displayed as pretty JSON
6. **Return to Table**: Press `Esc` or `q` to return to the table view (state is preserved)
7. **Exit**: Press `q` in table view or `Ctrl+C` at any time to exit

### Interactive Table Features

- **Highlighted Selection**: The currently selected row is highlighted with a colored background
- **Styled Headers**: Column headers are bold and separated from data with a border
- **Preserved State**: When you return from detail view, the table cursor stays at the same position
- **Smooth Navigation**: Natural keyboard navigation with arrow keys

## Output Format

### Table Display

The tool displays results in a table with the following columns:

#### Without --since filter:
- **RANK**: Position in the sorted list (by total change count)
- **SPACE ID**: Space identifier (full ID shown)
- **TREE ID**: Tree identifier (full ID shown)
- **CHANGES**: Total number of changes in the tree
- **LAST MODIFIED**: Relative time or absolute timestamp of last change

#### With --since filter:
- **RANK**: Position in the sorted list (by recent change count)
- **SPACE ID**: Space identifier (full ID shown)
- **TREE ID**: Tree identifier (full ID shown)
- **TOTAL CHANGES**: Total number of changes in the tree (all time)
- **RECENT CHANGES**: Number of changes within the time window
- **LAST MODIFIED**: Relative time or absolute timestamp of last change

When using `--since`, only trees with at least one change in the time window are shown, and they are sorted by the number of recent changes.

### Example Output

During analysis:
```
Scanning databases in: /path/to/spaces
--------------------------------------------------------------------------------
Processed space: space1 (found 15 trees)
Processed space: space2 (found 8 trees)
--------------------------------------------------------------------------------
Total: 2 spaces processed, 0 skipped, 23 trees found
```

Interactive table view:
```
Spacetop

┌──────┬─────────────────────────────────────────────┬─────────────────────────────────────────────┬──────────┬───────────────┐
│ Rank │ Space ID                                    │ Tree ID                                     │ Changes  │ Last Modified │
├──────┼─────────────────────────────────────────────┼─────────────────────────────────────────────┼──────────┼───────────────┤
│ 1    │ bafyreib5nxm3q2k7z...                      │ bafyreiabc123xyz...                         │ 1247     │ 15m ago       │
│ 2    │ bafyreib5nxm3q2k7z...                      │ bafyreixyz789abc...                         │ 856      │ 2h ago        │  ← Selected
│ 3    │ bafyreic7pqr8s3m9y...                      │ bafyreidef456ghi...                         │ 523      │ 1d ago        │
└──────┴─────────────────────────────────────────────┴─────────────────────────────────────────────┴──────────┴───────────────┘

↑/↓: navigate • Enter: view object details • q: quit • Ctrl+C: force quit
```

### Object Details Display

When you select a tree, the screen switches to show the object details with scrolling support:

```
Object Details: bafyreiabc123xyz...
Name: My Document • Layout: 1
────────────────────────────────────────────────────────────────────────────────

{
  "id": "bafyreiabc123xyz...",
  "name": "My Document",
  "resolvedLayout": 1,
  "data": "<bytes: 1024 bytes>",
  "type": "object",
  "created": 1699876543,
  ...
  (scrollable content)
}

────────────────────────────────────────────────────────────────────────────────
↑/↓: scroll • Esc: back to table • q: quit • 25%
```

**Features:**
- **Header Information**: Shows object name and layout (if available)
- **Scrollable Content**: For large JSON objects, use arrow keys to scroll
- **Scroll Indicator**: Shows current scroll position as percentage

## How It Works

1. **Directory Scanning**: Iterates through all subdirectories in the provided path
2. **Database Opening**: Opens each `store.db` file within space directories
3. **Tree Iteration**: Uses headstorage to iterate through all non-deleted trees
4. **Change Counting**:
   - Without `--since`: Counts total changes per tree
   - With `--since`: Counts both total changes AND recent changes within the time window
5. **Timestamp Extraction**: Retrieves the last change timestamp from the "addedKey" field
6. **Filtering**:
   - Applies space ID filter if `--space-id` is specified
   - When `--since` is used, only includes trees with at least one change in the time window
7. **Sorting**:
   - Without `--since`: Sorts by total change count (descending)
   - With `--since`: Sorts by recent change count (descending)
8. **Interactive Table Display**: Shows the top N trees in a Bubble Tea interactive table
9. **Navigation**: Arrow keys to navigate, Enter to select
10. **Object Retrieval**: When a tree is selected:
    - Opens the objectstore database at `../objectstore/<spaceid>/objects.db`
    - Queries the "objects" collection for the document with id matching the tree ID
    - Converts the document to a map and displays as pretty JSON
11. **View Switching**: Seamlessly switch between table view and detail view with state preservation

## Technical Details

The tool uses:

- `github.com/anyproto/any-store` for database operations
- `github.com/anyproto/any-sync/commonspace` for space storage and head storage
- `github.com/spf13/cobra` for CLI framework
- `github.com/charmbracelet/bubbletea` for the interactive TUI framework
- `github.com/charmbracelet/bubbles/table` for the interactive table component
- `github.com/charmbracelet/lipgloss` for terminal styling

It queries the following anystore collections:
- `changes` collection (in spacestore) for tree changes and metadata
- `objects` collection (in objectstore) for object data

The UI follows the Elm Architecture (Model-Update-View pattern):
- **Model**: Application state (table data, current view, selected tree, etc.)
- **Update**: Event handler for keyboard input and state transitions
- **View**: Renders the current state as terminal output

## Directory Structure

The tool expects the following directory structure:

```
<root-path>/
├── <space-id-1>/
│   └── store.db          # Spacestore database
├── <space-id-2>/
│   └── store.db
└── ...

../objectstore/            # Relative to <root-path>
├── <space-id-1>/
│   └── objects.db         # Objectstore database
├── <space-id-2>/
│   └── objects.db
└── ...
```

## Error Handling

- Databases that cannot be opened are skipped with a warning message
- If an objectstore doesn't exist, a clear error message is shown
- If an object ID is not found in the objectstore, an error is displayed
- The tool continues processing even if individual spaces fail
- A summary shows how many spaces were processed vs. skipped

## Performance Considerations

- The tool opens databases with `synchronous: off` for better read performance
- Resources (databases, collections, iterators) are properly closed after use
- For large datasets, consider using filters to reduce the result set
- Interactive mode loads results into memory for quick navigation

## Troubleshooting

**No trees found:**
- Check that the path is correct and contains space databases
- Verify that the time filter (--since) is not too restrictive
- Try running without filters first to see if any data exists

**Database errors:**
- Ensure databases are not corrupted
- Check file permissions on the database directories
- Verify the database format is compatible with this tool

**Object not found:**
- Verify that the objectstore exists at `../objectstore/<spaceid>/objects.db`
- Check that the object ID actually exists in the objectstore
- Ensure the objectstore database is not corrupted

**Permission denied:**
- Ensure you have read access to both spacestore and objectstore directories
- On macOS, you may need to grant terminal access to the directory

## Keyboard Shortcuts

### In Table View:
- `↑` / `↓` or `j` / `k`: Navigate up/down through rows
- `Page Up` / `Page Down`: Scroll by page
- `Home` / `End` or `g` / `G`: Jump to first/last row
- `Enter`: View object details for selected tree
- `q`: Quit the program
- `Ctrl+C`: Force quit at any time

### In Detail View:
- `↑` / `↓` or `j` / `k`: Scroll up/down through JSON content
- `Page Up` / `Page Down`: Scroll by page
- `Home` / `End` or `g` / `G`: Jump to top/bottom of content
- `Esc`: Return to table view (state preserved)
- `q`: Return to table view
- `Ctrl+C`: Force quit at any time
