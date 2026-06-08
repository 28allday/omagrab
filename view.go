package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Nerd Font glyphs (Omarchy terminals ship a Nerd Font).
const (
	iconDownload = "" // nf-fa-download
	iconGear     = "" // nf-fa-cog
)

var (
	accent  = lipgloss.Color("13")  // magenta
	dim     = lipgloss.Color("240")
	good    = lipgloss.Color("10")
	bad     = lipgloss.Color("9")
	fgFaint = lipgloss.Color("245")

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(accent)
	tabOn       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(accent).Padding(0, 1)
	tabOff      = lipgloss.NewStyle().Foreground(dim).Padding(0, 1)
	dimStyle    = lipgloss.NewStyle().Foreground(dim)
	faintStyle  = lipgloss.NewStyle().Foreground(fgFaint)
	goodStyle   = lipgloss.NewStyle().Foreground(good)
	badStyle    = lipgloss.NewStyle().Foreground(bad)
	cursorStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	helpStyle   = lipgloss.NewStyle().Foreground(dim)
	flashStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

func (m model) View() string {
	var body string
	if m.view == configView {
		body = m.configCard()
	} else {
		body = m.queueCard()
	}
	// Center the whole card horizontally and vertically in the window.
	if m.w > 0 && m.h > 0 {
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, body)
	}
	return body
}

// width returns the usable terminal width, with a sane fallback before the
// first WindowSizeMsg arrives.
func (m model) width() int {
	if m.w > 0 {
		return m.w
	}
	return 72
}

// cardWidth is the width of the centered content column.
func (m model) cardWidth() int {
	w := m.width() - 8
	if w < 44 {
		w = 44
	}
	if w > 60 {
		w = 60
	}
	return w
}

func (m model) tabs() string {
	if m.mode == audioMode {
		return tabOn.Render("Audio") + "   " + tabOff.Render("Video")
	}
	return tabOff.Render("Audio") + "   " + tabOn.Render("Video")
}

func (m model) queueCard() string {
	cw := m.cardWidth()
	center := lipgloss.NewStyle().Width(cw).Align(lipgloss.Center)
	leftCol := lipgloss.NewStyle().Width(cw).Align(lipgloss.Left)

	title := center.Render(titleStyle.Render(iconDownload + "  omagrab"))
	tabs := center.Render(m.tabs())
	input := center.Render(m.input.View())

	var queue string
	if len(m.items) == 0 {
		queue = center.Render(dimStyle.Render("queue empty — paste a URL above"))
	} else {
		rows := make([]string, len(m.items))
		for i, it := range m.items {
			rows[i] = m.renderItem(i, it, cw)
		}
		queue = leftCol.Render(strings.Join(rows, "\n"))
	}

	help := center.Render(
		helpStyle.Render("tab audio/video · enter add · ↑↓ select · esc quit") + "\n" +
			helpStyle.Render("empty box →  c config · d remove · q quit"))

	parts := []string{title, tabs, "", input, "", queue, "", help}
	if m.flash != "" {
		parts = append(parts, "", center.Render(flashStyle.Render(m.flash)))
	}
	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}

func (m model) renderItem(i int, it *item, w int) string {
	cur := "  "
	if i == m.cursor {
		cur = cursorStyle.Render("▌ ")
	}

	barW := 14
	if w < 72 {
		barW = 10
	}
	tailBudget := barW + 16 // bar + percent + speed, kept stable so names align

	var icon, tail string
	switch it.status {
	case stQueued:
		icon = dimStyle.Render("○")
		tail = dimStyle.Render("queued")
	case stFetching:
		icon = m.spinner.View()
		tail = dimStyle.Render("fetching…")
	case stDownloading:
		icon = m.spinner.View()
		tail = bar(it.percent, barW) + "  " + faintStyle.Render(it.speed)
	case stDone:
		icon = goodStyle.Render("✓")
		tail = goodStyle.Render("done")
	case stFailed:
		icon = badStyle.Render("✗")
		tail = badStyle.Render(truncate(it.err, tailBudget))
	}

	nameW := w - 6 - tailBudget
	if nameW < 16 {
		nameW = 16
	}
	if nameW > 50 {
		nameW = 50
	}
	name := truncate(it.label(), nameW)
	return fmt.Sprintf("%s%s %-*s  %s", cur, icon, nameW, name, tail)
}

func (m model) configCard() string {
	cw := m.cardWidth()
	center := lipgloss.NewStyle().Width(cw).Align(lipgloss.Center)
	leftCol := lipgloss.NewStyle().Width(cw).Align(lipgloss.Left)

	title := center.Render(titleStyle.Render(iconGear + "  omagrab · config"))

	fields := []struct{ label, val string }{
		{"Audio format", m.cfg.AudioFormat},
		{"Video quality", m.cfg.VideoQuality},
		{"Subtitles", onoff(m.cfg.Subtitles)},
	}
	rows := make([]string, len(fields))
	for i, f := range fields {
		marker := "  "
		val := f.val
		if i == m.cfgCur {
			marker = cursorStyle.Render("▌ ")
			val = cursorStyle.Render("‹ " + f.val + " ›")
		}
		rows[i] = fmt.Sprintf("%s%-16s %s", marker, f.label, val)
	}
	block := leftCol.Render(strings.Join(rows, "\n"))

	dirs := center.Render(
		faintStyle.Render("audio → "+m.cfg.AudioDir) + "\n" +
			faintStyle.Render("video → "+m.cfg.VideoDir))
	help := center.Render(helpStyle.Render("↑↓ move · ←→ change · esc save & back"))

	return lipgloss.JoinVertical(lipgloss.Center, title, "", block, "", dirs, "", help)
}

// bar renders a simple unicode progress bar.
func bar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * float64(width))
	full := lipgloss.NewStyle().Foreground(accent).Render(strings.Repeat("█", filled))
	empty := dimStyle.Render(strings.Repeat("░", width-filled))
	return full + empty + fmt.Sprintf(" %3.0f%%", frac*100)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func onoff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
