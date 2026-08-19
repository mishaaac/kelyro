# I-02 Student & Learning Core — Progress Log

## Estado general

Current step: 2 (pending authorization)
Last completed step: 1
Current release: v0.1.0-alpha.2
Foundation baseline: v0.1.0-alpha.2 (2a9eb2b)

## Registro

## Step 00 — Apertura formal de I-02

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Plan I-02 incorporado como memoria persistente en `docs/implementation/I-02-student-learning-core/PLAN.md`.
- Registro de progreso inicializado con el tag y commit reales de Foundation.
- `AGENTS.md` actualizado con el flujo de sesiones y las fronteras arquitectónicas de I-02.

### Decisions

- `v0.1.0-alpha.2` en `2a9eb2b` es el baseline inmutable de Foundation para iniciar I-02.
- Cada paso de I-02 se autoriza, implementa, verifica, documenta y comitea de forma independiente.
- Los desarrollos educativos usarán fixtures deterministas hasta que I-03 e I-04 entreguen investigación y Learning Packs reales.

### Verification

- `go test ./...`
- `go vet ./...`
- `go run ./tools/quality all`, incluyendo E2E Foundation y `go test -race ./...`.
- Revisión de `git status`, los 15 commits más recientes y el registro de cierre de I-01.

### Notes for next session

- El Paso 1 es el siguiente paso pendiente y requiere autorización explícita.

## Step 01 — Modelo de dominio educativo

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Paquete cohesivo `internal/learning` con el vocabulario completo de Student & Learning Core, sin dependencias de infraestructura o presentación.
- Value objects validados para IDs estables, timestamps UTC, mastery scores y mastery thresholds.
- Entidades y estados para estudiante, objetivo, curriculum, conceptos, evidencia, errores, sesiones, retención, reviews, streaks, achievements, milestones, analytics y daily plans.
- Invariantes de identidad, rangos, estados, relaciones temporales, prerequisitos, pertenencia de errores a conceptos conocidos y excepción explícita para reviews importadas.
- Fixtures educativos deterministas y versionados, con tests de constructores, value objects, límites y estados inválidos.
- Glosario, relaciones, límites y decisiones del dominio en `docs/architecture/student-learning-domain.md`.

### Decisions

- Se usa un único paquete `internal/learning`, separado por archivos de área, para mantener un lenguaje común y evitar micro-paquetes o ciclos prematuros. Los casos de uso futuros podrán revelar límites de paquete con valor real.
- Exposure lifecycle y mastery score son dimensiones independientes; ningún estado sustituye el score ni lo genera implícitamente.
- Las validaciones entre agregados reciben valores explícitos, como el catálogo de conceptos usado para validar un error, sin introducir repositorios o interfaces prematuras.
- El Paso 1 no incorpora algoritmos educativos. Mastery, retention, scheduling, streaks, achievements, analytics y daily-plan policies se implementarán y versionarán únicamente en sus pasos posteriores.

### Verification

- `GOCACHE=/tmp/kelyro-i02-step1-test-gocache go test ./...`
- `GOCACHE=/tmp/kelyro-i02-step1-vet-gocache go vet ./...`
- `GOCACHE=/tmp/kelyro-i02-step1-quality-gocache go run ./tools/quality all`, incluyendo E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- `git diff --check`.
- Revisión de imports: `internal/learning` depende únicamente de la librería estándar y no contiene `interface{}`/`any`.

### Notes for next session

- El Paso 2 es el siguiente paso pendiente y requiere autorización explícita.
