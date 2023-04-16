package main

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/DanArmor/GoTorrent/pkg/p2p"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/knipferrc/teacup/filetree"
)

type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	ViewTorrent key.Binding
	Settings    key.Binding
	Quit        key.Binding
	StartStop   key.Binding
	Remove      key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.StartStop, k.Remove, k.ViewTorrent, k.Settings, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.ViewTorrent},
		{k.Settings, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	ViewTorrent: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("↵/enter", "view torrent"),
	),
	Settings: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "settings"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("esc/q", "quit"),
	),
	StartStop: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "continue/stop torrent"),
	),
	Remove: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "remove torrent"),
	),
}

const (
	mainScreen = iota
	filePickScreen
	torrentViewScreen
)

type model struct {
	Width        int
	Height       int
	keys         keyMap
	help         help.Model
	t            table.Model
	f            filetree.Bubble
	v            viewport.Model
	mv           viewport.Model
	Screens      []tea.Model
	activeScreen int
	fileNotInit  bool
}

func CreateTable() table.Model {
	columns := []table.Column{
		{Title: "№", Width: 4},
		{Title: "Name", Width: 32},
		{Title: "Size", Width: 16},
		{Title: "Status", Width: 16},
		{Title: "Progress", Width: 16},
	}

	var rows []table.Row
	for i := range GlobalSettings.Torrents {
		status := ""
		if GlobalSettings.Torrents[i].InProgress {
			status = "Downloading"
		} else {
			status = "Stopped"
		}
		rows = append(rows, table.Row{
			strconv.Itoa(i + 1), GlobalSettings.Torrents[i].Name, formatBytes(GlobalSettings.Torrents[i].TotalSize), status, strconv.Itoa(GlobalSettings.Torrents[i].Downloaded),
		})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)
	return t
}

func (m *model) RedrawRows() {
	var rows []table.Row
	for i := range GlobalSettings.Torrents {
		status := ""
		if GlobalSettings.Torrents[i].InProgress {
			status = "Downloading"
		} else {
			status = "Stopped"
		}
		rows = append(rows, table.Row{
			strconv.Itoa(i + 1), GlobalSettings.Torrents[i].Name, formatBytes(GlobalSettings.Torrents[i].TotalSize), status, strconv.Itoa(GlobalSettings.Torrents[i].Downloaded),
		})
	}
	m.t.SetRows(rows)
}

func NewModel() model {
	return model{
		keys: keys,
		help: help.New(),
		t:    CreateTable(),
		f: filetree.New(
			true,
			true,
			"",
			"",
			lipgloss.AdaptiveColor{Light: "#000000", Dark: "63"},
			lipgloss.AdaptiveColor{Light: "#000000", Dark: "63"},
			lipgloss.AdaptiveColor{Light: "63", Dark: "63"},
			lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#ffffff"},
		),
		activeScreen: mainScreen,
		v:            viewport.New(30, 5),
		mv:           viewport.New(30, 5),
	}
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)
	cmd = tickCmd()
	cmds = append(cmds, m.f.Init(), cmd)
	return tea.Batch(cmds...)
}

func (m model) UpdateTree(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg)
		m.Resize()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.activeScreen = mainScreen
		case "enter":
			file := m.f.GetSelectedItem()
			GlobalSettings.AddTorent(file.FileName())
		}
	}

	m.f, cmd = m.f.Update(msg)
	cmds = append(cmds, cmd)

	m.RedrawRows()
	return m, tea.Batch(cmds...)
}

func (m *model) SetSize(msg tea.WindowSizeMsg) {
	m.Width = msg.Width
	m.Height = msg.Height
}

func (m *model) Resize() {
	if m.Width != 0 && m.Height != 0 {
		m.t.SetWidth(m.Width)
		m.t.SetHeight(m.Height - 11)
		m.f.SetSize(m.Width, m.Height)
		m.help.Width = m.Width
		m.mv.Width = m.Width
		m.v.Width = m.Width
	}
}

func (m model) UpdateMainScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg)
		m.Resize()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "p":
			GlobalSettings.Torrents[m.t.Cursor()].InProgress = !GlobalSettings.Torrents[m.t.Cursor()].InProgress
			if GlobalSettings.Torrents[m.t.Cursor()].InProgress {
				GlobalSettings.startTorrent(m.t.Cursor())
			} else {
				GlobalSettings.stopTorrent(m.t.Cursor())
			}
			m.RedrawRows()
			return m, nil
		case "r":
			GlobalSettings.RemoveTorrent(m.t.Cursor())
			m.RedrawRows()
			return m, nil
		case "o":
			m.activeScreen = filePickScreen
			if m.fileNotInit {
				return m, nil
			} else {
				m.fileNotInit = true
				return m, m.f.Init()
			}
		case "enter":
			m.activeScreen = torrentViewScreen
			return m, nil
		}
	}

	var cmds []tea.Cmd

	newTable, cmd := m.t.Update(msg)
	nmv, mvcmd := m.mv.Update(msg)
	m.mv = nmv
	
	m.t = newTable
	cmds = append(cmds, cmd, mvcmd)
	return m, tea.Batch(cmds...)
}

func (m model) UpdateTorrentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	newv, vcmd := m.v.Update(msg)
	m.v = newv
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg)
		m.Resize()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.activeScreen = mainScreen
			return m, nil
		}
	}
	return m, vcmd
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		m.RedrawRows()
		m.mv.SetContent(p2p.GetLogString())
		m.mv.GotoBottom()
		return m, tickCmd()
	}

	switch m.activeScreen {
	case mainScreen:
		return m.UpdateMainScreen(msg)
	case filePickScreen:
		return m.UpdateTree(msg)
	case torrentViewScreen:
		return m.UpdateTorrentView(msg)
	default:
		panic("No such screen")
	}
}

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

var (
	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")
	docStyle          = lipgloss.NewStyle().Padding(1, 2, 1, 2)
	highlightColor    = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	inactiveTabStyle  = lipgloss.NewStyle().Border(inactiveTabBorder, true).BorderForeground(highlightColor).Padding(0, 1)
	activeTabStyle    = inactiveTabStyle.Copy().Border(activeTabBorder, true)
	windowsStyle      = lipgloss.NewStyle().BorderForeground(highlightColor).Padding(2, 0).Align(lipgloss.Center).Border(lipgloss.NormalBorder()).UnsetBorderTop()
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

func (m model) filePickScreenView() string {
	return m.f.View()
}

func (m model) mainScreenView() string {
	return baseStyle.Render(m.t.View()) + "\n" + m.mv.View()
}

func (m model) torrentViewScreenView() string {
	tf := GlobalSettings.Torrents[m.t.Cursor()]
	m.v.SetContent(strings.Join([]string{tf.Name, tf.Announce, hex.EncodeToString(tf.InfoHash[:])}, "\n"))
	m.v.GotoBottom()
	return m.v.View()
}

func (m model) View() string {
	toRender := ""
	switch m.activeScreen {
	case mainScreen:
		toRender = m.mainScreenView()
	case filePickScreen:
		toRender = m.filePickScreenView()
	case torrentViewScreen:
		toRender = m.torrentViewScreenView()
	}
	helpView := m.help.View(m.keys)
	return toRender + "\n\n" + helpView
}
