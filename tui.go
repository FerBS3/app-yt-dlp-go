package main

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	checkingState state = iota
	downloadingBinState
	inputState
	loadingState
	selectState
	downloadingState
	doneState
	settingsState
	checkingUpdateState
	updatingState
	helpState
)

var (
	purple   = lipgloss.Color("#BB66FF")
	cyan     = lipgloss.Color("#66DDEE")
	green    = lipgloss.Color("#66EE88")
	red      = lipgloss.Color("#FF5577")
	gray     = lipgloss.Color("#888888")
	white    = lipgloss.Color("#EEEEEE")
	orange   = lipgloss.Color("#FFAA44")
	darkGray = lipgloss.Color("#999999")

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(purple).
			Align(lipgloss.Center)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Align(lipgloss.Center)

	labelStyle = lipgloss.NewStyle().
			Foreground(gray).
			Width(12)

	valueStyle = lipgloss.NewStyle().
			Foreground(white)

	selectedStyle = lipgloss.NewStyle().
			Foreground(purple).
			Bold(true)

	unselectedStyle = lipgloss.NewStyle().
			Foreground(white)

	successStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(darkGray).
			Align(lipgloss.Center)

	progressPercentStyle = lipgloss.NewStyle().
				Foreground(purple).
				Bold(true)

	progressInfoStyle = lipgloss.NewStyle().
				Foreground(cyan)

	settingsLabelStyle = lipgloss.NewStyle().
				Foreground(cyan).
				Width(20)

	versionStyle = lipgloss.NewStyle().
			Foreground(orange)

)

type etaTickMsg struct{}

type model struct {
	state    state
	width    int
	height   int

	textInput textinput.Model
	spinner   spinner.Model
	progress  progress.Model

	qualityIndex int
	dlPercent     float64
	dlSpeed       string
	dlETA         string
	dlETAEndTime  time.Time

	dlChan   chan tea.Msg
	dlCancel context.CancelFunc

	videoInfo *VideoInfo

	err        error
	inputError error
	successMsg string
	dlFilePath string

	selectedSource      string
	updateCurrentVersion string
	updateLatestVersion  string
	updateVersionMsg     string
	ytDlpVersion        string

	downloadLabel      string
	startupFfmpegCheck bool
	ffmpegVersion      string
	ffmpegStatus   string
	ffmpegStatusMsg string
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "https://..."
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 50

	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(purple)
	s.Spinner = spinner.Dot

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(50),
	)

	return model{
		state:        checkingState,
		textInput:    ti,
		spinner:      s,
		progress:     p,
		qualityIndex: 0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		checkYtDlp(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 20
		if m.progress.Width < 30 {
			m.progress.Width = 30
		}
		m.textInput.Width = msg.Width - 40
		if m.textInput.Width < 30 {
			m.textInput.Width = 30
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.dlCancel != nil {
				m.dlCancel()
			}
			return m, tea.Quit

		case "c", "C":
			if m.state == inputState || m.state == doneState {
				m.selectedSource = selectedSource
				m.ytDlpVersion, _ = getYtDlpVersion(getYtDlpPath(selectedSource))
				m.ffmpegVersion, m.ffmpegStatus = checkFfmpegStatus()
				m.ffmpegStatusMsg = ""
				m.updateVersionMsg = ""
				m.state = settingsState
				return m, nil
			}
			if m.state == settingsState {
				selectedSource = m.selectedSource
				saveConfig(m.selectedSource)
				m.state = inputState
				return m, nil
			}

		case "esc":
			if m.state == settingsState {
				m.selectedSource = selectedSource
				m.state = inputState
				return m, nil
			}
			if m.state == selectState {
				m.state = inputState
				m.videoInfo = nil
				m.qualityIndex = 0
				return m, nil
			}
			if m.state == helpState {
				m.state = inputState
				return m, nil
			}

		case "p", "P":
			if m.state == inputState {
				text, err := clipboard.ReadAll()
				if err == nil && text != "" {
					m.textInput.SetValue(text)
				}
				return m, nil
			}

		case "f1":
			if m.state == inputState {
				m.state = helpState
				return m, nil
			}

		case "1":
			if m.state == settingsState {
				m.selectedSource = "stable"
			}

		case "2":
			if m.state == settingsState {
				m.selectedSource = "nightly"
			}

		case "u", "U":
			if m.state == settingsState {
				selectedSource = m.selectedSource
				m.state = checkingUpdateState
				m.updateVersionMsg = ""
				return m, tea.Batch(checkForUpdatesCmd(), m.spinner.Tick)
			}

		case "f", "F":
			if m.state == settingsState && m.ffmpegStatus != "sistema" {
				m.state = downloadingBinState
				m.downloadLabel = "ffmpeg"
				return m, tea.Batch(downloadFfmpegCmd(), m.spinner.Tick)
			}

		case "enter":
			switch m.state {
			case inputState:
				url := strings.TrimSpace(m.textInput.Value())
			if url == "" {
				return m, nil
			}
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				m.inputError = fmt.Errorf("La URL debe comenzar con http:// o https://")
				return m, nil
			}
			m.inputError = nil
			m.state = loadingState
				m.err = nil
				return m, tea.Batch(fetchVideoInfo(url), m.spinner.Tick)

			case selectState:
				m.state = downloadingState
				ctx, cancel := context.WithCancel(context.Background())
				m.dlCancel = cancel
				outputDir := getDownloadsDir()
				return m, startDownload(ctx, strings.TrimSpace(m.textInput.Value()), outputDir, qualityPresets[m.qualityIndex])

			case doneState:
				if m.dlCancel != nil {
					m.dlCancel()
					m.dlCancel = nil
				}
				if m.err != nil && m.videoInfo != nil {
					m.state = selectState
					m.err = nil
					m.dlPercent = 0
					m.dlSpeed = ""
					m.dlETA = ""
					return m, nil
				}
				m.textInput.SetValue("")
				m.state = inputState
				m.err = nil
				m.inputError = nil
				m.successMsg = ""
				m.dlFilePath = ""
				m.videoInfo = nil
				m.qualityIndex = 0
				m.dlPercent = 0
				m.dlSpeed = ""
				m.dlETA = ""
				return m, nil
			}

		case "up", "k":
			if m.state == selectState && m.qualityIndex > 0 {
				m.qualityIndex--
			}

		case "down", "j":
			if m.state == selectState && m.qualityIndex < len(qualityPresets)-1 {
				m.qualityIndex++
			}
		}

		if m.state == inputState {
			m.inputError = nil
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

	case ytDlpFoundMsg:
		if msg.err != nil {
			m.state = doneState
			m.err = msg.err
		} else {
			if ffmpegAvailable() {
				m.state = inputState
			} else {
				m.state = downloadingBinState
				m.downloadLabel = "ffmpeg"
				m.startupFfmpegCheck = true
				return m, tea.Batch(downloadFfmpegCmd(), m.spinner.Tick)
			}
		}
		return m, nil

	case ytDlpNeededMsg:
		m.state = downloadingBinState
		m.downloadLabel = "yt-dlp " + msg.source
		return m, tea.Batch(downloadYtDlp(msg.targetPath, msg.source), m.spinner.Tick)

	case videoInfoMsg:
		if msg.Err != nil {
			m.state = doneState
			m.err = msg.Err
		} else {
			m.videoInfo = msg.Info
			m.state = selectState
		}
		return m, nil

	case downloadStartedMsg:
		m.dlChan = msg.ch
		return m, listenDownload(m.dlChan)

	case progressMsg:
		m.dlPercent = msg.Percent
		m.dlSpeed = msg.Speed
		m.dlETA = msg.ETA
		if msg.ETA != "" && msg.ETA != "NA" {
			if s, err := strconv.Atoi(msg.ETA); err == nil {
				m.dlETAEndTime = time.Now().Add(time.Duration(s) * time.Second)
			}
		} else {
			m.dlETAEndTime = time.Time{}
		}
		return m, tea.Batch(listenDownload(m.dlChan), tickSecond())

	case downloadDoneMsg:
		if m.dlCancel != nil {
			m.dlCancel()
			m.dlCancel = nil
		}
		m.dlChan = nil
		m.state = doneState
		if msg.Cancelled {
			m.err = fmt.Errorf("Descarga cancelada")
		} else if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.successMsg = "Descarga completada ✓"
			m.dlFilePath = msg.FilePath
			m.err = nil
		}
		return m, nil

	case etaTickMsg:
		if m.state == downloadingState && !m.dlETAEndTime.IsZero() {
			return m, tickSecond()
		}
		return m, nil

	case nil:
		m.state = doneState
		m.err = fmt.Errorf("Error inesperado en la descarga")
		return m, nil

	case spinner.TickMsg:
		if m.state == loadingState || m.state == checkingState || m.state == downloadingBinState || m.state == checkingUpdateState || m.state == updatingState || m.state == downloadingState {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case progress.FrameMsg:
		if m.state == downloadingState {
			var cmd tea.Cmd
			pModel, cmd := m.progress.Update(msg)
			m.progress = pModel.(progress.Model)
			return m, cmd
		}
		return m, nil

	case updateCheckResultMsg:
		m.selectedSource = selectedSource
		if msg.err != nil {
			m.state = settingsState
			m.updateVersionMsg = errorStyle.Render("Error: " + msg.err.Error())
		} else if msg.currentVersion != msg.latestVersion {
			m.state = updatingState
			binPath := getYtDlpPath(m.selectedSource)
			return m, tea.Batch(performUpdateCmd(binPath), m.spinner.Tick)
		} else {
			m.state = settingsState
			if msg.currentVersion == "" {
				m.updateVersionMsg = "No instalado. Versión más reciente: " + msg.latestVersion
			} else {
				m.updateVersionMsg = "Ya tenés la versión más reciente (" + msg.currentVersion + ") ✓"
			}
		}
		return m, nil

	case updateCompletedMsg:
		m.state = settingsState
		if msg.err != nil {
			m.updateVersionMsg = errorStyle.Render("Error al actualizar: " + msg.err.Error())
		} else {
			ver, _ := getYtDlpVersion(getYtDlpPath(m.selectedSource))
			if ver != "" {
				m.updateVersionMsg = successStyle.Render("Actualizado a versión " + ver + " ✓")
			} else {
				m.updateVersionMsg = successStyle.Render("Actualización completada ✓")
			}
		}
		return m, nil

	case ffmpegDownloadedMsg:
		if m.startupFfmpegCheck {
			m.startupFfmpegCheck = false
			if msg.Err == nil {
				m.ffmpegVersion = msg.Version
				m.ffmpegStatus = "estático"
			}
			m.state = inputState
		} else {
			m.state = settingsState
			if msg.Err != nil {
				m.ffmpegStatusMsg = errorStyle.Render("Error: " + msg.Err.Error())
			} else {
				m.ffmpegVersion = msg.Version
				m.ffmpegStatus = "estático"
				m.ffmpegStatusMsg = successStyle.Render("ffmpeg instalado ✓")
			}
		}
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	switch m.state {
	case checkingState:
		return m.checkingView()
	case downloadingBinState:
		return m.downloadingBinView()
	case inputState:
		return m.inputView()
	case loadingState:
		return m.loadingView()
	case selectState:
		return m.selectView()
	case downloadingState:
		return m.downloadingView()
	case doneState:
		return m.doneView()
	case settingsState:
		return m.settingsView()
	case helpState:
		return m.helpView()
	case checkingUpdateState:
		return m.checkingUpdateView()
	case updatingState:
		return m.updatingView()
	default:
		return ""
	}
}

func (m model) appHeader() string {
	return renderTitle(contentWidth(m.width))
}

func (m model) checkingView() string {
	w := contentWidth(m.width)
	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" Verificando yt-dlp...",
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) downloadingBinView() string {
	w := contentWidth(m.width)
	label := m.downloadLabel
	if label == "" {
		label = "yt-dlp " + m.selectedSource
	}
	msg := "Descargando " + label + "..."
	if m.startupFfmpegCheck {
		msg = "No se encontró ffmpeg, descargando..."
	}
	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" "+msg,
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) inputView() string {
	w := contentWidth(m.width)
	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n")
	b.WriteString(renderSubtitle(w, "Descarga videos y audio desde YouTube, Instagram y más"))
	if existingCfg, _ := loadConfig(); existingCfg != "" {
		b.WriteString("  ")
		b.WriteString(versionStyle.Render("[" + existingCfg + "]"))
	}
	b.WriteString("\n\n")
	b.WriteString(renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			"URL del video",
			"",
			m.textInput.View(),
			"",
		),
	))
	if m.inputError != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  "+m.inputError.Error()))
	}
	b.WriteString("\n")
	b.WriteString(renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			"Dir: "+getDownloadsDir(),
			"",
			renderHint(w, "Enter para analizar  ·  P para pegar"),
			renderHint(w, "C para configurar  ·  F1 para ayuda  ·  Q o Ctrl+C para salir"),
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) loadingView() string {
	w := contentWidth(m.width)
	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" Analizando video...",
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) selectView() string {
	w := contentWidth(m.width)
	info := m.videoInfo
	dur := formatDuration(int(math.Round(info.Duration)))

	outputFormat := "Fmt: formato original"
	if ffmpegAvailable() {
		outputFormat = "Fmt: MP4"
	}

	infoBlock := lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Render("Título:  ")+valueStyle.Render(info.Title),
		labelStyle.Render("Canal:   ")+valueStyle.Render(info.Channel),
		labelStyle.Render("Dur.:    ")+valueStyle.Render(dur),
		labelStyle.Render("Salida:  ")+valueStyle.Render(outputFormat),
	)

	var items []string
	firstAudio := -1
	for i, q := range qualityPresets {
		if q.AudioOnly && firstAudio == -1 {
			firstAudio = i
		}
	}
	for i, q := range qualityPresets {
		if i == firstAudio {
			items = append(items, "")
			items = append(items, renderSubtitle(w, "── Audio ──"))
		}
		if i == 0 && firstAudio > 0 {
			items = append(items, renderSubtitle(w, "── Video ──"))
		}
		if i == m.qualityIndex {
			items = append(items, selectedStyle.Render("▸ "+q.Label))
			if q.Description != "" {
				items = append(items, unselectedStyle.Render("  "+q.Description))
			}
		} else {
			items = append(items, unselectedStyle.Render("  "+q.Label))
		}
	}

	qualityBlock := lipgloss.JoinVertical(lipgloss.Left, items...)

	content := lipgloss.JoinVertical(lipgloss.Left,
		infoBlock,
		"",
		"Seleccioná calidad (↑/↓):",
		qualityBlock,
		"",
		renderHint(w, "Enter para descargar  ·  Esc para volver  ·  Q o Ctrl+C para salir"),
	)

	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(renderBorder(w, content))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) downloadingView() string {
	w := contentWidth(m.width)
	info := ""
	if m.dlSpeed != "" {
		info += progressInfoStyle.Render("↑ "+formatSpeed(m.dlSpeed)) + "  "
	}
	if !m.dlETAEndTime.IsZero() {
		remaining := time.Until(m.dlETAEndTime)
		if remaining > 0 {
			info += progressInfoStyle.Render("T: " + formatDuration(int(remaining.Seconds())))
		}
	}

	var content string
	if m.dlPercent > 0 {
		bar := m.progress.ViewAs(m.dlPercent / 100)
		pct := progressPercentStyle.Render(fmt.Sprintf("%.1f%%", m.dlPercent))
		content = lipgloss.JoinVertical(lipgloss.Center,
			"↓ Descargando...",
			"",
			bar,
			pct,
			"",
			info,
			"",
			renderHint(w, "Ctrl+C para cancelar"),
		)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Center,
			"... Preparando descarga...",
			"",
			m.spinner.View(),
			"",
			info,
			"",
			renderHint(w, "Ctrl+C para cancelar"),
		)
	}

	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(renderBorder(w, content))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) doneView() string {
	w := contentWidth(m.width)
	var icon, title string
	lines := []string{}
	if m.err != nil {
		icon = errorStyle.Render("✗")
		title = errorStyle.Render("Error")
		lines = append(lines, icon+"  "+title, "")
		lines = append(lines, m.err.Error())
	} else {
		icon = successStyle.Render("✓")
		title = successStyle.Render("Descarga completada")
		lines = append(lines, icon+"  "+title, "")
		if m.dlFilePath != "" {
			lines = append(lines, m.dlFilePath)
			lines = append(lines, "")
		}
	}
	lines = append(lines, "")
	if m.err != nil {
		lines = append(lines, renderHint(w, "Enter para reintentar  ·  C para configurar  ·  Q o Ctrl+C para salir"))
	} else {
		lines = append(lines, renderHint(w, "Enter para continuar  ·  C para configurar  ·  Q o Ctrl+C para salir"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(renderBorder(w, content))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) settingsView() string {
	w := contentWidth(m.width)

	stableRadio := "○"
	nightlyRadio := "○"
	if m.selectedSource == "stable" {
		stableRadio = "●"
	}
	if m.selectedSource == "nightly" {
		nightlyRadio = "●"
	}

	// Sección Directorio
	dirSection := renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			"Dir: "+truncate(getDownloadsDir(), w-8),
		),
	)

	// Sección Fuente
	fuenteSection := renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			settingsLabelStyle.Render("Fuente de yt-dlp:"),
			"  "+stableRadio+" "+selectedStyle.Render("1. Stable"),
			"  "+nightlyRadio+" "+selectedStyle.Render("2. Nightly"),
		),
	)

	// Sección yt-dlp
	var ytdlpStatus string
	if m.updateVersionMsg != "" {
		ytdlpStatus = truncate(m.updateVersionMsg, w-8)
	} else if m.ytDlpVersion != "" {
		ytdlpStatus = "✓ " + truncate(m.ytDlpVersion, w-8)
	} else {
		ytdlpStatus = "✗ No instalado"
	}
	ytdlpSection := renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			settingsLabelStyle.Render("yt-dlp:"),
			"  "+ytdlpStatus,
		),
	)

	// Sección ffmpeg
	var ffmpegStatus string
	if m.ffmpegStatusMsg != "" {
		ffmpegStatus = truncate(m.ffmpegStatusMsg, w-8)
	} else if m.ffmpegVersion != "" {
		ffmpegStatus = "✓ " + truncate(m.ffmpegVersion+" ("+m.ffmpegStatus+")", w-8)
	} else {
		ffmpegStatus = "✗ No encontrado"
	}
	ffmpegSection := renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			settingsLabelStyle.Render("ffmpeg:"),
			"  "+ffmpegStatus,
		),
	)

	// Hints
	hints := "1/2: fuente  ·  U: actualizar yt-dlp"
	if m.ffmpegStatus != "sistema" {
		hints += "  ·  F: descargar ffmpeg"
	}

	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(dirSection)
	b.WriteString("\n")
	b.WriteString(fuenteSection)
	b.WriteString("\n")
	b.WriteString(ytdlpSection)
	b.WriteString("\n")
	b.WriteString(ffmpegSection)
	b.WriteString("\n")
	b.WriteString(renderHint(w, hints))
	b.WriteString("\n")
	b.WriteString(renderHint(w, "C o Esc: volver  ·  Q o Ctrl+C: salir"))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) helpView() string {
	w := contentWidth(m.width)

	// Sección qué es DLP-Go
	dlpgoSection := renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			settingsLabelStyle.Render("¿Qué es DLP-Go?"),
			"",
			"  Un cliente gráfico para descargar videos y",
			"  audio desde YouTube, Instagram y otras",
			"  plataformas. Usa yt-dlp y ffmpeg por debajo.",
		),
	)

	// Sección qué es yt-dlp
	ytdlpSection := renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			settingsLabelStyle.Render("¿Qué es yt-dlp?"),
			"",
			"  yt-dlp es una herramienta de línea de comandos",
			"  para descargar videos de internet. Soporta",
			"  miles de sitios web y múltiples formatos.",
			"",
			"  Sitios soportados:",
			"  github.com/yt-dlp/yt-dlp/blob/master/supportedsites.md",
		),
	)

	// Sección qué es ffmpeg
	ffmpegSection := renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			settingsLabelStyle.Render("¿Qué es ffmpeg?"),
			"",
			"  ffmpeg convierte y edita archivos multimedia.",
			"  Se usa para unir video + audio en un solo",
			"  archivo, cambiar formatos y más.",
		),
	)

	// Sección cómo se usa
	usageSection := renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			settingsLabelStyle.Render("¿Cómo se usa?"),
			"",
			"  1. Copia la URL del video y presiona P para pegar",
			"  2. Presiona Enter para analizar",
			"  3. Elige la calidad o formato deseado",
			"  4. Espera a que termine la descarga",
			"  5. Los archivos se guardan en: Descargas/",
		),
	)

	// Sección contacto
	contactSection := renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Left,
			settingsLabelStyle.Render("Contacto y soporte"),
			"",
			"  github.com/FerBS3/app-yt-dlp-go",
		),
	)

	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(dlpgoSection)
	b.WriteString("\n")
	b.WriteString(ytdlpSection)
	b.WriteString("\n")
	b.WriteString(ffmpegSection)
	b.WriteString("\n")
	b.WriteString(usageSection)
	b.WriteString("\n")
	b.WriteString(contactSection)
	b.WriteString("\n")
	b.WriteString(renderHint(w, "Esc: volver  ·  Q o Ctrl+C: salir"))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) checkingUpdateView() string {
	w := contentWidth(m.width)
	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" Buscando actualizaciones...",
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) updatingView() string {
	w := contentWidth(m.width)
	var b strings.Builder
	b.WriteString(m.appHeader())
	b.WriteString("\n\n")
	b.WriteString(renderBorder(w,
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" Actualizando yt-dlp...",
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func formatSpeed(s string) string {
	if s == "" {
		return ""
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	switch {
	case v >= 1<<30:
		return fmt.Sprintf("%.1f GiB/s", v/(1<<30))
	case v >= 1<<20:
		return fmt.Sprintf("%.1f MiB/s", v/(1<<20))
	case v >= 1<<10:
		return fmt.Sprintf("%.1f KiB/s", v/(1<<10))
	default:
		return fmt.Sprintf("%.0f B/s", v)
	}
}

func tickSecond() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return etaTickMsg{}
	})
}

func formatDuration(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func contentWidth(windowWidth int) int {
	w := windowWidth - 16
	if w > 72 {
		w = 72
	}
	if w < 40 {
		w = 40
	}
	return w
}

func renderTitle(width int) string {
	return titleStyle.Width(width).Render(AppName)
}

func renderSubtitle(width int, text string) string {
	return subtitleStyle.Width(width).Render(text)
}

func renderHint(width int, text string) string {
	return hintStyle.Width(width).Render(text)
}

func renderBorder(width int, content string) string {
	return borderStyle.Width(width - 4).Render(content)
}
