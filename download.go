package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// progress lines are rendered by yt-dlp from this template; we tag them with a
// sentinel so they're trivially distinguished from yt-dlp's other chatter.
const progSentinel = "OMAGRABPROG|"
const progTemplate = "download:" + progSentinel +
	"%(progress._percent_str)s|%(progress._speed_str)s|%(progress._eta_str)s"

// checkDeps returns the list of required external binaries that are missing.
func checkDeps() []string {
	var missing []string
	for _, bin := range []string{"yt-dlp", "ffmpeg"} {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	return missing
}

// depPackages maps the missing binaries to their Arch package names.
func depPackages(missing []string) string {
	pkg := map[string]string{"yt-dlp": "yt-dlp", "ffmpeg": "ffmpeg"}
	var out []string
	for _, m := range missing {
		out = append(out, pkg[m])
	}
	return strings.Join(out, " ")
}

// buildArgs returns the yt-dlp arguments for one queued item, snapshotting the
// current config so later config changes don't affect an already-queued job.
func buildArgs(cfg Config, m mode) []string {
	args := []string{
		"--no-warnings", "--newline",
		"--progress-template", progTemplate,
		"-o", "%(title)s.%(ext)s",
	}
	if m == audioMode {
		args = append(args,
			"-P", ExpandHome(cfg.AudioDir),
			"-x", "--audio-format", cfg.AudioFormat,
			"--audio-quality", "0",
			"--embed-metadata", "--embed-thumbnail",
		)
	} else {
		var sel string
		if cfg.VideoQuality == "best" {
			sel = "bv*+ba/b"
		} else {
			sel = fmt.Sprintf("bv*[height<=%s]+ba/b[height<=%s]", cfg.VideoQuality, cfg.VideoQuality)
		}
		args = append(args,
			"-P", ExpandHome(cfg.VideoDir),
			"-f", sel,
			"--merge-output-format", "mp4",
			"--embed-metadata",
		)
		if cfg.Subtitles {
			args = append(args, "--embed-subs", "--sub-langs", "en.*")
		}
	}
	return args
}

// worker processes the queue sequentially, streaming events back to the UI.
func worker(p *tea.Program, jobs <-chan *item) {
	for it := range jobs {
		runOne(p, it)
	}
}

func runOne(p *tea.Program, it *item) {
	// 1. resolve a human title (best effort; falls back to the URL).
	p.Send(statusMsg{url: it.url, status: stFetching})
	if title := fetchTitle(it.url); title != "" {
		p.Send(titleMsg{url: it.url, title: title})
	}

	// 2. download, streaming progress.
	p.Send(statusMsg{url: it.url, status: stDownloading})
	args := append(append([]string{}, it.args...), it.url)
	cmd := exec.Command("yt-dlp", args...)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		p.Send(failMsg{url: it.url, err: err.Error()})
		return
	}

	tail := newRing(8)
	done := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, progSentinel) {
				if pct, speed, eta, ok := parseProg(line); ok {
					p.Send(progressMsg{url: it.url, percent: pct, speed: speed, eta: eta})
				}
				continue
			}
			tail.push(line)
		}
		close(done)
	}()

	err := cmd.Wait()
	pw.Close()
	<-done

	if err != nil {
		p.Send(failMsg{url: it.url, err: tail.last()})
		return
	}
	p.Send(doneMsg{url: it.url})
}

// fetchTitle does a lightweight metadata-only lookup.
func fetchTitle(url string) string {
	out, err := exec.Command("yt-dlp", "--no-warnings", "--skip-download",
		"--no-playlist", "--print", "%(title)s", url).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}

func parseProg(line string) (pct float64, speed, eta string, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(line, progSentinel), "|", 3)
	if len(parts) != 3 {
		return 0, "", "", false
	}
	ps := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[0]), "%"))
	pct, err := strconv.ParseFloat(ps, 64)
	if err != nil {
		return 0, "", "", false
	}
	return pct / 100, strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), true
}

// ring is a tiny fixed-size line buffer for capturing the tail of stderr.
type ring struct {
	buf []string
	n   int
}

func newRing(n int) *ring { return &ring{buf: make([]string, 0, n), n: n} }

func (r *ring) push(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	if len(r.buf) == r.n {
		r.buf = r.buf[1:]
	}
	r.buf = append(r.buf, s)
}

func (r *ring) last() string {
	if len(r.buf) == 0 {
		return "download failed"
	}
	return r.buf[len(r.buf)-1]
}
