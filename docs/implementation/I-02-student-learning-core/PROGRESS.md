# I-02 Student & Learning Core — Progress Log

## Estado general

Current step: 6 (pending authorization)
Last completed step: 5
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

## Step 04 — Student Profile persistente

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Perfil local único por workspace con ID estable `student.primary`, nombre opcional, experiencia general, idioma preferido, presupuesto diario, objetivo semanal, preferencias de aprendizaje, timezone y timestamps UTC.
- Defaults deterministas y privados (`novice`, `en`, 30 minutos, 5 días, sin preferencias y `UTC`) creados en el primer uso, sin inferir datos del host.
- Caso de uso `ProfileService` para create-on-first-use y edición parcial, reutilizando `StudentService` y manteniendo CLI/TUI independientes de SQLite.
- Migration forward-only v5 y adapter SQLite actualizado para conservar perfiles v4, permitir nombre opcional y persistir los nuevos campos sin modificar migrations publicadas.
- Factory workspace-scoped `learningdb` con cierre explícito de la base de datos y prueba de persistencia entre reaperturas.
- CLI `kelyro profile show` y `kelyro profile edit` con salida humana, validación de opciones y soporte para limpiar campos opcionales.
- Vista TUI simple de perfil, accesible desde Home, con refresh, estado de error responsive y reanudación mediante session state.
- Documentación de defaults, privacidad, límites, compatibilidad de schema y uso en `docs/architecture/student-profile.md`.

### Decisions

- El nivel de experiencia del perfil es deliberadamente general; el conocimiento específico se determinará por goal/diagnostic en pasos posteriores.
- El perfil no almacena edad, género, dirección, credenciales ni otra información sensible innecesaria.
- Idioma y timezone se validan en dominio; la base IANA se embebe desde la librería estándar para mantener comportamiento offline y cross-platform.
- La migration v5 conserva `display_name` y `weekly_minutes` como mirrors de compatibilidad de la v4 publicada; `preferred_display_name` es el campo autoritativo que permite ausencia real del nombre.
- La TUI es de solo lectura en este paso. Toda edición usa el mismo caso de uso application mediante la CLI; onboarding no fue adelantado.

### Verification

- `GOCACHE=/tmp/kelyro-i02-step4-target2-gocache go test ./internal/learning/... ./internal/storage/sqlite ./internal/infra/learningdb ./internal/app ./internal/cli ./internal/tui`
- `GOCACHE=/tmp/kelyro-i02-step4-test-gocache go test ./...`
- `GOCACHE=/tmp/kelyro-i02-step4-vet-gocache go vet ./...`
- `GOCACHE=/tmp/kelyro-i02-step4-final-gocache go run ./tools/quality all`, incluyendo E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- Smoke real `init → profile show → profile edit → profile show` sobre un workspace temporal, verificando defaults, salida humana y persistencia.
- `git diff --check`.

### Notes for next session

- El Paso 5 es el siguiente paso pendiente y requiere autorización explícita.
- Learning Goals debe consumir el `student.primary` persistido mediante application services; no debe acceder directamente al schema ni convertir experiencia general en conocimiento específico.

## Step 05 — Learning Goals

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- `LearningGoal` ampliado con descripción opcional, dominio extensible, resultado objetivo, nivel inicial específico del objetivo y timestamps explícitos de activación/completado.
- Transiciones de dominio validadas para activar, pausar, reanudar, completar y archivar, con cronología consistente y preservación de la primera activación.
- Caso de uso workspace-scoped para `show`, `set`, `pause` y `resume`; toda sustitución o reanudación pausa el objetivo activo dentro de la misma transacción.
- IDs opacos generados con entropía criptográfica, desacoplados del título visible y reemplazables por generadores deterministas en tests.
- Migration forward-only v6 con nuevas columnas, normalización compatible de filas anteriores, resolución conservadora de activos duplicados e índice parcial único por estudiante.
- Adapters SQLite y fake in-memory actualizados con semántica equivalente, aislamiento de timestamps y conservación completa del historial.
- CLI `kelyro goal show|set|pause|resume`, defaults explícitos para nivel/threshold y salida humana para el objetivo actual y el historial.
- Documentación de datos, lifecycle, selección determinista, atomicidad, compatibilidad y límites en `docs/architecture/learning-goals.md`.

### Decisions

- `domain` es texto abierto validado, no un enum; el Student Core permanece general para tecnología, matemáticas y otros campos.
- El nivel inicial del objetivo siempre es entrada explícita y nunca se infiere desde la experiencia general del perfil ni se confunde con un diagnóstico futuro.
- `goal set` crea una identidad nueva y pausa el activo anterior; no sobrescribe ni elimina historial.
- `goal resume` elige determinísticamente el objetivo pausado actualizado más recientemente y pausa cualquier activo diferente.
- La política de único activo se protege tanto en el caso de uso transaccional como en SQLite y en el fake in-memory.
- Completion/archival existen como transiciones de dominio, pero no se añadió superficie CLI fuera de los cuatro comandos solicitados ni se adelantó onboarding, diagnóstico, curriculum o TUI adicional.

### Verification

- `GOCACHE=/tmp/kelyro-i02-step5-final-gocache go test ./...`
- `GOCACHE=/tmp/kelyro-i02-step5-final-gocache go vet ./...`
- `GOCACHE=/tmp/kelyro-i02-step5-final-gocache go run ./tools/quality all`, incluyendo E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- Smoke real `init → goal show → goal set → goal set → goal show → goal pause → goal resume` sobre un workspace temporal, verificando persistencia, reemplazo con pausa, historial y reanudación.
- Upgrade v5 → v6 con dos goals activos heredados, conservando ambas filas y dejando exactamente uno activo.
- `git diff --check`.

### Notes for next session

- El Paso 6 es el siguiente paso pendiente y requiere autorización explícita.
- Onboarding debe reutilizar `ProfileService` y `GoalLifecycleService`, persistir checkpoints resumibles y no adelantar todavía el diagnóstico del Paso 7.
