# Kelyro — Plan de Implementación I-01: Foundation

> Objetivo de esta implementación: construir una base técnica estable, multiplataforma, local-first y extensible sobre la que puedan apoyarse después el Student Core, Research Engine, Curriculum Compiler, Learning Engine, AI Runtime, Plugins y demás subsistemas de Kelyro.
>
> Este documento está diseñado para ejecutarse con Spec-Driven Development usando Codex en sesiones independientes. Cada paso debe poder completarse, verificarse, registrarse y comitearse antes de iniciar el siguiente.

---

## Paso 0 — Establecer el protocolo de trabajo SDD y la memoria persistente del repositorio

- [x] Paso 0 completado

### Objetivo

Crear las reglas que permitirán trabajar con Codex por sesiones independientes sin depender del historial del chat. El repositorio debe ser la memoria del proyecto.

### Alcance

En este paso no se implementa funcionalidad de Kelyro. Se prepara el sistema de trabajo.

### Crear

```text
AGENTS.md
docs/
└── implementation/
    └── I-01-foundation/
        ├── PLAN.md
        └── PROGRESS.md
```

`PLAN.md` debe ser este mismo documento.

`PROGRESS.md` debe comenzar con:

```md
# I-01 Foundation — Progress Log

## Estado general

Current step: 0
Last completed step: none
Current release: unreleased

## Registro

```

### Reglas que debe contener `AGENTS.md`

Debe mantenerse corto y operativo.

Como mínimo:

1. Kelyro está escrito en Go.
2. No implementar pasos futuros sin autorización explícita.
3. Antes de modificar código:
   - leer el paso solicitado en `docs/implementation/I-01-foundation/PLAN.md`;
   - leer `docs/implementation/I-01-foundation/PROGRESS.md`;
   - revisar `git status`;
   - revisar los últimos commits relevantes;
   - inspeccionar únicamente los archivos necesarios.
4. Usar SDD:
   - entender especificación;
   - proponer plan del paso;
   - implementar;
   - verificar;
   - actualizar documentación;
   - comitear.
5. No marcar un paso como completado si fallan tests, build, lint o criterios de aceptación.
6. No sobrescribir código o archivos human-owned sin confirmación explícita del diseño.
7. Usar rutas multiplataforma; nunca concatenar rutas manualmente con `/` o `\`.
8. Mantener el core desacoplado de:
   - Bubble Tea;
   - SQLite;
   - GitHub;
   - proveedores de IA;
   - sistema operativo.
9. Todo cambio funcional debe tener tests cuando sea razonable.
10. Cada paso terminado debe:
    - actualizar su checkbox en `PLAN.md`;
    - actualizar `PROGRESS.md`;
    - dejar el working tree limpio;
    - tener al menos un commit coherente.
11. No hacer commits gigantes si durante un paso existen dos cambios claramente independientes.
12. Usar Conventional Commits.
13. No guardar secrets en el repositorio.
14. No introducir dependencias externas sin justificar por qué son necesarias.

### Formato de registro en `PROGRESS.md`

Al finalizar cada paso, añadir:

```md
## Step XX — <nombre>

Status: completed
Date: YYYY-MM-DD
Commit: <hash corto>
Release: <versión o unreleased>

### Delivered
- ...

### Decisions
- ...

### Verification
- `go test ./...`
- `go vet ./...`
- ...

### Notes for next session
- ...
```

Mantener cada entrada breve. El objetivo no es duplicar todo el plan.

### Prompt reutilizable para iniciar una nueva sesión de Codex

Usar algo similar a:

```text
Trabaja únicamente en el Paso XX de docs/implementation/I-01-foundation/PLAN.md.

Antes de modificar código:
1. Lee AGENTS.md.
2. Lee el Paso XX completo.
3. Lee PROGRESS.md.
4. Revisa git status y los últimos commits relevantes.
5. En Plan Mode, dime el plan concreto para ejecutar solo este paso.
6. No implementes pasos posteriores.
7. Cuando termine, ejecuta todos los criterios de verificación del paso.
8. Si todo pasa, actualiza PLAN.md y PROGRESS.md y crea el/los commits Conventional Commit indicados.
```

### Uso recomendado de Codex

Para ahorrar uso:

- **Plan Mode** antes de cambios grandes.
- **Reasoning medium** como valor por defecto para trabajo rutinario.
- **Reasoning high** para:
  - arquitectura;
  - diseño de interfaces;
  - migraciones;
  - filesystem;
  - concurrencia;
  - recovery;
  - bugs complejos.
- **xhigh** solo cuando un problema realmente lo justifique.
- No pedir a Codex que vuelva a explicar todo el proyecto en cada sesión.

### Criterios de aceptación

- Existe `AGENTS.md`.
- Existe este `PLAN.md`.
- Existe `PROGRESS.md`.
- El protocolo explica cómo iniciar y cerrar cada sesión.
- Codex puede reconstruir el estado leyendo archivos + Git.
- No existe aún código funcional innecesario.

### Commit sugerido

```text
docs(project): add I-01 SDD execution protocol
```

---

## Paso 1 — Inicializar el repositorio Go y la identidad básica del proyecto

- [x] Paso 1 completado

### Objetivo

Crear un repositorio Go mínimo, compilable y testeable que represente oficialmente a Kelyro.

### Alcance

Solo bootstrap del repositorio y estándares básicos.

### Crear estructura inicial

```text
kelyro/
├── AGENTS.md
├── README.md
├── LICENSE
├── go.mod
├── go.sum
├── cmd/
│   └── kelyro/
│       └── main.go
├── internal/
│   └── version/
│       ├── version.go
│       └── version_test.go
├── docs/
│   ├── architecture/
│   │   └── README.md
│   └── implementation/
│       └── I-01-foundation/
│           ├── PLAN.md
│           └── PROGRESS.md
├── .gitignore
└── .editorconfig
```

### Requisitos

1. Inicializar módulo Go.
2. Definir nombre de módulo definitivo o temporal claramente documentado.
3. Crear `cmd/kelyro/main.go`.
4. El binario debe ejecutar sin TUI todavía.
5. Debe soportar al menos:
   - `kelyro --version`;
   - `kelyro --help`.
6. La versión inicial debe venir del paquete `internal/version`.
7. El paquete `version` debe permitir inyección por `-ldflags` en el futuro.
8. No hardcodear lógica funcional en `main.go`.
9. `main.go` debe limitarse a bootstrap y delegación.
10. README debe declarar:
    - propósito;
    - estado inicial;
    - plataformas objetivo;
    - cómo compilar;
    - cómo ejecutar tests.
11. `.gitignore` debe excluir:
    - binarios;
    - cobertura;
    - archivos temporales;
    - secretos locales;
    - artefactos de IDE razonables.
12. No ignorar archivos educativos que en el futuro serán human-owned.

### Diseño esperado de versión

Debe existir una estructura equivalente a:

```go
type Info struct {
    Version string
    Commit  string
    Date    string
}
```

Sin depender todavía de Git en runtime.

### Tests

- versión por defecto válida;
- formato de salida;
- build de `cmd/kelyro`.

### Verificación

```bash
go test ./...
go vet ./...
go build ./cmd/kelyro
./kelyro --version
./kelyro --help
```

En Windows utilizar el ejecutable correspondiente.

### Criterios de aceptación

- El repositorio compila.
- Tests pasan.
- El binario imprime versión.
- `main.go` es pequeño.
- No hay lógica de dominio en `cmd`.
- El working tree queda limpio.

### Commit sugerido

```text
feat(core): bootstrap Kelyro Go project
```

### Versión

Si es el primer estado distribuible del proyecto, considerar tag de prerelease inicial, por ejemplo:

```text
v0.1.0-alpha.1
```

No crear tag si todavía no se desea considerar este punto distribuible.

---

## Paso 2 — Definir la arquitectura y los contratos estables del Foundation Core

- [x] Paso 2 completado

### Objetivo

Evitar que Bubble Tea, SQLite, Cobra, filesystem, editor o plataforma se conviertan en el core.

### Alcance

Diseño de interfaces y paquetes. Implementaciones mínimas o mocks; todavía no funcionalidades completas.

### Crear estructura objetivo

```text
internal/
├── app/
├── cli/
├── tui/
├── platform/
├── workspace/
├── config/
├── storage/
├── artifacts/
├── editor/
├── doctor/
├── logging/
├── audit/
├── backup/
├── privacy/
├── update/
└── version/
```

### Definir contratos mínimos

#### Platform

Responsable de información y operaciones dependientes del OS.

Debe cubrir conceptos como:

```go
type Platform interface {
    Name() string
    UserHomeDir() (string, error)
    UserConfigDir() (string, error)
    UserCacheDir() (string, error)
    CommandPath(name string) (string, bool)
    OpenPath(path string) error
    OpenURL(url string) error
}
```

No es obligatorio usar exactamente estas firmas si Codex propone algo mejor, pero debe conservarse el desacoplamiento.

#### Workspace

```go
type WorkspaceService interface {
    Discover(startDir string) (...)
    Init(root string) (...)
    Validate(root string) (...)
}
```

#### Config

```go
type ConfigStore interface {
    LoadGlobal(...)
    LoadProject(...)
    SaveGlobal(...)
    SaveProject(...)
}
```

#### State/Storage

Definir abstracción para estado persistente sin exponer SQLite al resto del sistema.

#### SecretStore

```go
type SecretStore interface {
    Get(name string) (string, error)
    Set(name, value string) error
    Delete(name string) error
}
```

#### Artifact ownership

Definir categorías:

```text
machine-owned
system-generated-human-readable
student-owned
```

#### Event/Audit boundary

No implementar Event Bus completo todavía, pero dejar contrato para registrar acciones críticas.

### Documentación

Crear:

```text
docs/architecture/foundation.md
```

Debe explicar:

- responsabilidades;
- dependencias permitidas;
- dependencias prohibidas;
- sentido de cada paquete;
- diferencia entre domain/application/infrastructure/UI;
- por qué la TUI no contiene lógica educativa;
- por qué SQLite no se expone directamente;
- qué significa local-first.

### Regla de dependencias

La dirección debe parecerse a:

```text
cmd / tui / cli
      ↓
application services
      ↓
interfaces / domain contracts
      ↓
infrastructure adapters
```

Evitar ciclos.

### Tests

Crear tests de compile-time cuando sea útil y mocks/fakes sencillos.

### Verificación

```bash
go test ./...
go vet ./...
go list ./...
```

Revisar ciclos y dependencias.

### Criterios de aceptación

- Los contratos principales están definidos.
- No hay imports desde core hacia Bubble Tea.
- No hay imports desde core hacia SQLite driver.
- No hay lógica de OS desperdigada fuera de `platform`.
- La arquitectura está documentada.

### Commit sugerido

```text
refactor(core): define foundation architecture contracts
```

---

## Paso 3 — Implementar la CLI base y el router de comandos

- [x] Paso 3 completado

### Objetivo

Permitir que Kelyro funcione tanto como TUI por defecto como CLI explícita.

### Alcance

Construir el esqueleto de comandos, sin implementar todavía todas sus funcionalidades.

### Comportamiento requerido

```bash
kelyro
kelyro help
kelyro version
kelyro init
kelyro doctor
kelyro config
kelyro status
kelyro open
```

En esta etapa:

- `kelyro` puede mostrar una pantalla TUI placeholder o mensaje de bootstrap;
- `init`, `doctor`, `config`, `status`, `open` pueden tener implementación parcial claramente marcada.

### Recomendación técnica

Usar un router CLI mantenible. Si se adopta Cobra:

- aislarlo dentro de `internal/cli`;
- ningún package de dominio debe importar Cobra;
- commands deben llamar application services.

### Requisitos

1. Salidas de error consistentes.
2. Exit codes definidos.
3. `--help` coherente.
4. `--version`.
5. `--no-color` reservado desde el principio.
6. `--verbose`.
7. `--quiet` si se considera útil.
8. `--workspace <path>` reservado para overrides futuros.
9. La ausencia de subcomando inicia la TUI.
10. Los comandos deben ser testeables sin lanzar procesos reales.

### Tests

- parsing de comandos;
- errores por argumentos inválidos;
- `--help`;
- `--version`;
- dispatch correcto hacia servicios falsos.

### Verificación

```bash
go test ./...
go vet ./...
go build ./cmd/kelyro
kelyro --help
kelyro version
```

### Criterios de aceptación

- Existe una CLI clara.
- La CLI no contiene lógica de negocio.
- El default puede dirigir a TUI.
- Los comandos son testeables.

### Commit sugerido

```text
feat(cli): add command router and foundation commands
```

---

## Paso 4 — Implementar la capa multiplataforma de filesystem y rutas

- [x] Paso 4 completado

### Objetivo

Garantizar que Kelyro no dependa de rutas Unix y pueda ejecutarse correctamente en Windows, macOS y Linux.

### Alcance

Rutas, directorios de usuario, operaciones comunes de OS y normalización.

### Requisitos

1. Usar `filepath.Join`, `filepath.Clean`, `filepath.Abs`.
2. Nunca construir paths con concatenación manual.
3. Utilizar APIs de Go para:
   - home;
   - config;
   - cache;
   - temp.
4. Diferenciar:
   - directorio global de Kelyro;
   - directorio del workspace.
5. Diseñar path helpers:
   - `GlobalConfigDir`;
   - `GlobalCacheDir`;
   - `WorkspaceInternalDir`;
   - `WorkspaceDBPath`;
   - `WorkspaceStatePath`;
   - `WorkspaceBackupDir`.
6. Manejar paths con espacios.
7. Manejar rutas relativas.
8. Manejar drive letters en Windows.
9. No asumir case sensitivity.
10. No usar shell cuando una API Go directa sea suficiente.

### Tests

Tabla de casos para rutas.

Simular distintos OS cuando sea posible sin depender del sistema real.

### Criterios de aceptación

- No existen separadores hardcodeados.
- Las funciones de ruta tienen tests.
- Se documentan convenciones de cada OS.
- El resto del proyecto consume helpers en vez de inventar paths.

### Commit sugerido

```text
feat(platform): add cross-platform path abstraction
```

---

## Paso 5 — Implementar creación, descubrimiento y validación del Kelyro Workspace

- [x] Paso 5 completado

### Objetivo

Permitir que el usuario entre a una carpeta cualquiera, ejecute Kelyro y convierta esa carpeta en su workspace educativo.

### Comportamiento esperado

```bash
mkdir backend-go
cd backend-go
kelyro init
```

Debe crear:

```text
backend-go/
├── .kelyro/
│   ├── state/
│   ├── cache/
│   ├── backups/
│   └── logs/
└── LEARNING.md
```

No crear todavía curriculum real.

### Requisitos

1. Detectar si la carpeta ya es workspace.
2. Si se ejecuta desde una subcarpeta, encontrar el root buscando `.kelyro`.
3. Evitar inicialización accidental dentro de otro workspace salvo opción explícita.
4. Guardar un `workspace_id` estable.
5. Registrar fecha de creación.
6. Guardar schema/version del workspace.
7. Operación idempotente.
8. Si `init` falla a mitad, no dejar estado corrupto.
9. No crear secretos.
10. Human files visibles separados de internals.
11. `.kelyro/` debe ser machine-owned.
12. `LEARNING.md` debe ser human-readable.
13. Definir reglas sobre qué puede modificar Kelyro automáticamente.

### Diseñar metadata mínima

Por ejemplo:

```json
{
  "workspace_id": "...",
  "schema_version": 1,
  "created_at": "...",
  "app_version": "..."
}
```

La representación exacta puede cambiar.

### Tests

- init nuevo;
- init repetido;
- discovery desde subdir;
- root inválido;
- permisos insuficientes;
- path con espacios;
- rollback de init incompleto.

### Verificación

Crear workspaces temporales en tests y validar estructura.

### Criterios de aceptación

- `kelyro init` funciona.
- Discovery funciona.
- No corrompe workspaces existentes.
- Human vs machine files están separados.

### Commit sugerido

```text
feat(workspace): add local workspace lifecycle
```

---

## Paso 6 — Implementar configuración global y por proyecto

- [x] Paso 6 completado

### Objetivo

Tener configuración escalable sin obligar al usuario normal a editar archivos manualmente.

### Capas de configuración

```text
defaults
   ↓
global config
   ↓
project config
   ↓
CLI override
```

El valor más específico gana.

### Configuración global inicial

Ejemplos:

```toml
[ui]
color = "auto"

[editor]
command = ""

[privacy]
allow_network = false

[updates]
check = true
```

### Configuración de proyecto inicial

Ejemplos:

```toml
[workspace]
name = "Backend Engineering with Go"

[learning]
mastery_threshold = 0.85
```

No implementar todavía lógica educativa del threshold; solo esquema.

### Requisitos

1. Formato TOML.
2. Defaults seguros.
3. Validación estricta.
4. Error legible por clave inválida.
5. Migrabilidad futura.
6. Escritura atómica.
7. No destruir comentarios si la estrategia elegida puede evitarlos; si no, documentar la limitación.
8. Distinguir global/project.
9. CLI:
   - `kelyro config show`;
   - `kelyro config path`;
   - `kelyro config get <key>`;
   - `kelyro config set <key> <value>`.
10. Preparar wizard interactivo para settings comunes.
11. Configuración avanzada sigue disponible por archivo.

### Secrets

No guardar API keys aquí.

### Tests

- precedence;
- defaults;
- invalid config;
- save/reload;
- project override;
- atomic write.

### Commit sugerido

```text
feat(config): add layered global and workspace configuration
```

---

## Paso 7 — Implementar almacenamiento seguro de secretos

- [x] Paso 7 completado

### Objetivo

Garantizar desde el inicio que tokens y API keys nunca se guarden en texto plano dentro del workspace.

### Requisitos

1. Definir `SecretStore`.
2. Implementar adapter para keychain del OS cuando esté disponible.
3. Permitir variables de entorno como alternativa.
4. Nunca imprimir el valor de un secret.
5. `config show` debe mostrar únicamente:
   - `configured`;
   - `not configured`;
   - nombre de referencia.
6. No persistir secret en:
   - `.kelyro`;
   - TOML;
   - logs;
   - audit;
   - backups;
   - export.
7. Añadir redaction utility.
8. Errores del keychain deben ser legibles.
9. En Linux/headless donde el keychain no esté disponible, informar cómo usar env vars.
10. El core solo conoce la interfaz, no el backend.

### CLI inicial

```bash
kelyro secrets status
kelyro secrets set <name>
kelyro secrets delete <name>
```

El ingreso debe evitar eco en terminal si se introduce manualmente.

### Tests

- fake SecretStore;
- redaction;
- ninguna serialización accidental;
- fallback de env;
- errores controlados.

### Criterios de aceptación

- No hay secretos en fixtures.
- No hay secrets en logs.
- No hay secrets en backups/export.
- El storage puede sustituirse.

### Commit sugerido

```text
feat(security): add cross-platform secret storage abstraction
```

---

## Paso 8 — Implementar SQLite, repositorios internos y sistema de migraciones

- [x] Paso 8 completado

### Objetivo

Crear la fuente estructurada local del estado de Kelyro sin acoplar el core al driver.

### Recomendación

Usar `database/sql` con un driver SQLite sin CGO para simplificar builds multiplataforma.

### Ubicación

```text
.kelyro/
└── learning.db
```

### Schema Foundation mínimo

No crear aún tablas educativas completas.

Tablas mínimas sugeridas:

```text
schema_migrations
workspace_meta
app_state
artifact_index
audit_events
```

### Requisitos

1. Abrir/cerrar DB correctamente.
2. Context timeouts.
3. Foreign keys activas si se utilizan.
4. Transacciones.
5. Migrations incrementales.
6. Migrations idempotentes cuando corresponda.
7. Nunca editar una migration ya publicada.
8. Backup antes de migraciones destructivas futuras.
9. Schema version accesible.
10. Adapter aislado en `internal/storage/sqlite`.
11. Repositories expuestos por interfaces.
12. Tests con DB temporal.
13. No usar DB global singleton.
14. No permitir corrupción silenciosa.

### Migration runner

Debe:

- detectar versión actual;
- aplicar pendientes;
- registrar cada migration;
- fallar de forma segura;
- reportar migration problemática.

### Tests

- DB nueva;
- migración 0→latest;
- migración repetida;
- rollback ante error;
- repositorios básicos.

### Commit sugerido

```text
feat(storage): add SQLite persistence and migration engine
```

---

## Paso 9 — Implementar ownership de archivos, integridad y sandbox del workspace

- [x] Paso 9 completado

### Objetivo

Evitar que Kelyro destruya o sobrescriba el trabajo del estudiante.

### Clasificación obligatoria

#### Machine-owned

Ejemplos:

```text
.kelyro/learning.db
.kelyro/state/*
.kelyro/cache/*
```

Kelyro puede modificar estos archivos.

#### System-generated, human-readable

Ejemplos futuros:

```text
LEARNING.md
ROADMAP.md
LESSON.md
```

Kelyro puede generarlos, pero debe conocer si el usuario los modificó antes de regenerar.

#### Student-owned

Ejemplos:

```text
main.go
notes.md
projects/*
```

Kelyro no debe sobrescribirlos silenciosamente.

### Artifact Index

Guardar:

- path;
- category;
- created_by;
- content hash;
- created_at;
- last_generated_at;
- expected template/version si aplica.

### Escritura segura

Antes de regenerar un human-readable artifact:

1. comparar hash;
2. detectar modificación externa;
3. si fue modificado:
   - no sobrescribir;
   - crear alternativa o pedir decisión;
4. usar temp + atomic rename.

### Sandbox

Todos los ejercicios futuros deberán poder residir bajo una raíz controlada del workspace.

En I-01 solo crear la infraestructura y validación de path traversal.

### Seguridad de paths

Bloquear:

```text
../../
symlink escape cuando aplique
absolute path injection
```

### Tests

- clasificación;
- modificación detectada;
- atomic writes;
- path traversal;
- symlink edge cases razonables por OS;
- no-overwrite.

### Commit sugerido

```text
feat(artifacts): protect workspace ownership and content integrity
```

---

## Paso 10 — Implementar artefactos Markdown humanos y el roadmap placeholder

- [x] Paso 10 completado

### Objetivo

Probar la filosofía “machine state interno, artefactos humanos legibles” sin implementar todavía el Curriculum Engine.

### Crear

```text
LEARNING.md
00-roadmap/
└── ROADMAP.md
```

### `LEARNING.md`

Debe mostrar estado Foundation simple:

```md
# Kelyro

Workspace: ...
Status: initialized

No learning path has been configured yet.
```

### `ROADMAP.md`

En esta etapa debe ser placeholder bien estructurado:

```md
# Roadmap

No learning path has been generated yet.
```

### Requisitos

1. Generadores de Markdown separados de persistencia.
2. Templates testeables.
3. Encoding UTF-8.
4. Saltos de línea consistentes.
5. No guardar JSON visible salvo debugging explícito.
6. Regeneración respeta Artifact Ownership.
7. El contenido interno debe poder cambiar sin romper Markdown humano.
8. Preparar frontmatter solo si existe necesidad clara; no añadir por decoración.

### Tests

Golden tests para Markdown generado.

### Commit sugerido

```text
feat(artifacts): add human-readable learning workspace documents
```

---

## Paso 11 — Implementar detección de editores y apertura de archivos

- [x] Paso 11 completado

### Objetivo

Permitir que Kelyro entregue un archivo y el usuario lo abra fácilmente con su editor favorito.

### Detectar

Como mínimo, cuando existan:

```text
code
nvim
vim
zed
cursor
```

No asumir que todos están disponibles en todas las plataformas.

### Configuración

```toml
[editor]
command = "code"
```

### Flujo

```text
Lesson generated.

Open in VS Code?
> Yes
  No
```

Aunque aún no hay lessons reales, probar con `LEARNING.md` y `ROADMAP.md`.

### CLI

```bash
kelyro open
kelyro open roadmap
```

### Requisitos

1. No usar shell string concatenation.
2. Ejecutar comando con args separados.
3. Detectar executable real.
4. Soportar system default como fallback.
5. Manejar editor faltante.
6. Configurable.
7. Testear command construction sin abrir procesos reales.
8. Soportar paths con espacios.
9. En TUI el prompt debe ser opcional/configurable.

### Commit sugerido

```text
feat(editor): add editor detection and safe artifact opening
```

---

## Paso 12 — Construir la TUI Foundation con Bubble Tea

- [x] Paso 12 completado

### Objetivo

Construir la primera TUI real de Kelyro sin meter lógica de negocio dentro de Bubble Tea.

### Stack

- Bubble Tea;
- Lip Gloss;
- Bubbles cuando simplifique componentes.

### Pantallas Foundation

#### Home

Mostrar:

```text
Kelyro
Workspace: <name>

Status
✓ Workspace initialized
✓ Database healthy
✓ Configuration loaded

No learning path yet.

[Enter] Continue
[d] Doctor
[c] Config
[r] Roadmap
[q] Quit
```

#### Doctor

Puede usar servicio parcial existente.

#### Config

Permitir al menos lectura y wizard mínimo.

#### Roadmap

Mostrar placeholder.

### Requisitos UX

1. Funcionar desde terminales estrechas razonables.
2. No requerir Nerd Fonts.
3. No depender exclusivamente de colores.
4. Respetar `NO_COLOR` o `--no-color`.
5. Navegación por teclado.
6. Atajos visibles.
7. Estados loading/error/empty.
8. Manejar resize.
9. No hardcodear lógica de workspace en `Update`.
10. TUI llama application services.
11. Salir limpiamente.
12. No dejar terminal corrupta tras panic recuperable.
13. Render estable en Windows Terminal, macOS terminals y Linux terminals razonables.

### Arquitectura TUI

Separar:

```text
model
messages
commands
views/components
styles
```

sin convertirlo en una abstracción exagerada.

### Tests

- update model;
- key handling;
- state transitions;
- render snapshots/golden tests cuando sea estable;
- terminal width small/normal/large.

### Commit sugerido

```text
feat(tui): add Foundation terminal interface
```

---

## Paso 13 — Implementar persistencia de sesión, resume y crash-safe state

- [x] Paso 13 completado

### Objetivo

Que cerrar Kelyro y volver a abrirlo no pierda el contexto operativo del workspace.

### Estado mínimo a guardar

- última vista;
- último artifact abierto;
- último comando significativo;
- flags de setup;
- timestamp de sesión;
- estado seguro para retomar.

No guardar información que deba reconstruirse fácilmente si eso aumenta complejidad.

### Requisitos

1. Persistencia transaccional.
2. State versionado.
3. Escritura solo en puntos relevantes.
4. No escribir DB en cada keypress de la TUI.
5. Graceful shutdown:
   - Ctrl+C;
   - quit normal.
6. Detectar sesión anterior incompleta.
7. Recuperarse de state inválido usando defaults.
8. Crash marker o mecanismo equivalente si aporta valor.
9. Nunca impedir abrir el proyecto por metadata secundaria corrupta.
10. Registrar recovery en logs/audit.

### Tests

- normal quit;
- resume;
- corrupt state;
- interrupted session;
- migration del state version.

### Commit sugerido

```text
feat(state): persist and safely resume Kelyro sessions
```

---

## Paso 14 — Implementar `doctor` y el registry de herramientas

- [x] Paso 14 completado

### Objetivo

Crear un sistema extensible para detectar dependencias y herramientas del entorno.

### Comando

```bash
kelyro doctor
```

### Checks Foundation

```text
Platform
✓ OS detected
✓ Workspace writable
✓ Internal directory writable

Kelyro
✓ Config valid
✓ Database healthy
✓ Migrations current
✓ Artifact index healthy

Development
✓ Go detected
✓ Git detected

Optional
○ VS Code
○ Neovim
○ Docker
○ lazygit
```

### Tool Registry

Definir metadata:

```text
id
display_name
command_candidates
required/recommended/optional
supported_platforms
why_needed
learn_more
```

### Context-aware requirement

Todavía no existe curriculum, pero el sistema debe permitir que en el futuro una fase diga:

```text
Docker required in module X
```

`doctor` debe poder recibir un contexto y mostrar solo lo relevante.

### Requisitos

1. Detección de executable.
2. Version parsing donde sea seguro.
3. Timeouts.
4. No ejecutar comandos peligrosos.
5. Mostrar motivo de cada requisito.
6. Distinguir:
   - required;
   - recommended;
   - optional.
7. No marcar recommended como failure.
8. Output usable tanto en TUI como CLI.
9. Modo machine-readable reservado si hace falta después; no introducir JSON visible como UX principal.

### Tests

Usar fake command resolver.

### Commit sugerido

```text
feat(doctor): add environment diagnostics and tool registry
```

---

## Paso 15 — Añadir recomendaciones educativas de herramientas

- [x] Paso 15 completado

### Objetivo

Evitar que `doctor` sea solo “te falta X”; explicar qué es la herramienta y por qué Kelyro la recomienda.

### Ejemplo

```text
○ lazygit — Recommended

Why:
A terminal UI for Git that can help you inspect branches,
commits and diffs. It is not required to continue.

Kelyro will teach Git using the Git CLI first.
```

### Requisitos

1. Tool metadata puede incluir explicación.
2. No imponer herramientas opcionales.
3. No convertir herramientas visuales en sustituto de fundamentos.
4. Mensajes adaptables por plataforma.
5. Preparar enlaces oficiales sin abrirlos automáticamente.
6. `kelyro doctor --explain <tool>` o interacción equivalente en TUI.
7. La explicación debe venir de metadata mantenida, no de una llamada LLM.

### Tests

- required/recommended/optional;
- explanation disponible;
- no false blocking.

### Commit sugerido

```text
feat(doctor): add contextual tool guidance
```

---

## Paso 16 — Implementar logging estructurado y audit trail

- [x] Paso 16 completado

### Objetivo

Poder diagnosticar problemas y saber qué acciones importantes modificaron el estado sin ensuciar la UX normal.

### Logs

Ubicación interna:

```text
.kelyro/logs/
```

### Requisitos de logging

1. Niveles:
   - debug;
   - info;
   - warn;
   - error.
2. Logging estructurado.
3. Rotación o política simple para no crecer indefinidamente.
4. Redacción de secrets.
5. No registrar contenido completo del estudiante por defecto.
6. `--verbose` puede ampliar información.
7. Logs no aparecen normalmente en pantalla salvo error relevante.
8. Contexto:
   - operation;
   - workspace;
   - component;
   - error category.
9. Test de redaction.

### Audit Trail

Registrar eventos persistentes como:

```text
workspace.initialized
config.changed
migration.applied
artifact.generated
artifact.regeneration_blocked
backup.created
import.completed
```

No registrar cada keypress.

### Audit fields

- timestamp;
- event;
- actor (`system`, `user`, `plugin` futuro);
- metadata segura;
- app version.

### CLI

```bash
kelyro logs path
kelyro audit
```

### Tests

- secret redaction;
- event persistence;
- bounded logging behavior;
- audit survives restart.

### Commit sugerido

```text
feat(observability): add structured logging and audit trail
```

---

## Paso 17 — Implementar backups y recovery antes de operaciones riesgosas

- [x] Paso 17 completado

### Objetivo

Proteger estado y archivos administrados por Kelyro frente a migraciones, imports o fallos.

### Backup debe incluir

Según operación:

- `learning.db`;
- config de proyecto;
- state;
- artifact index;
- metadata relevante.

No incluir secrets.

### Requisitos

1. Crear backup atómico.
2. Manifest del backup.
3. Timestamp.
4. App/schema version.
5. Validación de integridad.
6. Retention policy configurable.
7. No copiar caches innecesarios.
8. Restore explícito.
9. Confirmación antes de restore destructivo.
10. Backup automático antes de migration de riesgo.
11. Probar restore en temp antes de tocar workspace real cuando sea razonable.

### CLI

```bash
kelyro backup create
kelyro backup list
kelyro backup restore <id>
```

### Tests

- create/list/restore;
- corrupt archive;
- no secrets;
- retention;
- failed restore leaves original intact.

### Commit sugerido

```text
feat(backup): add safe workspace backup and restore
```

---

## Paso 18 — Implementar export e import portables

- [x] Paso 18 completado

### Objetivo

Evitar lock-in y permitir mover el workspace o conservar una copia legible.

### Export modes

#### Human export

Incluye:

- Markdown;
- notas;
- student-owned files seleccionados;
- roadmap visible.

No incluye machine internals innecesarios.

#### Full portable export

Incluye estado requerido para restaurar Kelyro en otra máquina.

No incluye secrets.

### Requisitos

1. Manifest.
2. Format version.
3. Checksums.
4. Paths relativos.
5. Protección path traversal durante import.
6. Import valida antes de escribir.
7. Conflict strategy explícita.
8. No sobrescribir student-owned files sin autorización.
9. Compatible con Windows/macOS/Linux en la medida razonable.
10. Permitir dry-run.

### CLI

```bash
kelyro export
kelyro export --full
kelyro import <file> --dry-run
kelyro import <file>
```

### Tests

- roundtrip;
- malformed archive;
- traversal;
- conflicts;
- no secrets;
- platform path normalization.

### Commit sugerido

```text
feat(portability): add workspace export and import
```

---

## Paso 19 — Formalizar privacidad, local-first y modo offline

- [x] Paso 19 completado

### Objetivo

Establecer que el usuario posee su información y que el Foundation Core funciona sin Internet.

### Política local-first

Por defecto:

- SQLite local;
- Markdown local;
- logs local;
- config local;
- no telemetría automática;
- no red salvo función que lo requiera y esté permitida.

### Privacy Policy técnica

Config:

```toml
[privacy]
allow_network = false
allow_ai_content = false
allow_usage_telemetry = false
```

Los nombres exactos pueden variar.

### Network Gate

Crear una abstracción para que componentes futuros consulten si tienen permiso de red.

No intentar construir aún el Research Engine.

### Requisitos offline

Sin red deben funcionar:

```text
kelyro
kelyro status
kelyro roadmap
kelyro doctor
kelyro config
kelyro backup
kelyro export
```

`self-update` o recursos externos deben fallar elegantemente.

### Requisitos

1. Ningún request HTTP oculto.
2. Telemetría opt-in, si alguna vez existe.
3. Privacy config documentada.
4. No enviar paths del usuario accidentalmente.
5. Testear offline mediante fake network boundary.
6. Logs muestran cuando una operación fue bloqueada por privacy.
7. Plugins/AI futuros deberán usar este boundary.

### Commit sugerido

```text
feat(privacy): enforce local-first and offline Foundation behavior
```

---

## Paso 20 — Implementar el sistema de comprobación de actualizaciones

- [x] Paso 20 completado

### Objetivo

Preparar auto-update sin acoplar el core a GitHub ni ejecutar actualizaciones inseguras.

### Alcance Foundation

Debe poder:

- conocer versión actual;
- consultar metadata de una versión disponible mediante adapter;
- decidir si existe update;
- mostrarlo;
- no actualizar silenciosamente.

### Arquitectura

```text
UpdateService
    ↓
ReleaseProvider interface
    ↓
GitHub adapter (opcional)
```

El core no debe depender de GitHub.

### Requisitos

1. Respetar SemVer.
2. Soportar prerelease channel si se habilita.
3. No downgrade automático.
4. No update automático sin consentimiento.
5. Verificar checksum/firma cuando el mecanismo de distribución esté listo.
6. Si no existe red:
   - no fallar toda la app.
7. Config:
   - update check enabled/disabled;
   - channel stable/prerelease.
8. Cachear check para no consultar en cada command.
9. No pedir update en tests.
10. Permitir `kelyro update check`.
11. `kelyro update` puede quedar en modo seguro/unsupported hasta tener artefactos de release firmados; debe explicarlo claramente si todavía no es posible.

### Tests

- no update;
- newer patch/minor/major;
- prerelease;
- offline;
- malformed version.

### Commit sugerido

```text
feat(update): add version-aware update checks
```

---

## Paso 21 — Configurar calidad automática, CI y matriz multiplataforma

- [ ] Paso 21 completado

### Objetivo

Hacer cumplir al propio Kelyro las prácticas que luego enseñará.

### Local quality gates

Como mínimo:

```bash
go test ./...
go vet ./...
go test -race ./...
```

`-race` puede tener consideraciones por plataforma; documentar excepciones.

### Añadir

```text
Makefile
```

o alternativa multiplataforma en Go si se prefiere evitar Make como requisito.

Preferencia: no hacer que un desarrollador Windows necesite Make para contribuir.

Puede crearse un `Taskfile` solo si no añade fricción; una opción más Go-native es:

```bash
go run ./tools/...
```

o scripts Go.

### CI

Crear matriz al menos:

```text
ubuntu-latest
windows-latest
macos-latest
```

Checks:

1. checkout;
2. setup Go;
3. dependencies;
4. `go test ./...`;
5. `go vet ./...`;
6. build binary;
7. smoke test version/help.

### Requisitos

1. Tests no dependen de internet innecesariamente.
2. Paths temporales independientes.
3. Tests de filesystem no asumen Unix.
4. Golden files estables.
5. Race tests donde sean confiables.
6. Coverage report opcional.
7. CI falla si build falla en una plataforma.
8. No guardar credentials en workflow.
9. Dependencias pinneadas razonablemente.

### Commit sugerido

```text
ci: add cross-platform build and test matrix
```

---

## Paso 22 — Añadir pruebas E2E de Foundation

- [ ] Paso 22 completado

### Objetivo

Validar que un usuario real puede instalar/compilar Kelyro y ejecutar el flujo Foundation completo.

### Escenarios E2E

#### Caso 1 — Nuevo workspace

```text
create temp dir
kelyro init
validate .kelyro
validate LEARNING.md
validate ROADMAP.md
```

#### Caso 2 — Reabrir

```text
kelyro
quit
kelyro
resume
```

#### Caso 3 — Doctor

```text
kelyro doctor
```

Debe dar resultado coherente sin modificar estado inesperadamente.

#### Caso 4 — Config

```text
set project config
restart
verify precedence
```

#### Caso 5 — Artifact protection

Modificar `LEARNING.md`, intentar regenerar y comprobar que no se pisa silenciosamente.

#### Caso 6 — Backup/restore

Cambiar estado, restaurar y verificar.

#### Caso 7 — Export/import

Exportar workspace, importarlo en temp dir y comparar estado relevante.

#### Caso 8 — Offline

Desactivar network adapter y comprobar commands Foundation.

### Requisitos

- E2E aislados;
- ejecutables en CI;
- errores útiles;
- no depender de editor real;
- no depender de keychain real en tests: usar fake.

### Commit sugerido

```text
test(e2e): cover Foundation workspace lifecycle
```

---

## Paso 23 — Preparar distribución y releases reproducibles

- [ ] Paso 23 completado

### Objetivo

Poder producir binarios para Windows, macOS y Linux con metadata de versión correcta.

### Targets

Como mínimo considerar:

```text
windows/amd64
windows/arm64 cuando sea viable
darwin/amd64
darwin/arm64
linux/amd64
linux/arm64
```

### Requisitos

1. Builds reproducibles razonables.
2. Inyectar:
   - version;
   - commit;
   - build date.
3. Checksums.
4. Artefactos con nombres consistentes.
5. No introducir CGO obligatorio.
6. Documentar instalación manual.
7. Preparar automatización de release.
8. No publicar release desde cualquier branch.
9. Release solo con:
   - clean tree;
   - CI verde;
   - version válida;
   - tag correspondiente.
10. Annotated Git tags para releases.
11. Changelog/release notes generables desde commits, pero revisadas antes de publicar.

### SemVer durante `0.x`

Mantener SemVer aunque la API esté en evolución:

```text
0.1.0
0.1.1
0.2.0
...
```

Reglas:

- fix compatible → PATCH;
- feature compatible → MINOR;
- breaking change pre-1.0 debe seguir documentándose explícitamente;
- `1.0.0` solo cuando se considere estable la API pública principal.

### Conventional Commits

Usar:

```text
feat:
fix:
refactor:
docs:
test:
ci:
build:
chore:
```

Breaking change:

```text
feat(plugin)!:
```

o footer `BREAKING CHANGE:` cuando corresponda.

### Tests/verification

Crear artefactos localmente o en CI y ejecutar smoke test.

### Commit sugerido

```text
build(release): add cross-platform release pipeline
```

---

## Paso 24 — Ejecutar hardening de seguridad y portabilidad de I-01

- [ ] Paso 24 completado

### Objetivo

Revisar Foundation como sistema completo antes de declararlo listo para soportar I-02.

### Revisar

#### Filesystem

- path traversal;
- symlink behavior;
- permissions;
- atomic writes;
- Windows paths;
- spaces;
- Unicode.

#### Secrets

- logs;
- exports;
- backups;
- config;
- error messages.

#### SQLite

- DB corruption handling;
- migrations;
- busy/locked handling;
- close;
- transactions.

#### TUI

- Ctrl+C;
- terminal resize;
- no-color;
- unexpected errors.

#### CLI

- invalid args;
- exit codes;
- stderr/stdout correctness.

#### Import

- archive traversal;
- collision handling;
- malformed manifest.

#### Update

- malicious/malformed version metadata;
- offline;
- timeouts.

### Añadir tests de regresión

Cada bug encontrado durante hardening debe incluir test cuando sea razonable.

### Commits

No hacer un commit genérico con diez fixes inconexos.

Ejemplos:

```text
fix(workspace): reject path traversal during import
fix(storage): recover cleanly from locked SQLite database
fix(tui): restore terminal state after initialization error
```

Cada fix que merezca release debe afectar SemVer según las reglas.

### Criterios de aceptación

- No quedan findings críticos conocidos.
- Bugs importantes tienen tests.
- CI completa verde.
- Windows/macOS/Linux builds pasan.
- No secrets filtrados.

---

## Paso 25 — Declarar Foundation estable para continuar con I-02

- [ ] Paso 25 completado

### Objetivo

Cerrar formalmente I-01 sin añadir funcionalidades de Student Core.

### Ejecutar checklist técnico completo

```bash
go test ./...
go vet ./...
go test -race ./...
```

Además:

- CI verde en Linux;
- CI verde en macOS;
- CI verde en Windows;
- build de targets soportados;
- E2E Foundation;
- doctor sano;
- backup/restore;
- export/import;
- offline smoke test;
- no secret leaks;
- clean working tree.

### Revisar documentación

Deben estar actualizados:

```text
README.md
AGENTS.md
docs/architecture/foundation.md
docs/implementation/I-01-foundation/PLAN.md
docs/implementation/I-01-foundation/PROGRESS.md
```

### Revisar contratos

Confirmar que:

- TUI depende de application services, no al revés.
- CLI depende de application services, no al revés.
- SQLite es adapter.
- filesystem/platform es adapter.
- update provider es adapter.
- secret store es adapter.
- el core no conoce proveedores de IA.
- el core no conoce GitHub como requisito.
- no existe lógica educativa hardcodeada.

### Crear reporte de cierre

Añadir en `PROGRESS.md`:

```md
## I-01 Foundation Completion

Status: completed
Foundation release: <version>
Completed steps: 0-25
Known limitations:
- ...

Ready for:
I-02 Student & Learning Core
```

### Release

Elegir versión según el historial real de cambios.

No asumir que debe ser `v1.0.0`.

Ejemplo posible:

```text
v0.8.0
```

si Foundation queda funcional pero el producto todavía está en desarrollo inicial.

Crear:

1. release commit si hace falta;
2. annotated tag;
3. artefactos;
4. checksums;
5. release notes.

### Commit sugerido

Si solo cambia documentación de estado:

```text
docs(roadmap): mark I-01 Foundation complete
```

El tag se crea después sobre el commit correcto.

---

# Checklist final — I-01 Foundation

## Protocolo y repositorio

- [x] Paso 0 — Protocolo SDD y memoria persistente del repositorio
- [ ] Paso 1 — Bootstrap Go e identidad del proyecto
- [ ] Paso 2 — Arquitectura y contratos Foundation
- [ ] Paso 3 — CLI base
- [ ] Paso 4 — Plataforma y rutas multiplataforma
- [ ] Paso 5 — Kelyro Workspace
- [ ] Paso 6 — Configuración global/proyecto
- [ ] Paso 7 — Secret management
- [ ] Paso 8 — SQLite y migrations
- [ ] Paso 9 — Ownership, integridad y sandbox
- [ ] Paso 10 — Markdown human-readable + roadmap placeholder
- [ ] Paso 11 — Editor detection/open
- [ ] Paso 12 — TUI Foundation
- [ ] Paso 13 — Resume y crash-safe state
- [ ] Paso 14 — Doctor y tool registry
- [ ] Paso 15 — Tool guidance
- [ ] Paso 16 — Logging y audit trail
- [ ] Paso 17 — Backups y restore
- [ ] Paso 18 — Export/import
- [ ] Paso 19 — Privacy, local-first y offline
- [ ] Paso 20 — Update checks
- [ ] Paso 21 — CI y matriz multiplataforma
- [ ] Paso 22 — E2E Foundation
- [ ] Paso 23 — Distribución y releases
- [ ] Paso 24 — Hardening de seguridad y portabilidad
- [ ] Paso 25 — Cierre formal de I-01

## Cobertura funcional de I-01

- [ ] Workspace local inicializable desde cualquier carpeta
- [ ] TUI funcional en terminal
- [ ] CLI funcional
- [ ] Windows soportado
- [ ] macOS soportado
- [ ] Linux soportado
- [ ] `.kelyro/` contiene internals
- [ ] Human-readable artifacts quedan visibles
- [ ] Markdown generado legible
- [ ] Student-owned files protegidos
- [ ] Editor favorito detectable/configurable
- [ ] Apertura de artifacts segura
- [ ] Roadmap placeholder visible
- [ ] Roadmap Markdown visible
- [ ] `doctor` funcional
- [ ] Tool registry extensible
- [ ] Tool recommendations disponibles
- [ ] Sandbox base seguro
- [ ] Secrets fuera de config/workspace
- [ ] Resume de sesión
- [ ] Recovery ante estado dañado razonable
- [ ] Content integrity
- [ ] Backups
- [ ] Restore
- [ ] Export
- [ ] Import
- [ ] Local-first
- [ ] Privacy boundaries
- [ ] Offline Foundation
- [ ] Update check
- [ ] Config global
- [ ] Config por workspace
- [ ] Config wizard básico
- [ ] Config avanzada por archivo
- [ ] Logs
- [ ] Audit trail
- [ ] Migration engine
- [ ] Development standards
- [ ] Unit tests
- [ ] Integration tests
- [ ] E2E tests
- [ ] Cross-platform CI
- [ ] Stable internal contracts

## Definition of Done de I-01

- [ ] `go test ./...` pasa
- [ ] `go vet ./...` pasa
- [ ] race tests aplicables pasan
- [ ] CI Linux pasa
- [ ] CI Windows pasa
- [ ] CI macOS pasa
- [ ] Builds soportados compilan
- [ ] E2E Foundation pasa
- [ ] No quedan secrets en repo/logs/export/backups
- [ ] No existe path handling hardcodeado por OS
- [ ] No existe lógica de Student Core implementada prematuramente
- [ ] No existe lógica de AI providers implementada prematuramente
- [ ] TUI y CLI comparten application services
- [ ] SQLite permanece detrás de interfaces
- [ ] Platform permanece detrás de interfaces
- [ ] Workspace puede abrirse offline
- [ ] Working tree limpio
- [ ] Todos los pasos completados tienen registro en `PROGRESS.md`
- [ ] Todos los pasos relevantes tienen commits Conventional Commit
- [ ] La versión final de Foundation respeta SemVer
- [ ] Existe tag anotado si se publicó una release
- [ ] `PROGRESS.md` declara explícitamente I-01 listo para I-02
