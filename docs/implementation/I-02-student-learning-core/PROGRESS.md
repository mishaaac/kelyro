# I-02 Student & Learning Core — Progress Log

## Estado general

Current step: complete
Last completed step: 33
Current release: v0.1.0-alpha.3 (published prerelease)
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

## Step 10 — Curriculum Instance y estado personalizado

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Agregados `CurriculumInstance` e `InstanceConceptState` que separan definición inmutable, identidad learner/goal y progreso sparse por instancia.
- Source kinds `fixture`, `import` y `pack`, lifecycle durable `active|paused|completed|archived`, referencias exactas `curriculum_id/version` y timestamps UTC validados.
- Estado de concepto con exposure, mastery, first/last seen, mastered/review-due, flags manuales opacos y proyección explícita al snapshot del Prerequisite Engine sin duplicar evidence.
- Huella canónica SHA-256 sobre el contrato curricular completo; instalación idempotente de contenido idéntico y conflicto si una misma referencia/version intenta cambiar definición.
- `CurriculumInstanceService` transaccional para crear, obtener, listar y persistir estado, verificando goal activo, ownership, pertenencia del concepto, cronología y protección contra updates regresivos.
- Inicialización lazy: crear una instancia no escribe estados; el primer acceso válido materializa únicamente ese concepto como `not_seen`.
- Puertos y adapters in-memory/SQLite para definiciones, instancias y estados, con aislamiento por `curriculum_instance_id` y protección de duplicados lógicos.
- Migration forward-only v9 con `curriculum_definition_fingerprints`, `learner_curriculum_instances` y `learner_curriculum_concept_states`, sin modificar migrations ni tablas publicadas.
- `learningdb` expone el servicio workspace-scoped y conserva instancia, versión y estado al cerrar/reabrir la DB.
- `PrerequisiteService` ahora exige instance ID, valida que el graph corresponda a la misma versión y carga exactamente un snapshot mediante `ListByInstance`.
- Diseño, compatibilidad, política lazy, aislamiento y preparación de migration futura documentados en `docs/architecture/learner-curriculum-instances.md`.

### Decisions

- La tabla v4 `curriculum_instances` conserva su semántica histórica de catálogo de definiciones; v9 añade tablas `learner_*` en vez de reinterpretar o modificar una migration publicada.
- Los estados son lazy para evitar escrituras O(n) al crear curricula grandes y representar conceptos intactos como ausencia de fila; `States` lista solo filas materializadas.
- El tuple `(student, goal, curriculum_id, curriculum_version)` es único, pero versiones diferentes pueden coexistir como instancias aisladas para permitir una migration curricular explícita futura.
- Una migration de versión futura deberá comparar definiciones, mapear stable concept IDs y decidir transferencias; este paso no copia ni mezcla estados automáticamente.
- Upgrade v8 → v9 conserva `student_concept_states` pero no lo asigna a instancias: las filas legacy no contienen provenance de goal/curriculum/version y cualquier inferencia sería ambigua.
- Evidence permanece en su agregado append-only. Instance state guarda solo la proyección actual y los flags manuales no implementan unlock overrides.
- La huella protege toda la definición aunque el schema compacto publicado solo materialice los campos de consulta preexistentes; una definición legacy sin huella no se adopta silenciosamente.
- No se añadió CLI/TUI, importer, Learning Pack, Curriculum Compiler, Exercise Engine, diagnóstico ni cálculo de mastery.

### Verification

- Tests de dominio para huella canónica, mutación de metadata, default lazy y reglas temporales/flags.
- Tests application para create, protección de duplicados y definición inmutable, lazy materialization, aislamiento entre versiones y graph/version mismatch.
- Tests SQLite para schema v9 y upgrade v8 → v9 preservando estado legacy sin inventar instancias.
- Reapertura real con `foundation-demo`: instancia, referencia versionada, mastery, exposure y flags sobreviven al cierre del workspace.
- `GOCACHE=/tmp/kelyro-i02-step10-gocache GOMODCACHE=/tmp/kelyro-i02-step10-modcache go test ./...`.
- `GOCACHE=/tmp/kelyro-i02-step10-gocache GOMODCACHE=/tmp/kelyro-i02-step10-modcache go vet ./...`.
- `GOCACHE=<workspace>/.step10-gocache GOTMPDIR=<workspace>/.step10-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go run ./tools/quality all`, incluyendo tests, E2E Foundation, `go test -race ./...`, build y smoke checks de CLI. Los caches workspace-scoped fueron eliminados después del gate.
- `git diff --check`.

### Notes for next session

- El Paso 11 es el siguiente paso pendiente y requiere autorización explícita.
- El diagnóstico debe escribir evidencia determinista y asociarse a una Curriculum Instance explícita; no debe volver a usar el estado legacy global ni adelantar el Mastery Engine del Paso 13.
- La migration automática entre versiones curriculares permanece futura; el diagnóstico no debe copiar progreso entre instancias.

## Step 11 — Diagnóstico inicial determinista

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Contrato de diagnóstico versionado con `Diagnostic`, secciones, cuatro tipos genéricos de ítem, intentos resumibles, observaciones calificadas, resultados y estimaciones por concepto.
- Evaluadores deterministas para single choice, multiple choice exacto, short answer normalizado y self-report calibrado, sin code runner, LLM ni respuestas generadas.
- Política explícita `diagnostic-scoring/v1`: evidencia objetiva con peso `1.0`, self-report con peso `0.25`, confidence separada y conceptos sin observación reportados como `unknown`.
- Branching adaptativo puro que corta descendientes de un prerequisito fundamental fallido, continúa respuestas positivas y omite preguntas redundantes al acumular evidencia objetiva suficiente.
- Caso de uso workspace-scoped para start/resume/submit/skip/result, con creación atómica de `EvidenceDiagnostic`, reanudación idempotente y protección por fingerprint de la definición.
- Privacidad por diseño: la respuesta cruda se evalúa en memoria y no se persiste; el intento conserva solo item, concepto, score, evidence ID y timestamp.
- Migration forward-only v10 con intentos ligados a una Curriculum Instance y observaciones normalizadas con FK a conceptos y evidencia; adapters SQLite e in-memory con terminales inmutables.
- Fixture estricto y determinista `foundation-demo/diagnostic.json`, asociado exactamente a `foundation-demo@1.0.0` y cubriendo los cuatro tipos de ítem.
- Documento arquitectónico con fórmulas, lifecycle, branching, evidencia, persistencia, privacidad y fronteras en `docs/architecture/deterministic-initial-diagnostic.md`.

### Decisions

- Diagnostic result expresa únicamente estimated mastery y confidence; el Paso 11 no escribe mastery, exposure ni `InstanceConceptState`, y no masteriza automáticamente ningún concepto.
- La confidence v1 es `min(1, sum(weights)/2)`, por lo que una respuesta objetiva perfecta queda en confidence `0.5` y un self-report perfecto en `0.125`.
- Una definición queda inmutable por su fingerprint SHA-256; reanudar con el mismo ID/version pero contenido modificado falla como invalid state.
- Solo existe un intento inicial por `(student, curriculum instance, diagnostic ID/version)`; `Start` recupera ese intento y un estado terminal no puede reabrirse o mutarse.
- Skip solo es válido antes de responder; un intento parcial se conserva para resume en lugar de descartar evidencia ya registrada.
- Evidence conserva su agregado append-only existente; la relación explícita con Curriculum Instance se registra en provenance y mediante `diagnostic_observations → diagnostic_attempts`.
- La integración automática onboarding → diagnostic → Student State permanece reservada para el Paso 12.

### Verification

- Tests de dominio para evaluadores, score/confidence/unknown, lifecycle, fingerprint, branching transitivo y omisión de redundancia.
- Tests application para complete, skipped, partial, resume, evidence creation, curriculum mismatch y definición mutada.
- Tests del loader estricto para estabilidad del fixture y presencia de single choice, multiple choice, short answer y self-report.
- Tests SQLite para roundtrip normalizado, FK entre observación y evidence, terminales inmutables, índices y upgrade aditivo v9 → v10 conservando datos.
- Reapertura real de workspace con intento parcial `foundation-demo`, verificando el mismo attempt, observación y siguiente ítem después de cerrar/abrir la DB.
- `GOCACHE=<workspace>/.step11-gocache GOTMPDIR=<workspace>/.step11-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go test ./...`.
- `GOCACHE=<workspace>/.step11-gocache GOTMPDIR=<workspace>/.step11-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go vet ./...`.
- `GOCACHE=<workspace>/.step11-gocache GOTMPDIR=<workspace>/.step11-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go run ./tools/quality all`, incluyendo tests, E2E Foundation, `go test -race ./...`, build y smoke checks de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 12 es el siguiente paso pendiente y requiere autorización explícita.
- El flujo integrado debe consumir `DiagnosticService`, respetar la decisión de opt-in del onboarding y marcar setup completo solo después de inicializar de forma consistente la Curriculum Instance y Student State.
- No recalcular estimated mastery dentro de TUI/CLI ni convertirlo directamente en mastery confirmado; la política de inicialización debe permanecer explícita y transaccional.

## Step 12 — Integrated onboarding, diagnostic, and learner-state initialization

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Agregado durable `LearnerSetup` con estados `awaiting_onboarding`, `awaiting_diagnostic`, `initializing` y `completed`; `setup_completed_at` es la única señal autoritativa de finalización.
- `LearnerSetupService` coordina los casos de uso existentes de perfil, goal, mastery, onboarding, Curriculum Instance y diagnóstico sin duplicar sus reglas.
- Reanudación automática del onboarding o intento diagnóstico exacto y recuperación idempotente desde `initializing` después de una falla.
- Fixture de desarrollo/demo `foundation-demo@1.0.0` encapsulado fuera del core y verificado contra los fixtures versionados canónicos.
- Inicialización transaccional de todos los conceptos de la instancia con exposure `not_seen` y mastery `0`, sin convertir estimated diagnostic mastery en mastery confirmado.
- Migration forward-only v11 y repositorios SQLite/in-memory para el checkpoint, con constraints de lifecycle, FKs y persistencia al reabrir el workspace.
- TUI de primer inicio que abre o retoma setup, muestra el resumen antes de confirmar, permite completar o saltar el diagnóstico y habilita la learning path únicamente al terminar.
- CLI `setup status` y reset seguro para desarrollo/demo con confirmación explícita; el reset conserva perfil, historial de goals y datos Foundation.
- E2E del binario para onboarding completo, creación de goal/curriculum/state, estado CLI persistente y reapertura de la TUI.
- Diseño y fronteras documentados en `docs/architecture/integrated-learner-setup.md`.

### Decisions

- Setup es un checkpoint de orquestación, no un aggregate alternativo para onboarding, goal, diagnóstico o curriculum.
- Onboarding completado no equivale a setup completo; la Curriculum Instance y todos sus estados iniciales deben existir antes de escribir `setup_completed_at`.
- La materialización y el transition final comparten una transacción. Una falla conserva un checkpoint recuperable, no deja estados parciales visibles y `Show` puede reintentar.
- El diagnóstico conserva evidence y estimaciones del Paso 11, pero el estado inicial permanece sin aprendizaje observado. La combinación de evidencias pertenece al Mastery Engine del Paso 13.
- El fixture es solo el bridge determinista de I-02 para desarrollo/demo; no implementa selección personalizada ni adelanta I-04.
- Reset elimina únicamente el subgrafo creado por este setup; no borra perfil, goals históricos, tablas Foundation ni instancias no asociadas.

### Verification

- Tests de dominio para invariantes y transitions de `LearnerSetup`.
- Tests application para opt-out, diagnóstico parcial/reanudado, rollback/recovery y gating/preservación del reset.
- Tests SQLite de migration v10 → v11 aditiva y reapertura/reset sobre una DB real.
- Tests de app, CLI y TUI para routing, render, confirmación, auto-start y edición/diagnóstico.
- `GOCACHE=<workspace>/.step12-gocache GOTMPDIR=<workspace>/.step12-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go test ./...`.
- `GOCACHE=<workspace>/.step12-gocache GOTMPDIR=<workspace>/.step12-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go test -tags=e2e ./tests/e2e`.
- `GOCACHE=<workspace>/.step12-gocache GOTMPDIR=<workspace>/.step12-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go vet ./...`.
- `GOCACHE=<workspace>/.step12-gocache GOTMPDIR=<workspace>/.step12-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go run ./tools/quality all`, incluyendo E2E, race, build y smoke checks de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 13 es el siguiente paso pendiente y requiere autorización explícita.
- El Mastery Engine debe combinar Evidence con una fórmula versionada; no debe inferir que los estados iniciales `not_seen/0` son evidencia negativa.
- La selección de curriculum personalizada sigue reservada para I-04; no extender el fixture bridge como UX final.

## Step 13 — Evidence Model y Mastery Engine v1

Status: completed
Date: 2026-08-19
Release: unreleased

### Delivered

- Modelo append-only de Evidence ampliado con nueve tipos semánticos, score, confidence, independence, dificultad normalizada, source ref, timestamp UTC y versión del algoritmo productor.
- Política explícita `mastery-v1`: media ponderada por fuerza del tipo, confidence, independence y dificultad, sin decay temporal oculto.
- Resultado que distingue `unknown` de fallo observado, comparación inclusiva contra threshold y breakdown auditable por cada evidencia.
- Orden canónico por timestamp e ID para recalculación determinista, incluso con timestamps iguales o repositorios basados en maps.
- `MasteryCalculationService` presentation-neutral para calcular y explicar el resultado leyendo únicamente el historial de Evidence.
- Diagnóstico actualizado para emitir `diagnostic_objective` o `diagnostic_self_report` con metadata y versión `diagnostic-scoring/v1`.
- Migration forward-only v12 con metadata de mastery, constraints de rango y clasificación determinista de filas legacy sin modificar la migration v4 publicada.
- Fórmula, pesos, compatibilidad, determinismo, explainability y frontera con retention/progression documentados en `docs/architecture/mastery-v1.md`.

### Decisions

- `mastery-v1` usa `Σ(score × weight) / Σ(weight)`; el peso es base por tipo × confidence × factor de independence × factor de dificultad.
- Un concepto sin evidencia devuelve `Known=false`; una evidencia válida con score cero devuelve `Known=true` y mastery cero.
- La antigüedad no cambia el peso en v1. El Retention Engine del Paso 18 deberá introducir cualquier efecto temporal mediante una política explícita y versionada.
- Independence conserva un factor mínimo de 0.25 para mantener visible una observación asistida, mientras que dificultad neutral `0.5` no altera el peso.
- El tipo semántico nuevo se persiste junto a la categoría gruesa publicada en v4; así se amplía el contrato sin reconstruir la tabla ni romper las FKs de diagnóstico.
- Las filas legacy de diagnóstico se clasifican conservadoramente como objetivas porque el schema anterior no conservaba el tipo del ítem; la provenance y el score originales permanecen intactos.
- Este paso no persiste mastery calculado, cambia exposure ni desbloquea prerequisitos. Esas mutaciones pertenecen al Paso 14.

### Verification

- Tests exhaustivos de no evidence, fallo observado, evidencia única, conflicto, confidence baja/alta, independence, dificultad, boundary threshold, orden determinista, timestamps iguales y evidencia malformed/mismatched/duplicada.
- Tests application de cálculo y explicación para mastery conocido y unknown.
- Tests SQLite de roundtrip de todos los metadatos, constraints y upgrade v11 → v12 preservando y clasificando evidencia legacy.
- `GOCACHE=<workspace>/.step13-gocache GOTMPDIR=<workspace>/.step13-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go test ./...`.
- `GOCACHE=<workspace>/.step13-gocache GOTMPDIR=<workspace>/.step13-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go vet ./...`.
- `GOCACHE=<workspace>/.step13-gocache GOTMPDIR=<workspace>/.step13-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go run ./tools/quality all`, incluyendo E2E, race, build y smoke checks de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 14 es el siguiente paso pendiente y requiere autorización explícita.
- Progression deberá consumir `MasteryCalculationService` y `MasteryPolicyService`, actualizar el `InstanceConceptState` correcto de forma atómica y conservar Evidence inmutable.
- No duplicar la fórmula, los pesos ni la comparación de threshold dentro de SQLite, CLI/TUI o el Prerequisite Engine.

## Step 14 — Concept State y Progression Policy

Status: completed
Date: 2026-08-20
Release: unreleased

### Delivered

- Política pura y versionada `progression-v1` que conecta mastery conocido, threshold efectivo y `InstanceConceptState` sin dependencias de persistencia o presentación.
- Transiciones explícitas desde Evidence hacia `introduced`, `learning`, `practicing` y `mastered`, conservando la etapa de aprendizaje más avanzada observada.
- Mastery reversible: una recalculación bajo threshold devuelve un concepto dominado a `practicing` sin borrar su `MasteredAt` histórico.
- Separación de retention: `review_due` y `ReviewDueAt` se preservan aunque cambie el mastery; sus transitions siguen reservadas para los pasos de Retention/Review.
- `ProgressionService.RecordEvidence` agrega Evidence, ejecuta `mastery-v1`, actualiza el state de la Curriculum Instance y deriva unlocks dentro de una sola unidad de trabajo.
- `ProgressionService.Recalculate` reproduce el mismo flujo sin reescribir Evidence, preparando upgrades futuros del algoritmo.
- Resultado explicable por dependiente con elegibilidad anterior, decisión completa de prerequisitos y señal `NewlyEligible`; no se persiste ningún unlock duplicado.
- Validación reforzada de `MasteryCalculation` antes de aplicar progression, incluyendo ownership, versión, totales, contribuciones únicas y orden canónico.
- Política, atomicidad, timestamps, lifecycle, unlock derivado y fronteras documentados en `docs/architecture/concept-state-progression-v1.md`.

### Decisions

- `KnowledgeGraph.EvaluateIntroduction`/`CanIntroduce` permanece como única fuente de verdad para unlock; no se añadió tabla, flag ni cache de elegibilidad.
- El servicio evalúa solo dependientes directos del concepto actualizado. Los descendientes transitivos continúan bloqueados por sus propios prerequisitos hasta que éstos cambien.
- Threshold equality masteriza mediante la política inclusiva `threshold-v1`; presets y precedencia no se duplican en progression.
- Evidence es longitudinal por student/concept y state permanece aislado por Curriculum Instance. Evidence anterior a la instancia conserva su timestamp original, mientras su proyección first/last seen se limita al inicio de la instancia.
- Un cálculo unknown deja el state intacto y nunca convierte ausencia de evidencia en score observado cero.
- Evidence futura, ownership incorrecto, duplicate ID o cualquier fallo posterior causa rollback de Evidence y state.
- No fue necesaria una migration: v12 ya persiste Evidence y el schema v9 ya persiste `InstanceConceptState`; SQLite no recibe fórmulas educativas.
- No se añadieron CLI/TUI, Mistake Memory, Retention, Review scheduling ni Exercise Engine.

### Verification

- Tests de dominio para threshold exacto y just-below, stages por tipo de Evidence, furthest exposure, reversibilidad, unknown, review_due, evidence histórica y timestamp futuro.
- Tests application para unlock nuevo, prereq todavía ausente, múltiples prerequisitos, relock, recalculation, ownership y rollback por duplicate/future Evidence.
- `GOCACHE=<workspace>/.step14-gocache GOTMPDIR=<workspace>/.step14-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go test ./...`.
- `GOCACHE=<workspace>/.step14-gocache GOTMPDIR=<workspace>/.step14-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go vet ./...`.
- `GOCACHE=<workspace>/.step14-gocache GOTMPDIR=<workspace>/.step14-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go run ./tools/quality all`, incluyendo E2E, race, build y smoke checks de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 15 es el siguiente paso pendiente y requiere autorización explícita.
- Mistake Memory deberá registrar/deduplicar errores mediante sus propios casos de uso; cualquier Evidence asociada deberá entrar por `ProgressionService` cuando corresponda actualizar mastery/state.
- No persistir unlocks ni duplicar `progression-v1`, `mastery-v1`, `threshold-v1` o traversal del Knowledge Graph en Mistake Memory.

## Step 15 — Mistake Memory persistente

Status: completed
Date: 2026-08-20
Release: unreleased

### Delivered

- Modelo de dominio genérico para patrones de error con `mistake_key` estable, categorías neutrales al tema, resumen y source acotados, primera/última aparición, ocurrencias y estados `recent`, `reinforced` y `resolved`.
- Lifecycle explícito para incrementar, reforzar, resolver y reabrir tras recurrencia, preservando `first_seen_at` y sin borrar resoluciones históricas.
- Historial inmutable `MistakeEvent` para observaciones, refuerzos y resoluciones, ordenado canónicamente por timestamp e ID.
- `MistakeMemoryService` presentation-neutral como único límite de escritura para evaluadores y clasificadores futuros, con dedupe, transiciones e historial atómicos mediante `UnitOfWork`.
- Adapter in-memory determinista y adapter SQLite con consultas por estudiante, concepto, ID y clave; dedupe único e historial durable.
- Migration forward-only v13 que amplía `mistakes`, crea `mistake_events`, índices y guards, y migra filas legacy sin modificar migrations publicadas ni reescribir su descripción original.
- Store workspace-scoped con persistencia verificada entre reaperturas y CLI de inspección `kelyro mistakes` / `kelyro mistakes show <id>`.
- Contrato, límites de privacidad, lifecycle, compatibilidad y fronteras con Evidence/Retention documentados en `docs/architecture/mistake-memory.md`.

### Decisions

- La identidad de dedupe es `(student_id, concept_id, mistake_key)`; reutilizar una clave con categoría o summary distintos es conflicto para evitar mezclar patrones no equivalentes.
- Categoría y summary son clasificación inmutable del patrón; cada recurrencia solo incrementa el contador, avanza `last_seen_at`, actualiza el source más reciente y reabre el estado.
- Refuerzo no cuenta como nueva ocurrencia. Un error resuelto requiere una recurrencia real antes de poder reforzarse otra vez.
- El agregado es la proyección actual y `mistake_events` es la historia auditable; ambos cambian dentro de una transacción.
- Summaries se limitan a 500 bytes, keys a 128 y source refs a 256. No se almacenan respuestas completas, código grande ni payloads de ejercicios.
- Legacy usa `unknown`, una clave `legacy:<id>` acotada con fallback por row para IDs extraordinariamente largos, un summary acotado separado y eventos históricos sintetizados; IDs, ownership, timestamps y descripción publicada permanecen intactos.
- El runner SQLite renueva el timeout configurado por migration, manteniendo el límite individual y la cancelación del caller sin aplicar un único deadline acumulativo a toda la historia forward-only.
- Mistake Memory no crea Evidence ni modifica mastery, Concept State, unlocks, retention o reviews. Un evaluator que necesite afectar progresión debe llamar separadamente a `ProgressionService`.

### Verification

- Tests de dominio para creación, límites, cronología, incrementos, refuerzo, resolución, resolución duplicada y reapertura por recurrencia.
- Tests application para dedupe, colisión de key, incrementos, resolve/reopen, historia canónica, concepto desconocido y rollback atómico.
- Tests SQLite para roundtrip, unique dedupe, eventos, constraints y upgrade v12 → v13, incluyendo compatibilidad con ID/description legacy largos.
- Test de persistencia workspace-scoped entre reaperturas y tests app/CLI para list/show y parsing inválido.
- Smoke real `init → mistakes` sobre un workspace temporal.
- `GOCACHE=<workspace>/.step15-gocache GOTMPDIR=<workspace>/.step15-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go test ./...`.
- `GOCACHE=<workspace>/.step15-gocache GOTMPDIR=<workspace>/.step15-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go vet ./...`.
- `GOCACHE=<workspace>/.step15-gocache GOTMPDIR=<workspace>/.step15-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go run ./tools/quality all`, incluyendo E2E, race, build y smoke checks de CLI.
- `GOCACHE=<workspace>/.step15-gocache GOTMPDIR=<workspace>/.step15-gotmp GOMODCACHE=/tmp/kelyro-i02-step10-modcache go test -race ./internal/infra/learningdb -count=2`.
- `git diff --check`.

### Notes for next session

- El Paso 16 es el siguiente paso pendiente y requiere autorización explícita.
- Study Sessions deberá reutilizar los conceptos y Curriculum Instance persistidos, sin inferir actividad detallada desde Mistake Memory.
- No adelantar Retention, Review scheduling, warm-ups, analytics o el Exercise Engine completo.

## Step 16 — Lifecycle persistente de Study Sessions

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Agregado `StudySession` con ownership explícito de student, goal y Curriculum Instance, estados `active`, `completed`, `interrupted` y `recovered`, timestamps UTC, duración activa y conteo de actividades significativas.
- Política pura y versionada `study-session-v1`, timeout idle configurable con default de 15 minutos y configuración capturada por sesión para no reinterpretar sesiones abiertas.
- Acumulación determinista que limita cada intervalo al idle timeout, sin registrar keypresses, navegación, heartbeats ni actividad externa a Kelyro.
- `StudySessionLifecycleService` transaccional para start, current, record activity, stop, interrupt y recovery; duplicate active reciente es conflicto y un active obsoleto se recupera antes de crear su reemplazo.
- Adapters in-memory y SQLite con create/get/active/list/update, identidad inmutable, orden canónico y enforcement de una sola sesión activa por workspace learner.
- Migration forward-only v14 con tabla lifecycle, FK compuesta exacta a `(curriculum instance, student, goal)`, constraints y partial unique index, conservando intactas las tablas publicadas v4 de sesiones completadas legacy.
- Persistencia workspace-scoped verificada entre reaperturas/crash y configuración pública de idle timeout para nuevas sesiones.
- CLI `kelyro session status|stop`, incluyendo un resultado explícito cuando no existe active session.
- Arquitectura, fórmula, recovery, compatibilidad, privacidad y frontera TUI documentadas en `docs/architecture/study-session-lifecycle.md`.

### Decisions

- `recovered` es terminal: una sesión se considera obsoleta solo después del boundary estricto y termina en `last_activity_at + idle_timeout`; una caída reciente conserva el mismo active para reanudarlo.
- `RecordActivity` representa únicamente eventos educativos significativos y es el único que incrementa `activity_count`; stop/recovery acumulan tiempo acotado sin inventar actividad.
- El timeout persistido en la sesión es autoritativo para todo su lifecycle. Cambiar el default afecta solo sesiones futuras.
- `completed` significa fin explícito del periodo de estudio, nunca completion de lesson/concept. Cerrar la TUI tampoco completa lecciones implícitamente.
- Foundation app session y Study Session permanecen separados. Como todavía no existe una pantalla real de lesson/exercise, abrir Home/Roadmap, onboarding, diagnóstico o la TUI no inicia tiempo de estudio; la futura superficie educativa deberá llamar explícitamente al servicio con goal e instance exactos.
- No se migraron filas v4 a la tabla lifecycle porque carecen de Curriculum Instance autoritativa; las filas legacy se conservan y Step 17 podrá combinarlas como historia sin fabricar ownership.
- No se implementaron Study History, rangos de time tracking, Retention, reviews, warm-ups, streaks, analytics ni Exercise Engine.

### Verification

- Tests de dominio para start/complete/interrupt, transición inválida, boundary idle, acumulación acotada, activity count y crash recovery.
- Tests application para duplicate active, start/stop, múltiples intervalos, recovery reciente/obsoleto y reemplazo atómico.
- Tests SQLite para roundtrip, unique active, canonical listing, migration v13 → v14 y preservación de sesiones legacy sin filas fabricadas.
- Test de persistencia workspace-scoped a través de múltiples reaperturas, recuperación de crash y reemplazo durable.
- Tests app/CLI para `session status|stop`, no-active, parsing inválido y salida humana.
- `GOCACHE=/tmp/kelyro-step16-test-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step16-vet-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-step16-quality-gocache go run ./tools/quality all`, incluyendo E2E, race, build y smokes de CLI.
- Smoke real `init → session status` sobre un workspace temporal.
- `git diff --check`.

### Notes for next session

- El Paso 17 es el siguiente paso pendiente y requiere autorización explícita.
- Study History deberá consumir sessions terminales y eventos educativos significativos sin duplicar el Audit Trail técnico.
- Time Tracking debe usar `active_duration` ya acotado por `study-session-v1`, conservar UTC y presentar rangos/timezones sin recontar idle bruto.
- No adelantar Retention, spaced repetition, warm-ups, streaks, achievements, analytics o el Exercise Engine completo.

## Step 17 — Study History y Time Tracking

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Contrato inmutable y versionado `study-history-v1` para los ocho eventos educativos del plan, con source semántico idempotente, timestamps UTC y scopes opcionales de goal, Curriculum Instance y concepto.
- Registro transaccional de Evidence, introducción/mastery de concepto, finalización de diagnóstico y cierre explícito de Study Session; onboarding completado se registra de forma idempotente y recuperable.
- Repositorios in-memory y SQLite con record/get/list, orden canónico newest-first, filtros `[from,to)`, conflictos semánticos y validación de filas persistidas.
- Migration forward-only v15 con tabla e índices de timeline/concept y backfill conservador de onboarding, diagnóstico, Evidence, concept state, reviews, sesiones lifecycle/legacy y achievements ya autoritativos.
- Política explícita `time-tracking-v1` que agrega únicamente `StudySession.ActiveDuration` para today/week/month/total, con conteos de sesión y anchors definidos, sin reconstruir wall time ni idle bruto.
- Ventanas de calendario según la timezone IANA del perfil, semanas desde lunes, almacenamiento/queries UTC y construcción DST-aware para días de 23/25 horas.
- Breakdown conservador por concepto y módulo: atribuye la sesión completa solo cuando los eventos observados dentro de su instance/intervalo son inequívocos; ante ausencia o ambigüedad no fabrica un reparto.
- Application/store workspace-scoped y CLI `kelyro history`, `kelyro history --today` y `kelyro time`, con timestamps locales y explicación visible de la política.
- Arquitectura, semántica, compatibilidad legacy, privacidad y fronteras documentadas en `docs/architecture/study-history-time-tracking.md`.

### Decisions

- Study History contiene hechos educativos reconocibles y no copia comandos, migrations, diagnósticos o privacy denials del Audit Trail técnico de I-01.
- La clave `(student, event_type, source_id)` hace retries idempotentes; reutilizar el mismo origen con contenido distinto es conflicto.
- Los rangos usan inicio inclusivo y fin exclusivo. Today/week/month se construyen con aritmética de calendario local y se convierten a UTC antes de consultar.
- Una sesión terminal pertenece al periodo de su `ended_at`; una sesión active usa `last_activity_at`. Anchors futuros se excluyen.
- Las sesiones legacy v4 se muestran como historia mediante backfill, pero no entran en los totales porque no tienen `active_duration` acotado ni Curriculum Instance autoritativa.
- El lookup de módulo recorre la jerarquía curricular genérica `concept → topic → lesson → module`; no introduce supuestos de lenguaje, ecosistema o materia.
- Reviews y achievements existentes se proyectan durante migration, pero Step 17 no implementa sus políticas/lifecycles futuros ni Retention, scheduling, streaks, analytics o Exercise Engine.

### Verification

- Tests de dominio para validación/versiones, scopes, periodos, semana desde lunes y bordes DST spring/fall de 23/25 horas.
- Tests application para orden cronológico, filtro today por timezone, rangos today/week/month/total, active duration, atribución inequívoca e integración atómica con onboarding, diagnóstico, progression y Study Sessions.
- Tests SQLite para roundtrip, idempotencia/conflicto, filtro temporal, UTC, migration v14 → v15 y backfill de Evidence/sesiones.
- Tests app/CLI para dispatch, parsing, `--today`, errores de uso, timestamps locales y salida de breakdown/política.
- `GOCACHE=/tmp/kelyro-i02-step17-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i02-step17-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-i02-step17-quality-gocache go run ./tools/quality all`, incluyendo E2E, race, build y smokes de CLI.
- Smoke real `init → history → history --today → time` sobre un workspace temporal nuevo.
- `git diff --check`.

### Notes for next session

- El Paso 18 es el siguiente paso pendiente y requiere autorización explícita.
- Retention v1 deberá consumir Evidence/mastery/reviews sin reinterpretar `time-tracking-v1` ni modificar el historial educativo inmutable.
- No adelantar spaced repetition, warm-ups, streaks, achievements, analytics, daily plans o el Exercise Engine completo.

## Step 18 — Retention Model v1

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Política pura y versionada `retention-v1` que diferencia mastery histórico de fuerza de recuerdo estimada y consume `mastery-v1`, evidencia inmutable y un instante UTC explícito.
- Estado durable por concepto con último recall exitoso, última práctica, conteos de reviews exitosas/fallidas, estabilidad, strength, estados `fresh|stable|weakening|due|overdue|unknown`, próxima fecha y versión del algoritmo.
- Fórmula determinista que incorpora mastery previo, dificultad normalizada, historia de reviews y resultado reciente, con estabilidad acotada entre seis horas y noventa días y precisión persistida de segundos.
- Semántica explícita de evidencia recall-bearing: self-report/manual import no inician el reloj; `review_recall` es la única fuente de outcomes de review y usa threshold de éxito `0.70`.
- Caso de uso workspace-scoped con clock inyectable que recalcula y persiste dentro de una transacción, proyecta `review_due` únicamente sobre estados mastered de Curriculum Instances activos y nunca modifica mastery ni Evidence.
- Migration forward-only v16 que amplía `retention_state`, conserva snapshots anteriores como `legacy-retention/v0` desconocidos y añade constraints/triggers de integridad.
- Adapters SQLite e in-memory con roundtrip completo, clonación segura y la misma validación de dominio; `ProfileStore` expone el servicio para futuros consumidores.
- Arquitectura, fórmula, boundaries, compatibilidad y frontera con scheduling documentadas en `docs/architecture/retention-v1.md`.

### Decisions

- `strength` es una predicción de recall al instante de medición; nunca sustituye ni reduce el mastery derivado de evidencia.
- El exacto `next_due_at` ya está `due`; `overdue` comienza estrictamente después de otro intervalo de estabilidad. Due significa necesidad de comprobar, no prueba de olvido.
- La dificultad del último evento recall-bearing es la entrada v1; reviews exitosas extienden estabilidad y un fallo reciente la acorta agresivamente, con clamps explícitos para evitar intervalos extremos.
- Un snapshot es `unknown` cuando no hay mastery conocido o evidencia objetiva de recall. Evidencia futura respecto al clock inyectado se rechaza, no se ignora.
- `ApplyRetentionV1` solo alterna `mastered ↔ review_due`, conserva score y `mastered_at`, y no promueve estados learning/practicing aunque exista mastery histórico.
- La migración preserva el strength/measurement de snapshots v4 sin fingir que fueron calculados por v1; un recálculo posterior los sustituye desde Evidence.
- Step 18 no crea `ReviewSchedule`, `ReviewItem`, colas, prioridades ni warm-ups; esas decisiones pertenecen a Steps 19 y 20.

### Verification

- Tests de dominio para concepto nuevo, mastered today, due later, due/overdue exactos, review exitoso, fallo, dificultad, orden determinista, clock boundary y proyección/limpieza de `review_due` sin cambiar mastery.
- Tests application para clock inyectable, persistencia transaccional y proyección sobre Curriculum Instance activo.
- Tests SQLite para migration v15 → v16, preservación legacy, roundtrip v1 y rejection de conteos inconsistentes por trigger.
- `GOCACHE=/tmp/kelyro-step18-target2-gocache go test ./internal/learning/... ./internal/storage/sqlite ./internal/infra/learningdb`.
- `GOCACHE=/tmp/kelyro-step18-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step18-full-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-step18-quality-gocache go run ./tools/quality all`, incluyendo E2E, `go test -race ./...`, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 19 es el siguiente paso pendiente y requiere autorización explícita.
- Spaced Repetition Scheduler v1 deberá consumir `RetentionState.NextDueAt`/`Status`, crear o actualizar review records idempotentes y mantener la fórmula fuera de SQL.
- No reinterpretar strength como mastery ni tratar due/overdue como fallo observado.
- No adelantar warm-ups, streaks, achievements, analytics, daily plans o el Exercise Engine completo.

## Step 19 — Spaced Repetition Scheduler v1

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Política pura y versionada `review-scheduler-v1` que consume concept state, `retention-v1`, historial de reviews, criticidad de prerequisitos y un instante UTC explícito.
- Metadata general de review `quick_recall|standard_review|deep_review`, con estimaciones fijas de 5/10/20 minutos y selección determinista por strength o fallo reciente, sin generar ejercicios de I-05.
- Cola due estable con prioridad lexicográfica overdue → menor strength → prerequisito crítico → due más antiguo → ID, y selección greedy dentro de `Availability.DailyMinutes` que reporta trabajo diferido.
- Lifecycle completo para posponer, saltar y completar: postponement explícito se preserva, skip difiere 24 horas sin crear Evidence y success/failure se deriva del threshold de recall `0.70`.
- Completion transaccional que agrega Evidence inmutable `review_recall`, recalcula mastery/retention, proyecta `review_due`, registra Study History y crea la siguiente review; los fallos producen `deep_review` con el intervalo reducido de Retention v1.
- Idempotencia mediante IDs deterministas, outcome repetible con el mismo score y unicidad de un solo pendiente por estudiante/concepto; score distinto sobre una review terminada es conflicto.
- Clock inyectable, almacenamiento/comparaciones UTC y presentación CLI según la timezone IANA del perfil en `kelyro reviews` y `kelyro reviews due`.
- Migration forward-only v17 con metadata de scheduling/lifecycle, backfill `legacy-review/v0`, deduplicación determinista de pendientes legacy, índice parcial único y triggers de integridad.
- Puertos y adapters SQLite/in-memory ampliados, wiring workspace-scoped y documentación en `docs/architecture/review-scheduler-v1.md`.

### Decisions

- `RetentionState.NextDueAt` sigue siendo la autoridad del intervalo; el scheduler decide tipo, prioridad, presupuesto y lifecycle, y nunca reinterpreta strength como mastery.
- Un concepto es prerequisito crítico cuando es requerido por otro concepto del curriculum activo. La criticidad desempata después de overdue y weakness, sin adelantar Knowledge Graph nuevo.
- Si un elemento prioritario no cabe en el tiempo restante se difiere, pero elementos posteriores más breves pueden aprovechar el presupuesto; el backlog total y sus minutos siguen visibles.
- Postpone requiere un due estrictamente posterior al actual y al instante de la acción. Skip cierra el item sin outcome y crea una nueva oportunidad explícitamente diferida; no equivale a pass ni a failure.
- Un solo item pending por concepto es una invariante compartida por dominio, adapters y SQLite. La migración conserva como pending el legacy más antiguo por `(due_at,id)`.
- `reviews` lista todos los pendientes programados por fecha; `reviews due` aplica prioridad y presupuesto solo a los ya vencidos. Las timezones nunca entran al cálculo educativo.
- Step 19 no selecciona warm-ups, no mezcla contenido nuevo, no genera ejercicios y no implementa streaks, achievements, analytics ni daily plans.

### Verification

- Tests de dominio para selección de tipo, sorting due, budget limitado, postpone, failure, duplicados e invariantes de lifecycle.
- Tests application para clock inyectable, idempotencia, prerequisitos críticos, presupuesto, postpone, skip sin Evidence, success y failure con nueva schedule.
- Tests SQLite para migration v16 → v17, deduplicación legacy, roundtrip de lifecycle, pending único y triggers score/outcome.
- Tests app/CLI para dispatch, parsing de ambos comandos, errores de uso y timestamp local `America/Lima`.
- `GOCACHE=/tmp/kelyro-step19-target2-gocache go test ./internal/learning/... ./internal/storage/sqlite ./internal/infra/learningdb ./internal/app ./internal/cli`.
- `GOCACHE=/tmp/kelyro-step19-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step19-vet-gocache go vet ./...`.
- `GOCACHE=<workspace>/.step19-gocache GOTMPDIR=<workspace>/.step19-gotmp go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI; caches eliminados tras el gate.
- Smoke real `init → reviews → reviews due` sobre un workspace temporal nuevo.
- `git diff --check`.

### Notes for next session

- El Paso 20 es el siguiente paso pendiente y requiere autorización explícita.
- Warm-up Selector deberá consumir el backlog/schedule ya durable sin cambiar `review-scheduler-v1`, inventar Evidence ni mezclar Exercise Engine.
- No adelantar streaks, achievements, analytics, daily plans, TUI Student Core completo o generación de ejercicios.

## Step 20 — Warm-up Selector

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Política pura y versionada `warm-up-selector-v1` que selecciona conceptos antes de una lección candidata a partir de prerequisitos directos, reviews vencidas, errores persistentes y tiempo disponible.
- Plan explícito y validado con concepto, razón, prioridad, explicación, estimación de cinco minutos, presupuesto, tiempo utilizado, instante UTC y versión del algoritmo; el resultado vacío es válido.
- Prioridad determinista prerequisito due → prerequisito con error repetido → review due → error repetido, con deduplicación de señales y desempate estable por recencia de warm-ups, vencimiento, ocurrencias, última observación e ID.
- Rotación mediante contexto efímero de conceptos seleccionados recientemente, favoreciendo alternativas no recientes y luego la selección menos reciente sin introducir cache o persistencia opaca.
- Presupuesto de warm-up `floor_to_5(min(15, available_minutes / 3))`, adicionalmente acotado por la disponibilidad diaria del perfil en application, para reservar siempre tiempo al contenido nuevo.
- Caso de uso workspace-scoped con clock inyectable que lee curriculum, cola due y mistake memory de forma coherente, filtra al curriculum candidato y no crea Evidence, reviews, ejercicios ni cambios de progreso.
- Wiring en `ProfileStore`, documentación de fórmula, prioridades, límites y frontera con I-05/Step 24 en `docs/architecture/warm-up-selector-v1.md`; no fue necesaria una migration.

### Decisions

- El selector recibe la lección candidata y sus prerequisitos desde el consumidor; elegir la lección de hoy pertenece al Adaptive Daily Plan del Paso 24.
- Solo son elegibles reviews realmente pending/due y patrones no resueltos con al menos dos ocurrencias. Un prerequisito sin señal de repaso no se añade para llenar tiempo.
- Las señales se limitan al curriculum versionado de la lección candidata, evitando mezclar backlog de otro curriculum en un warm-up contextual.
- Una señal due y una señal de error para el mismo concepto producen un solo item; la razón refleja la prioridad dominante y la explicación conserva la señal secundaria.
- La lista recent es input explícito newest-first y caller-owned; Step 20 no persiste un nuevo plan ni adelanta Daily Plans o Study Sessions.
- Step 20 no añade CLI/TUI porque la especificación solo requiere selección. I-05 seguirá siendo dueño de generar, ejecutar y evaluar el ejercicio concreto.

### Verification

- Tests de dominio para ausencia de reviews due, prerequisito crítico due, error repetido sin review, presupuesto corto/largo, preservación de contenido nuevo, rotación reciente y desempate determinista por ID.
- Test application para clock inyectable, lectura de señales durables, prioridad contextual, límite de disponibilidad del perfil y ausencia de mutaciones en reviews/Evidence.
- `GOCACHE=/tmp/kelyro-step20-target-gocache go test ./internal/learning/... ./internal/infra/learningdb ./internal/app`.
- `GOCACHE=/tmp/kelyro-step20-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step20-vet-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-step20-quality-gocache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 21 es el siguiente paso pendiente y requiere autorización explícita.
- Streaks deberá basarse en actividad educativa significativa y la timezone del perfil sin reinterpretar warm-ups, study history o review outcomes.
- No adelantar achievements, analytics, daily plans, TUI Student Core completo ni Exercise Engine.

## Step 21 — Streaks sin comportamiento punitivo

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Política pura, configurable y versionada `streak-v1` con threshold default de diez minutos activos o una actividad educativa significativa completada.
- Cálculo determinista desde Study History y Study Sessions que deduplica fechas locales, agrega sesiones cortas del mismo día y produce current, longest, last active local date y total active days.
- Semántica no punitiva: abrir Kelyro/TUI, completar onboarding, desbloquear un achievement o detener una sesión vacía no cuenta; el streak nunca modifica mastery, prerequisites, progression, reviews ni acceso a contenido.
- Calendario basado en la timezone IANA actual del perfil, conservación del current streak si la última actividad fue hoy o ayer y aritmética de calendario correcta sobre días DST de 23/25 horas.
- Recálculo completo en cada `StreakService.Show`, reparación atómica del estado materializado y comportamiento de cambio de timezone sin acumulación permanente de fechas duplicadas.
- Migration forward-only v18 que amplía `streak_state` con fecha local, total, timezone, threshold y policy version, preservando filas publicadas como `legacy-streak/v0` hasta su siguiente recálculo.
- Adapters SQLite/in-memory, wiring workspace-scoped y validaciones/triggers de integridad para la proyección materializada.
- CLI `kelyro streak` y vista TUI `Study consistency` accesible con `[k] Streak`, con texto neutral, longest/total como contexto y aclaración explícita de que no afecta el aprendizaje.
- Decisiones, fórmula, señales, timezone/DST, compatibilidad y límites documentados en `docs/architecture/non-punitive-study-streak-v1.md`.

### Decisions

- Son actividades educativas significativas `diagnostic.completed`, `concept.introduced`, `evidence.recorded`, `concept.mastered` y `review.completed`; eventos derivados duplicados solo cuentan una vez por fecha.
- `session.completed` no evita el threshold: la autoridad es `StudySession.ActiveDuration`, ya acotada por `study-session-v1`; sesiones active/interrupted/recovered con tiempo significativo siguen siendo actividad intencional válida.
- Como no existen intervalos activos por minuto, una sesión se atribuye a la fecha local de su end o, si sigue activa, de su última actividad significativa, igual que `time-tracking-v1`.
- El current streak conserva el último run durante todo el día posterior a la última actividad. Un hueco mayor lo muestra en cero sin borrar longest, total o historial.
- Cambiar timezone recalcula todos los UTC facts: puede agrupar o separar observaciones cercanas a medianoche, pero nunca incrementa permanentemente un contador; volver a la timezone anterior reproduce la proyección anterior.
- La fila `streak_state` es un materialized projection reparable, no una fuente educativa ni un contador incremental. Legacy state se conserva, no se presenta como si hubiera sido calculado por v1.
- No se añadieron mensajes de pérdida, culpa, recuperación, notificaciones ni mecanismos de bloqueo; achievements y milestones permanecen reservados para el Paso 22.

### Verification

- Tests de dominio para mismo día, día siguiente, día omitido, grace de ayer, timezone/reversión, DST spring/fall, longest streak, total de fechas y acumulación del threshold entre sesiones.
- Tests application para clock inyectable, recálculo desde facts durables, reparación de estado legacy, persistencia e idempotencia.
- Tests SQLite para migration v17 → v18, preservación legacy, roundtrip v1 y trigger de consistencia longest/total.
- Tests app/CLI para dispatch, parsing, texto neutral, singular/plural y ausencia de lenguaje punitivo.
- Tests TUI para navegación, carga desde application, contenido neutral, refresh/layout responsive y nuevos golden files.
- `GOCACHE=/tmp/kelyro-step21-gate-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step21-gate-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-step21-gate-gocache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- Smoke real `init → streak` sobre un workspace temporal nuevo, verificando estado vacío, timezone, policy y mensaje no punitivo.
- `git diff --check`.

### Notes for next session

- El Paso 22 es el siguiente paso pendiente y requiere autorización explícita.
- Achievement & Milestone Framework deberá reconocer progreso real mediante reglas deterministas sin convertir streaks en moneda, barrera o fuente de culpa.
- No adelantar analytics, adaptive daily plans, TUI Student Core completo ni Exercise Engine.

## Step 22 — Achievement & Milestone Framework

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Catálogo Foundation determinista y versionado con `first_session`, `first_concept_mastered`, `seven_active_days`, `ten_hours_studied`, `first_review_completed` y `module_mastered`.
- Definiciones como datos validados con título, descripción, tipo/configuración de criterio, visibilidad y `achievement-definition/v1`, sin IA ni contenido generado.
- Evaluador puro `achievement-v1` que reconstruye eligibility, timestamp histórico y contexto explicable desde Study Sessions, Study History, reviews y mastery instance-scoped.
- Proyección pública de días activos compartida con `streak-v1`, preservando exactamente threshold, timezone, calendario local y DST para el hito de siete días.
- Criterio general de módulo que exige mastery de todos los conceptos de un mismo módulo e instancia curricular, aplicable al fixture Foundation sin acoplar el dominio a una materia.
- Caso de uso transaccional `AchievementService.Refresh` que instala definiciones, evalúa todos los criterios, inserta solo ausentes y registra `achievement.unlocked` únicamente para unlocks nuevos.
- IDs deterministas y unicidad por `(student_id, achievement_key)` en adapters SQLite/in-memory, con respuesta separada de achievements totales y recién desbloqueados.
- Migration forward-only v19 con metadata de definición, configuración JSON, hidden, contexto y versiones; filas v4 preservadas como `legacy-achievement/v0` y guards SQLite para aggregates v1.
- Integración workspace-scoped y mensaje TUI sutil al iniciar un learning path listo: `Milestone unlocked` más los títulos visibles, sin pantalla/CLI adicional ni economía de puntos.
- Fórmulas, fuentes de verdad, desempates, atomicidad, compatibilidad y frontera de presentación documentadas en `docs/architecture/learning-achievements-v1.md`.

### Decisions

- Un achievement persistido recuerda reconocimiento pero nunca sustituye Study History, sesiones, reviews o estados curriculares; borrar y recalcular reproduce un unlock si los hechos siguen presentes.
- `first_session` exige una sesión `completed` con al menos una actividad educativa significativa; detener una sesión vacía no cuenta como progreso real.
- Los thresholds de tiempo cruzan en el primer anchor durable que alcanza el total; los días activos cruzan en el séptimo día calificado bajo la política compartida `streak-v1`.
- `module_mastered` significa todos los conceptos del módulo dentro de una misma curriculum instance; review_due conserva el mastery histórico mediante `MasteredAt`.
- Empates temporales se resuelven con IDs estables, y el contexto almacena solo IDs/thresholds explicativos, sin secretos ni contenido de ejercicios.
- La inserción condicional y el evento histórico ocurren en una sola transacción. Un retry o refresh concurrente no puede desbloquear ni anunciar dos veces la misma definición.
- Hidden es metadata de presentación: se persiste y evalúa normalmente, pero el aviso TUI solo muestra recognitions visibles.
- Step 22 no añade puntos, recompensas, niveles, penalizaciones, notificaciones, gates, Analytics, Daily Plans ni Exercise Engine.

### Verification

- Tests de dominio para las seis condiciones simultáneas, timestamps históricos, contextos, ausencia de falsos unlocks y reutilización de active days v1.
- Tests application para recálculo desde hechos preexistentes, múltiples unlocks, segunda ejecución idempotente, un solo evento por achievement y persistencia del catálogo.
- Tests SQLite para upgrade v18 → v19, preservación legacy, roundtrip de definición/contexto v1, insert-if-absent y trigger de configuración.
- Tests app/TUI para dispatch workspace-scoped, cierre de store, refresh durante inicialización, ocultamiento de definitions hidden y mensaje profesional exacto.
- `GOCACHE=/tmp/kelyro-step22-gate-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step22-gate-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-step22-quality-gocache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 23 es el siguiente paso pendiente y requiere autorización explícita.
- Learning Analytics v1 deberá leer estas fuentes durables y no usar achievements como reemplazo de métricas de progreso, tiempo, mastery o reviews.
- No adelantar adaptive daily plans, Progress Dashboard, TUI Student Core completo ni Exercise Engine.

## Step 23 — Learning Analytics v1

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Política pura, configurable y versionada `learning-analytics-v1` que produce un snapshot tipado y validado de progress, time, mastery, retention, activity y pace.
- Definiciones explícitas para concepts introduced/learning/mastered, reviews due, tiempo today/week/month/total, mastery conocido, strongest/weakest, retention fresh/due/overdue, active days/streaks y ritmo semanal.
- Exclusión correcta de estados `not_seen` del promedio y rankings de mastery: un perfil vacío conserva average ausente, mientras un score conocido de cero sigue siendo dato válido.
- Ventanas today/week/month y rolling pace construidas con calendario local IANA, inicio inclusivo/fin exclusivo, semana desde lunes y semántica DST compartida con Time Tracking.
- Rankings top/bottom configurables con desempate por curriculum instance/concept ID y suma del promedio en orden estable, independiente del orden devuelto por repositorios.
- Retention recalculada al instante del snapshot desde `next_due_at` y estabilidad para no convertir un status materializado potencialmente viejo en verdad analítica.
- Pace default de cuatro semanas locales para concepts mastered/week y study minutes/week, sin forecast de graduación o terminación.
- `LearningAnalyticsService.Snapshot` workspace-scoped que carga concept states, retention, reviews, Study Sessions y Study History dentro de una sola unidad de trabajo y calcula desde fuentes primarias.
- Wiring en `ProfileStore`/`learningdb`, documentación de métricas, fórmulas, fuentes y límites en `docs/architecture/explainable-learning-analytics-v1.md`.

### Decisions

- La identidad analítica de concepto es `(curriculum_instance_id, concept_id)` para no mezclar estados de distintas versiones curriculares.
- `review_due` conserva el conteo mastered: una revisión vencida solicita comprobación pero no borra el mastery histórico.
- Reviews due incluye solo items pending cuyo due ya llegó. Retention legacy/unknown no se clasifica artificialmente como fresh, due u overdue.
- Study time suma únicamente `StudySession.ActiveDuration`; sesiones terminales usan `ended_at` y activas usan `last_activity_at`, igual que `time-tracking-v1`.
- Activity reutiliza exactamente `streak-v1` desde history/sessions en vez de leer la proyección materializada de streak o redefinir un día activo.
- La tabla y servicio legacy de analytics permanecen por compatibilidad con el schema publicado, pero v1 no los lee ni escribe. No se añadió migration ni cache porque no se justificó por rendimiento.
- Step 23 no añade dashboard, CLI/TUI, daily plan, Exercise Engine, Research Engine, Curriculum Compiler, proveedor de IA ni plugins.

### Verification

- Tests de dominio para exclusión de unknown, perfil vacío, score conocido cero, rangos locales, orden independiente del input, descripciones y fixture determinista de 5,000 conceptos.
- Test application que prueba cálculo desde fuentes primarias e ignora deliberadamente un snapshot legacy obsoleto.
- Test de integración `learningdb` para wiring y snapshot vacío sobre SQLite real sin nueva persistencia analítica.
- `GOCACHE=/tmp/kelyro-step23-final-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step23-final-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-step23-quality-gocache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 24 es el siguiente paso pendiente y requiere autorización explícita.
- Adaptive Daily Plan v1 podrá consumir este snapshot como lectura explicable, pero deberá tomar sus decisiones desde fuentes/políticas autoritativas y no convertir analytics en un cache obligatorio.
- No adelantar Progress Dashboard, TUI Student Core completo, Markdown de progreso ni Exercise Engine.

## Step 24 — Adaptive Daily Plan v1

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Política pura, configurable y versionada `daily-plan-v1` que selecciona trabajo conceptual sin generar lesson, practice, assessment ni contenido educativo.
- Frontier curricular estricto basado en el orden jerárquico y el mastery threshold resuelto: ningún concepto posterior puede saltarse una debilidad o prerequisito bloqueante.
- Priorización determinista de critical overdue prerequisites, reviews vencidas, blocking weaknesses, siguiente concepto elegible y práctica opcional por mistake no resuelto.
- Presupuesto con defaults explícitos de 5 minutos de warm-up, 10 de reinforcement, 25 de new learning, mínimo útil de 10 y buffer de 5; el aggregate impide que planned más buffer excedan availability.
- Items explicables con role, selection reason, texto humano, concepto único, minutos y posición; estados `ready`, `review_only`, `nothing_urgent` y `time_limited` distinguen resultados completos y vacíos.
- Fingerprint SHA-256 de fuentes/políticas y IDs deterministas que permiten reutilizar el snapshot del día si nada relevante cambió y regenerarlo explícitamente por cambio de source o policy.
- `AdaptiveDailyPlanService.Today` workspace-scoped que resuelve fecha local, goal e instancia activos, mastery policy y hechos primarios dentro del boundary application; no consume el snapshot de Analytics como verdad.
- Proyección mínima de planning curriculum en adapters memory/SQLite, preservando orden de jerarquía y prerequisitos sin exponer nodos o SQL al planner.
- Migration forward-only v20 con metadata v1, guards de presupuesto/shape y compatibilidad completa de filas publicadas como `legacy-daily-plan/v0`.
- Wiring en `ProfileStore`/`learningdb` y documentación de prioridades, desempates, budget, regeneración, persistencia y límites en `docs/architecture/daily-plan-v1.md`.

### Decisions

- Daily Plan selecciona únicamente qué concepto y clase de trabajo realizar; I-05 conserva la construcción y evaluación del contenido concreto.
- El primer concepto bajo threshold es el frontier autoritativo. Si ya fue visto pero no dominado, o si un prerequisito no satisface mastery, el día refuerza la debilidad y no introduce material posterior.
- Un critical prerequisite es overdue después de más de una stability interval desde `next_due_at`, consistente con `retention-v1` y Learning Analytics v1.
- Reviews usan su estimate persistido; warm-up, reinforcement y new-learning usan slots indivisibles/configurados salvo que new learning pueda reducirse hasta su mínimo útil.
- El buffer se reserva solo cuando el presupuesto permite al menos el mínimo útil de new learning más buffer; el límite total es estricto y no intenta optimización combinatoria perfecta.
- El fingerprint excluye el instante exacto de generación, pero incluye fecha local, timezone, availability, curriculum/prerequisitos, mastery policy/state, reviews/retention, mistakes, history y configuración completa del planner.
- La llamada `Today` es el boundary explícito de generación: reutiliza policy+fingerprint idénticos y reemplaza atómicamente solo el snapshot obsoleto del mismo goal/día; fechas anteriores permanecen como historial.
- El servicio retorna `not_found` sin active goal o active curriculum instance. Step 24 no añade dashboard, CLI/TUI, Markdown, Exercise Engine, Research Engine, Curriculum Compiler, IA ni plugins.

### Verification

- Tests de dominio para brand-new student, reviews con critical warm-up, blocked next lesson, todo mastered, tiny budget y determinismo ante reordenamiento de facts.
- Tests application para ausencia de active goal, persistencia inicial, reutilización sin cambios y regeneración al cambiar mastery state.
- Tests memory/SQLite para orden de hierarchy, prerequisites, upgrade v19 → v20, preservación legacy, roundtrip v1 y rechazo de snapshots sobre presupuesto.
- `GOCACHE=/tmp/kelyro-step24-early-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step24-early-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-step24-early-gocache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 25 es el siguiente paso pendiente y requiere autorización explícita.
- Progress Dashboard debe consumir servicios/read models coherentes sin duplicar queries ni recalcular en presentation las políticas de analytics o daily planning.
- No adelantar TUI Student Core completo, CLI Student Core, Markdown de progreso, migration/recalculation general ni Exercise Engine.

## Step 25 — Progress Dashboard application service

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Read model versionado `progress-dashboard/v1` con goal, curriculum activo, progreso general, ubicación phase/module/lesson/topic/concept, resumen de mastery, reviews vencidas, plan de hoy, tiempo, streak, milestone reciente y conceptos débiles titulados.
- Empty states explícitos: goal, curriculum, ubicación, plan y milestone usan ausencia real; mastery desconocido permanece `nil`; contadores de reviews, tiempo y streak conservan ceros significativos.
- Métricas curriculares acotadas a la instancia activa, mientras tiempo y streak permanecen facts longitudinales del estudiante; milestones se acotan al goal activo.
- Refresh que reutiliza `AdaptiveDailyPlanService.Today` para regeneración por fingerprint y calcula el resto desde hechos primarios coherentes mediante `learning-analytics-v1` dentro de un unit of work.
- Frontier de navegación determinista: primer concepto en orden jerárquico que no está `mastered`/`review_due`, sin sustituir las decisiones de prerequisitos o Daily Plan.
- Proyección `CurriculumOutlineNode` y puerto `Outline` implementados en memory/SQLite; SQLite lee toda la jerarquía en una query y application usa índices in-memory sin N+1.
- Construcción de fixtures memory optimizada mediante índices y paths precalculados para mantener tiempos razonables con miles de conceptos.
- Wiring workspace-scoped en `ProfileStore`/`learningdb`, sin exponer repositorios ni SQLite a CLI/TUI.
- Contrato, coherencia, disponibilidad, ordenamiento, complejidad y fronteras documentados en `docs/architecture/progress-dashboard-read-model.md`.

### Decisions

- El dashboard es una proyección application, no un nuevo agregado persistido; no se añadió migration ni cache que pueda convertirse en una segunda fuente de verdad.
- Los campos opcionales no usan entidades placeholder. Un active goal sin instancia conserva solo el goal; sin active goal se omite todo el contexto curricular incluso si existe historial de goals pausados.
- Overall progress, mastery, reviews due y weak concepts describen únicamente el curriculum activo; study time y streak son globales porque cambiar de goal no borra el esfuerzo histórico.
- La ubicación actual sigue el primer concepto no dominado en hierarchy order. Un curriculum completamente dominado no apunta artificialmente a contenido terminado.
- El plan se refresca mediante el boundary autorizado del Paso 24 y luego se valida contra student/goal/instance activos; el dashboard no reimplementa selección diaria.
- La persistencia publicada no conserva el título raíz del curriculum, por lo que el read model expone la referencia versionada y los títulos de nodos disponibles sin inventar metadata.
- Step 25 no añade presentación CLI/TUI, Markdown de progreso, Exercise Engine, Research Engine, Curriculum Compiler, IA ni plugins.

### Verification

- Tests application para new student, progreso parcial, review vencida, goal pausado, refresh tras cambios de estado/sesión y 5.000 conceptos.
- Test SQLite de la proyección completa de hierarchy y test de wiring/empty dashboard en `learningdb`.
- `GOCACHE=/tmp/kelyro-step25-final-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step25-final-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-step25-final-gocache go test -race ./internal/infra/learningdb`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-step25-final-gocache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 26 es el siguiente paso pendiente y requiere autorización explícita.
- La TUI debe consumir `ProfileStore.Dashboard().Show` y renderizar sus empty states; no debe consultar SQLite, ensamblar métricas ni recalcular la ubicación.
- No adelantar CLI Student Core completo, Markdown de progreso, migration/recalculation general, Exercise Engine ni I-03+.

## Step 26 — TUI Student Core

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Home educativo persistente con goal activo, barra de progreso ASCII, reviews vencidas, siguiente concepto del plan diario, mastery efectivo requerido, streak y tiempo semanal.
- Pantallas Today, Progress, Concept detail, Reviews, History, Goal y Profile integradas junto al onboarding y roadmap, conservando Doctor, Config y Study consistency como utilidades secundarias.
- Boundary interno `ActionDashboard` que expone `ProfileStore.Dashboard().Show` a adapters de presentación sin añadir todavía un comando CLI público del Student Core.
- Proyección de roadmap añadida a `progress-dashboard/v1`, ordenada canónicamente como phase/module/lesson/topic/concept y con estados `mastered`, `current`, `available`, `locked` y `review_due`.
- Motivos de bloqueo resueltos en application a partir de prerequisitos y estado, con mastery desconocido representado por ausencia y score conocido de cero preservado como `0%`.
- Today renderiza el `daily-plan-v1` persistido y sus razones con títulos humanos, sin generar ejercicios ni recalcular selección en Bubble Tea.
- Reviews e History reutilizan sus application services existentes con carga, refresh, error y empty states propios; Goal y Profile permanecen read-only en la TUI.
- Destinos Student Core incorporados al session state resumible; onboarding completado refresca el dashboard antes de volver a Home.
- Layout sin dependencia semántica de color o Unicode, wrapping/truncation para terminales estrechas y goldens actualizados para 32, 80 y 120 columnas.
- Documentación de arquitectura, navegación, refresh, estados vacíos, accesibilidad y límites en `docs/architecture/student-core-tui.md`.

### Decisions

- La TUI renderiza un único dashboard coherente y nunca abre SQLite, ensambla métricas, decide prerequisitos ni calcula la ubicación actual.
- La proyección roadmap usa `Outline` y `PlanningConcepts` en lecturas compactas; application reordena el outline como árbol porque adapters pueden devolver filas en orden de identidad, no de jerarquía.
- `current` es el frontier de navegación existente; los conceptos posteriores con prerequisitos insatisfechos se muestran `locked` con una explicación, mientras `review_due` conserva un estado visible distinto de `mastered`.
- Progress explica que completion cuenta conceptos dominados del curriculum y que average mastery incluye solo conceptos conocidos; unknown nunca se presenta como `0%`.
- El mastery requerido usa la resolución autoritativa de `MasteryPolicyService`, incluida la precedencia de workspace override sobre student default; no se presenta el threshold original del goal como si siempre fuera efectivo.
- Concept detail muestra el concepto actual y su contexto jerárquico. Navegación arbitraria y ejecución de contenido permanecen fuera de este paso y del Student Core.
- El refresh explícito reemplaza el read model completo y deja que `AdaptiveDailyPlanService.Today` decida reutilización o regeneración por fingerprint.
- No se añadió migration, cache de dashboard, comando CLI Student Core, export Markdown, Exercise Engine, Research Engine, Curriculum Compiler, IA, plugin ni actividad de red.

### Verification

- Tests application para orden jerárquico, estados mastered/current/locked, mastery opcional y explicación de prerequisitos.
- Tests app para dispatch workspace-scoped del dashboard y cierre del store.
- Tests TUI para navegación, contenido de todas las vistas, carga de Reviews/History, onboarding → Home, refresh, empty/error/locked states, resume y terminales de 24/40/80/120 columnas.
- Goldens de Home para 32, 80 y 120 columnas.
- E2E Foundation actualizado para aceptar el Home educativo real después de onboarding.
- `GOCACHE=/tmp/kelyro-step26-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-step26-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-step26-gocache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 27 es el siguiente paso pendiente y requiere autorización explícita.
- La CLI del Student Core debe reutilizar los mismos application services/read models sin lanzar la TUI ni añadir JSON como interfaz principal.
- No adelantar export Markdown, recalculación/migración general de algoritmos, hardening, E2E Student Core completo, Exercise Engine ni I-03+.

## Step 27 — CLI Student Core

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Comandos human-readable `status`, `progress`, `roadmap` y `today` sobre el mismo `progress-dashboard/v1` consumido por la TUI.
- Superficie mínima completa con `profile`, `goal`, `history`, `time`, `reviews`, `mistakes` y `mastery`; `profile`, `goal` y `mastery` ahora usan su lectura por defecto sin exigir un subcomando redundante.
- `status` presenta goal, ubicación curricular actual, mastery efectivo y conteos de mastered/learning/review due.
- `progress` separa completion de average mastery conocido y añade reviews, tiempo intencional, streak, milestone reciente y conceptos que necesitan refuerzo.
- `roadmap` presenta la jerarquía curricular resuelta, estados textuales, mastery conocido y razones de bloqueo; el documento Foundation permanece accesible como `kelyro open roadmap`.
- `today` presenta el `daily-plan-v1` persistido, su presupuesto, orden, roles y explicaciones con títulos humanos sin generar ejercicios.
- Empty states accionables para onboarding/goal incompleto, curriculum ausente y plan diario ausente, sin convertir una lectura válida en error.
- Routing explícito que nunca lanza la TUI desde subcommands, conserva `--workspace`, mantiene output exitoso en stdout y errores en stderr, y usa exit codes 0/1/2 para éxito, fallo operacional y uso inválido.
- Ayuda actualizada sin introducir JSON ni otra interfaz machine-readable primaria.
- Documentación del contrato, routing, salida, empty states, exit codes y límites en `docs/architecture/student-core-cli.md`.

### Decisions

- `ActionStatus`, `ActionProgress`, `ActionRoadmap` y `ActionToday` convergen en `executeDashboard`; los adapters seleccionan qué parte renderizar, pero no ensamblan hechos educativos ni recalculan políticas.
- El antiguo `kelyro roadmap` que imprimía una ruta Foundation se reemplaza por el roadmap educativo requerido; `kelyro open roadmap` conserva el acceso explícito al artefacto.
- Una configuración educativa incompleta es un estado consultable exitoso y muestra la acción siguiente; un workspace inexistente o una dependencia de Student Core no disponible conserva exit code operacional `1`.
- Average mastery ausente se muestra como `unknown`; un score conocido de cero conserva `0%`.
- No se añadió migration, JSON, export Markdown, recalculación general, Exercise Engine, Research Engine, Curriculum Compiler, IA, plugin ni actividad de red.

### Verification

- Tests CLI de routing para toda la superficie, aliases de lectura, `--workspace`, bypass de TUI, ayuda, argumentos inválidos y exit codes.
- Tests de salida determinista para status, progress, roadmap y today, incluidos títulos humanos, unknown semantics y explicaciones de lock/plan.
- Tests de onboarding incompleto y workspace no inicializado en las fronteras CLI y app.
- Tests app para las cinco acciones consumidoras del dashboard, propagación de workspace override y cierre del store.
- Smoke del binario real sobre un workspace recién inicializado, verificando `--workspace status` y su guía de setup incompleto.
- `GOCACHE=/tmp/kelyro-i02-step27-final-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i02-step27-final-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i02-step27-final-gocache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 28 es el siguiente paso pendiente y requiere autorización explícita.
- El export Markdown deberá reutilizar read models existentes, escribir solo rutas system-owned y no convertir Markdown en source of truth.
- No adelantar recalculación/migración general, hardening, E2E Student Core completo, Exercise Engine ni I-03+.

## Step 28 — Artefactos Markdown humanos de progreso

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Export explícito `kelyro progress export` que construye una sola snapshot `progress-dashboard/v1` y regenera las tres vistas humanas del workspace.
- `LEARNING.md` con goal activo, ubicación curricular actual, plan de hoy, requisito efectivo de mastery, progreso conocido y reviews vencidas.
- `00-roadmap/ROADMAP.md` como jerarquía legible del Curriculum Instance activo, con estados, mastery conocido y razones humanas de bloqueo.
- `00-roadmap/PROGRESS.md` con tiempo intencional, conceptos, completion, reviews due, streak y milestone reciente.
- Empty states accionables y deterministas para setup incompleto, sin fabricar goal, curriculum, plan ni métricas.
- Templates versionados independientes y renderer puro sin acceso a SQLite o filesystem.
- Integración con Artifact Ownership de Foundation: rutas system-generated, hashes persistidos, writes atómicos por archivo y conflictos para artifacts no rastreados o editados.
- Clasificación exacta de `00-roadmap/PROGRESS.md`; otros archivos `PROGRESS.md`, incluidos logs de implementación, permanecen student-owned.
- Normalización/escape de texto y omisión deliberada de IDs internos, perfil, descripciones, errores, evidence, respuestas diagnósticas, secretos y metadata machine-owned.
- Golden tests para los tres documentos, más tests de privacidad, canonical UTF-8/LF, estado incompleto, versión de read model, routing CLI, coordinación application y propagación de conflictos.
- Contrato, contenido, versiones, privacidad, ownership, regeneración y límites documentados en `docs/architecture/student-learning-markdown-artifacts.md`.

### Decisions

- SQLite continúa como source of truth; Markdown es únicamente una proyección descartable y nunca se lee para reconstruir estado educativo.
- Los tres documentos consumen una única snapshot coherente del dashboard; templates y adapters no recalculan mastery, retention, streaks, analytics o daily plans.
- La regeneración ocurre solo por el comando explícito, no en keypresses de TUI ni como side effect de lecturas.
- Los placeholders Foundation de `LEARNING.md` y `ROADMAP.md` se reemplazan solo si su hash sigue intacto; la nueva vista `PROGRESS.md` usa la misma política desde su primera creación.
- La escritura conserva el contrato publicado de atomicidad por archivo de Foundation. Un conflicto detiene el resto de la regeneración y nunca sobrescribe el artifact en conflicto.
- El Markdown prioriza títulos y métricas humanas y no publica identidades técnicas aunque formen parte del read model.

### Verification

- `GOCACHE=/tmp/kelyro-i02-step28-gocache go test ./internal/artifacts/... ./internal/infra/artifactfs ./internal/app ./internal/cli`.
- `GOCACHE=/tmp/kelyro-i02-step28-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i02-step28-vet-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i02-step28-quality-gocache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- Smoke real `init → progress export` sobre workspace temporal, verificando las tres rutas, empty states y salida CLI.
- Smoke de conflicto tras editar manualmente `00-roadmap/PROGRESS.md`, verificando exit code de fallo, error `generated artifact was modified externally` y preservación del contenido humano.
- `git diff --check`.

### Notes for next session

- El Paso 29 es el siguiente paso pendiente y requiere autorización explícita.
- Compatibilidad, migración y recalculación general de Student Algorithms permanecen reservadas para el Paso 29.
- No adelantar hardening de pasos 30–31, Exercise Engine, Research Engine, Curriculum Compiler, IA, plugins ni I-03+.

## Step 29 — Compatibilidad, recalculación y migración de Student Algorithms

Status: completed
Date: 2026-08-21
Release: unreleased

### Delivered

- Suite interna configurable `LearningAlgorithmSuite` para mastery, retention y daily plan, con wrappers de las políticas v1 publicadas e inyección controlada de implementaciones compatibles.
- Servicio transaccional de recalculación que vuelve a derivar concept states, retention, schedules/reviews pendientes y el plan del día, con dry-run e impacto desglosado por agregado.
- Migration forward-only v21 que registra `mastery_algorithm_version` y `progression_policy_version` en concept state y backfillea estado publicado como `mastery-v1`/`progression-v1`.
- Lectura determinista `EvidenceRepository.ListByStudent` sin añadir operaciones de update/delete; recalculación v1 y v2 simulada comprueban que la secuencia completa de evidence no cambia.
- Comando avanzado `kelyro maintenance recalculate [--dry-run]` con salida de versiones previa/objetivo, registros leídos, conceptos inspeccionados y cambios reales o proyectados.
- Apply protegido por backup Foundation previo con razón `learning-algorithm-recalculation`; un fallo de backup impide abrir la transacción educativa.
- Auditoría segura `learning.recalculation.completed` con backup, versiones objetivo y contadores agregados, sin contenido educativo ni payloads de evidence.
- Wiring workspace-scoped en `ProfileStore`/`learningdb`, pruebas memory/SQLite/app/CLI y documentación en `docs/architecture/versioned-learning-state-recalculation.md`.

### Decisions

- Las implementaciones actuales continúan siendo las funciones puras v1 existentes; la suite solo define el punto de reemplazo y valida que cada resultado declare la versión configurada.
- `unversioned/v0` representa estado sparse creado antes de un cálculo, evitando atribuir falsamente una versión; las filas ya publicadas reciben v1 mediante migration v21.
- Todos los writes de apply comparten un `UnitOfWork`; un error tardío revierte concept state, retention, reviews y daily plan como una sola unidad.
- Completed/skipped reviews y daily plans históricos no se reescriben. Solo se alinea el review pendiente aplicable, preservando postergaciones explícitas, y se reemplaza el plan de hoy cuando cambia versión o fingerprint.
- Un estado derivado semánticamente idéntico conserva su timestamp, haciendo idempotente una recalculación v1 con el mismo reloj.
- La v2 fake cubre el contrato de evolución de mastery/retention sin publicar prematuramente fórmulas v2 ni flexibilizar schemas para versiones de producción inexistentes.
- El backup se crea fuera de la transacción SQLite y su ID es obligatorio para apply; dry-run ejecuta la misma proyección pero no crea backup, audit de apply ni escritura.
- No se adelantaron corrección explícita de evidence, hardening, nuevos algoritmos, Exercise Engine, Research Engine, Curriculum Compiler, IA, plugins ni I-03+.

### Verification

- Tests dirigidos para v1, v2 fake, dry-run, apply, idempotencia, rollback, backup fallido, evidence inmutable, migration v20 → v21, wiring y CLI.
- `GOCACHE=/tmp/kelyro-i02-step29-final-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go test ./...`.
- `GOCACHE=/tmp/kelyro-i02-step29-final-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i02-step29-final-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- Smoke real `init → maintenance recalculate --dry-run → maintenance recalculate` sobre workspace temporal; se verificaron ausencia de backup en preview, backup schema 21 con razón dedicada y ambos eventos de audit en apply.
- `git diff --check`.

### Notes for next session

- El Paso 30 es el siguiente paso pendiente y requiere autorización explícita.
- El hardening de integridad, privacidad y rendimiento debe reutilizar este boundary y añadir migraciones forward-only si requiere nuevo estado persistido.
- No implementar corrección de evidence, algoritmos v2 reales, Exercise Engine, Research Engine, Curriculum Compiler, IA, plugins ni I-03+ sin su paso y autorización.

## Step 30 — Hardening de integridad, privacidad y rendimiento

Status: completed
Date: 2026-08-23
Release: unreleased

### Delivered

- Scan de integridad del Student Core al abrir SQLite y al validar backups: `foreign_key_check`, duplicados activos/pendientes, rangos de mastery, pertenencia curricular, ownership diagnóstico y timezones IANA.
- Errores de integridad seguros que nombran el tipo de hallazgo sin exponer IDs, answers, evidence sources, mistake text ni contenido de filas.
- Migration forward-only v22 con guards de pertenencia para concept state y diagnostic observations, sin modificar migrations publicadas.
- Índices de timeline para curriculum instances, study sessions y review items, verificados mediante `EXPLAIN QUERY PLAN` sin imponer umbrales temporales dependientes de la máquina.
- Fixture determinista `student-core-scale/v1` con 50 phases, 150 modules, 500 lessons, 500 topics, 2.000 concepts, 1.500 prerequisite edges, 2.000 concept states y 6.000 evidence records.
- Logging defensivo de inputs educativos: onboarding/setup/profile/goal se marcan sensibles y los errores de validación no repiten answers rechazadas.
- Regresiones de corrupción física-relacional, dangling state, diagnostic ownership, timezone inválido, allowlist de profile, doble write concurrente y clasificación SQLite busy/locked.
- Contrato de hardening documentado en `docs/architecture/student-core-hardening.md` y enlazado desde el índice arquitectónico.

### Decisions

- `PRAGMA quick_check` no detecta foreign-key violations; por eso el open path conserva el chequeo físico antes de migrar y añade checks relacionales/semánticos después de alcanzar el schema actual.
- Los guards v22 protegen relaciones que una FK simple a `concept_registry` no puede expresar: un concepto debe pertenecer a la versión exacta de la instancia y una observación diagnóstica debe coincidir con student/concept/evidence/curriculum.
- Diferencias entre un timezone histórico válido y el timezone actual del perfil no son corrupción; todos los identificadores se validan, mientras refresh/recalculation conserva la responsabilidad de snapshots nuevos.
- El fixture grande se valida por cardinalidad y query plans, no por milisegundos arbitrarios. Se ejecuta en la suite normal; bajo `-race` se omite únicamente ese caso de 11.000 writes, mientras las pruebas concurrentes sí corren instrumentadas.
- Múltiples curriculum instances versionadas para un mismo goal siguen siendo válidas por diseño; el hardening no introduce una unicidad que rompería aislamiento histórico ya publicado.
- Profile y export mantienen allowlists explícitas; no se añadió telemetry, background work, red, corrección/borrado de evidence ni algoritmo educativo nuevo.

### Verification

- `GOCACHE=/tmp/kelyro-step30-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go test ./...`.
- `GOCACHE=/tmp/kelyro-step30-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-step30-quality-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- Race dirigido de concurrencia/busy y suite completa de `internal/storage/sqlite` con el fixture de escala correctamente omitido solo bajo build tag `race`.
- `git diff --check`.

### Notes for next session

- El Paso 31 es el siguiente paso pendiente y requiere autorización explícita.
- El E2E completo debe recorrer el Student & Learning Core desde onboarding/diagnóstico hasta plan diario, historial y artefactos, reutilizando los boundaries endurecidos en este paso.
- No implementar Exercise Engine, Research Engine, Curriculum Compiler, IA, plugins ni I-03+ sin su paso y autorización.

## Step 31 — E2E completo del Student & Learning Core

Status: completed
Date: 2026-08-23
Release: unreleased

### Delivered

- Suite `TestStudentLearningCoreEndToEnd` con los diez escenarios del plan sobre workspaces temporales aislados y el binario real con build tag `e2e`.
- Recorrido de nuevo estudiante por `init`, TUI, onboarding, objetivo, threshold, opt-out diagnóstico, Today y reapertura con verificación de persistencia.
- Diagnóstico integrado completo sobre `foundation-demo@1.0.0`, con reapertura, estimates, cuatro Evidence tipadas y Concept States iniciales que no inventan mastery confirmado.
- Harness de aplicación sobre el adaptador SQLite y fixture determinista `student-core-lifecycle@1.0.0` para evidence/mastery/recalculate/unlock, Mistake Memory/warm-up, retention/review reschedule, Daily Plan y history/streak.
- Clock mutable inyectado para vencimiento de retention, reviews y simulación de tres días consecutivos; IDs y Evidence metadata de prueba explícitamente versionados.
- Daily Plan E2E con prerrequisito crítico vencido, review due independiente y siguiente concepto elegible, comprobando prioridad v1, posiciones y presupuesto de 60 minutos.
- Protección E2E de Markdown generado por `progress export`: una edición manual produce conflicto accionable y se conserva byte por byte.
- Fixture de schema Foundation I-01 v3 bajo build tag `e2e`, construido desde las migrations publicadas, y migración real por el binario hasta v22 preservando `app_state`.
- Smoke offline de todos los comandos Student Core expuestos, con policy de red desactivada y adaptador E2E que falla si fuese invocado.
- Gate y documentación renombrados a Foundation and Student Core E2E; las matrices CI y release lo ejecutan en Linux, macOS y Windows.

### Decisions

- El harness compone servicios de aplicación existentes sobre `sqlite.Database`; no expone handles SQLite al dominio, no añade comandos de mutación educativa y no adelanta Exercise Engine.
- El helper de migración I-01 solo existe con build tag `e2e` y reutiliza `foundationMigrations[:3]`, evitando una copia divergente del SQL o cambios a migrations publicadas.
- El E2E de TUI se omite en Windows porque stdin por pipe no es un console handle; los recorridos equivalentes de setup/persistencia y todos los demás escenarios sí compilan y se ejecutan donde son viables.
- El historial público se verifica en su contrato newest-first; el planner se verifica contra el orden publicado: warm-up crítico, review importante y nuevo aprendizaje.
- Los tests validan selección y scheduling metadata, no generan ejercicios ni incorporan contenido, Research, Curriculum Compiler, IA, plugins o red.

### Verification

- `GOCACHE=/tmp/kelyro-step31-e2e-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go test -tags=e2e ./tests/e2e`.
- Tests dirigidos de `tools/quality` e `internal/storage/sqlite`.
- Compilación de la suite E2E para `windows/amd64` y `darwin/amd64` con `CGO_ENABLED=0` y `-exec=/bin/true`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-step31-quality-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 32 es el siguiente paso pendiente y requiere autorización explícita.
- El dogfooding debe usar estos recorridos como baseline, registrar fricción real y evitar convertir observaciones en cambios fuera del alcance autorizado.
- No implementar Exercise Engine, Research Engine, Curriculum Compiler, IA, plugins ni I-03+ sin su paso y autorización.

## Step 32 — Controlled I-02 dogfooding

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Multiple real binary/TUI sessions on clean temporary workspaces covering onboarding start, durable abandon/resume, deterministic diagnostic, reopen/resume, threshold changes, Roadmap/Concept/Today/Progress/Reviews/History/Streak navigation, locked explanations, and 24-column rendering.
- Backup/restore roundtrip that restored the effective 85% workspace mastery override after an intentional reset, followed by a healthy integrity scan.
- Full portable export dry-run/import roundtrip into a second workspace, preserving the profile, active goal, curriculum, diagnostic-derived history, mastery policy, session view and database integrity.
- Explicit Markdown ownership check in which a learner edit to `00-roadmap/PROGRESS.md` produced an actionable conflict and remained byte-for-byte intact.
- Offline command pass with `privacy.allow_network=false`; the E2E network adapter remained unreachable for all Student Core read commands.
- Application/E2E dogfooding for evidence-driven mastery and dependent unlock, repeated-mistake deduplication and warm-up selection, fake-clock retention/review rescheduling, explainable Daily Plan ordering, multi-day History/time/streak and analytics.
- Ephemeral `step32-dogfood/v1` UX fixture with 2 phases, 6 modules, 18 lessons, 18 topics and 72 concepts (116 nodes total); the helper was removed after seeding and no dogfooding-only product surface remains.
- Height-bounded scrolling for long read-only TUI views after the large fixture exposed that Roadmap previously rendered only its final terminal page. The fix keeps the first/current concept visible and supports line, page, Home and End navigation.

### Finding and fix

- Reproduction: reopen Roadmap for the 116-node fixture in an 80x20 terminal. Before the fix, Bubble Tea emitted all 178 rendered lines and the terminal displayed only the final concepts and footer; the current concept and beginning of the hierarchy were unreachable.
- Regression: `TestLongRoadmapUsesHeightBoundedScrollableViewport` asserts terminal-height bounds, initial/current content, range guidance, End and Home behavior.
- Fix: commit `1e64e1b` (`fix(tui): add height-bounded view scrolling`) adds an internal viewport without a new dependency, leaves Config/onboarding input keys untouched, resets ephemeral scroll on navigation and clamps it on resize.
- Manual confirmation: the same 80x20 workspace opened at `1-18/178`, Page Down advanced to `18-35/178`, End reached `161-178/178`, and Home returned to `1-18/178`. The 24-column TUI remained navigable.
- No patch release is required: I-02 remains unreleased and the published Foundation release does not contain this TUI.

### Decisions

- Public CLI mutation commands were not added for dogfooding. Evidence, mistakes, clock advancement and review outcomes continue to use the test/application harness so the Exercise Engine boundary is not weakened.
- Scroll position and terminal height remain presentation-only ephemeral state; neither is checkpointed or persisted with educational state.
- Linux received the real interactive/manual pass available in this environment. macOS and Windows received cross-platform compilation of the TUI and complete E2E package; platform CI remains the authoritative real-host execution gate.
- The small researched-content substitute remains explicitly fixture-sourced. No claim is made that the fixture teaches a production subject, and no Research Engine, Curriculum Compiler, generated exercise, AI provider, plugin or network behavior was introduced.

### Exit gate

- No observed progress loss or database corruption.
- Mastery recalculation and prerequisite unlock remained deterministic.
- No duplicate pending review or onboarding resume failure was observed.
- Backup/restore, full export/import, offline operation and Markdown protection preserved managed state.
- No frequent crash or known cross-platform compilation blocker remains.

### Verification

- Manual tagged-E2E binary sessions on Linux at 80x20 and 24x20 over clean, restored, imported and large-fixture workspaces.
- `GOCACHE=/tmp/kelyro-step32-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go test ./internal/tui`.
- `GOCACHE=/tmp/kelyro-step32-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go test -tags=e2e -run '^TestStudentLearningCoreEndToEnd$' ./tests/e2e` before and after the fix.
- `GOCACHE=/tmp/kelyro-step32-analytics-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go test -run 'Analytics|ProgressDashboardHandlesThousands' ./internal/learning ./internal/learning/application`.
- `GOCACHE=/tmp/kelyro-step32-scale-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go test -run '^TestVersionedLargeStudentCoreFixtureUsesBoundedIndexedProjections$' ./internal/storage/sqlite`.
- Cross-platform compile checks with `GOOS=windows` and `GOOS=darwin`, `CGO_ENABLED=0`, `-tags=e2e` and `-exec=/bin/true` for `./internal/tui ./tests/e2e`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-step32-quality-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go run ./tools/quality all`.
- `git diff --check`.

### Notes for next session

- El Paso 33 es el siguiente paso pendiente y requiere autorización explícita.
- El cierre formal debe usar este gate de dogfooding como evidencia y no reabrir features de I-02 sin un bug reproducible.
- No comenzar I-03 ni implementar Research Engine, Curriculum Compiler, Exercise Engine, IA o plugins sin su implementación y autorización propias.

## Step 33 — Formal I-02 Student & Learning Core closure

Status: completed
Date: 2026-08-24
Release at source closure: unreleased; subsequent `v0.1.0-alpha.3` publication
is recorded below

### Delivered

- Formal audit of all 34 implementation steps against their per-step completion records and Conventional Commits.
- Final execution, capability, and Definition of Done checklists synchronized with the delivered and verified repository state.
- Public status updated to distinguish completed source from the latest published Foundation prerelease.
- I-02 compatibility boundaries retained in `AGENTS.md`, with later implementations requiring their own specification and authorization.
- Stable domain, curriculum, mastery, retention, daily-plan, dashboard, TUI, and adapter boundaries reviewed and indexed as completed v1 contracts.
- Dependency audit confirming that `internal/learning` remains standard-library-only and does not know Bubble Tea, SQLite, GitHub, AI providers, research, curriculum generation, or the operating system.
- Full quality and cross-platform GitHub Actions gates for Foundation and Student Core, including Linux race coverage and Linux/macOS/Windows test, E2E, vet, build, and smoke checks.
- At formal closure no release or tag was created: I-02 was complete in source while `v0.1.0-alpha.2` remained the latest published SemVer release.

### Decisions

- Completion of an implementation and publication of a release remain separate events. A future release must choose its version from the then-current SemVer history and use an annotated tag; Step 33 does not guess that version.
- The conditional annotated-tag Definition of Done item is satisfied by explicitly recording that no release was published, so no tag is required or permitted for this closure.
- Hosted CI on `main` is the authoritative platform signal. Local cross-compilation and the full quality runner provide pre-push evidence but do not masquerade as hosted Windows/macOS execution.
- The final checklist is evidence-backed rather than a second implementation tracker: each checked capability maps to a completed step, regression suite, architecture record, or controlled dogfooding result.
- I-03 is ready to begin as a separately specified implementation; no Research Engine, production Learning Pack, full Exercise/Assessment Engine, AI runtime, provider, or plugin was added during closure.

### Hosted CI finding and fix

- The first post-merge matrix exposed an intermittent macOS onboarding failure: two chronologically ordered `RFC3339Nano` values did not always preserve that order when SQLite compared their variable-width TEXT encodings.
- SQLite timestamp writes now use a fixed-width nine-digit fractional representation, while reads remain compatible with previously persisted RFC3339 timestamps.
- `TestTimestampEncodingPreservesTextOrderAndReadsLegacyPrecision` covers the failing fractional-second boundary and legacy decoding; the complete onboarding E2E passes with the corrected codec.
- This is a persistence portability correction only. It does not change domain time semantics, published migrations, educational algorithms, or the I-02 scope.

### Verification

- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-step33-final-quality-gocache GOMODCACHE=/tmp/kelyro-i02-step27-modcache go run ./tools/quality all`.
- Targeted SQLite timestamp regression, learning application tests, and Foundation/Student Core E2E after the hosted macOS finding.
- `go mod verify` and `gofmt -l` over all tracked Go files.
- Direct-import audit of `./internal/learning` and repository search for forbidden Student Core dependencies or premature I-03+ packages.
- Versioned-secret signature scan over tracked files, with no findings.
- Cross-platform GitHub Actions `CI` matrix on merged `main`: Ubuntu, macOS, and Windows all green.
- `git diff --check` and clean working tree after the closure commit.

### Notes for next implementation

- I-02 Student & Learning Core is closed. Reopen it only for a reproducible regression or an explicitly scoped compatibility change.
- I-03 Research & Source Intelligence may now define its own PLAN, PROGRESS, source-verification policies, and authorization boundary.
- Production Learning Packs, generated exercises, assessment execution, AI runtime/providers, and plugins remain outside I-02.

## I-02 Student & Learning Core Completion

Status: completed
Release: v0.1.0-alpha.3 (published prerelease)
Completed steps: 0-33

Algorithms and versioned contracts:
- `curriculum-consumption/v1`
- `diagnostic/v1` and `diagnostic-scoring/v1`
- `threshold-v1`, `mastery-v1`, `prerequisite-v1`, and `progression-v1`
- `study-session-v1`, `study-history-v1`, and `time-tracking-v1`
- `retention-v1`, `review-scheduler-v1`, and `warm-up-selector-v1`
- `streak-v1`, `achievement-v1`, and `learning-analytics-v1`
- `daily-plan-v1` and `progress-dashboard/v1`

Known limitations:
- No Research Engine or source-intelligence pipeline yet.
- No researched production Learning Packs or Curriculum Compiler yet.
- No full Exercise/Assessment Engine or generated learning content yet.
- No AI runtime, AI provider, plugin runtime, or automatic network dependency.
- I-02 is published in `v0.1.0-alpha.3`; its fixture-content and later-engine
  boundaries remain explicit release limitations.

Ready for:
I-03 Research & Source Intelligence

## Published prerelease v0.1.0-alpha.3

Status: published; manual acceptance passed
Date: 2026-08-24
Release: v0.1.0-alpha.3
Published: 2026-08-24T13:12:44Z

### Delivered

- Canonical SemVer prerelease selected for the completed I-02 Student & Learning Core milestone.
- Release-specific scope and limitations recorded in `docs/releases/v0.1.0-alpha.3.md`.
- Annotated tag targets clean release commit `751f6b9995c058b1d0eea1f2e67cdbe5b3cb076d` on `main`.
- The protected workflow created a reviewable GitHub draft with six platform
  archives and `SHA256SUMS`; the maintainer reviewed its metadata, replaced the
  generated notes with the release-specific notes, marked it as a prerelease,
  and published it manually.

### Verification

- Release target is the same `main` history that passed the hosted CI matrix on Ubuntu, macOS, and Windows, including Linux race coverage.
- `go mod verify`, tracked-file formatting, documentation-link checks, `git diff --check`, and clean-tree validation.
- Release workflow independently repeats quality gates and validates the annotated tag, source ancestry, SemVer, archives, embedded metadata, smoke test, and checksums.
- Manual Linux `amd64` acceptance verified SHA-256
  `b9d37a0bd86677960e615fc658f1b9f06db1e35d123aa542fda1a7a516d0f270`,
  the packaged static binary, embedded version and source commit, onboarding,
  diagnostic evidence, learner views, persistence, backup, full portable
  export/import, Markdown ownership protection, maintenance `--dry-run`,
  operation inside an isolated network namespace, and final database,
  migration, configuration, and artifact-index health.
- The accepted state retained the expected I-02 boundaries: diagnostic evidence
  remained estimated rather than confirmed mastery, the deterministic
  curriculum remained fixture-sourced, and no Research Engine, Curriculum
  Compiler, full Exercise/Assessment Engine, AI runtime/provider, or plugin was
  claimed or introduced.
- Final GitHub verification reported `draft=false`, `prerelease=true`, seven
  uploaded assets, and no blocking acceptance defect.

## Post-closure regression — Backup and generated-artifact coherence

Status: completed
Date: 2026-08-30
Release: unreleased

### Reproduction

- A manual packaged-binary pass exported generated learning Markdown at a 90%
  workspace mastery override, created a backup, regenerated the same documents
  at 70%, and restored the backup.
- The database correctly returned to 90%, while the visible Markdown correctly
  remained outside the machine-state backup at 70%.
- `kelyro progress export` then failed with `generated artifact was modified
  externally` because the restored database also restored the older
  `artifact_index` hashes. Kelyro therefore misclassified its own current 70%
  projection as a human edit.

### Fix

- Restore now reconciles the current integrity records for visible generated
  artifacts into the staged database before its atomic swap. The staged
  database receives a second full snapshot validation after reconciliation.
- Backup contents and format remain unchanged: visible Markdown is still not
  copied or overwritten by backup restore.
- A genuine human edit remains protected because its bytes still differ from
  the preserved current content hash.
- No migration, educational algorithm, I-03 behavior, or backup allowlist was
  changed.

### Verification

- Regression tests cover successful regeneration of all three progress
  documents after restore and continued rejection of a genuine human edit.
- Foundation E2E covers the exact `90% -> backup -> 70% -> restore -> progress
  export` CLI path.
- Full quality gate passed: unit/integration tests, tagged E2E, `go vet`, Linux
  race suite, build, version smoke, and help smoke.
- Release tooling built six archives for commit `f412d28`; all entries in
  `SHA256SUMS` verified, and the installed Linux `amd64` binary was static with
  correct embedded version, commit, and build date.
- Manual isolated Linux acceptance passed onboarding through the TUI, the
  corrected restore/regeneration path, post-restore human-edit protection,
  full export/dry-run/import, goal pause/resume, learner views, offline update
  and Research/Source commands, cache maintenance, recalculation dry-run,
  audit inspection, and final Doctor health.

## Post-closure regression — TUI dashboard and session-write serialization

Status: completed
Date: 2026-08-30
Release: v0.2.0-alpha.1

### Reproduction

- Release-candidate CI run `33327245490` passed unit tests on Ubuntu but the
  Foundation/Student/Research E2E failed after onboarding with `SQLITE_BUSY`.
- The completed-setup view started a dashboard refresh, which may persist the
  adaptive daily plan. Pressing Enter immediately started a session checkpoint
  through a second SQLite connection while that refresh was still active.
- The collision was timing-sensitive: the candidate passed locally and on
  macOS/Windows, while the slower Ubuntu runner exposed it reliably in that
  run.

### Fix

- Session checkpoints requested during a dashboard load are now deferred until
  the dashboard succeeds or fails, then resumed exactly once.
- Graceful quit follows the same ordering and waits for an active dashboard
  load before completing the durable session.
- The change is confined to TUI command coordination. It does not change the
  SQLite schema, transaction semantics, educational policies, or Research
  behavior.

### Verification

- Unit regressions prove that onboarding-to-home does not checkpoint while the
  dashboard command is active and that quit does not complete the session
  concurrently with that command.
- The exact failed `reopen_and_resume` E2E path passed 10 consecutive runs.
- The complete tagged E2E package passed five consecutive runs.
- The full local quality gate passed: unit/integration tests, tagged E2E,
  `go vet`, Linux race suite, build, version smoke, and help smoke.
