---
description: Agrupa cambios sin stage por función y crea commits convencionales en español. Pregunta tipo, scope y descripción para cada grupo. Nunca comitea sin confirmación del usuario.
---

# Supercommit

Creá commits convencionales siguiendo la especificación https://www.conventionalcommits.org/en/v1.0.0/ pero con mensajes en español.

## Formato del commit

```
tipo(ámbito): descripción en español en imperativo
```

### Tipos permitidos

| Tipo       | Uso                                                  |
|------------|------------------------------------------------------|
| `feat`     | Nueva funcionalidad                                  |
| `fix`      | Corrección de bug                                    |
| `refactor` | Cambio interno sin agregar funcionalidad ni corregir bugs |
| `docs`     | Cambios en documentación                             |
| `chore`    | Mantenimiento, build, dependencias                   |
| `perf`     | Mejora de rendimiento                                |
| `style`    | Formato, estilos (no confundir con CSS)              |
| `test`     | Agregar o corregir tests                             |

### Ámbitos usados en este proyecto

- `ffmpeg` — descarga, extracción y detección de ffmpeg
- `download` — subprocess de yt-dlp, pipes, progreso
- `view` — vistas de Bubble Tea, styles
- `settings` — pantalla de configuración, teclas
- `types` — tipos, helpers, config load/save
- `cli` — entry point, main.go

## Flujo

1. Ejecutá `git status` y `git diff` para listar los cambios sin stage
2. Agrupá los archivos por ámbito lógico (ej: cambios en ytdlp.go van a `download`, cambios en tui.go van a `view`)
3. Para cada grupo, **preguntale al usuario**:
   - Tipo (de la tabla de arriba)
   - Ámbito (de la tabla del proyecto, o uno nuevo si corresponde)
   - Descripción corta en imperativo y español
4. Hacé `git add <archivos del grupo>`
5. Hacé `git commit -m "tipo(ámbito): descripción"`
6. Repetí para cada grupo lógico

## Reglas

- **Nunca comitees sin preguntar primero**.
- Si un archivo toca múltiples ámbitos, preguntale al usuario cómo dividirlo.
- Si el usuario no especifica un ámbito, inferilo del archivo.
- Los mensajes van **siempre en español**, en imperativo.
- Si hay cambios sin seguimiento (untracked), preguntá si incluirlos.
- No hagas push. Solo commit local.
