# I-01 Foundation — Progress Log

## Estado general

Current step: none (awaiting authorization)
Last completed step: 15
Current release: unreleased

## Registro

## Step 00 — Protocolo SDD y memoria persistente

Status: completed
Date: 2026-08-12
Commit: f3a4de7
Release: unreleased

### Delivered
- Repositorio Git inicializado en la rama `main`.
- Protocolo operativo en `AGENTS.md` y memoria persistente en `PLAN.md` y `PROGRESS.md`.

### Decisions
- Cada paso requiere autorización explícita y se cierra con documentación y commits coherentes.
- El commit de entrega se registra mediante un segundo commit documental.

### Verification
- `git status --short --branch`
- `git diff --cached --check`
- Existencia y revisión de `AGENTS.md`, `PLAN.md` y `PROGRESS.md`.
- Ausencia de código funcional fuera del alcance del Paso 0.

### Notes for next session
- Al cerrar este registro, el Paso 1 era el siguiente paso pendiente de autorización.

## Step 01 — Inicialización Go e identidad básica

Status: completed
Date: 2026-08-12
Commit: 761d847
Release: unreleased

### Delivered
- Módulo Go compilable, binario `kelyro` y metadatos de versión inyectables mediante `-ldflags`.
- Soporte mínimo para `--help` y `--version`, con pruebas unitarias y archivos base del repositorio.

### Decisions
- `github.com/mishaaac/kelyro` es la ruta canónica del módulo.
- El bootstrap usa solo la biblioteca estándar; el router CLI completo queda reservado para el Paso 3.
- No se creó tag: este estado continúa como prerelease no distribuida.

### Verification
- `go test ./...`
- `go vet ./...`
- `go build ./cmd/kelyro`
- `./kelyro --version`
- `./kelyro --help`

### Notes for next session
- El Paso 2 no se ha iniciado y requiere autorización explícita.

## Step 02 — Arquitectura y contratos Foundation

Status: completed
Date: 2026-08-12
Commit: 4a1175a
Release: unreleased

### Delivered
- Contratos neutrales para plataforma, workspace, configuración, estado, secrets, artefactos y auditoría.
- Límites de paquetes Foundation, fakes de compilación y documentación de arquitectura y dependencias.

### Decisions
- El core expone tipos propios y datos opacos; frameworks, SQLite, servicios externos y operaciones del OS quedan detrás de adaptadores.
- Los paquetes reservados solo declaran responsabilidad; su funcionalidad continúa aplazada a los pasos correspondientes.

### Verification
- `GOCACHE=/tmp/kelyro-step2-gocache GOMODCACHE=/tmp/kelyro-step2-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step2-gocache GOMODCACHE=/tmp/kelyro-step2-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step2-gocache GOMODCACHE=/tmp/kelyro-step2-modcache go list ./...`
- Revisión de imports directos y `git diff --check`.

### Notes for next session
- El Paso 3 está autorizado y es el siguiente paso a ejecutar.

## Step 03 — CLI base y router de comandos

Status: completed
Date: 2026-08-12
Commit: 9b9130c
Release: unreleased

### Delivered
- Router CLI inyectable con TUI por defecto y comandos `help`, `version`, `init`, `doctor`, `config`, `status` y `open`.
- Flags globales reservados, códigos de salida, errores uniformes y placeholders explícitos detrás de un servicio de aplicación.
- Pruebas de parsing, ayuda/versión, dispatch, workspace override, errores, quiet mode y cancelación.

### Decisions
- Se usó la biblioteca estándar: el alcance actual no justifica añadir Cobra como dependencia externa.
- La CLI solo parsea y renderiza; las acciones Foundation se despachan mediante `app.FoundationService`.

### Verification
- `GOCACHE=/tmp/kelyro-step3-gocache GOMODCACHE=/tmp/kelyro-step3-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step3-gocache GOMODCACHE=/tmp/kelyro-step3-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step3-gocache GOMODCACHE=/tmp/kelyro-step3-modcache go build -o /tmp/kelyro-step3 ./cmd/kelyro`
- `/tmp/kelyro-step3 --help`
- `/tmp/kelyro-step3 version`
- `git diff --check`

### Notes for next session
- El Paso 4 no se ha iniciado y requiere autorización explícita.

## Step 04 — Capa multiplataforma de filesystem y rutas

Status: completed
Date: 2026-08-12
Commit: ec7619d
Release: unreleased

### Delivered
- Helpers absolutos y normalizados para directorios estándar, configuración/cache globales y rutas internas del workspace.
- Pruebas tabulares para rutas relativas, espacios, limpieza, case preservation y semántica de drive letters en Windows.
- Convenciones nativas de Windows, macOS y sistemas Unix documentadas.

### Decisions
- La composición usa exclusivamente `os` y `path/filepath`; no se añadió ninguna dependencia externa.
- Los helpers solo resuelven rutas: la creación, discovery y validación del workspace permanecen reservadas al Paso 5.
- El estado se reserva bajo `.kelyro/state`, la base de datos en `.kelyro/learning.db` y los backups en `.kelyro/backups`.

### Verification
- `GOCACHE=/tmp/kelyro-step4-gocache GOMODCACHE=/tmp/kelyro-step4-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step4-gocache GOMODCACHE=/tmp/kelyro-step4-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step4-gocache GOMODCACHE=/tmp/kelyro-step4-modcache go build -o /tmp/kelyro-step4 ./cmd/kelyro`
- Cross-compilation of tests for `windows/amd64` and `darwin/amd64` using `go test -exec=/bin/true ./...`.
- `git diff --check`

### Notes for next session
- El Paso 5 no se ha iniciado y requiere autorización explícita.

## Step 05 — Ciclo de vida local del workspace

Status: completed
Date: 2026-08-12
Commit: 7141dfc
Release: unreleased

### Delivered
- Creación idempotente de `.kelyro/`, metadata estable y `LEARNING.md` human-readable mediante `kelyro init`.
- Discovery ascendente, validación de estructura, protección contra nesting accidental y rollback ante fallos de inicialización.
- Adaptador filesystem aislado, integración de aplicación/CLI y reglas explícitas de ownership.

### Decisions
- `.kelyro/` es machine-owned; `LEARNING.md` y los demás archivos visibles son student-owned y no se sobrescriben automáticamente.
- La excepción para workspaces anidados requiere `--allow-nested`; no se añadieron dependencias externas ni se creó la base de datos reservada.
- La inicialización publica internals preparados en staging y revierte todo artefacto nuevo ante errores reportados.

### Verification
- `GOCACHE=/tmp/kelyro-step5-gocache GOMODCACHE=/tmp/kelyro-step5-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step5-gocache GOMODCACHE=/tmp/kelyro-step5-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step5-gocache GOMODCACHE=/tmp/kelyro-step5-modcache go build -o /tmp/kelyro-step5 ./cmd/kelyro`
- Cross-compilation of tests for `windows/amd64` and `darwin/amd64` using `go test -exec=/bin/true ./...`.
- Prueba CLI real de init nuevo/repetido, estructura, metadata, ruta con espacios y nesting explícito.
- `git diff --check`

### Notes for next session
- El Paso 6 no se ha iniciado y requiere autorización explícita.

## Step 06 — Configuración global y por proyecto

Status: completed
Date: 2026-08-12
Commit: 251d6ab
Release: unreleased

### Delivered
- Esquema tipado con defaults seguros, validación estricta y precedencia defaults/global/project/CLI.
- Store TOML global y por workspace con versión de esquema, escritura atómica y preservación de comentarios en `config set`.
- CLI funcional para `config show`, `path`, `get` y `set`, con selección `--global`/`--project` y metadata para un wizard futuro.

### Decisions
- Se implementó el subconjunto TOML escalar requerido usando la biblioteca estándar; no se añadió una dependencia externa.
- Sin alcance explícito, las lecturas incluyen el workspace descubierto y las escrituras usan proyecto dentro de uno o global fuera de él.
- El guardado masivo canonicaliza TOML y no conserva comentarios; las actualizaciones normales de una clave sí los preservan.

### Verification
- `GOCACHE=/tmp/kelyro-step6-gocache GOMODCACHE=/tmp/kelyro-step6-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step6-gocache GOMODCACHE=/tmp/kelyro-step6-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step6-race-gocache GOMODCACHE=/tmp/kelyro-step6-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step6-gocache GOMODCACHE=/tmp/kelyro-step6-modcache go build -o /tmp/kelyro-step6 ./cmd/kelyro`
- Cross-compilation of tests for `windows/amd64` and `darwin/amd64` using `go test -exec=/bin/true ./...`.
- Prueba CLI real de alcances global/project, precedencia, `show`, `path`, `get` y `set` en rutas aisladas.
- `git diff --check`

### Notes for next session
- El Paso 7 no se ha iniciado y requiere autorización explícita.

## Step 07 — Almacenamiento seguro de secretos

Status: completed
Date: 2026-08-12
Commit: a411cb7
Release: unreleased

### Delivered
- Contrato sustituible con estado seguro, redacción y adaptadores para variables de entorno, Secret Service, macOS Keychain y Windows Credential Manager.
- CLI `secrets status`, `set` y `delete`, entrada sin eco y representación sin valores en `config show`.
- Fallback accionable `KELYRO_SECRET_<NAME>` para Linux/headless y pruebas de no serialización ni exposición accidental.

### Decisions
- Las variables de entorno tienen precedencia y nunca se copian; `set` y `delete` operan únicamente sobre el keychain nativo.
- El índice de nombres se guarda dentro del keychain, contiene solo referencias y permite enumerar estado sin persistir valores en archivos.
- Se usó únicamente la biblioteca estándar; los backends y la terminal se aíslan por sistema operativo.

### Verification
- `GOCACHE=/tmp/kelyro-step7-gocache GOMODCACHE=/tmp/kelyro-step7-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step7-race-gocache GOMODCACHE=/tmp/kelyro-step7-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step7-gocache GOMODCACHE=/tmp/kelyro-step7-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step7-gocache GOMODCACHE=/tmp/kelyro-step7-modcache go build -o /tmp/kelyro-step7 ./cmd/kelyro`
- Cross-compilation of tests for `windows/amd64` and `darwin/amd64` using `go test -exec=/bin/true ./...`.
- Prueba CLI real de estado headless y fallback por entorno sin renderizar el valor.
- `git diff --check`

### Notes for next session
- El Paso 8 no se ha iniciado y requiere autorización explícita.

## Step 08 — SQLite, repositorios internos y migraciones

Status: completed
Date: 2026-08-12
Commit: ab46653
Release: unreleased

### Delivered
- Base `.kelyro/learning.db` con apertura/cierre explícitos, timeouts, foreign keys, chequeo de integridad y schema Foundation versionado.
- Runner incremental transaccional con checksums, rollback seguro, diagnóstico de migration y bloqueo de cambios destructivos sin backup.
- Repositorios neutrales para metadata, estado, índice de artifacts y auditoría, incluyendo unidad de trabajo transaccional.

### Decisions
- `modernc.org/sqlite v1.45.0` es el único módulo directo: provee SQLite sin CGO y conserva Go 1.24.
- La migration inicial solo crea tablas Foundation; los modelos educativos continúan fuera de alcance.
- El backup concreto queda para el Paso 17; hasta entonces una migration destructiva falla si no recibe y completa un callback de backup.

### Verification
- `GOCACHE=/tmp/kelyro-step8-gocache GOMODCACHE=/tmp/kelyro-step8-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step8-race-gocache GOMODCACHE=/tmp/kelyro-step8-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step8-vet-gocache GOMODCACHE=/tmp/kelyro-step8-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step8-build-gocache GOMODCACHE=/tmp/kelyro-step8-modcache go build -o /tmp/kelyro-step8 ./cmd/kelyro`
- Cross-compilation of tests for `windows/amd64` and `darwin/amd64` with `CGO_ENABLED=0` using `go test -exec=/bin/true ./...`.
- `go mod verify`
- `git diff --check`

### Notes for next session
- El Paso 9 no se ha iniciado y requiere autorización explícita.

## Step 09 — Ownership, integridad y sandbox del workspace

Status: completed
Date: 2026-08-12
Commit: 4d7cdc4
Release: unreleased

### Delivered
- Clasificación obligatoria y artifact index con creador, SHA-256, tiempos de generación y versión esperada persistidos mediante una migration incremental.
- Escritura atómica multiplataforma que protege contenido student-owned, no indexado o modificado externamente sin sobrescribirlo.
- Sandbox de rutas relativas con bloqueo de traversal, rutas absolutas y escapes por symlink.

### Decisions
- Un artifact human-readable existente sin índice requiere una decisión explícita; un hash divergente devuelve conflicto y conserva tanto archivo como metadata.
- Las rutas del índice se guardan normalizadas con separadores portables y los archivos machine-owned se limitan a `.kelyro/`.
- No se añadieron dependencias externas ni se generaron todavía los Markdown o ejercicios reservados para pasos posteriores.

### Verification
- `GOCACHE=/tmp/kelyro-step9-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step9-race-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step9-vet-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step9-build-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go build -o /tmp/kelyro-step9 ./cmd/kelyro`
- Cross-compilation of tests for `windows/amd64` and `darwin/amd64` with `CGO_ENABLED=0` using `go test -exec=/bin/true ./...`.
- `go mod verify`
- `git diff --check`

### Notes for next session
- El Paso 10 no se ha iniciado y requiere autorización explícita.

## Step 10 — Artefactos Markdown humanos y roadmap placeholder

Status: completed
Date: 2026-08-12
Commit: 3494fb3
Release: unreleased

### Delivered
- Generadores puros y golden tests para `LEARNING.md` y `00-roadmap/ROADMAP.md`, con UTF-8 y saltos LF consistentes.
- Integración de `kelyro init` con escritura atómica e índice de integridad SQLite desde la primera generación.
- Regeneración idempotente que conserva documentos modificados externamente y propaga el conflicto sin sobrescribirlos.

### Decisions
- El adaptador de workspace crea solo la estructura machine-owned; la aplicación coordina los documentos mediante contratos neutrales de artifact store.
- Los templates usan un modelo humano mínimo, versiones explícitas y ningún frontmatter o estado interno serializado.
- No se añadieron dependencias externas; el factory compone los adaptadores existentes de filesystem y SQLite.

### Verification
- `GOCACHE=/tmp/kelyro-step10-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step10-race-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step10-vet-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step10-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go build -o /tmp/kelyro-step10 ./cmd/kelyro`
- Cross-compilation of tests for `windows/amd64` and `darwin/amd64` with `CGO_ENABLED=0` using `go test -exec=/bin/true ./...`.
- Prueba CLI real de creación y regeneración idempotente en una ruta con espacios.
- `GOCACHE=/tmp/kelyro-step10-gocache go mod verify`
- `git diff --check`

### Notes for next session
- El Paso 11 no se ha iniciado y requiere autorización explícita.

## Step 11 — Detección de editores y apertura segura

Status: completed
Date: 2026-08-12
Commit: 7c9715f
Release: unreleased

### Delivered
- Detección configurable y automática de VS Code, Neovim, Vim, Zed y Cursor, con fallback nativo para Linux, macOS y Windows.
- CLI funcional `kelyro open` y `kelyro open roadmap`, con discovery del workspace, configuración por capas y paths con espacios.
- Contrato sustituible y construcción testeable de procesos con ejecutable y argumentos separados, además de `editor.prompt` para el futuro flujo TUI opcional.

### Decisions
- `editor.command` acepta un único nombre o path de ejecutable; no interpreta argumentos ni strings de shell.
- Un editor configurado pero ausente produce un error accionable; la detección y el fallback sólo aplican cuando no existe configuración explícita.
- No se añadieron dependencias externas ni se implementó la TUI reservada al Paso 12.

### Verification
- `GOCACHE=/tmp/kelyro-step11-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step11-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step11-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step11-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go build -o /tmp/kelyro-step11 ./cmd/kelyro`
- Cross-compilation of tests for `windows/amd64` and `darwin/amd64` with `CGO_ENABLED=0` using `go test -exec=/bin/true ./...`.
- Prueba CLI real de `open` y `open roadmap` en un workspace con espacios usando un ejecutable inocuo configurado.
- `GOCACHE=/tmp/kelyro-step11-verify-gocache go mod verify`
- `git diff --check`

### Notes for next session
- El Paso 12 no se ha iniciado y requiere autorización explícita.

## Step 12 — TUI Foundation con Bubble Tea

Status: completed
Date: 2026-08-12
Commit: 365f0f9
Release: unreleased

### Delivered
- TUI real con Home, Doctor, Config y Roadmap, navegación visible, resize y estados loading/error/empty.
- Snapshot Foundation tipado, diagnóstico parcial y wizard mínimo de configuración mediante servicios de aplicación.
- Integración CLI por defecto con lifecycle de terminal, `NO_COLOR`/`--no-color`, recovery y snapshots por anchura.

### Decisions
- Bubble Tea v1.3.10 y Lip Gloss v1.1.0 quedan confinados a `internal/tui`; Bubbles no es necesario para las pantallas actuales.
- `Update` solo gestiona estado de presentación; discovery, configuración, salud SQLite y apertura del roadmap permanecen en aplicación/adaptadores.
- El wizard alterna opciones acotadas; valores escalares libres continúan editándose con `kelyro config set`.

### Verification
- `GOCACHE=/tmp/kelyro-step12-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step12-race-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step12-vet-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step12-build-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go build -o /tmp/kelyro-step12 ./cmd/kelyro`
- Cross-compilation de tests para `windows/amd64` y `darwin/amd64` con `CGO_ENABLED=0` y `go test -vet=off -exec=/bin/true ./...`.
- Prueba TUI real en PTY de Home, salida con `q`, restauración del terminal y ausencia de color bajo `NO_COLOR`.
- `go mod tidy`, `go mod verify`, `git diff --check` y `git diff --cached --check`.

### Notes for next session
- El Paso 13 no se ha iniciado y requiere autorización explícita.

## Step 13 — Persistencia, resume y estado crash-safe de sesión

Status: completed
Date: 2026-08-12
Commit: 984b4cf
Release: unreleased

### Delivered
- Estado de sesión versionado con última vista, artifact, comando significativo, flags de setup, timestamp y marcador seguro de resume.
- Resume transaccional en SQLite, detección de sesiones incompletas, defaults ante metadata inválida y recovery auditado.
- Checkpoints TUI serializados solo en transiciones relevantes y cierre persistido tanto con `q` como con Ctrl+C.

### Decisions
- `internal/session` conserva formato y política independientes de SQLite/Bubble Tea; `internal/infra/sessiondb` enlaza cada operación a una transacción de estado y auditoría.
- El payload completo ocupa una sola entrada de `app_state`; el marcador `active` actúa como crash marker sin añadir una migration de esquema.
- Los errores de metadata secundaria no bloquean la TUI y los snapshots reconstruibles no se persisten.

### Verification
- `GOCACHE=/tmp/kelyro-step13-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step13-race-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step13-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step13-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go build -o /tmp/kelyro-step13 ./cmd/kelyro`
- Cross-compilation de tests para `windows/amd64` y `darwin/amd64` con `CGO_ENABLED=0` y `go test -vet=off -exec=/bin/true ./...`.
- Prueba TUI real en PTY de checkpoint, salida limpia y resume en Roadmap dentro de un workspace con espacios.
- `go mod tidy`, `go mod verify`, `git diff --check` y `git diff --cached --check`.

### Notes for next session
- El Paso 14 no se ha iniciado y requiere autorización explícita.

## Step 14 — Doctor y registry contextual de herramientas

Status: completed
Date: 2026-08-12
Commit: 0e724d3
Release: unreleased

### Delivered
- `kelyro doctor` funcional con checks de plataforma, escritura, configuración, SQLite, migrations, índice de artifacts y herramientas Foundation.
- Registry extensible con niveles required/recommended/optional, plataformas, candidatos, propósito, documentación y contexto por módulo.
- Reporte tipado compartido por CLI/TUI, detección y versiones seguras con timeout, output acotado y directorio temporal aislado.

### Decisions
- Solo los checks required determinan el exit code; recommended y optional son informativos y siempre explican su propósito.
- El dominio Doctor permanece independiente de `os/exec`, SQLite y Bubble Tea mediante resolver, environment y storage probe sustituibles.
- No se añadieron dependencias externas; los probes ejecutan exclusivamente argumentos definidos por el registry y nunca usan shell.

### Verification
- `GOCACHE=/tmp/kelyro-step14-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step14-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step14-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step14-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go build -o /tmp/kelyro-step14 ./cmd/kelyro`
- Cross-compilation de tests para `windows/amd64` y `darwin/amd64` con `CGO_ENABLED=0` y `go test -vet=off -exec=/bin/true ./...`.
- Prueba CLI real de reporte saludable y de workspace ausente con exit code 1, en una ruta con espacios.
- `go mod verify`, `git diff --check` y `git diff --cached --check`.

### Notes for next session
- El Paso 15 no se ha iniciado y requiere autorización explícita.

## Step 15 — Recomendaciones educativas de herramientas

Status: completed
Date: 2026-08-12
Commit: 7a99527
Release: unreleased

### Delivered
- Guidance mantenido para cada herramienta Foundation con descripción, motivo, fundamentos, notas por plataforma y enlace oficial.
- `kelyro doctor --explain <tool>` funcional incluso fuera de un workspace, sin abrir enlaces ni ejecutar instalaciones.
- Pruebas de metadata, niveles required/recommended/optional, adaptación a plataforma y política no bloqueante.

### Decisions
- La orientación vive en el registry local y no depende de una llamada LLM.
- Editores, Docker y lazygit continúan opcionales; las herramientas visuales se presentan como complemento de CLI y fundamentos.
- La aplicación consulta guidance sin ejecutar el diagnóstico de workspace; CLI y dominio permanecen desacoplados.

### Verification
- `GOCACHE=/tmp/kelyro-step15-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test ./...`
- `GOCACHE=/tmp/kelyro-step15-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go test -race ./...`
- `GOCACHE=/tmp/kelyro-step15-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go vet ./...`
- `GOCACHE=/tmp/kelyro-step15-verify-gocache GOMODCACHE=/tmp/kelyro-step9-modcache go build -o /tmp/kelyro-step15 ./cmd/kelyro`
- Cross-compilation de tests para `windows/amd64` y `darwin/amd64` con `CGO_ENABLED=0` y `go test -vet=off -exec=/bin/true ./...`.
- Prueba CLI real de guidance para herramientas recommended y optional fuera de un workspace.
- `go mod tidy`, `go mod verify`, `git diff --check` y `git diff --cached --check`.

### Notes for next session
- El Paso 16 no se ha iniciado y requiere autorización explícita.
