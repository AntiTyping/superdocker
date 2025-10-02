package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

var (
	baseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Padding(0, 1)
)

type model struct {
	containersTable table.Model
	imagesTable     table.Model
	volumesTable    table.Model
	networksTable   table.Model
	err             error
	loading         bool
	focusIndex      int // 0: containers, 1: images, 2: volumes, 3: networks
}

type dataLoadedMsg struct {
	containers []container.Summary
	images     []imagetypes.Summary
	volumes    []volumetypes.Volume
	networks   []networktypes.Summary
	err        error
}

func loadData() tea.Msg {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return dataLoadedMsg{err: err}
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return dataLoadedMsg{err: err}
	}

	images, err := cli.ImageList(ctx, imagetypes.ListOptions{})
	if err != nil {
		return dataLoadedMsg{err: err}
	}

	vresp, err := cli.VolumeList(ctx, volumetypes.ListOptions{})
	if err != nil {
		return dataLoadedMsg{err: err}
	}
	var volumes []volumetypes.Volume
	if vresp.Volumes != nil {
		volumes = make([]volumetypes.Volume, 0, len(vresp.Volumes))
		for _, v := range vresp.Volumes {
			if v != nil {
				volumes = append(volumes, *v)
			}
		}
	}

	networks, err := cli.NetworkList(ctx, networktypes.ListOptions{})
	if err != nil {
		return dataLoadedMsg{err: err}
	}

	return dataLoadedMsg{containers: containers, images: images, volumes: volumes, networks: networks}
}

func initialModel() model {
	// Containers table
	containerCols := []table.Column{
		{Title: "Container ID", Width: 12},
		{Title: "Image", Width: 25},
		{Title: "Command", Width: 20},
		{Title: "Status", Width: 20},
		{Title: "Name", Width: 20},
	}
	containersTable := table.New(
		table.WithColumns(containerCols),
		table.WithFocused(true),
		table.WithHeight(12),
	)

	// Images table
	imageCols := []table.Column{
		{Title: "Repository:Tag", Width: 30},
		{Title: "Image ID", Width: 12},
		{Title: "Size", Width: 10},
	}
	imagesTable := table.New(
		table.WithColumns(imageCols),
		table.WithFocused(false),
		table.WithHeight(8),
	)

	// Volumes table
	volumeCols := []table.Column{
		{Title: "Name", Width: 25},
		{Title: "Driver", Width: 12},
		{Title: "Mountpoint", Width: 40},
	}
	volumesTable := table.New(
		table.WithColumns(volumeCols),
		table.WithFocused(false),
		table.WithHeight(8),
	)

	// Networks table
	networkCols := []table.Column{
		{Title: "Name", Width: 22},
		{Title: "Network ID", Width: 12},
		{Title: "Driver", Width: 10},
		{Title: "Scope", Width: 10},
	}
	networksTable := table.New(
		table.WithColumns(networkCols),
		table.WithFocused(false),
		table.WithHeight(12),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("170"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	containersTable.SetStyles(s)
	imagesTable.SetStyles(s)
	volumesTable.SetStyles(s)
	networksTable.SetStyles(s)

	return model{
		containersTable: containersTable,
		imagesTable:     imagesTable,
		volumesTable:    volumesTable,
		networksTable:   networksTable,
		loading:         true,
	}
}

func (m model) Init() tea.Cmd {
	return loadData
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, loadData
		case "tab":
			m.focusIndex = (m.focusIndex + 1) % 4
			// Update focus states
			switch m.focusIndex {
			case 0: // containers
				m.containersTable.Focus()
				m.imagesTable.Blur()
				m.volumesTable.Blur()
				m.networksTable.Blur()
			case 1: // images
				m.containersTable.Blur()
				m.imagesTable.Focus()
				m.volumesTable.Blur()
				m.networksTable.Blur()
			case 2: // volumes
				m.containersTable.Blur()
				m.imagesTable.Blur()
				m.volumesTable.Focus()
				m.networksTable.Blur()
			case 3: // networks
				m.containersTable.Blur()
				m.imagesTable.Blur()
				m.volumesTable.Blur()
				m.networksTable.Focus()
			}
			return m, nil
		}

	case dataLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		// Containers rows
		cRows := []table.Row{}
		for _, c := range msg.containers {
			id := c.ID
			if len(id) > 12 {
				id = id[:12]
			}
			image := c.Image
			if len(image) > 25 {
				image = image[:22] + "..."
			}
			cmdStr := c.Command
			if len(cmdStr) > 20 {
				cmdStr = cmdStr[:17] + "..."
			}
			status := c.Status
			name := ""
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}

			cRows = append(cRows, table.Row{id, image, cmdStr, status, name})
		}
		m.containersTable.SetRows(cRows)

		// Images rows
		iRows := []table.Row{}
		for _, img := range msg.images {
			repoTag := "<none>:<none>"
			if len(img.RepoTags) > 0 {
				repoTag = img.RepoTags[0]
			}
			imgID := img.ID
			if strings.HasPrefix(imgID, "sha256:") {
				imgID = imgID[len("sha256:"):]
			}
			if len(imgID) > 12 {
				imgID = imgID[:12]
			}
			sizeMB := fmt.Sprintf("%.1fMB", float64(img.Size)/1024.0/1024.0)
			iRows = append(iRows, table.Row{repoTag, imgID, sizeMB})
		}
		m.imagesTable.SetRows(iRows)

		// Volumes rows
		vRows := []table.Row{}
		for _, v := range msg.volumes {
			name := v.Name
			driver := v.Driver
			mount := v.Mountpoint
			if len(mount) > 40 {
				mount = mount[:37] + "..."
			}
			vRows = append(vRows, table.Row{name, driver, mount})
		}
		m.volumesTable.SetRows(vRows)

		// Networks rows
		nRows := []table.Row{}
		for _, n := range msg.networks {
			name := n.Name
			id := n.ID
			if strings.HasPrefix(id, "sha256:") {
				id = id[len("sha256:"):]
			}
			if len(id) > 12 {
				id = id[:12]
			}
			driver := n.Driver
			scope := n.Scope
			nRows = append(nRows, table.Row{name, id, driver, scope})
		}
		m.networksTable.SetRows(nRows)
		return m, nil
	}

	// Route events to the focused table
	switch m.focusIndex {
	case 0:
		m.containersTable, cmd = m.containersTable.Update(msg)
	case 1:
		m.imagesTable, cmd = m.imagesTable.Update(msg)
	case 2:
		m.volumesTable, cmd = m.volumesTable.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.\n", m.err)
	}

	if m.loading {
		return "\n  Loading data...\n"
	}

	containersTitle := titleStyle.Render("Docker Containers")
	imagesTitle := titleStyle.Render("Docker Images")
	volumesTitle := titleStyle.Render("Docker Volumes")
	networksTitle := titleStyle.Render("Docker Networks")
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("\n  ↑/↓: navigate • Tab: switch list • r: refresh • q: quit\n")

	leftCol := fmt.Sprintf(
		"\n%s\n\n%s\n\n%s\n\n%s\n\n%s\n\n%s",
		containersTitle,
		baseStyle.Render(m.containersTable.View()),
		imagesTitle,
		baseStyle.Render(m.imagesTable.View()),
		volumesTitle,
		baseStyle.Render(m.volumesTable.View()),
	)
	rightCol := fmt.Sprintf(
		"\n%s\n\n%s",
		networksTitle,
		baseStyle.Render(m.networksTable.View()),
	)
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
	return fmt.Sprintf("%s\n%s", content, help)
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
