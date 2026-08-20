# I-02 Student & Learning Core — Progress Log

## Estado general

Current step: 10 (pending authorization)
Last completed step: 9
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
- Onboarding debe reutilizar `ProfileService` y `GoalLifecycleService`, persistir checkpoints resumibles y no adelantar todavía el diagnóstico del Paso 11.

## Step 06 — Framework de onboarding resumible

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Agregado de onboarding determinista y versionado con estados `not_started`, `in_progress`, `completed` y `cancelled`, current question, respuestas por ID estable y timestamps auditables.
- Flow core `core.onboarding@1` con las diez secciones requeridas, preguntas de texto/selección/revisión/confirmación y configuración sustituible para futuras extensiones de Learning Packs.
- Caso de uso presentation-neutral para `Show`, `Start`, `Submit`, `Back`, `Cancel` y `Confirm`, con validación y checkpoint durable en cada transición.
- Confirmación que reutiliza `ProfileService` y `GoalLifecycleService`, actualiza perfil, activa el goal definitivo solo al confirmar y conserva opt-in diagnóstico sin ejecutar ningún diagnóstico.
- Recuperación idempotente si el proceso cae después de activar el goal y antes del checkpoint final, evitando goals duplicados al reintentar.
- Migration forward-only v7 y adapters SQLite/in-memory para el draft versionado, con constraints de lifecycle, FK al estudiante y detección de payload corrupto.
- Experiencia TUI responsive con edición de texto, selección, back, abandono resumible, cancelación explícita, summary y confirmación, separada de las reglas del wizard.
- Documentación de flow, lifecycle, persistencia, recuperación y fronteras en `docs/architecture/resumable-onboarding.md`.

### Decisions

- Las preguntas comunes usan IDs estables y neutrales al dominio; un pack futuro puede añadir preguntas a un flow versionado sin mover validación o navegación a Bubble Tea.
- Solo las respuestas enviadas con Enter son checkpoints educativos; el buffer aún no enviado permanece como estado efímero de presentación.
- Escape abandona la pantalla sin cancelar y Ctrl+C usa el cierre normal de sesión; el último checkpoint de onboarding se persiste independientemente del session state TUI.
- La confirmación reutiliza los servicios existentes y es reintentable: un active goal que coincide exactamente con el draft confirmado se reconoce como el mismo resultado después de una caída.
- La selección de estrictitud se persiste como threshold del goal. Defaults globales, presets, custom ranges y precedencia siguen reservados para el Paso 7.
- El opt-in de diagnóstico se conserva únicamente como respuesta; no se implementó ni ejecutó el diagnóstico, curriculum, Exercise Engine o IA.
- El draft evolutivo usa JSON dentro de una fila lifecycle por estudiante; perfil y goals confirmados permanecen normalizados en sus tablas existentes.

### Verification

- Tests de dominio para transiciones, back, cancel/restart, input inválido y confirmación final.
- Tests application para resume persistente, cancel sin side effects, aplicación de perfil/goal y recuperación idempotente ante fallo del checkpoint final.
- Roundtrip SQLite, FK/schema v7, detección de JSON corrupto y persistencia entre reaperturas del workspace.
- Tests TUI para navegación, entrada de texto (incluyendo `q` sin salir), dispatch por application service, rendering responsive y golden views.
- `GOCACHE=/tmp/kelyro-i02-step6-final-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i02-step6-final-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-i02-step6-final-gocache go run ./tools/quality all`, incluyendo E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 7 es el siguiente paso pendiente y requiere autorización explícita.
- Mastery Threshold debe tomar la selección persistida por onboarding y añadir defaults, presets, custom range y precedencia sin convertir el threshold en una nota de examen.
- El diagnóstico determinista permanece pendiente para su paso autorizado posterior; `diagnostic.opt_in` ya está disponible como respuesta durable.

## Step 07 — Mastery Threshold y política de avance

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Política de dominio versionada `threshold-v1` con regla inclusiva `calculated mastery >= required threshold`, separada del cálculo de mastery y de cualquier unlock.
- Presets Relaxed 70%, Standard 80%, Strict 85% y Mastery 90%, más valores Custom dentro del rango inclusivo 50–99%.
- Settings durables con default del estudiante, override opcional del workspace, fuente efectiva y resolución determinista `pack > workspace > student`.
- Contrato futuro `PackMasteryOverride` que exige límites mínimo/máximo explícitos y rechaza valores fuera de esos límites o del rango global.
- Caso de uso `MasteryPolicyService` para consultar, cambiar default, establecer/limpiar override y resolver el valor efectivo sin depender de SQLite o presentación.
- Integración de onboarding: la estrictitud confirmada se guarda explícitamente como default del estudiante, además del threshold histórico del goal.
- Migration forward-only v8 y adapters SQLite/in-memory; upgrades conservan el threshold del goal activo válido como default y usan Standard para valores ausentes o legacy fuera de rango.
- CLI `kelyro mastery threshold`, `set`, `set-default` y `reset`, con porcentaje, mode, source, policy version y explicación humana.
- Nuevos goals restringidos a progression thresholds 0.50–0.99; el value object genérico `[0,1]` permanece compatible con datos históricos publicados.
- Fórmula, presets, rangos, precedencia, persistencia y límites documentados en `docs/architecture/mastery-threshold-policy.md`.

### Decisions

- El threshold nunca es una nota de examen: solo compara mastery ya calculado y no calcula evidencia, cambia exposure, desbloquea prerequisitos ni ejecuta assessments.
- `MasteryThreshold` conserva el rango genérico `[0,1]`; `MasteryRequirement` encapsula y versiona la política de progresión más estrecha 50–99%.
- `mastery threshold set PERCENT` escribe el override del workspace; `set-default` cambia el default del estudiante sin borrar el override y `reset` elimina solo el override.
- El CLI acepta porcentajes enteros 50–99; las APIs de dominio/application permiten decimales finitos dentro de 0.50–0.99.
- El pack override no implementa Learning Packs: define únicamente el contrato validado que un consumidor futuro podrá entregar al resolver.
- La migration v8 usa el goal activo válido para conservar la elección de onboarding en workspaces creados antes de este paso.

### Verification

- Tests de presets, custom boundaries, inputs inválidos, comparación inclusiva y transición temporal.
- Tests de precedencia student/workspace/pack y límites obligatorios del pack override.
- Tests application de defaults, set/clear, persistencia, rechazo de out-of-range e integración con confirmación de onboarding.
- Tests SQLite de roundtrip, constraints, FK y upgrade v7 → v8 conservando el threshold activo.
- Tests app/CLI de routing, parsing, output humano, comandos inválidos y rango de nuevos goals.
- Smoke real `init → mastery threshold → set 85 → set-default 70 → reset`, verificando persistencia y precedencia efectiva.
- `GOCACHE=/tmp/kelyro-i02-step7-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i02-step7-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-i02-step7-gocache go run ./tools/quality all`, incluyendo E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 8 es el siguiente paso pendiente y requiere autorización explícita.
- Curriculum consumption deberá consultar `MasteryPolicyService` como boundary de política; no debe duplicar presets o precedencia en nodos curriculares, CLI o TUI.
- Prerequisite unlocking y mastery calculation continúan pendientes para sus pasos autorizados posteriores.

## Step 08 — Contrato consumible de curriculum

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Contrato de dominio versionado `curriculum-consumption/v1` para definiciones curriculares inmutables por identidad, con jerarquía visible `phase → module → lesson → topic → concept` expresada mediante padres explícitos.
- Nodos con ID estable, tipo, título, descripción, orden entre hermanos, hints de display, metadata de estado y versión propia.
- Definición pedagógica de concepto con objetivos, prerequisitos tipados (`introduced` o `mastered`), dificultad general 1–5, esfuerzo estimado, `theory_required` y expectativas de assessment sin contenido generado.
- Validación global de IDs duplicados, tipos, metadata, padres ausentes o inválidos, órdenes negativos/duplicados, jerarquías cíclicas, prerequisitos desconocidos/duplicados y ciclos del knowledge graph.
- Canonicalización determinista por tipo, padre, orden e ID, con copia defensiva de slices y canonicalización de prerequisitos.
- Adapter `internal/infra/curriculumyaml` para decodificar exactamente un documento YAML desde `io.Reader`, con rechazo de campos desconocidos, mapping keys duplicadas y documentos extra.
- Fixture versionado `testdata/curricula/foundation-demo/curriculum.yaml`, neutral al ecosistema y explícitamente no presentado como pack investigado real.
- Tests de fixture válido, carga repetida, strict YAML, duplicate ID, tipos/padres/orden inválidos, hierarchy cycle, prerequisite dangling/cycle y fixture determinista de 1.500 conceptos.
- Contrato, escalas, determinismo, validaciones, dependencia y fronteras I-02/I-03/I-04 documentados en `docs/architecture/curriculum-consumption-contract.md`.

### Decisions

- La jerarquía es un modelo plano con `ParentID`: facilita validación, referencias estables y rendering sin convertir el orden visual en knowledge graph.
- Los prerequisitos viven en la definición del concepto y declaran el requisito futuro, pero este paso solo valida datos; traversal, explicaciones y desbloqueo permanecen en el Paso 9.
- No se añadieron migraciones ni curriculum instances por estudiante. La asociación durable a goal/source y el estado personalizado pertenecen al Paso 10.
- `order` es zero-based, no negativo y único entre hermanos; no hay límites artificiales de cantidad o profundidad curricular más allá de la jerarquía contractual.
- El dominio `internal/learning` permanece standard-library-only. YAML se aísla en un adapter que consume `io.Reader`.
- Se añadió `go.yaml.in/yaml/v3 v3.0.5` porque la librería estándar no decodifica YAML y el contrato requiere strict known-field decoding. Se eligió la línea estable v3; v4 continuaba en release candidate.
- Objectives y assessment expectations conservan el orden autoral; nodos y prerequisitos se canonicalizan porque su orden textual no define semántica.

### Verification

- Tests dirigidos de dominio y loader YAML: `GOCACHE=/tmp/kelyro-i02-step8-target-gocache go test ./internal/learning ./internal/infra/curriculumyaml`.
- `GOCACHE=/tmp/kelyro-i02-step8-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i02-step8-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-i02-step8-gocache go run ./tools/quality all`, incluyendo E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- `git diff --check`.
- Revisión de imports: `internal/learning` no importa YAML, SQLite, Bubble Tea ni adapters de presentación.

### Notes for next session

- El Paso 9 es el siguiente paso pendiente y requiere autorización explícita.
- El Prerequisite Engine debe consumir `ConceptPrerequisite.Requirement`, `MasteryPolicyService` y estados del estudiante sin hacer traversal dentro de repositorios ni del adapter YAML.
- Persistencia de Curriculum Instance y asociación a goal/source siguen reservadas para el Paso 10.

## Step 09 — Knowledge Graph y Prerequisite Engine

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- `KnowledgeGraph` in-memory sobre un `Curriculum` validado, sin dependencias de DB, YAML, presentación u OS.
- Índices defensivos para conceptos, prerequisitos directos y dependientes, con `GetPrerequisites`, `GetDependents`, `Ancestors`, `CanIntroduce`, `MissingPrerequisites` y `TopologicalOrder`.
- Orden topológico determinista mediante Kahn + priority queue por stable ID, independiente del orden visual, maps, títulos o YAML.
- Política educativa versionada `prerequisite-v1`, con AND sobre prerequisitos directos, missing state bloqueante y conceptos raíz introducibles.
- Semántica separada: `introduced` exige `exposure != not_seen`; `mastered` usa únicamente calculated mastery contra `threshold-v1` inclusivo.
- `StudentStateSnapshot` validado, con rechazo de estados inválidos, duplicados o pertenecientes a estudiantes diferentes y copias defensivas.
- Decisiones explicables con checks estructurados, reason codes estables, score/exposure observados, threshold requerido y resumen humano.
- Validación auditable de `ResolvedMasteryThreshold`, incluyendo requirement, source y policy version.
- `application.PrerequisiteService`, que resuelve política mediante `MasteryPolicyService`, carga estados una sola vez y delega traversal al dominio.
- Tests de chain, diamond, múltiples prerequisitos, root concept, missing state, separación exposure/mastery, cycle, unknown concept, threshold boundary, ordering determinista, snapshot inválido y cadena de 3.000 conceptos.
- Diseño, fórmula, complejidad, explainability y boundary de persistencia documentados en `docs/architecture/knowledge-graph-prerequisite-engine.md`.

### Decisions

- El knowledge edge apunta prerequisite → dependent; `ParentID` y `order` continúan siendo solo jerarquía/UX.
- `CanIntroduce` evalúa prerequisitos directos declarados. `Ancestors` expone el cierre transitivo de forma independiente y foundations-first.
- Mastery y exposure conservan independencia: un requisito de mastery no exige un exposure label específico, y un requisito de exposure no usa el score.
- El graph engine recibe `ResolvedMasteryThreshold`; no conoce presets, precedencia, goal, pack loading ni cálculo de mastery.
- La evaluación no muta student state ni curriculum y no implementa overrides manuales de unlock.
- El application service realiza un único `ListByStudent` por evaluación; no existen N+1 queries ni traversal dentro de repositories.
- No se añadieron migrations, dependencias externas, CLI/TUI o Curriculum Instances; estas últimas permanecen reservadas para el Paso 10.

### Verification

- Tests dirigidos: `GOCACHE=/tmp/kelyro-i02-step9-target-gocache go test ./internal/learning ./internal/learning/application`.
- `GOCACHE=/tmp/kelyro-i02-step9-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i02-step9-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-i02-step9-gocache go run ./tools/quality all`, incluyendo E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- `git diff --check`.
- Revisión de imports: `prerequisite_graph.go` depende solo de la librería estándar y del mismo paquete de dominio.

### Notes for next session

- El Paso 10 es el siguiente paso pendiente y requiere autorización explícita.
- Curriculum Instance debe persistir curriculum ID/version, goal, source kind, lifecycle y student-state isolation sin mutar la definición ni duplicar evidence.
- Reutilizar `KnowledgeGraph` y `PrerequisiteService`; no reconstruir traversal, threshold precedence o unlock explanations dentro de SQLite/CLI/TUI.
