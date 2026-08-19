# I-02 Student & Learning Core — Progress Log

## Estado general

Current step: 4 (pending authorization)
Last completed step: 3
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

## Step 02 — Repositorios y application services del Student Core

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Paquete `internal/learning/application` con repositorios pequeños por agregado/caso de uso, sin una mega-interface de persistencia.
- Contratos iniciales de `StudentService`, `GoalService`, `ProgressService`, `SessionService`, `ReviewService`, `AnalyticsService` y `DailyPlanService` para consumidores CLI/TUI neutrales a infraestructura.
- Implementaciones delgadas que validan invariantes de dominio, orquestan repositorios y no incorporan políticas educativas reservadas para pasos posteriores.
- Taxonomía estable y causal de errores: `not_found`, `conflict`, `invalid_state`, `unavailable` y `persistence_failure`.
- Límite `UnitOfWork` que entrega repositorios coherentes dentro de una misma transacción sin exponer `sql.Tx`, `sql.Row`, drivers ni structs SQLite.
- Adaptador fake in-memory determinista con aislamiento de datos y semántica comprobable de commit/rollback para tests sin SQLite.
- Documentación arquitectónica de dependencias, puertos, servicios, errores y transacciones en `docs/architecture/student-learning-application.md`.

### Decisions

- `Repositories` es solo el conjunto de puertos entregado a un callback transaccional; no es un repositorio y no llega a presentación.
- Los repositorios usan operaciones `Create`/`Append` para identidades o hechos inmutables y reportan conflictos, mientras que las consultas singulares reportan not found y las consultas de colección devuelven slices vacíos.
- `ConceptProgress` ensambla únicamente hechos persistidos (estado, evidencia y errores); no calcula mastery ni cambia exposure.
- El fake in-memory vive como adapter de pruebas reutilizable y publica el estado de una transacción solo si el callback termina correctamente.
- Se añadió `RetentionRepository` por ser estado persistente del dominio, y `DailyPlanRepository` porque existe un servicio inicial equivalente; ninguno implementa todavía algoritmos de retención o planificación.

### Verification

- `GOCACHE=/tmp/kelyro-i02-step2-target2-gocache go test ./internal/learning/...`
- `GOCACHE=/tmp/kelyro-i02-step2-test-gocache go test ./...`
- `GOCACHE=/tmp/kelyro-i02-step2-vet-gocache go vet ./...`
- `GOCACHE=/tmp/kelyro-i02-step2-quality-gocache go run ./tools/quality all`, incluyendo E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- `git diff --check`.
- Revisión de imports: `internal/learning` no importa SQLite, Bubble Tea, drivers ni presentation adapters.

### Notes for next session

- El Paso 3 es el siguiente paso pendiente y requiere autorización explícita.
- El adapter SQLite del Paso 3 debe implementar estos puertos y mapear errores del driver con `application.Classify`.

## Step 03 — Schema SQLite y persistencia de Student Core

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Migration forward-only v4 con tablas normalizadas para perfil, goals, curriculum versionado, concept state, evidence, mistakes, retention, sessions, reviews, streaks, achievements, milestones, analytics y daily plans.
- Foreign keys, checks de enums/rangos/tiempo y accesos indexados para conceptos, reviews vencidas, goal activo, historial y rangos de sesiones.
- Adapters SQLite para todos los repositorios definidos en el Paso 2 y `UnitOfWork` transaccional, sin filtrar SQL ni errores del driver hacia application.
- Seeder de fixtures curriculares deterministas y versionados, limitado a infraestructura de pruebas y sin implementar Curriculum Compiler.
- Escrituras compuestas atómicas y reconstrucción validada de entidades de dominio.
- Pruebas de DB nueva, upgrade Foundation v3 → Student Core v4 con conservación de estado, migración repetida, constraints, índices, FKs/cascade, clasificación de errores, roundtrips de todos los repositorios y rollback.
- Decisiones de schema y adapter documentadas en `docs/architecture/student-learning-persistence.md`.

### Decisions

- `concept_registry` desacopla la identidad estable de un concepto de su aparición en una versión curricular concreta y permite FKs desde el estado longitudinal del estudiante.
- `curriculum_nodes` conserva la jerarquía genérica y versionada; `curriculum_edges` representa prerequisitos dentro de la misma instancia curricular.
- No se añadieron caches de cálculos educativos. Analytics snapshots y daily plans se persisten como resultados históricos auditables, no como valores derivados transparentes.
- Los timestamps se escriben en RFC3339Nano UTC y el schema exige su representación `Z`.
- La migration v4 es aditiva y no destructiva, por lo que reutiliza el mecanismo y formato de backup de Foundation sin exigir backup previo.

### Verification

- `GOCACHE=/tmp/kelyro-i02-step3-target-gocache go test ./internal/storage/sqlite`
- `GOCACHE=/tmp/kelyro-i02-step3-test-gocache go test ./...`
- `GOCACHE=/tmp/kelyro-i02-step3-vet-gocache go vet ./...`
- `GOCACHE=/tmp/kelyro-i02-step3-quality-gocache go run ./tools/quality all`, incluyendo E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- `GOCACHE=/tmp/kelyro-i02-step3-final-race-gocache go test -race ./internal/storage/sqlite`.
- `git diff --check`.
- Revisión de imports: `internal/learning` no importa SQLite, drivers ni presentation adapters.

### Notes for next session

- El Paso 4 es el siguiente paso pendiente y requiere autorización explícita.
- Student Profile puede usar `application.StudentService` con `Database.LearningRepositories().Students`; no debe leer o escribir tablas directamente.
