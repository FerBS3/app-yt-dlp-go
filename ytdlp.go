package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func startDownload(ctx context.Context, url, outputDir string, preset QualityPreset) tea.Cmd {
	return func() tea.Msg {
		if ctx.Err() != nil {
			return downloadDoneMsg{Cancelled: true}
		}

		ch := make(chan tea.Msg, 100)

		format := preset.Format
		if !preset.AudioOnly && !ffmpegAvailable() {
			for _, seg := range strings.Split(format, "/") {
				if !strings.Contains(seg, "+") {
					format = strings.TrimSpace(seg)
					break
				}
			}
		}

		args := []string{
			"--newline",
			"--socket-timeout", "30",
			"--progress-template",
			`{"percent":"%(progress.percent)s","speed":"%(progress.speed)s","eta":"%(progress.eta)s"}`,
			"-f", format,
			"-o", filepath.Join(outputDir, "%(title)s.%(ext)s"),
		}

		if !preset.AudioOnly {
			if _, err := exec.LookPath("ffmpeg"); err == nil {
				args = append(args, "--merge-output-format", "mp4")
			} else if _, err := os.Stat(getFfmpegPath()); err == nil {
				args = append(args, "--merge-output-format", "mp4")
				args = append(args, "--ffmpeg-location", getFfmpegDir())
			}
		}

		pathFile, err := os.CreateTemp("", "dlpgo-filepath-*.txt")
		if err != nil {
			return downloadDoneMsg{Err: fmt.Errorf("error al crear archivo temporal: %w", err)}
		}
		pathFile.Close()
		pathFileName := pathFile.Name()

		args = append(args, "--print-to-file", "after_move:filepath", pathFileName)
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

		sendProgress := func(p progressData) bool {
			percent := 0.0
			if p.Percent != "" {
				fmt.Sscanf(strings.TrimSuffix(p.Percent, "%"), "%f", &percent)
			}
			select {
			case ch <- progressMsg{Percent: percent, Speed: p.Speed, ETA: p.ETA}:
				return true
			case <-ctx.Done():
				return false
			}
		}

		var errBuf strings.Builder
		stderrDone := make(chan struct{})
		go func() {
			defer close(stderrDone)
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				var p progressData
				if json.Unmarshal([]byte(line), &p) == nil {
					if !sendProgress(p) {
						return
					}
					continue
				}
				errBuf.WriteString(line + "\n")
			}
		}()

		var lastOutputPath string
		stdoutDone := make(chan struct{})
		go func() {
			defer close(stdoutDone)
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				var p progressData
				if json.Unmarshal([]byte(line), &p) == nil {
					if !sendProgress(p) {
						return
					}
					continue
				}
				if line != "" {
					lastOutputPath = line
				}
			}
		}()

		err := cmd.Wait()
		<-stderrDone
		<-stdoutDone

		if data, readErr := os.ReadFile(pathFileName); readErr == nil {
			if p := strings.TrimSpace(string(data)); p != "" {
				lastOutputPath = p
			}
		}
		os.Remove(pathFileName)

			if ctx.Err() != nil {
				select {
				case ch <- downloadDoneMsg{Cancelled: true}:
				case <-ctx.Done():
				}
				return
			}

			if err != nil {
				errMsg := strings.TrimSpace(errBuf.String())
				if errMsg == "" {
					errMsg = fmt.Sprintf("yt-dlp falló (código %d)", cmd.ProcessState.ExitCode())
				}
				select {
				case ch <- downloadDoneMsg{Err: fmt.Errorf("%s", errMsg)}:
				case <-ctx.Done():
				}
			} else {
				select {
				case ch <- downloadDoneMsg{Success: true, FilePath: lastOutputPath}:
				case <-ctx.Done():
				}
			}
		}()

		return downloadStartedMsg{ch: ch}
	}
}
