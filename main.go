package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const version = "0.1.0"

func main() {
	var initialURL string
	var useClip bool
	for _, a := range os.Args[1:] {
		switch a {
		case "-v", "--version":
			fmt.Println("omagrab", version)
			return
		case "-h", "--help":
			fmt.Println("omagrab — a yt-dlp TUI for Omarchy\n\n" +
				"Usage: omagrab [URL]\n\n" +
				"  -c, --clip   pre-fill the URL from the clipboard (wl-paste)\n\n" +
				"Paste URLs into the queue; Tab switches Audio/Video mode.\n" +
				"Launch with a URL (or an omagrab: scheme link) to pre-fill it.\n" +
				"Config lives in ~/.config/omagrab/config.json")
			return
		case "-c", "--clip":
			useClip = true
		default:
			if u := normalizeArgURL(a); u != "" {
				initialURL = u
			}
		}
	}
	if initialURL == "" && useClip {
		initialURL = clipboardURL()
	}

	if missing := checkDeps(); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "omagrab: missing required tools: %v\n", missing)
		fmt.Fprintf(os.Stderr, "install with:  sudo pacman -S %s\n", depPackages(missing))
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "omagrab: config error:", err)
		os.Exit(1)
	}

	jobs := make(chan *item, 256)
	p := tea.NewProgram(newModel(cfg, jobs, initialURL), tea.WithAltScreen())
	go worker(p, jobs)

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// clipboardURL reads the Wayland clipboard and returns it if it's an http(s) URL.
func clipboardURL() string {
	out, err := exec.Command("wl-paste", "-n").Output()
	if err != nil {
		return ""
	}
	return normalizeArgURL(string(out))
}

// normalizeArgURL accepts either a plain http(s) URL or an "omagrab:" / "omagrab://"
// scheme link (as delivered by the browser extension via xdg-open) and returns a
// clean http(s) URL, or "" if the argument isn't a usable URL.
func normalizeArgURL(a string) string {
	s := strings.TrimSpace(a)
	for _, pfx := range []string{"omagrab://", "omagrab:"} {
		if strings.HasPrefix(s, pfx) {
			s = strings.TrimPrefix(s, pfx)
			if dec, err := url.QueryUnescape(s); err == nil {
				s = dec
			}
			break
		}
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	return ""
}
