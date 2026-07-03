package main

import (
	"context"
	"fmt"
	"math"
	"strings"

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
)

var (
	purple   = lipgloss.Color("#BB66FF")
	cyan     = lipgloss.Color("#66DDEE")
	green    = lipgloss.Color("#66EE88")
	red      = lipgloss.Color("#FF5577")
	gray     = lipgloss.Color("#888888")
	white    = lipgloss.Color("#EEEEEE")
	orange   = lipgloss.Color("#FFAA44")
	darkGray = lipgloss.Color("#444444")

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(purple).
			Align(lipgloss.Center).
			Width(60)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Align(lipgloss.Center).
			Width(60)

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
			Align(lipgloss.Center).
			Width(60)

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

type model struct {
	state    state
	width    int
	height   int

	textInput textinput.Model
	spinner   spinner.Model
	progress  progress.Model

	qualityIndex int
	dlPercent    float64
	dlSpeed      string
	dlETA        string

	dlChan   chan tea.Msg
	dlCancel context.CancelFunc

	videoInfo *VideoInfo

	err        error
	successMsg string

	selectedSource      string
	updateCurrentVersion string
	updateLatestVersion  string
	updateVersionMsg     string
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "https://youtube.com/watch?v=..."
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

		case "p", "P":
			if m.state == inputState {
				text, err := clipboard.ReadAll()
				if err == nil && text != "" {
					m.textInput.SetValue(text)
				}
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

		case "d", "D":
			if m.state == settingsState && m.updateVersionMsg != "" && strings.HasPrefix(m.updateVersionMsg, "Actualización disponible") {
				m.state = updatingState
				binPath := getYtDlpPath(m.selectedSource)
				return m, tea.Batch(performUpdateCmd(binPath), m.spinner.Tick)
			}

		case "enter":
			switch m.state {
			case inputState:
				url := strings.TrimSpace(m.textInput.Value())
				if url == "" {
					return m, nil
				}
				if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
					m.err = fmt.Errorf("la URL debe comenzar con http:// o https://")
					m.state = doneState
					return m, nil
				}
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
				m.textInput.SetValue("")
				m.state = inputState
				m.err = nil
				m.successMsg = ""
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
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

	case ytDlpFoundMsg:
		if msg.err != nil {
			m.state = doneState
			m.err = msg.err
		} else {
			m.state = inputState
		}
		return m, nil

	case ytDlpNeededMsg:
		m.state = downloadingBinState
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
		return m, listenDownload(m.dlChan)

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
			m.err = nil
		}
		return m, nil

	case nil:
		m.state = doneState
		m.err = fmt.Errorf("Error inesperado en la descarga")
		return m, nil

	case spinner.TickMsg:
		if m.state == loadingState || m.state == checkingState || m.state == downloadingBinState || m.state == checkingUpdateState || m.state == updatingState {
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
		m.state = settingsState
		if msg.err != nil {
			m.updateVersionMsg = errorStyle.Render("Error: " + msg.err.Error())
		} else if msg.currentVersion == "" {
			m.updateVersionMsg = "Versión actual: no instalado\nVersión más reciente: " + msg.latestVersion +
				"\n\nPresiona D para descargar la última versión"
		} else if msg.currentVersion != msg.latestVersion {
			m.updateVersionMsg = fmt.Sprintf("Versión actual: %s\nVersión más reciente: %s\n\nPresiona D para actualizar",
				msg.currentVersion, msg.latestVersion)
		} else {
			m.updateVersionMsg = fmt.Sprintf("Ya tenés la versión más reciente (%s) ✓", msg.currentVersion)
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
	case checkingUpdateState:
		return m.checkingUpdateView()
	case updatingState:
		return m.updatingView()
	default:
		return ""
	}
}

func (m model) checkingView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("yt-dlp-go-prueba"))
	b.WriteString("\n\n")
	b.WriteString(borderStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" Verificando yt-dlp...",
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) downloadingBinView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("yt-dlp-go-prueba"))
	b.WriteString("\n\n")
	b.WriteString(borderStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" Descargando yt-dlp "+m.selectedSource+"...",
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) inputView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("yt-dlp-go-prueba"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Descarga videos desde la terminal"))
	if existingCfg, _ := loadConfig(); existingCfg != "" {
		b.WriteString("\n")
		b.WriteString(versionStyle.Render("["+existingCfg+"]"))
	}
	b.WriteString("\n\n")
	b.WriteString(borderStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			"Pega la URL del video:",
			"",
			m.textInput.View(),
			"",
			hintStyle.Render("Enter para analizar  ·  P para pegar  ·  C para configurar  ·  Ctrl+C para salir"),
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) loadingView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("yt-dlp-go-prueba"))
	b.WriteString("\n\n")
	b.WriteString(borderStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" Analizando video...",
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) selectView() string {
	info := m.videoInfo
	dur := formatDuration(int(math.Round(info.Duration)))

	infoBlock := lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Render("Título:  ")+valueStyle.Render(truncate(info.Title, 50)),
		labelStyle.Render("Canal:   ")+valueStyle.Render(info.Channel),
		labelStyle.Render("Dur.:    ")+valueStyle.Render(dur),
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
			items = append(items, subtitleStyle.Render("Audio sin conversión:"))
		}
		if i == m.qualityIndex {
			items = append(items, selectedStyle.Render("▸ "+q.Label))
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
		hintStyle.Render("Enter para descargar  ·  Ctrl+C para salir"),
	)

	var b strings.Builder
	b.WriteString(titleStyle.Render("yt-dlp-go-prueba"))
	b.WriteString("\n\n")
	b.WriteString(borderStyle.Render(content))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) downloadingView() string {
	bar := m.progress.ViewAs(m.dlPercent / 100)
	pct := progressPercentStyle.Render(fmt.Sprintf("%.1f%%", m.dlPercent))
	info := ""
	if m.dlSpeed != "" {
		info += progressInfoStyle.Render("Vel: "+m.dlSpeed) + "  "
	}
	if m.dlETA != "" {
		info += progressInfoStyle.Render("ETA: "+m.dlETA+"s")
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		"Descargando...",
		"",
		bar,
		pct,
		"",
		info,
		"",
		hintStyle.Render("Ctrl+C para cancelar"),
	)

	var b strings.Builder
	b.WriteString(titleStyle.Render("yt-dlp-go-prueba"))
	b.WriteString("\n\n")
	b.WriteString(borderStyle.Render(content))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) doneView() string {
	var icon, title string
	if m.err != nil {
		icon = errorStyle.Render("✗")
		title = errorStyle.Render("Error")
	} else {
		icon = successStyle.Render("✓")
		title = successStyle.Render("¡Completado!")
	}

	lines := []string{icon + "  " + title, ""}
	if m.err != nil {
		lines = append(lines, m.err.Error())
	} else if m.successMsg != "" {
		lines = append(lines, m.successMsg)
	}
	lines = append(lines, "")
	lines = append(lines, hintStyle.Render("Enter para continuar  ·  C para configurar  ·  Ctrl+C para salir"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	var b strings.Builder
	b.WriteString(titleStyle.Render("yt-dlp-go-prueba"))
	b.WriteString("\n\n")
	b.WriteString(borderStyle.Render(content))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) settingsView() string {
	stableRadio := "  "
	nightlyRadio := "  "
	if m.selectedSource == "stable" {
		stableRadio = "◉ "
	} else {
		stableRadio = "○ "
	}
	if m.selectedSource == "nightly" {
		nightlyRadio = "◉ "
	} else {
		nightlyRadio = "○ "
	}

	var lines []string
	lines = append(lines, titleStyle.Render("Configuración"))
	lines = append(lines, "")
	lines = append(lines, settingsLabelStyle.Render("Fuente:"))
	lines = append(lines, "  "+stableRadio+selectedStyle.Render("1. Stable"))
	lines = append(lines, "  "+nightlyRadio+selectedStyle.Render("2. Nightly"))
	lines = append(lines, "")
	lines = append(lines, settingsLabelStyle.Render("Versión:"))
	if m.updateVersionMsg != "" {
		lines = append(lines, "  "+m.updateVersionMsg)
	} else {
		ver, _ := getYtDlpVersion(getYtDlpPath(m.selectedSource))
		if ver != "" {
			lines = append(lines, "  "+ver)
		} else {
			lines = append(lines, "  No instalado")
		}
	}
	lines = append(lines, "")
	lines = append(lines, hintStyle.Render("1/2: fuente  ·  U: buscar actualizaciones  ·  D: descargar/actualizar"))
	lines = append(lines, hintStyle.Render("C o Esc: volver  ·  Ctrl+C: salir"))

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)

	var b strings.Builder
	b.WriteString(borderStyle.Render(content))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) checkingUpdateView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("yt-dlp-go-prueba"))
	b.WriteString("\n\n")
	b.WriteString(borderStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" Buscando actualizaciones...",
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m model) updatingView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("yt-dlp-go-prueba"))
	b.WriteString("\n\n")
	b.WriteString(borderStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View()+" Actualizando yt-dlp...",
		),
	))
	b.WriteString("\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
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
