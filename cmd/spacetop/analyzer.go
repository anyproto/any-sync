package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/query"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
)

type TreeInfo struct {
	SpaceID        string
	TreeID         string
	ChangeCount    int
	RecentChanges  int // Changes within time filter (if --since provided)
	LastModified   time.Time
	HasTimeFilter  bool
}

func runAnalyzer() error {
	ctx := context.Background()

	// Parse time filter if provided
	var afterTime time.Time
	if since != "" {
		duration, err := time.ParseDuration(since)
		if err != nil {
			return fmt.Errorf("invalid duration format for --since: %w (use format like 10m, 1h, 24h)", err)
		}
		afterTime = time.Now().Add(-duration)
	}

	// Verify root path exists
	if _, err := os.Stat(rootPath); err != nil {
		return fmt.Errorf("path does not exist: %s", rootPath)
	}

	// Analyze all spaces
	trees, err := analyzeSpaces(ctx, rootPath, afterTime, spaceID)
	if err != nil {
		return err
	}

	// Sort by change count (descending)
	// If time filter is active, sort by recent changes; otherwise by total changes
	sort.Slice(trees, func(i, j int) bool {
		if len(trees) > 0 && trees[0].HasTimeFilter {
			return trees[i].RecentChanges > trees[j].RecentChanges
		}
		return trees[i].ChangeCount > trees[j].ChangeCount
	})

	// Limit to top N
	if topN > 0 && len(trees) > topN {
		trees = trees[:topN]
	}

	// Display results
	displayResults(trees)

	return nil
}

func analyzeSpaces(ctx context.Context, rootPath string, afterTime time.Time, filterSpaceID string) ([]TreeInfo, error) {
	var results []TreeInfo

	// Read all directories in the root path
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", rootPath, err)
	}

	fmt.Printf("Scanning databases in: %s\n", rootPath)
	fmt.Println(strings.Repeat("-", 80))

	spacesProcessed := 0
	spacesSkipped := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		spaceIDFromDir := entry.Name()

		// Apply space ID filter if specified
		if filterSpaceID != "" && spaceIDFromDir != filterSpaceID {
			continue
		}

		dbPath := filepath.Join(rootPath, spaceIDFromDir, "store.db")

		// Check if database file exists
		if _, err := os.Stat(dbPath); err != nil {
			spacesSkipped++
			fmt.Printf("Skipping %s: database file not found at %s\n", spaceIDFromDir, dbPath)
			continue
		}

		// Try to open the database
		db, err := anystore.Open(ctx, dbPath, &anystore.Config{
			SQLiteConnectionOptions: map[string]string{"synchronous": "off"},
		})
		if err != nil {
			spacesSkipped++
			fmt.Printf("Skipping %s: failed to open database: %v\n", spaceIDFromDir, err)
			continue
		}

		// Process the space
		spaceResults, err := processSpace(ctx, db, spaceIDFromDir, afterTime)
		db.Close()

		if err != nil {
			spacesSkipped++
			fmt.Printf("Error processing %s: %v\n", spaceIDFromDir, err)
			continue
		}

		results = append(results, spaceResults...)
		spacesProcessed++
		fmt.Printf("Processed space: %s (found %d trees)\n", spaceIDFromDir, len(spaceResults))
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total: %d spaces processed, %d skipped, %d trees found\n\n", spacesProcessed, spacesSkipped, len(results))

	return results, nil
}

func processSpace(ctx context.Context, db anystore.DB, spaceIDFromDir string, afterTime time.Time) ([]TreeInfo, error) {
	var results []TreeInfo

	// Create space storage
	spaceStorage, err := spacestorage.New(ctx, spaceIDFromDir, db)
	if err != nil {
		return nil, fmt.Errorf("failed to create space storage: %w", err)
	}

	// Get head storage
	headStorage := spaceStorage.HeadStorage()

	// Iterate through all trees
	err = headStorage.IterateEntries(ctx, headstorage.IterOpts{
		Deleted: false, // Skip deleted trees
	}, func(entry headstorage.HeadsEntry) (bool, error) {
		treeID := entry.Id

		// Open changes collection
		changesColl, err := db.OpenCollection(ctx, objecttree.CollName)
		if err != nil {
			return false, fmt.Errorf("failed to open changes collection: %w", err)
		}
		defer changesColl.Close()

		// Count total changes for this tree
		totalCount, err := changesColl.Find(query.Key{
			Path:   []string{objecttree.TreeKey},
			Filter: query.NewComp(query.CompOpEq, treeID),
		}).Count(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to count total changes: %w", err)
		}

		// Count recent changes if time filter is active
		var recentCount int
		hasTimeFilter := !afterTime.IsZero()
		if hasTimeFilter {
			recentCount, err = changesColl.Find(query.And{
				query.Key{Path: []string{objecttree.TreeKey}, Filter: query.NewComp(query.CompOpEq, treeID)},
				query.Key{Path: []string{"a"}, Filter: query.NewComp(query.CompOpGte, float64(afterTime.Unix()))},
			}).Count(ctx)
			if err != nil {
				return false, fmt.Errorf("failed to count recent changes: %w", err)
			}

			// Skip trees with no recent changes when time filter is active
			if recentCount == 0 {
				return true, nil
			}
		}

		// Get last modified time
		qry := changesColl.Find(query.Key{
			Path:   []string{objecttree.TreeKey},
			Filter: query.NewComp(query.CompOpEq, treeID),
		}).Sort("-a").Limit(1) // Sort by addedKey descending

		iter, err := qry.Iter(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to query last change: %w", err)
		}
		defer iter.Close()

		var lastModified time.Time
		if iter.Next() {
			doc, err := iter.Doc()
			if err == nil {
				timestamp := doc.Value().GetFloat64("a") // addedKey
				lastModified = time.Unix(int64(timestamp), 0)
			}
		}

		results = append(results, TreeInfo{
			SpaceID:       spaceIDFromDir,
			TreeID:        treeID,
			ChangeCount:   totalCount,
			RecentChanges: recentCount,
			LastModified:  lastModified,
			HasTimeFilter: hasTimeFilter,
		})

		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to iterate trees: %w", err)
	}

	return results, nil
}

func displayResults(trees []TreeInfo) {
	if len(trees) == 0 {
		fmt.Println("No trees found matching the criteria.")
		return
	}

	// Check if time filter was used
	hasTimeFilter := len(trees) > 0 && trees[0].HasTimeFilter

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	// Print header
	if hasTimeFilter {
		fmt.Fprintf(w, "RANK\tSPACE ID\tTREE ID\tTOTAL CHANGES\tRECENT CHANGES\tLAST MODIFIED\n")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			strings.Repeat("-", 4),
			strings.Repeat("-", 8),
			strings.Repeat("-", 7),
			strings.Repeat("-", 13),
			strings.Repeat("-", 14),
			strings.Repeat("-", 13))
	} else {
		fmt.Fprintf(w, "RANK\tSPACE ID\tTREE ID\tCHANGES\tLAST MODIFIED\n")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			strings.Repeat("-", 4),
			strings.Repeat("-", 8),
			strings.Repeat("-", 7),
			strings.Repeat("-", 7),
			strings.Repeat("-", 13))
	}

	// Print rows
	for i, tree := range trees {
		lastModifiedStr := formatTimestamp(tree.LastModified)

		if hasTimeFilter {
			fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%d\t%s\n",
				i+1,
				tree.SpaceID,
				tree.TreeID,
				tree.ChangeCount,
				tree.RecentChanges,
				lastModifiedStr)
		} else {
			fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\n",
				i+1,
				tree.SpaceID,
				tree.TreeID,
				tree.ChangeCount,
				lastModifiedStr)
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}

	// Show relative time for recent changes
	duration := time.Since(t)
	if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	} else if duration < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	}

	// For older changes, show the actual date
	return t.Format("2006-01-02 15:04")
}
