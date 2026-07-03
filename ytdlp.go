package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

func startDownload(ctx context.Context, url, outputDir string, preset QualityPreset) tea.Cmd {
	return func() tea.Msg {
		if ctx.Err() != nil {
			return downloadDoneMsg{Cancelled: true}
		}

		ch := make(chan tea.Msg, 100)

		args := []string{
			"--newline",
			"--progress-template",
			`json:{"percent":"%(progress.percent)s","speed":"%(progress.speed)s","eta":"%(progress.eta)s"}`,
			"-f", preset.Format,
			"-o", filepath.Join(outputDir, "%(title)s.%(ext)s"),
		}

		if preset.AudioOnly {
			args = append(args, "-x", "--audio-format", "mp3", "--audio-quality", "0")
		}

		args = append(args, url)

		cmd := exec.CommandContext(ctx, ytDlpBin, args...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return downloadDoneMsg{Err: fmt.Errorf("error al crear pipe: %w", err)}
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return downloadDoneMsg{Err: fmt.Errorf("error al crear pipe: %w", err)}
		}

		if err := cmd.Start(); err != nil {
			return downloadDoneMsg{Err: fmt.Errorf("no se pudo iniciar yt-dlp: %w", err)}
		}

		go func() {
			defer close(ch)

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if !strings.HasPrefix(line, "{") {
						continue
					}
					var p progressData
					if json.Unmarshal([]byte(line), &p) != nil || p.Percent == "" {
						continue
					}
					percent := 0.0
					if n, _ := fmt.Sscanf(strings.TrimSuffix(p.Percent, "%"), "%f", &percent); n != 1 {
						continue
					}

					select {
					case ch <- progressMsg{Percent: percent, Speed: p.Speed, ETA: p.ETA}:
					case <-ctx.Done():
						return
					}
				}
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()

				var stderrBuf strings.Builder
				stderrDone := make(chan struct{}, 1)
				go func() {
					io.Copy(&stderrBuf, stderr)
					close(stderrDone)
				}()

				err := cmd.Wait()
				<-stderrDone

				if ctx.Err() != nil {
					select {
					case ch <- downloadDoneMsg{Cancelled: true}:
					case <-ctx.Done():
					}
					return
				}

				if err != nil {
					errMsg := strings.TrimSpace(stderrBuf.String())
					if errMsg == "" {
						errMsg = fmt.Sprintf("yt-dlp falló (código %d)", cmd.ProcessState.ExitCode())
					}
					select {
					case ch <- downloadDoneMsg{Err: fmt.Errorf("%s", errMsg)}:
					case <-ctx.Done():
					}
				} else {
					select {
					case ch <- downloadDoneMsg{Success: true}:
					case <-ctx.Done():
					}
				}
			}()

			wg.Wait()
		}()

		return downloadStartedMsg{ch: ch}
	}
}
