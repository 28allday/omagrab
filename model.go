package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type mode int

const (
	audioMode mode = iota
	videoMode
)

type status int

const (
	stQueued status = iota
	stFetching
	stDownloading
	stDone
	stFailed
)

// item is one entry in the download queue.
type item struct {
	url     string
	title   string
	args    []string // yt-dlp args snapshotted at enqueue time
	status  status
	percent float64
	speed   string
	eta     string
	err     string
}

func (i *item) label() string {
	if i.title != "" {
		return i.title
	}
	return i.url
}

// --- messages from the worker ---

type titleMsg struct{ url, title string }
type statusMsg struct {
	url    string
	status status
}
type progressMsg struct {
	url     string
	percent float64
	speed   string
	eta     string
}
type doneMsg struct{ url string }
type failMsg struct{ url, err string }

type view int

const (
	queueView view = iota
	configView
)

type model struct {
	cfg     Config
	mode    mode
	view    view
	input   textinput.Model
	spinner spinner.Model
	items   []*item
	cursor  int
	cfgCur  int // selected row in the config view
	jobs    chan<- *item
	w, h    int
	flash   string
}

func newModel(cfg Config, jobs chan<- *item, initialURL string) model {
	ti := textinput.New()
	ti.Placeholder = "paste a URL and press enter…"
	ti.Prompt = "▸ "
	ti.Focus()
	ti.CharLimit = 2048
	if initialURL != "" {
		ti.SetValue(initialURL)
		ti.CursorEnd()
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return model{
		cfg:     cfg,
		mode:    videoMode, // start in Video mode
		input:   ti,
		spinner: sp,
		jobs:    jobs,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// find returns the item with the given URL, or nil.
func (m *model) find(url string) *item {
	for _, it := range m.items {
		if it.url == url {
			return it
		}
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.Width = m.cardWidth() - 4
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case titleMsg:
		if it := m.find(msg.url); it != nil {
			it.title = msg.title
		}
		return m, nil

	case statusMsg:
		if it := m.find(msg.url); it != nil {
			it.status = msg.status
		}
		return m, nil

	case progressMsg:
		if it := m.find(msg.url); it != nil {
			it.status = stDownloading
			it.percent, it.speed, it.eta = msg.percent, msg.speed, msg.eta
		}
		return m, nil

	case doneMsg:
		if it := m.find(msg.url); it != nil {
			it.status, it.percent = stDone, 1
		}
		return m, nil

	case failMsg:
		if it := m.find(msg.url); it != nil {
			it.status, it.err = stFailed, msg.err
		}
		return m, nil

	case tea.KeyMsg:
		if m.view == configView {
			return m.updateConfig(msg)
		}
		return m.updateQueue(msg)
	}
	return m, nil
}

func (m model) updateQueue(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.flash = ""
	key := msg.String()
	switch key {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "tab":
		if m.mode == audioMode {
			m.mode = videoMode
		} else {
			m.mode = audioMode
		}
		return m, nil
	case "enter":
		return m.enqueue()
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "ctrl+n":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, nil
	}

	// Single-letter commands act only when the URL box is empty (its resting
	// state), so they don't get swallowed while you're typing/pasting a URL.
	if strings.TrimSpace(m.input.Value()) == "" {
		switch key {
		case "c":
			m.view = configView
			return m, nil
		case "d", "x":
			return m.removeSelected()
		case "q":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) enqueue() (tea.Model, tea.Cmd) {
	url := strings.TrimSpace(m.input.Value())
	if url == "" {
		return m, nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		m.flash = "that doesn't look like a URL"
		return m, nil
	}
	it := &item{url: url, status: stQueued, args: buildArgs(m.cfg, m.mode)}
	m.items = append(m.items, it)
	m.input.SetValue("")
	m.jobs <- it
	return m, nil
}

func (m model) removeSelected() (tea.Model, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return m, nil
	}
	it := m.items[m.cursor]
	if it.status == stDownloading || it.status == stFetching {
		m.flash = "can't remove an active download"
		return m, nil
	}
	m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
	if m.cursor >= len(m.items) && m.cursor > 0 {
		m.cursor--
	}
	return m, nil
}

// config view rows
const (
	cfgAudioFmt = iota
	cfgVideoQual
	cfgSubs
	cfgRowCount
)

func (m model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "ctrl+e", "q":
		_ = saveConfig(m.cfg)
		m.view = queueView
		m.flash = "config saved"
		return m, nil
	case "up", "k":
		if m.cfgCur > 0 {
			m.cfgCur--
		}
	case "down", "j":
		if m.cfgCur < cfgRowCount-1 {
			m.cfgCur++
		}
	case "left", "h", "right", "l", "enter", " ":
		m.cycleConfig()
	}
	return m, nil
}

func (m *model) cycleConfig() {
	switch m.cfgCur {
	case cfgAudioFmt:
		m.cfg.AudioFormat = cycle(AudioFormats, m.cfg.AudioFormat)
	case cfgVideoQual:
		m.cfg.VideoQuality = cycle(VideoQualities, m.cfg.VideoQuality)
	case cfgSubs:
		m.cfg.Subtitles = !m.cfg.Subtitles
	}
}

func cycle(opts []string, cur string) string {
	for i, o := range opts {
		if o == cur {
			return opts[(i+1)%len(opts)]
		}
	}
	return opts[0]
}
