# I-01 Foundation — Progress Log

## Estado general

Current step: none (awaiting authorization)
Last completed step: 9
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
