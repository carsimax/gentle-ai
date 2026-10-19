# Fork de gentle-ai — guía de mantenimiento

Este repo es un fork de [Gentleman-Programming/gentle-ai](https://github.com/Gentleman-Programming/gentle-ai).
Este archivo explica cómo está organizado y cómo traer novedades del original sin perder los cambios propios.

## Remotes

| Remote     | URL                                                    | Uso                                   |
|------------|--------------------------------------------------------|---------------------------------------|
| `origin`   | https://github.com/carsimax/gentle-ai.git              | Mi fork (aquí se hace push)           |
| `upstream` | https://github.com/Gentleman-Programming/gentle-ai.git | Repo original (solo lectura, `no_push`) |

## Ramas

- **`main`** — espejo exacto de `upstream/main`. **Nunca se hacen commits aquí.**
- **`cmx`** — rama con mis adecuaciones. Es la rama por defecto del fork.
- **`feat/...`, `fix/...`** — ramas de trabajo que salen de `cmx` y se integran de vuelta a ella.

## Configuración en una máquina nueva

Después de clonar el fork:

```bash
git remote add upstream https://github.com/Gentleman-Programming/gentle-ai.git
git remote set-url --push upstream no_push
git fetch upstream --tags
git config rerere.enabled true      # recuerda resoluciones de conflictos
git config rerere.autoupdate true
git config alias.sync-main '!git fetch upstream --tags && git checkout main && git merge --ff-only upstream/main && git push origin main'
git config alias.latest-release '!git fetch upstream --tags -q && git tag --sort=-creatordate | head -1'
git checkout cmx
```

## Instalar gentle-ai desde este fork

```bash
./scripts/install-fork.sh    # compila e instala en ~/go/bin/gentle-ai
gentle-ai --version          # ej. 4.2.0-cmx.<commit>
gentle-ai sync               # re-aplica configs y skills a los agentes con esta versión
```

- `~/go/bin` debe estar en el `PATH` antes que cualquier otra instalación (Homebrew, install.sh).
- La versión es `<base upstream>-cmx.<versión del fork>`, por ejemplo `4.2.0-cmx.1.0.0`;
  termina en `.dirty` si había cambios sin commit.
- Las builds del fork **nunca** ofrecen la release oficial: ni al arrancar, ni en la TUI, ni con
  `gentle-ai upgrade` (`internal/update/fork.go`). Para traer novedades, pórtalas (ver abajo).

## Versión del fork

La numeración propia vive en `fork/VERSION` (`MAJOR.MINOR.PATCH`, hoy `1.0.0`) y se lee como
**cmx-1.0.0**. El binario la reporta como `4.2.0-cmx.1.0.0`:

- `4.2.0` es la base upstream: el tag `v*` más reciente en la historia de `cmx`. Mantiene la
  versión como semver válido y mayor o igual a 2.0.0, que el plugin de revisión de OpenCode exige.
- `cmx.1.0.0` es la versión del fork.

Para publicar una versión nueva: sube `fork/VERSION` (PATCH para arreglos, MINOR para
funciones, MAJOR para cambios incompatibles), haz commit en `cmx` y etiqueta ese commit
`git tag cmx-<versión>`.

## Flujo diario

```bash
git checkout cmx
git checkout -b feat/mi-cambio
# ... trabajar, commits chicos ...
git checkout cmx
git merge feat/mi-cambio
git push
```

## Tests

`go test ./...` tal cual **no sirve** aquí: `internal/cli` tiene ~1900 tests secuenciales
y revienta el timeout de 10 minutos. Usa `scripts/test-fast.sh`, que corre los paquetes en
paralelo y parte los más lentos en varios procesos (shards):

```bash
scripts/test-fast.sh --changed         # ciclo diario: paquetes cambiados vs cmx + los que los importan
scripts/test-fast.sh ./internal/cli    # paquetes concretos
scripts/test-fast.sh                   # suite completa (~5 min sin caché en M-series de 10 núcleos)
COUNT=1 scripts/test-fast.sh           # ignora la caché de tests
JOBS=6 scripts/test-fast.sh            # menos procesos si necesitas la máquina libre
```

- Cada shard tiene su propia entrada en la caché de `go test`: al repetir, solo corre lo que cambió.
  Ojo: varios tests de `internal/cli` hacen `stat` de `.git`, así que cualquier comando de git
  que reescriba el índice (incluso `git status`) invalida la caché de ese paquete.
- La suite completa ya está limitada por CPU: subir `JOBS` o los shards no la acelera más.
- Fallos conocidos solo de entorno en esta Mac: los tests de scripts de release en `internal/update`
  (piden bash 4; macOS trae 3.2) y `TestOwnershipReuseRejectsLedgerWithoutMutation` en
  `internal/components/reviewassets` (el texto del error JSON cambió en Go 1.27; `go.mod` pide 1.25).
- Si un paquete nuevo se vuelve lento, agrégalo a `shards_for()` en el script.
- El cuello de botella restante es `TestReleasedSDDAgentDigestsMatchGenerator` (~140 s, un solo test).

## Traer novedades del original (cherry-pick)

Desde `feat/macos-arm64-only`, `cmx` **ya no mergea** upstream: borró el soporte de
Windows y Linux, así que cada merge chocaría con esos archivos. Las novedades se
portan PR por PR con `cherry-pick` desde `main`:

```
upstream/main ──(ff)──▶ main ──(cherry-pick -x -m 1, por PR)──▶ cmx
```

```bash
git sync-main                          # main = upstream/main (solo fast-forward)
git checkout cmx
scripts/port-upstream.sh list          # PRs de main desde el último tag upstream-synced/*
scripts/port-upstream.sh pick <sha>... # porta los elegidos, en orden
go build ./... && scripts/test-fast.sh --changed
scripts/port-upstream.sh mark <sha>    # marca hasta dónde de main ya revisaste
git push && git push --tags
./scripts/install-fork.sh
```

`list` marca cada commit de `main` como:

| Estado    | Significado |
|-----------|-------------|
| `pending` | Falta portarlo y toca código que el fork conserva. |
| `skip`    | Solo toca rutas borradas (`fork/removed-paths.txt`); no hace falta portarlo. |
| `ported`  | `cmx` ya tiene un commit con `cherry picked from commit <sha>`. |

- Porta **por PR completo** (el merge commit), no commits sueltos: así no te
  saltas un commit del que depende otro.
- `pick` borra solo las rutas del manifiesto cuando un PR las toca o las vuelve a
  agregar. Cualquier otro conflicto se resuelve a mano y se sigue con
  `git cherry-pick --continue`.
- Si un PR agrega archivos nuevos solo para Windows o Linux con otro nombre,
  agrégalos a `fork/removed-paths.txt`.
- Antes del primer tag, `list` revisa desde el punto de corte `4acaf424` (cámbialo con `BASE=<ref>`).

Si un cherry-pick sale mal antes de terminar: `git cherry-pick --abort`.

## Reglas para minimizar conflictos

1. No editar ramas `runtime.GOOS` en archivos compartidos: el binario solo compila
   para `darwin/arm64` y quitarlas solo genera conflictos al portar.
2. Tocar lo menos posible los archivos del original; preferir archivos/paquetes nuevos.
3. Commits pequeños con mensajes claros (para saber qué es mío al resolver conflictos).
4. Portar seguido, no solo en releases grandes.
4. Si un cambio sirve a todos, proponerlo como PR al original: lo que entra upstream ya no hay que mantenerlo.

## Estado inicial

Fork creado a partir de `upstream/main` en `4acaf424` (posterior a `v4.2.0`), 2026-10-18.

## Alcance: solo macOS con Apple Silicon

El fork soporta **únicamente macOS en Apple Silicon (`darwin/arm64`)**.

- Se borraron los archivos que nunca compilan en `darwin/arm64`, los instaladores y
  workflows de Windows y Linux. La lista completa está en `fork/removed-paths.txt`.
- El binario rechaza cualquier otra plataforma al arrancar (`cmd/gentle-ai/platform_guard.go`),
  igual que `scripts/install.sh` y `scripts/install-fork.sh`.
- GoReleaser y CI generan y prueban solo `darwin/arm64` en `macos-14`.

## Alcance: solo Claude Code

Las adecuaciones de este fork son **exclusivamente para Claude Code**. No se
implementan ni se mantienen cambios para otros agentes (OpenCode, Codex, Pi, Kiro, etc.).

- El código compartido se puede tocar, pero sin romper a los demás agentes,
  para que portar cambios del original siga siendo limpio.
- El ciclo rápido de tests de desarrollo se enfoca en los paquetes que afectan a Claude Code.
