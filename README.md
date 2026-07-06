# DLP Go

Aplicacion CLI con interfaz TUI (Terminal User Interface) escrita en Go que gestiona automaticamente
[yt-dlp](https://github.com/yt-dlp/yt-dlp) (versiones stable y nightly), permite pegar una URL de
video, seleccionar preset de calidad, descargar con barra de progreso y verificar actualizaciones
de yt-dlp desde la misma terminal.

Construida con [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles) y
[Lipgloss](https://github.com/charmbracelet/lipgloss).

---

## Caracteristicas

- Gestor automatico de yt-dlp: si no encuentra el binario en el sistema ni en el directorio
  local, lo descarga automaticamente desde GitHub.
- Soporte para versiones **stable** y **nightly** de yt-dlp, seleccionables desde la pantalla
  de configuracion.
- 7 presets de calidad: 1080p, 720p, 480p, mejor calidad, audio AAC, audio Opus y mejor audio.
- **ffmpeg automatico**: si no esta disponible en el sistema, se puede descargar desde la
  configuracion (tecla `F`). Cuando esta presente, la salida es MP4.
- Visualizacion de informacion del video (titulo, canal, duracion, formato de salida) antes de
  descargar.
- Barra de progreso en vivo con velocidad y ETA durante la descarga.
- Muestra la ruta del archivo descargado al completar.
- Verificador de actualizaciones de yt-dlp via GitHub API y descarga directa desde la app.
- Cancelacion segura de descargas con Ctrl+C.
- Portatil: un solo binario que se puede copiar a otra maquina y funciona.
- Sin dependencias externas obligatorias: yt-dlp y ffmpeg se auto-descorgan si es necesario.
- Multiplataforma: Linux y Windows.

---

## Requisitos

### Para compilar (desarrolladores)

- [Go](https://go.dev/dl/) version 1.26 o superior.
- Conexion a internet para descargar las dependencias (`go mod download`).

### Para ejecutar (usuarios finales)

- Ninguno. Solo el binario compilado. La aplicacion descarga y gestiona yt-dlp por si sola,
  y opcionalmente ffmpeg.

---

## Compilacion y ejecucion

### Ejecutar en modo desarrollo

```bash
go run .
```

### Compilar binario para Linux

```bash
go build -o DLP-Go .
```

### Compilar con optimizaciones y generar release

Usar el script incluido:

```bash
./build.sh
```

Esto compila para Linux amd64 y Windows amd64 con `-ldflags="-s -w"`, empaqueta cada binario
en un ZIP y los deja en la carpeta `release/`.

### Verificar el codigo

```bash
go vet ./...
```

### Compilacion cruzada para Windows

```bash
GOOS=windows GOARCH=amd64 go build -o DLP-Go.exe .
```

---

## Como usar

Al iniciar la aplicacion, esta verifica si yt-dlp esta disponible. Si no lo encuentra, lo
descarga automaticamente.

### Flujo principal

1.  Pantalla de inicio: se muestra un campo de texto para pegar la URL del video.
    Tambien se ve la ruta donde se guardaran las descargas.
2.  Ingresar una URL (debe comenzar con `http://` o `https://`) y presionar Enter.
3.  La app analiza el video con `--dump-json` y muestra titulo, canal, duracion y formato
    de salida (MP4 si hay ffmpeg, formato original si no).
4.  Seleccionar un preset de calidad con las flechas y presionar Enter.
5.  La descarga comienza con barra de progreso, velocidad y ETA.
6.  Al finalizar se muestra un mensaje de exito o error con la ruta del archivo.

### Pantalla de configuracion

Presionar `C` desde la pantalla principal o desde la pantalla de resultado para abrir la
configuracion. Alli se puede:

- Elegir fuente **stable** (tecla `1`) o **nightly** (tecla `2`).
- Verificar y actualizar yt-dlp con la tecla `U`.
- Descargar ffmpeg estatico con la tecla `F` (solo si no esta disponible en el sistema).

### Atajos de teclado

| Tecla               | Accion                                      |
|---------------------|---------------------------------------------|
| Enter               | Confirmar URL, iniciar descarga, continuar  |
| C                   | Abrir/cerrar pantalla de configuracion      |
| P                   | Pegar URL desde el portapapeles             |
| 1                   | Seleccionar fuente stable (en configuracion)|
| 2                   | Seleccionar fuente nightly (en configuracion)|
| U                   | Buscar y actualizar yt-dlp (en configuracion)|
| F                   | Descargar ffmpeg estatico (en configuracion)|
| Flecha arriba / k   | Navegar hacia arriba en la lista de calidad |
| Flecha abajo / j    | Navegar hacia abajo en la lista de calidad  |
| Esc                 | Volver desde configuracion                  |
| Ctrl+C / q          | Salir de la aplicacion (cancela descargas)  |

### Presets de calidad disponibles

1.  Mejor calidad (1080p)
2.  720p
3.  480p
4.  Mejor calidad (sin limite de resolucion)
5.  Audio AAC (m4a)
6.  Audio Opus
7.  Mejor audio

Los presets de video y audio se muestran separados visualmente en la lista de seleccion.

---

## Estructura de directorios

```
DLP-Go/
  |-- main.go               Punto de entrada de la aplicacion
  |-- tui.go                Modelo Bubble Tea, vistas y estilos
  |-- ytdlp_types.go        Tipos, configuracion, GitHub API, descargas
  |-- ytdlp.go              Gestor del subproceso yt-dlp con progreso
  |-- yt-dlp/
  |   |-- config.json       Configuracion: {"source":"nightly"} o {"source":"stable"}
  |   |-- nightly/
  |   |   |-- yt-dlp        Binario nightly de yt-dlp
  |   |-- stable/
  |   |   |-- yt-dlp        Binario stable de yt-dlp
  |   |-- ffmpeg/
  |       |-- ffmpeg        Binario estatico de ffmpeg
  |-- Descargas/            Videos descargados
  |-- release/              Zips de distribucion (generados por build.sh)
  |-- build.sh              Script de compilacion y empaquetado
  |-- go.mod / go.sum       Modulo Go y dependencias
```

### Notas sobre el directorio de trabajo

La aplicacion usa el directorio actual de trabajo (`os.Getwd()`) como raiz. Alli crea las
carpetas `yt-dlp/` (con subcarpetas `nightly/`, `stable/` y `ffmpeg/`) y `Descargas/`.
El binario principal de la app se encuentra en la raiz del proyecto.

---

## Portabilidad

- El binario compilado se puede copiar a cualquier maquina del mismo SO y arquitectura
  (Linux amd64, Windows amd64) y funcionara sin instalar nada mas.
- La primera vez que se ejecuta, la app crea los directorios necesarios y descarga la
  version de yt-dlp seleccionada si no esta presente.
- ffmpeg tambien se puede descargar bajo demanda desde la configuracion (tecla `F`).
  Si el sistema ya tiene `ffmpeg` en el PATH, la app lo usara directamente.
- No requiere Python ni ninguna otra dependencia externa.

---

## Compatibilidad con Windows

Para compilar para Windows desde Linux:

```bash
GOOS=windows GOARCH=amd64 go build -o DLP-Go.exe .
```

El binario `.exe` funciona de la misma manera: crea los directorios, descarga el binario
de yt-dlp para Windows (con extension `.exe`) y procede con la descarga de videos.

---

## Arquitectura interna

La aplicacion utiliza una maquina de estados con 10 estados:

| Estado               | Descripcion                                      |
|----------------------|--------------------------------------------------|
| checkingState        | Verifica si yt-dlp existe al inicio              |
| downloadingBinState  | Descarga automatica de yt-dlp o ffmpeg           |
| inputState           | Pantalla principal: pegar URL                    |
| loadingState         | Obtiene informacion del video via --dump-json    |
| selectState          | Seleccion de preset de calidad                   |
| downloadingState     | Descarga con barra de progreso                   |
| doneState            | Mensaje de exito o error                         |
| settingsState        | Pantalla de configuracion                        |
| checkingUpdateState  | Consulta de version via GitHub API               |
| updatingState        | Descarga de nueva version de yt-dlp              |

### Componentes del codigo

- **main.go** — Inicializa la aplicacion, crea los directorios y lanza el programa Bubble Tea
  con pantalla alternativa (`tea.WithAltScreen()`).
- **tui.go** — Define el modelo, el bucle de actualizacion (`Update`), todas las vistas
  (`View`) y los estilos con Lipgloss. Maneja los atajos de teclado y las transiciones de
  estado.
- **ytdlp_types.go** — Tipos de datos (VideoInfo, QualityPreset, mensajes), carga/guardado
  de configuracion, llamadas a la API de GitHub para obtener versiones, descarga de archivos,
  deteccion de ffmpeg, y funciones auxiliares (rutas, directorios).
- **ytdlp.go** — Ejecuta yt-dlp como subproceso con `exec.CommandContext`, canaliza la salida
  JSON con el progreso de descarga desde stderr y la envia como mensajes a Bubble Tea para
  actualizar la barra de progreso.

---

## URLs de referencia

- Descarga nightly: https://github.com/yt-dlp/yt-dlp-nightly-builds/releases/latest/download/yt-dlp
- Descarga stable: https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp
- API GitHub nightly: https://api.github.com/repos/yt-dlp/yt-dlp-nightly-builds/releases/latest
- API GitHub stable: https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest
