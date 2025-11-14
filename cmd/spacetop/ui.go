package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	tableView viewState = iota
	detailView
)

type analyzerModel struct {
	ctx              context.Context
	currentView      viewState
	table            table.Model
	viewport         viewport.Model
	trees            []TreeInfo
	rootPath         string
	detailContent    string
	objectName       string
	resolvedLayout   int
	err              error
	quitting         bool
	width            int
	height           int
	viewportReady    bool
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginBottom(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

func runInteractiveUI(ctx context.Context, trees []TreeInfo, rootPath string) error {
	if len(trees) == 0 {
		fmt.Println("No trees found matching the criteria.")
		return nil
	}

	// Build columns based on whether time filter is active
	columns := buildColumns(trees)
	rows := treesToRows(trees)

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	t.SetStyles(customTableStyles())

	m := analyzerModel{
		ctx:         ctx,
		currentView: tableView,
		table:       t,
		trees:       trees,
		rootPath:    rootPath,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()

	if err != nil {
		return err
	}

	// Check if there was an error during execution
	if m, ok := finalModel.(analyzerModel); ok && m.err != nil {
		return m.err
	}

	return nil
}

func (m analyzerModel) Init() tea.Cmd {
	return nil
}

func (m analyzerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update table height based on terminal size
		m.table.SetHeight(msg.Height - 8) // Leave room for header and footer

		// Initialize viewport if not ready
		if !m.viewportReady {
			m.viewport = viewport.New(msg.Width, msg.Height-10)
			m.viewportReady = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 10
		}

	case tea.KeyMsg:
		// Global quit with ctrl+c
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		// View-specific handling
		switch m.currentView {
		case tableView:
			switch msg.String() {
			case "q":
				m.quitting = true
				return m, tea.Quit

			case "enter":
				// Get selected tree
				idx := m.table.Cursor()
				if idx < len(m.trees) {
					selected := m.trees[idx]

					// Load object details
					details, name, layout, err := loadObjectDetails(m.ctx, selected, m.rootPath)
					if err != nil {
						m.detailContent = errorStyle.Render(fmt.Sprintf("Error loading object: %v", err))
						m.objectName = ""
						m.resolvedLayout = 0
					} else {
						m.detailContent = details
						m.objectName = name
						m.resolvedLayout = layout
						m.viewport.SetContent(details)
						m.viewport.GotoTop()
					}

					m.currentView = detailView
					return m, nil
				}
			}

			// Pass to table for navigation
			m.table, cmd = m.table.Update(msg)

		case detailView:
			switch msg.String() {
			case "esc", "q":
				m.currentView = tableView
				return m, nil
			default:
				// Pass other keys to viewport for scrolling
				m.viewport, cmd = m.viewport.Update(msg)
			}
		}
	}

	return m, cmd
}

func (m analyzerModel) View() string {
	if m.quitting {
		return "Exiting...\n"
	}

	switch m.currentView {
	case tableView:
		return m.renderTableView()
	case detailView:
		return m.renderDetailView()
	}
	return ""
}

func (m analyzerModel) renderTableView() string {
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("Spacetop"))
	s.WriteString("\n\n")

	// Table
	s.WriteString(m.table.View())
	s.WriteString("\n")

	// Help text
	help := "↑/↓: navigate • Enter: view object details • q: quit • Ctrl+C: force quit"
	s.WriteString(helpStyle.Render(help))

	return s.String()
}

func (m analyzerModel) renderDetailView() string {
	var s strings.Builder

	// Title with object metadata
	idx := m.table.Cursor()
	var title string
	if idx < len(m.trees) {
		selected := m.trees[idx]
		title = fmt.Sprintf("Object Details: %s", truncateID(selected.TreeID, 50))
	} else {
		title = "Object Details"
	}
	s.WriteString(titleStyle.Render(title))
	s.WriteString("\n")

	// Object metadata header
	if m.objectName != "" || m.resolvedLayout != 0 {
		metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		var meta strings.Builder
		if m.objectName != "" {
			meta.WriteString(fmt.Sprintf("Name: %s", m.objectName))
		}
		if m.resolvedLayout != 0 {
			if meta.Len() > 0 {
				meta.WriteString(" • ")
			}
			meta.WriteString(fmt.Sprintf("Layout: %d", m.resolvedLayout))
		}
		s.WriteString(metaStyle.Render(meta.String()))
		s.WriteString("\n")
	}

	// Divider
	divider := dividerStyle.Render(strings.Repeat("─", 80))
	s.WriteString(divider)
	s.WriteString("\n")

	// Scrollable content via viewport
	s.WriteString(m.viewport.View())
	s.WriteString("\n")

	// Divider
	s.WriteString(divider)
	s.WriteString("\n")

	// Help text with scroll indicators
	scrollPercent := int(m.viewport.ScrollPercent() * 100)
	help := fmt.Sprintf("↑/↓: scroll • Esc: back to table • q: quit • %d%%", scrollPercent)
	s.WriteString(helpStyle.Render(help))

	return s.String()
}

func buildColumns(trees []TreeInfo) []table.Column {
	hasTimeFilter := len(trees) > 0 && trees[0].HasTimeFilter

	if hasTimeFilter {
		return []table.Column{
			{Title: "Rank", Width: 6},
			{Title: "Space ID", Width: 74},
			{Title: "Tree ID", Width: 59},
			{Title: "Total", Width: 10},
			{Title: "Recent", Width: 10},
			{Title: "Last Modified", Width: 15},
		}
	}

	return []table.Column{
		{Title: "Rank", Width: 6},
		{Title: "Space ID", Width: 74},
		{Title: "Tree ID", Width: 59},
		{Title: "Changes", Width: 10},
		{Title: "Last Modified", Width: 15},
	}
}

func treesToRows(trees []TreeInfo) []table.Row {
	rows := make([]table.Row, len(trees))

	for i, tree := range trees {
		rank := fmt.Sprintf("%d", i+1)
		lastMod := formatTimestamp(tree.LastModified)

		if tree.HasTimeFilter {
			rows[i] = table.Row{
				rank,
				tree.SpaceID,
				tree.TreeID,
				fmt.Sprintf("%d", tree.ChangeCount),
				fmt.Sprintf("%d", tree.RecentChanges),
				lastMod,
			}
		} else {
			rows[i] = table.Row{
				rank,
				tree.SpaceID,
				tree.TreeID,
				fmt.Sprintf("%d", tree.ChangeCount),
				lastMod,
			}
		}
	}

	return rows
}

func customTableStyles() table.Styles {
	s := table.DefaultStyles()

	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("86"))

	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	return s
}

func loadObjectDetails(ctx context.Context, tree TreeInfo, rootPath string) (jsonStr string, name string, layout int, err error) {
	// Calculate objectstore path relative to spacestore path
	objectStorePath := filepath.Join(rootPath, "..", "objectstore", tree.SpaceID, "objects.db")

	// Check if objectstore exists
	if _, statErr := os.Stat(objectStorePath); statErr != nil {
		err = fmt.Errorf("objectstore not found at %s: %w", objectStorePath, statErr)
		return
	}

	// Open objectstore database
	db, openErr := anystore.Open(ctx, objectStorePath, &anystore.Config{
		SQLiteConnectionOptions: map[string]string{"synchronous": "off"},
	})
	if openErr != nil {
		err = fmt.Errorf("failed to open objectstore: %w", openErr)
		return
	}
	defer db.Close()

	// Open objects collection
	objectsColl, collErr := db.OpenCollection(ctx, "objects")
	if collErr != nil {
		err = fmt.Errorf("failed to open objects collection: %w", collErr)
		return
	}
	defer objectsColl.Close()

	// Find object by ID
	doc, docErr := objectsColl.FindId(ctx, tree.TreeID)
	if docErr != nil {
		err = fmt.Errorf("object not found with ID %s: %w", tree.TreeID, docErr)
		return
	}

	// Convert document to map for JSON display
	objectData := docToMap(doc)

	// Extract name and resolvedLayout fields
	if nameVal, ok := objectData["name"]; ok {
		if nameStr, ok := nameVal.(string); ok {
			name = nameStr
		}
	}
	if layoutVal, ok := objectData["resolvedLayout"]; ok {
		switch v := layoutVal.(type) {
		case int:
			layout = v
		case float64:
			layout = int(v)
		}
	}

	// Pretty print as JSON
	jsonBytes, marshalErr := json.MarshalIndent(objectData, "", "  ")
	if marshalErr != nil {
		err = fmt.Errorf("failed to marshal object to JSON: %w", marshalErr)
		return
	}

	jsonStr = string(jsonBytes)
	return
}

func docToMap(doc anystore.Doc) map[string]interface{} {
	result := make(map[string]interface{})

	// Use GoType() to convert the anyenc.Value to a Go type
	value := doc.Value()
	obj := value.GetObject()
	if obj == nil {
		return result
	}

	// Visit all keys in the object
	obj.Visit(func(key []byte, val *anyenc.Value) {
		keyStr := string(key)
		// Use GoType() to get the Go representation
		result[keyStr] = val.GoType()
	})

	return result
}
