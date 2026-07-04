package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type VideoInfo struct {
	Title     string  `json:"title"`
	Channel   string  `json:"channel"`
	Uploader  string  `json:"uploader"`
	Duration  float64 `json:"duration"`
	Thumbnail string  `json:"thumbnail"`
}

type QualityPreset struct {
	Label     string
	Format    string
	AudioOnly bool
}

const AppName = "DLP Go"

var qualityPresets = []QualityPreset{
	{Label: "Mejor calidad (1080p)", Format: "bestvideo[height<=1080]+bestaudio/best[height<=1080]"},
	{Label: "720p", Format: "bestvideo[height<=720]+bestaudio/best[height<=720]"},
	{Label: "480p", Format: "bestvideo[height<=480]+bestaudio/best[height<=480]"},
	{Label: "Mejor calidad", Format: "bestvideo+bestaudio/best"}
	{Label: "Audio AAC (m4a)", Format: "bestaudio[ext=m4a]/bestaudio", AudioOnly: true},
	{Label: "Audio Opus", Format: "bestaudio[ext=opus]/bestaudio", AudioOnly: true},
	{Label: "Mejor audio", Format: "bestaudio/best", AudioOnly: true},
}

var ytDlpBin string
var selectedSource = "nightly"

type progressData struct {
	Percent string `json:"percent"`
	Speed   string `json:"speed"`
	ETA     string `json:"eta"`
}

type progressMsg struct {
	Percent float64
	Speed   string
	ETA     string
}

type downloadDoneMsg struct {
	Success   bool
	Cancelled bool
	FilePath  string
	Err       error
}

type videoInfoMsg struct {
	Info *VideoInfo
	Err  error
}

type downloadStartedMsg struct {
	ch chan tea.Msg
}

type ytDlpFoundMsg struct {
	path string
	err  error
}

type ytDlpNeededMsg struct {
	targetPath string
	source     string
}

type updateCheckResultMsg struct {
	currentVersion string
	latestVersion  string
	err            error
}

type updateCompletedMsg struct {
	err error
}

func fetchVideoInfo(url string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command(ytDlpBin, "--dump-json", "--no-download", url)
		output, err := cmd.Output()
		if err != nil {
			var exitErr *exec.ExitError
			msg := fmt.Sprintf("yt-dlp: %s", err)
			if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
				msg = fmt.Sprintf("yt-dlp: %s", string(exitErr.Stderr))
			}
			return videoInfoMsg{Err: fmt.Errorf("%s", msg)}
		}

		var info VideoInfo
		if err := json.Unmarshal(output, &info); err != nil {
			return videoInfoMsg{Err: fmt.Errorf("error al analizar: %w", err)}
		}
		if info.Channel == "" {
			info.Channel = info.Uploader
		}

		return videoInfoMsg{Info: &info}
	}
}

func listenDownload(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func getAppDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func getYtDlpDir() string {
	return filepath.Join(getAppDir(), "yt-dlp")
}

func getYtDlpPath(source string) string {
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(getYtDlpDir(), source, name)
}

func getDownloadsDir() string {
	return filepath.Join(getAppDir(), "Descargas")
}

func getConfigPath() string {
	return filepath.Join(getYtDlpDir(), "config.json")
}

func loadConfig() (string, error) {
	data, err := os.ReadFile(getConfigPath())
	if err != nil {
		return "nightly", nil
	}
	var cfg struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "nightly", nil
	}
	if cfg.Source != "stable" && cfg.Source != "nightly" {
		return "nightly", nil
	}
	return cfg.Source, nil
}

func saveConfig(source string) error {
	cfg := struct {
		Source string `json:"source"`
	}{Source: source}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(getConfigPath(), data, 0644)
}

func getYtDlpDownloadURL(source string) string {
	repo := "yt-dlp/yt-dlp-nightly-builds"
	if source == "stable" {
		repo = "yt-dlp/yt-dlp"
	}
	base := fmt.Sprintf("https://github.com/%s/releases/latest/download/yt-dlp", repo)
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func getLatestReleaseAPIURL(source string) string {
	repo := "yt-dlp/yt-dlp-nightly-builds"
	if source == "stable" {
		repo = "yt-dlp/yt-dlp"
	}
	return fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
}

func getYtDlpVersion(binPath string) (string, error) {
	cmd := exec.Command(binPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getLatestReleaseVersion(source string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(getLatestReleaseAPIURL(source))
	if err != nil {
		return "", fmt.Errorf("no se pudo conectar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub respondió HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("error al leer respuesta: %w", err)
	}

	return release.TagName, nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("no se pudo descargar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d al descargar", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("no se pudo crear archivo: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		os.Remove(dest)
		return fmt.Errorf("error al escribir: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(dest, 0755); err != nil {
			return fmt.Errorf("no se pudo dar permisos: %w", err)
		}
	}

	return nil
}

func ensureAppDirs() error {
	dirs := []string{
		getDownloadsDir(),
		getYtDlpDir(),
		filepath.Dir(getYtDlpPath("nightly")),
		filepath.Dir(getYtDlpPath("stable")),
		getFfmpegDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("no se pudo crear directorio %s: %w", d, err)
		}
	}
	return nil
}

func checkYtDlp() tea.Cmd {
	return func() tea.Msg {
		selectedSource, _ = loadConfig()

		if path, err := exec.LookPath("yt-dlp"); err == nil {
			ytDlpBin = path
			return ytDlpFoundMsg{path: path}
		}

		targetPath := getYtDlpPath(selectedSource)
		if _, err := os.Stat(targetPath); err == nil {
			if out, err := exec.Command(targetPath, "--version").Output(); err == nil && len(out) > 0 {
				ytDlpBin = targetPath
				return ytDlpFoundMsg{path: targetPath}
			}
		}

		return ytDlpNeededMsg{targetPath: targetPath, source: selectedSource}
	}
}

func downloadYtDlp(targetPath, source string) tea.Cmd {
	return func() tea.Msg {
		url := getYtDlpDownloadURL(source)
		if err := downloadFile(url, targetPath); err != nil {
			return ytDlpFoundMsg{err: err}
		}
		ytDlpBin = targetPath
		return ytDlpFoundMsg{path: targetPath}
	}
}

func checkForUpdatesCmd() tea.Cmd {
	return func() tea.Msg {
		binPath := getYtDlpPath(selectedSource)

		current := ""
		if _, err := os.Stat(binPath); err == nil {
			if v, err := getYtDlpVersion(binPath); err == nil {
				current = v
			}
		}

		latest, err := getLatestReleaseVersion(selectedSource)
		if err != nil {
			return updateCheckResultMsg{err: err}
		}

		return updateCheckResultMsg{
			currentVersion: current,
			latestVersion:  latest,
		}
	}
}

func performUpdateCmd(targetPath string) tea.Cmd {
	return func() tea.Msg {
		url := getYtDlpDownloadURL(selectedSource)
		if err := downloadFile(url, targetPath); err != nil {
			return updateCompletedMsg{err: err}
		}
		ytDlpBin = targetPath
		return updateCompletedMsg{}
	}
}

func getFfmpegDir() string {
	return filepath.Join(getYtDlpDir(), "ffmpeg")
}

func getFfmpegPath() string {
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(getFfmpegDir(), name)
}

func getFfmpegDownloadURL() string {
	if runtime.GOOS == "windows" {
		return "https://github.com/BtbN/FFmpeg-Builds/releases/latest/download/ffmpeg-master-latest-win64-gpl.zip"
	}
	return "https://github.com/BtbN/FFmpeg-Builds/releases/latest/download/ffmpeg-master-latest-linux64-gpl.tar.xz"
}

type ffmpegDownloadedMsg struct {
	Version string
	Err     error
}

func downloadFfmpegCmd() tea.Cmd {
	return func() tea.Msg {
		dest := getFfmpegPath()
		os.MkdirAll(filepath.Dir(dest), 0755)

		archive, err := os.CreateTemp("", "ffmpeg-*")
		if err != nil {
			return ffmpegDownloadedMsg{Err: fmt.Errorf("no se pudo crear temporal: %w", err)}
		}
		tmpPath := archive.Name()
		archive.Close()
		defer os.Remove(tmpPath)

		url := getFfmpegDownloadURL()
		if err := downloadFile(url, tmpPath); err != nil {
			return ffmpegDownloadedMsg{Err: err}
		}

		if runtime.GOOS == "windows" {
			err = extractFfmpegZip(tmpPath, dest)
		} else {
			err = extractFfmpegTarXz(tmpPath, dest)
		}
		if err != nil {
			return ffmpegDownloadedMsg{Err: err}
		}

		ver := getFfmpegVersion(dest)
		return ffmpegDownloadedMsg{Version: ver}
	}
}

func extractFfmpegTarXz(archive, dest string) error {
	tmpDir, err := os.MkdirTemp("", "ffmpeg-extract")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("tar", "-xJf", archive, "-C", tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error al extraer: %s", string(out))
	}

	var src string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Mode().IsRegular() && filepath.Base(path) == "ffmpeg" {
			src = path
			return filepath.SkipAll
		}
		return nil
	})

	if src == "" {
		return fmt.Errorf("no se encontró ffmpeg en el archivo")
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return os.Chmod(dest, 0755)
}

func extractFfmpegZip(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if !f.FileInfo().IsDir() && strings.HasSuffix(f.Name, "ffmpeg.exe") {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, rc)
			return err
		}
	}

	return fmt.Errorf("no se encontró ffmpeg.exe en el archivo")
}

func getFfmpegVersion(binPath string) string {
	cmd := exec.Command(binPath, "-version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.SplitN(string(out), "\n", 2)
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

func ffmpegAvailable() bool {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return true
	}
	if _, err := os.Stat(getFfmpegPath()); err == nil {
		return true
	}
	return false
}

func checkFfmpegStatus() (string, string) {
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		ver := getFfmpegVersion(path)
		if ver != "" {
			return ver, "sistema"
		}
		return "ffmpeg (en PATH)", "sistema"
	}

	localPath := getFfmpegPath()
	if _, err := os.Stat(localPath); err == nil {
		ver := getFfmpegVersion(localPath)
		if ver != "" {
			return ver, "estático"
		}
		return "ffmpeg (local)", "estático"
	}

	return "", ""
}
