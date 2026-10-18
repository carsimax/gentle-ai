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
- **`mi-version`** — rama con mis adecuaciones. Es la rama por defecto del fork.
- **`feat/...`, `fix/...`** — ramas de trabajo que salen de `mi-version` y se integran de vuelta a ella.

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
git checkout mi-version
```

## Flujo diario

```bash
git checkout mi-version
git checkout -b feat/mi-cambio
# ... trabajar, commits chicos ...
git checkout mi-version
git merge feat/mi-cambio
git push
```

## Traer una release del original

```bash
git sync-main                # actualiza main con upstream y la sube a mi fork
git latest-release           # muestra el tag más reciente (ej. v4.2.0)
git checkout mi-version
git merge v4.2.0             # o: git merge upstream/main para lo más reciente
# resolver conflictos si aparecen, luego:
go test ./...
git push
```

**Se usa `merge`, no `rebase`**: `mi-version` está publicada, y el rebase reescribe la
historia (obliga a `push --force`) y hace resolver los conflictos commit por commit.

Si un merge sale mal antes de hacer commit: `git merge --abort`.

## Reglas para minimizar conflictos

1. Tocar lo menos posible los archivos del original; preferir archivos/paquetes nuevos.
2. Commits pequeños con mensajes claros (para saber qué es mío al resolver conflictos).
3. Sincronizar seguido, no solo en releases grandes.
4. Si un cambio sirve a todos, proponerlo como PR al original: lo que entra upstream ya no hay que mantenerlo.

## Estado inicial

Fork creado a partir de `upstream/main` en `4acaf424` (posterior a `v4.2.0`), 2026-10-18.
