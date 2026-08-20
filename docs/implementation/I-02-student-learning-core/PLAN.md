# Kelyro — Plan de Implementación I-02: Student & Learning Core

> Objetivo de esta implementación: construir el cerebro educativo local de Kelyro. Al terminar I-02, Kelyro debe poder conocer al estudiante, realizar onboarding y diagnóstico determinista, registrar un objetivo de aprendizaje, representar una ruta estructurada, calcular dominio por concepto, bloquear/desbloquear progreso según mastery, recordar errores, programar repasos, mantener historial, tiempo, streaks, achievements y analytics, y producir un plan diario adaptativo.
>
> I-02 **NO** implementa todavía el Research Engine (I-03), el Curriculum Compiler/Learning Packs reales (I-04), el motor completo de ejercicios/evaluaciones (I-05), proveedores de IA (I-07) ni plugins (I-08). Para desarrollar y probar I-02 se usarán **fixtures curriculares deterministas y versionados**.
>
> Este documento está diseñado para Spec-Driven Development con Codex en sesiones independientes. Cada paso debe poder planearse, implementarse, verificarse, registrarse y comitearse sin depender del historial del chat.

---

# Cobertura funcional de I-02

Esta implementación cubre las capacidades maestras definidas para:

```text
#5–18
Onboarding, diagnóstico, learning goal, mastery threshold,
estructura granular de curriculum, determinismo, grafo,
prerequisitos, phases, modules, lessons, topics,
theory-first y progressive difficulty.

#28–35
Student Model, SQLite Brain, concept mastery,
evidence-based mastery, mistake memory,
retention tracking, spaced repetition y warm-ups.

#134–140
Progress dashboard, study history, time tracking,
streaks, achievements, analytics y adaptive daily plan.
```

## Frontera de responsabilidad

I-02 debe construir:

```text
Student
Profile
Goal
Diagnostic
Knowledge / Concept State
Mastery
Evidence
Mistakes
Retention
Reviews
Sessions
History
Time
Streaks
Achievements
Analytics
Daily Plan

           +

Contratos para CONSUMIR un curriculum estructurado
```

I-02 no debe construir:

```text
Web research
Source verification
Curriculum generation desde Internet
Learning Pack marketplace
AI tutor
AI providers
Generated exercises
Full assessment engine
Git project workflows
Plugins
Notifications externas
```

---

# Principios obligatorios de I-02

1. **El repositorio es la memoria**, no el chat.
2. **SQLite es infraestructura**, no el dominio.
3. **Bubble Tea es UI**, no el dominio.
4. **El Student Core funciona offline**.
5. **No se necesita IA para que I-02 funcione**.
6. **El curriculum consumido es determinista**.
7. **La IA futura no podrá saltarse prerequisitos por sí sola**.
8. **Mastery se calcula con evidencia**, no con una sola nota.
9. **Completar una lección no equivale a dominar un concepto**.
10. **Los conceptos tienen IDs estables**, no dependen del título visible.
11. **La granularidad del conocimiento no tiene límite artificial**.
12. **El estado del estudiante es persistente y auditable**.
13. **No guardar contenido sensible del estudiante innecesariamente**.
14. **El alumno debe poder inspeccionar su progreso en formatos humanos**.
15. **Los algoritmos educativos deben estar versionados** para permitir cambios futuros.
16. **Los cálculos deben ser deterministas y testeables**.
17. **Las métricas deben explicar su significado**; no mostrar números mágicos.
18. **No implementar I-03/I-04/I-05 prematuramente** para “hacer que la demo se vea completa”.

---

## Paso 0 — Abrir formalmente I-02 y establecer el protocolo de transición desde Foundation

- [x] Paso 0 completado

### Objetivo

Abrir la implementación I-02 sin perder la trazabilidad de I-01 y preparar la memoria persistente para nuevas sesiones de Codex.

### Precondiciones

I-01 debe estar marcada como completada y probada.

Antes de modificar código:

```bash
git status
git log --oneline --decorate -n 15
go test ./...
go vet ./...
```

Si existe suite E2E Foundation, ejecutarla.

No continuar si existen fallos críticos conocidos de I-01 que puedan contaminar I-02.

### Crear

```text
docs/
└── implementation/
    └── I-02-student-learning-core/
        ├── PLAN.md
        └── PROGRESS.md
```

`PLAN.md` debe contener este documento.

`PROGRESS.md`:

```md
# I-02 Student & Learning Core — Progress Log

## Estado general

Current step: 0
Last completed step: none
Current release: <versión actual del repositorio>
Foundation baseline: <tag/commit de I-01>

## Registro
```

### Actualizar `AGENTS.md`

Agregar, sin convertirlo en un archivo gigante:

1. Para I-02 leer:
   - `docs/implementation/I-02-student-learning-core/PLAN.md`;
   - `docs/implementation/I-02-student-learning-core/PROGRESS.md`.
2. No implementar Research Engine, Curriculum Compiler, AI Provider, Plugins ni Exercise Engine completo.
3. Para pruebas educativas usar fixtures deterministas.
4. No agregar lógica educativa dentro de handlers TUI/CLI.
5. Toda fórmula/algoritmo de mastery, retention, streak o scheduling debe:
   - estar encapsulado;
   - tener tests;
   - tener versión/configuración explícita cuando corresponda.
6. No modificar migrations ya publicadas.
7. No asumir que existe un único lenguaje/programación como dominio futuro.

### Prompt reutilizable de sesión

```text
Trabaja únicamente en el Paso XX de
docs/implementation/I-02-student-learning-core/PLAN.md.

Antes de modificar código:
1. Lee AGENTS.md.
2. Lee el Paso XX completo.
3. Lee PROGRESS.md.
4. Revisa git status.
5. Revisa los commits relevantes más recientes.
6. Inspecciona solo los paquetes necesarios.
7. En Plan Mode, propón el plan concreto para ejecutar únicamente este paso.
8. No implementes pasos posteriores ni features de I-03+.
9. Al terminar ejecuta todos los criterios de verificación.
10. Si todo pasa, marca el checkbox correspondiente, actualiza PROGRESS.md y crea commits Conventional Commit coherentes.
```

### Uso recomendado de esfuerzo en Codex

**High por defecto.**

Considerar **xhigh** especialmente para:

```text
Paso 1   Domain model
Paso 6   Diagnostic framework
Paso 8   Curriculum consumption model
Paso 9   Knowledge graph/prerequisites
Paso 13  Mastery Engine
Paso 18  Retention model
Paso 19  Spaced repetition
Paso 24  Adaptive Daily Plan
Paso 29  Data migrations/compatibility
Paso 30  Integrity/privacy/performance hardening
Paso 31  Final hardening
```

### Criterios de aceptación

- I-01 está estable.
- PLAN y PROGRESS de I-02 existen.
- AGENTS.md conoce la frontera de I-02.
- El baseline de Foundation queda registrado.
- Working tree limpio.

### Commit sugerido

```text
docs(roadmap): open I-02 Student and Learning Core
```

---

## Paso 1 — Diseñar el modelo de dominio educativo de Kelyro

- [x] Paso 1 completado

### Objetivo

Definir el lenguaje del dominio antes de diseñar tablas, TUI o comandos.

### Crear paquetes de dominio

Estructura sugerida:

```text
internal/
└── learning/
    ├── student/
    ├── goal/
    ├── curriculum/
    ├── mastery/
    ├── evidence/
    ├── mistake/
    ├── session/
    ├── review/
    ├── streak/
    ├── achievement/
    ├── analytics/
    └── dailyplan/
```

La estructura exacta puede simplificarse si Codex justifica una alternativa más clara. Evitar micro-paquetes sin valor.

### Entidades y value objects mínimos

Definir conceptos equivalentes a:

```text
Student
StudentProfile
LearningGoal
ExperienceLevel
StudyPreference
Availability
MasteryThreshold

CurriculumRef
Phase
Module
Lesson
Topic
Concept
Prerequisite

ConceptState
MasteryScore
Evidence
EvidenceType
Mistake
LearningSession
StudyActivity

RetentionState
ReviewSchedule
ReviewItem

Streak
Achievement
Milestone

AnalyticsSnapshot
DailyPlan
DailyPlanItem
```

### IDs

Usar IDs estables.

Ejemplo conceptual:

```text
go.variables.short-declaration
```

Nunca utilizar el título visible como primary identity.

### Reglas de dominio

Definir invariantes:

- mastery ∈ rango válido;
- threshold ∈ rango válido;
- conceptos no pueden prerequerirse a sí mismos;
- una sesión tiene start antes de end;
- un goal debe tener estado;
- evidence tiene source/type/timestamp;
- errores pertenecen a conceptos conocidos;
- una review no puede quedar due antes de haber sido introducido el concepto salvo import explícito;
- IDs no vacíos;
- timestamps UTC internamente.

### Estados sugeridos

#### Learning Goal

```text
draft
active
paused
completed
archived
```

#### Concept lifecycle

Separar **exposure state** de **mastery**.

Ejemplo:

```text
not_seen
introduced
learning
practicing
mastered
review_due
```

No convertir estos estados en sustituto del score.

### No implementar

- DB;
- TUI;
- AI;
- curriculum real.

### Documentación

Crear:

```text
docs/architecture/student-learning-domain.md
```

Incluir glosario preciso.

### Tests

- constructores/validación;
- límites de score;
- IDs;
- estados inválidos;
- timestamps;
- value objects.

### Verificación

```bash
go test ./...
go vet ./...
```

### Criterios de aceptación

- Existe lenguaje de dominio coherente.
- No depende de SQLite.
- No depende de Bubble Tea.
- No depende de tecnología específica como Go.
- No existe ningún `interface{}`/`any` innecesario para evitar modelar correctamente el dominio.

### Commit sugerido

```text
feat(learning): define Student Core domain model
```

---

## Paso 2 — Diseñar repositorios y application services del Student Core

- [x] Paso 2 completado

### Objetivo

Definir cómo el dominio será utilizado por CLI/TUI sin acoplarlo a SQLite.

### Interfaces de repositorio

Diseñar interfaces equivalentes a:

```text
StudentRepository
GoalRepository
CurriculumStateRepository
ConceptStateRepository
EvidenceRepository
MistakeRepository
SessionRepository
ReviewRepository
StreakRepository
AchievementRepository
AnalyticsRepository
```

No crear una mega-interface `LearningRepository`.

### Application Services iniciales

Crear contratos equivalentes a:

```text
StudentService
GoalService
ProgressService
SessionService
ReviewService
AnalyticsService
DailyPlanService
```

### Reglas

1. Application services orquestan casos de uso.
2. Entidades de dominio mantienen invariantes.
3. Repositories persisten.
4. CLI/TUI solo llaman servicios.
5. No filtrar `sql.Row`, `*sql.DB`, SQL errors ni structs SQLite hacia application/domain.
6. Errores deben poder clasificarse:
   - not found;
   - conflict;
   - invalid state;
   - unavailable;
   - persistence failure.

### Transacciones

Definir un patrón de Unit of Work o transaction boundary si se necesita para:

```text
record evidence
recalculate mastery
update concept state
schedule review
append history
```

No sobrearquitecturar; debe ser testeable.

### Fakes

Crear repositories in-memory para tests.

### Tests

- services con fake repositories;
- error mapping;
- transacciones simuladas si aplica.

### Criterios de aceptación

- UI puede usar Student Core sin conocer SQLite.
- Tests pueden ejecutarse sin SQLite.
- Interfaces son pequeñas y orientadas a casos de uso.

### Commit sugerido

```text
refactor(learning): define Student Core service boundaries
```

---

## Paso 3 — Añadir schema SQLite y migrations de Student Core

- [x] Paso 3 completado

### Objetivo

Persistir el modelo del estudiante usando el Migration Engine creado en I-01.

### No modificar migrations antiguas

Agregar nuevas migrations únicamente.

### Tablas sugeridas

La normalización exacta debe decidirse durante Plan Mode, pero cubrir como mínimo:

```text
students
student_profiles
learning_goals
student_preferences

curriculum_instances
curriculum_nodes
curriculum_edges
student_concept_states

learning_evidence
mistakes

study_sessions
study_activities

review_schedule

streak_state
achievement_definitions
student_achievements

analytics_snapshots
daily_plans
```

Puede que algunas tablas no deban crearse todavía si serían puro almacenamiento derivado. Justificar cualquier omisión.

### Reglas de DB

1. Stable IDs.
2. Foreign keys.
3. Índices para:
   - concept lookups;
   - due reviews;
   - history timeline;
   - active goal;
   - session ranges.
4. UTC timestamps.
5. No guardar datos calculables innecesariamente si aumenta inconsistencia.
6. Cuando sí se cachee un cálculo, incluir estrategia de invalidación.
7. No guardar secrets.
8. No guardar transcript completo de IA; I-07 no existe.
9. Migration forward-only.
10. Backup compatible con I-01.

### Repositories SQLite

Implementar adapters para lo necesario en pasos inmediatos.

### Tests

- migration Foundation → I-02 schema;
- DB nueva;
- constraints;
- índices principales;
- repository roundtrips;
- FK behavior;
- migration repetida.

### Verificación

```bash
go test ./...
go vet ./...
```

### Criterios de aceptación

- Workspaces I-01 migran sin perder estado.
- DB nueva llega al schema actual.
- Domain sigue sin importar SQLite.

### Commit sugerido

```text
feat(storage): add Student Core persistence schema
```

### SemVer

La migration compatible que habilita nueva funcionalidad normalmente acompaña un **MINOR** release, no una nueva major por sí sola.

---

## Paso 4 — Implementar el Student Profile

- [x] Paso 4 completado

### Objetivo

Permitir que Kelyro represente quién está aprendiendo sin convertir el perfil en una red social.

### Datos iniciales

Guardar únicamente información necesaria para personalizar aprendizaje.

Ejemplo:

```text
display_name (opcional)
experience_level
preferred_language
daily_time_budget
weekly_days_target
learning_style_preferences
timezone
created_at
updated_at
```

Evitar recopilar:

```text
edad
género
dirección
información sensible
```

salvo que exista un caso educativo futuro claramente justificado.

### Experience level

No limitarlo a:

```text
beginner/intermediate/advanced
```

porque alguien puede ser experto en Java y principiante en Go.

El perfil global puede incluir experiencia general, mientras el diagnóstico por objetivo determina conocimiento específico.

### CLI

Agregar soporte inicial:

```bash
kelyro profile show
kelyro profile edit
```

### TUI

Pantalla de perfil simple, todavía no onboarding completo.

### Tests

- create/update;
- validation;
- persistence;
- default values;
- timezone;
- empty optional fields.

### Human-readable output

`kelyro profile show` debe ser legible, no JSON por defecto.

### Commit sugerido

```text
feat(student): add persistent learner profile
```

---

## Paso 5 — Implementar Learning Goals

- [x] Paso 5 completado

### Objetivo

Representar explícitamente lo que el estudiante quiere conseguir.

### Ejemplos

```text
Backend Engineer with Go
Learn Differential Equations
Master Git for professional development
```

El Student Core debe ser general aunque los primeros Learning Packs sean tecnológicos.

### Campos sugeridos

```text
goal_id
title
description
domain
target_outcome
starting_level
status
created_at
activated_at
completed_at
```

`domain` no debe ser un enum rígido imposible de extender.

### Regla inicial

Un workspace puede comenzar soportando **un goal activo**.

Diseñar el dominio para permitir múltiples goals en el futuro sin reescribir IDs/tablas.

### CLI

```bash
kelyro goal show
kelyro goal set
kelyro goal pause
kelyro goal resume
```

### Reglas

- no dos goals activos si el Foundation UX todavía no los soporta;
- activar uno pausa/archiva explícitamente al anterior según flujo elegido;
- no eliminar historial por cambiar objetivo.

### Tests

- lifecycle;
- active uniqueness;
- persistence;
- history.

### Commit sugerido

```text
feat(goal): add learning goal lifecycle
```

---

## Paso 6 — Construir el framework de onboarding resumible

- [x] Paso 6 completado

### Objetivo

Crear una entrevista inicial estructurada que pueda interrumpirse y retomarse.

### No usar IA

Las preguntas son deterministas y provienen del sistema/pack futuro.

### Onboarding sections

Como mínimo:

```text
1. Identity / preferred display name
2. Goal
3. General background
4. Prior technical/subject experience
5. Time availability
6. Study preferences
7. Mastery strictness
8. Diagnostic opt-in
9. Summary
10. Confirm
```

### State machine

Modelar:

```text
not_started
in_progress
completed
cancelled
```

y current section.

### Requisitos

1. Cada respuesta se valida.
2. Puede volver atrás.
3. Puede abandonar y continuar otro día.
4. No crea goal definitivo hasta confirmación o usa draft state.
5. No perder respuestas al cerrar TUI.
6. Ctrl+C no corrompe state.
7. La entrevista debe poder ejecutarse:
   - en TUI;
   - eventualmente CLI no interactiva mediante services.
8. No mezclar rendering Bubble Tea con reglas del wizard.
9. Los pasos de onboarding deben ser configurables por futura `Learning Pack`.
10. Las preguntas comunes pertenecen al core; preguntas específicas al pack futuro.

### TUI

Crear experiencia cuidada:

```text
Kelyro Setup
Step 3 of 8

What is your current programming experience?

> None
  I have tried tutorials
  I can build small programs
  I work professionally
```

### Tests

- state transitions;
- resume;
- back;
- cancel;
- invalid input;
- crash/resume;
- persistence.

### Commit sugerido

```text
feat(onboarding): add resumable learner interview
```

---

## Paso 7 — Implementar Mastery Threshold y política de avance

- [x] Paso 7 completado

### Objetivo

Permitir definir cuán estricto será Kelyro antes de considerar un concepto/módulo suficientemente dominado.

### Presets

Ejemplo:

```text
Relaxed     0.70
Standard    0.80
Strict      0.85
Mastery     0.90
Custom      0.50–0.99
```

Los rangos exactos deben documentarse.

### Regla importante

El threshold no es una nota de examen.

Es el valor mínimo de **mastery calculado** requerido por la política de progresión.

### Scope

Permitir:

```text
global student default
workspace override
future pack override con límites
```

Precedence debe ser clara.

### Configuración desde onboarding

La entrevista debe guardar el threshold elegido.

### CLI

```bash
kelyro mastery threshold
kelyro mastery threshold set 85
```

Output humano:

```text
Required mastery: 85%
Mode: Strict
```

### Tests

- presets;
- custom;
- invalid ranges;
- config precedence;
- persistence.

### No implementar todavía

- full unlock override;
- assessments I-05.

### Commit sugerido

```text
feat(mastery): add configurable progression threshold
```

---

## Paso 8 — Definir el modelo consumible de Curriculum, Phases, Modules, Lessons, Topics y Concepts

- [x] Paso 8 completado

### Objetivo

Permitir que el Student Core consuma un curriculum granular sin construir todavía el Curriculum Compiler.

### Fuente en I-02

Crear fixtures de desarrollo:

```text
testdata/
└── curricula/
    └── foundation-demo/
        ├── curriculum.yaml
        └── ...
```

No llamarlo oficialmente `backend-go` si no es un pack investigado real.

### Jerarquía visible

```text
Curriculum
└── Phase
    └── Module
        └── Lesson
            └── Topic
                └── Concept
```

### Knowledge graph real

La jerarquía sirve para UX.

Los prerequisitos reales se representan como grafo entre conceptos/nodos.

### Requisitos de nodos

Cada nodo debe tener:

```text
stable ID
type
title
description
order/display hints
status metadata
version
```

### Concept

Debe permitir:

```text
objectives
prerequisites
difficulty
estimated effort
theory_required
assessment expectations
```

Sin almacenar todavía contenido generado.

### Granularidad

No imponer límites como:

```text
máximo 10 modules
máximo 5 topics
```

### Determinismo

Un mismo curriculum fixture/version debe cargar el mismo grafo.

### Validation

Rechazar:

- duplicate IDs;
- missing parents;
- invalid node types;
- dangling edges;
- cycles donde no sean permitidos;
- invalid order;
- unknown prerequisites.

### Tests

- valid fixture;
- duplicate ID;
- cycle;
- missing prerequisite;
- large graph fixture;
- deterministic load.

### Documentación

Crear:

```text
docs/architecture/curriculum-consumption-contract.md
```

Explicar claramente:

> I-02 consume. I-03 investiga. I-04 compila/versiona packs reales.

### Commit sugerido

```text
feat(curriculum): define deterministic curriculum consumption model
```

---

## Paso 9 — Implementar Knowledge Graph y Prerequisite Engine

- [x] Paso 9 completado

### Objetivo

Determinar qué conocimientos habilitan otros conocimientos sin depender del orden visual de carpetas.

### Operaciones requeridas

```text
GetPrerequisites(concept)
GetDependents(concept)
Ancestors(concept)
CanIntroduce(concept, studentState)
MissingPrerequisites(concept, studentState)
TopologicalOrder(...)
```

### Política inicial

Un concepto puede introducirse cuando sus prerequisitos cumplen la política configurada.

Separar:

```text
introduced prerequisite
mastered prerequisite
```

Algunos conceptos pueden requerir solo exposición; otros mastery.

El curriculum debe poder declarar la política.

### Requisitos

1. Determinista.
2. Eficiente para cientos/miles de conceptos.
3. Detectar ciclos.
4. Mensajes explicables.
5. No esconder por qué algo está bloqueado.
6. No mezclar graph traversal con DB directamente.

### Ejemplo de explicación

```text
Pointers is locked.

Required:
✓ Variables — 91%
✓ Functions — 88%
✗ Memory model — 63% (requires 85%)
```

### Tests

- chain;
- diamond graph;
- multiple prereqs;
- missing state;
- cycle;
- large graph;
- threshold boundary.

### Commit sugerido

```text
feat(curriculum): add prerequisite knowledge graph engine
```

---

## Paso 10 — Crear Curriculum Instance y estado personalizado por estudiante

- [x] Paso 10 completado

### Objetivo

Separar el curriculum versionado de la instancia que está siguiendo una persona.

### Conceptos

```text
Curriculum Definition
        ↓
Curriculum Instance
        ↓
Student Concept State
```

### Curriculum Instance

Debe guardar:

```text
curriculum_id
curriculum_version
goal_id
created_at
status
source kind = fixture/import/pack future
```

### Student Concept State

Por concepto:

```text
exposure_state
mastery_score
first_seen_at
last_seen_at
mastered_at
review_due_at
manual flags futuros
```

No duplicar evidence aquí.

### Requisitos

1. Instanciar fixture.
2. Inicializar estados lazily o eager, decisión documentada.
3. Versionar referencia al curriculum.
4. No mutar definición original por progreso del estudiante.
5. Reabrir produce el mismo estado.
6. Preparar futura migration de curriculum version.

### Tests

- create instance;
- duplicate protection;
- state isolation entre instancias;
- version reference.

### Commit sugerido

```text
feat(curriculum): add learner curriculum instances
```

---

## Paso 11 — Implementar diagnóstico inicial determinista

- [x] Paso 11 completado

### Objetivo

Permitir estimar conocimiento previo sin usar un LLM.

### Importante

I-02 implementa el **framework de diagnóstico**.

Las preguntas reales de cada pack serán responsabilidad futura del contenido curricular.

### Diagnostic model

```text
Diagnostic
DiagnosticSection
DiagnosticItem
DiagnosticAttempt
DiagnosticResult
ConceptEstimate
```

### Tipos de ítem iniciales

Mantener pocos tipos genéricos:

```text
single choice
multiple choice
short answer con evaluator determinista
self-report calibration
```

No construir code runner completo; corresponde a I-05.

### Fixture

Crear un diagnóstico de desarrollo pequeño asociado al curriculum fixture.

### Adaptive behavior básico

No necesita ser IA.

Puede:

- saltar preguntas redundantes si ya hay evidencia suficiente;
- detener una rama si falla un prerequisito fundamental;
- continuar para validar respuestas positivas.

### Salida

No decir:

```text
You are beginner.
```

únicamente.

Generar:

```text
Concept A: estimated 0.90
Concept B: estimated 0.45
Concept C: unknown
```

Marcar claramente que es **estimated mastery**, no mastery confirmado.

### Reglas

1. Diagnóstico puede omitirse.
2. Resultados se guardan como evidence con tipo específico.
3. Self-report pesa menos que evidencia objetiva.
4. No “masterizar” automáticamente conceptos complejos solo por una respuesta.
5. Mantener confidence.

### Tests

- complete;
- skipped;
- partial;
- resume;
- scoring;
- evidence creation;
- adaptive branching.

### Commit sugerido

```text
feat(diagnostic): add deterministic initial assessment framework
```

---

## Paso 12 — Integrar onboarding, goal, diagnóstico e inicialización del Student State

- [x] Paso 12 completado

### Objetivo

Completar el primer flujo educativo real de Kelyro.

### Flujo

```text
kelyro
  ↓
No student setup
  ↓
Onboarding
  ↓
Goal
  ↓
Mastery threshold
  ↓
Diagnostic?
  ├── yes → diagnostic
  └── no
  ↓
Curriculum fixture selection SOLO en dev/demo
  ↓
Curriculum Instance
  ↓
Initial concept states
  ↓
Setup complete
```

### Nota crítica

La selección del fixture no representa la UX final.

Cuando I-04 exista:

```text
Goal + Research/Pack
        ↓
compiled personalized path
```

I-02 solo necesita una fuente determinista para verificar Student Core.

### Requisitos

1. Flujo resumible.
2. Si falla DB en un punto, no dejar “setup completed”.
3. Operación transaccional cuando corresponda.
4. `setup_completed_at`.
5. Puede reiniciarse en modo desarrollo con comando seguro.
6. No borrar datos de I-01.
7. TUI muestra resumen antes de confirmar.

### Tests

E2E del onboarding.

### Commit sugerido

```text
feat(onboarding): initialize learner state from interview and diagnostic
```

---

## Paso 13 — Implementar Evidence Model y Mastery Engine v1

- [x] Paso 13 completado

### Objetivo

Calcular dominio por concepto a partir de múltiples evidencias.

### No diseñar una “nota mágica”

La fórmula debe:

- ser explícita;
- documentada;
- testeada;
- versionada;
- sustituible en el futuro.

### Evidence types iniciales

Ejemplo:

```text
diagnostic_objective
diagnostic_self_report
knowledge_check
practice_success
practice_failure
assessment
project_evidence
review_recall
manual_import
```

En I-02 algunos tipos no se generan todavía, pero el modelo los puede reservar sin sobreimplementar.

### Evidence fields

```text
evidence_id
concept_id
type
score
confidence
independence
difficulty
occurred_at
source_ref
algorithm_version
```

Los nombres exactos pueden variar.

### Mastery Engine v1

Diseñar una política simple y explicable.

Ejemplo conceptual:

```text
weighted recent evidence
+ confidence
+ independence
+ difficulty normalization
```

No implementar una fórmula compleja sin base.

### Requisitos

1. Score [0,1].
2. Sin evidence → unknown, no 0.
3. Diferenciar unknown vs failed.
4. Evidencia más fuerte pesa más.
5. Hints futuros pueden reducir independence.
6. Evidencia antigua puede ser tratada por Retention Engine, no ocultamente aquí.
7. Recalculation determinista.
8. Versionar algoritmo:
   - `mastery-v1`.
9. Al cambiar algoritmo futuro:
   - recalcular;
   - no reescribir evidence histórico.

### Explicabilidad

Application service debe poder responder:

```text
Why is Variables mastery 82%?
```

con breakdown resumido.

### Tests exhaustivos

- no evidence;
- one evidence;
- conflicting evidence;
- low/high confidence;
- boundary threshold;
- deterministic ordering;
- same timestamps;
- malformed evidence rejected.

### Documentar

```text
docs/architecture/mastery-v1.md
```

### Commit sugerido

```text
feat(mastery): add evidence-based mastery engine v1
```

---

## Paso 14 — Implementar actualización de Concept State y Progression Policy

- [x] Paso 14 completado

### Objetivo

Conectar mastery, exposure state y prerequisitos.

### Flujo

```text
new evidence
    ↓
recalculate mastery
    ↓
update concept state
    ↓
check threshold
    ↓
unlock dependents if eligible
```

### Reglas

Ejemplo:

```text
unknown
introduced
learning
practicing
mastered
review_due
```

El cambio debe ser calculado por políticas explícitas.

### Importante

`mastered` no debe ser irrevocable.

Si Retention Engine detecta review pendiente:

```text
mastered → review_due
```

sin necesariamente borrar el hecho histórico de que fue dominado.

### Unlock

Guardar de manera derivada o explícita según diseño.

Preferir que `CanIntroduce()` sea fuente de verdad para evitar inconsistencias.

### Tests

- threshold exact;
- just below;
- prereq still missing;
- multiple prereqs;
- mastery state transitions;
- recalculation.

### Commit sugerido

```text
feat(progress): connect mastery to curriculum progression
```

---

## Paso 15 — Implementar Mistake Memory

- [ ] Paso 15 completado

### Objetivo

Recordar patrones de error para reforzarlos posteriormente.

### Mistake model

No guardar solamente:

```text
"failed exercise 4"
```

Debe poder capturar:

```text
concept
category
summary
first_seen
last_seen
occurrences
resolved/recent status
source_ref
```

### Categorías genéricas

Ejemplo:

```text
conceptual
syntax
procedure
misconception
careless
tooling
unknown
```

No asumir programación solamente.

### Dedupe

Errores similares pueden acumular ocurrencias si comparten una `mistake_key` estable.

En I-02 el fixture/test puede registrar errores manualmente desde evaluator.

### Reglas

1. No almacenar contenido enorme.
2. No almacenar código completo si no es necesario.
3. El estudiante puede inspeccionar errores.
4. Un mistake puede marcarse como reinforced/resolved, pero conservar historia.
5. Future AI reviewers podrán proponer mistake classifications a través de application services, no escribir DB directamente.

### CLI

```bash
kelyro mistakes
kelyro mistakes show <id>
```

### Tests

- create;
- dedupe;
- increment;
- resolve;
- reopen after recurrence;
- history.

### Commit sugerido

```text
feat(learning): add persistent mistake memory
```

---

## Paso 16 — Implementar lifecycle de Study Sessions

- [ ] Paso 16 completado

### Objetivo

Saber cuándo y cómo estudia el usuario sin registrar cada movimiento.

### Session model

```text
session_id
goal_id
curriculum_instance_id
started_at
ended_at
status
active_duration
activity_count
```

### Estados

```text
active
completed
interrupted
recovered
```

### Requisitos

1. Solo una sesión activa por workspace.
2. Reanudar después de crash razonablemente.
3. Idle time no debe contarse indefinidamente.
4. Definir política de idle:
   - configurable;
   - default razonable.
5. TUI start puede iniciar sesión educativa cuando corresponde.
6. Foundation app open no necesariamente cuenta como study session.
7. Cerrar TUI no siempre significa completar lección.
8. Separar app session de study session.

### CLI futura

```bash
kelyro session status
kelyro session stop
```

### Tests

- start/stop;
- duplicate active;
- crash recovery;
- idle;
- time accumulation.

### Commit sugerido

```text
feat(session): add persistent study session lifecycle
```

---

## Paso 17 — Implementar Study History y Time Tracking

- [ ] Paso 17 completado

### Objetivo

Construir un historial útil para el estudiante y para analytics.

### Study Activity

Registrar eventos educativos significativos:

```text
onboarding.completed
diagnostic.completed
concept.introduced
evidence.recorded
concept.mastered
review.completed
session.completed
achievement.unlocked
```

No duplicar Audit Trail técnico de I-01.

Diferencia:

```text
Audit = qué modificó el sistema
Study History = qué hizo/aprendió el estudiante
```

### Time Tracking

Calcular:

```text
today
this week
this month
total
per module/concept cuando exista evidencia suficiente
```

### Privacidad

- local;
- no telemetry;
- sin capturar keystrokes;
- no medir editor fuera de Kelyro en I-02.

### CLI

```bash
kelyro history
kelyro history --today
kelyro time
```

### Tests

- chronological ordering;
- filters;
- time ranges;
- timezone display;
- UTC storage;
- DST display edge cases.

### Commit sugerido

```text
feat(history): add study timeline and time tracking
```

---

## Paso 18 — Implementar Retention Model v1

- [ ] Paso 18 completado

### Objetivo

Diferenciar “lo aprendí una vez” de “todavía puedo recordarlo”.

### Importante

No pretender modelar perfectamente la memoria humana.

Crear un algoritmo inicial, explícito y sustituible.

### Retention State

Por concepto:

```text
last_successful_recall
last_practice
review_count
successful_reviews
failed_reviews
stability_estimate
retention_status
next_due_at
algorithm_version
```

### Estados posibles

```text
fresh
stable
weakening
due
overdue
unknown
```

### Retention Engine v1

Debe considerar de manera simple:

- mastery previo;
- tiempo desde última evidencia fuerte;
- historial de reviews;
- dificultad;
- éxito/fallo reciente.

No mezclar la fórmula dentro de SQL.

### Versionar

```text
retention-v1
```

### Requisitos

1. Determinista.
2. Clock inyectable en tests.
3. No depender de wall clock global directamente.
4. Timezone solo para presentación; cálculo en UTC.
5. No bajar mastery histórico simplemente por paso del tiempo sin registrar qué significa.
6. `review_due` es una necesidad de comprobación, no prueba automática de olvido.

### Tests

- new concept;
- mastered today;
- due later;
- overdue;
- successful recall extends interval;
- failure shortens interval;
- clock boundary.

### Documentación

```text
docs/architecture/retention-v1.md
```

### Commit sugerido

```text
feat(retention): add versioned retention model v1
```

---

## Paso 19 — Implementar Spaced Repetition Scheduler v1

- [ ] Paso 19 completado

### Objetivo

Programar repasos de conceptos ya introducidos utilizando el Retention Model.

### Scheduler

Entradas:

```text
concept state
retention state
student availability
review history
```

Salida:

```text
ReviewSchedule
```

### Tipos de review

I-02 solo necesita scheduling metadata:

```text
quick_recall
standard_review
deep_review
```

La generación concreta de ejercicios de review será I-05.

### Requisitos

1. Cola de reviews due.
2. Prioridad por:
   - overdue;
   - weak concepts;
   - critical prerequisites.
3. No programar 50 reviews imposibles para un día sin priorización.
4. Respetar time budget.
5. Posponer review explícitamente.
6. Saltar no equivale a aprobar.
7. Failure genera nueva schedule.
8. Idempotencia.
9. Clock injectable.
10. Algorithm version:
    - `review-scheduler-v1`.

### CLI

```bash
kelyro reviews
kelyro reviews due
```

### Tests

- due sorting;
- limited time;
- postpone;
- success;
- failure;
- duplicates;
- timezone display.

### Commit sugerido

```text
feat(review): add spaced repetition scheduler v1
```

---

## Paso 20 — Implementar Warm-up Selector

- [ ] Paso 20 completado

### Objetivo

Seleccionar pequeñas revisiones antes de introducir contenido nuevo.

### Importante

I-02 selecciona **qué conceptos** repasar.

I-05 generará/ejecutará ejercicios reales.

### Input

```text
today's candidate lesson
its prerequisites
due reviews
mistakes
available time
```

### Output

```text
WarmUpPlan
- concept
- reason
- priority
- estimated duration
```

### Ejemplo

```text
Before continuing:

1. Scope — review due
2. Short declarations — repeated mistake
```

### Reglas

1. Warm-up no debe consumir toda la sesión.
2. Priorizar prerequisitos del contenido de hoy.
3. No repetir siempre lo mismo.
4. Explicar por qué fue seleccionado.
5. Puede ser vacío.

### Tests

- no due reviews;
- critical prereq due;
- repeated mistake;
- time budget;
- deterministic tie-breaking.

### Commit sugerido

```text
feat(review): add contextual warm-up selection
```

---

## Paso 21 — Implementar Streaks sin comportamiento punitivo

- [ ] Paso 21 completado

### Objetivo

Mostrar consistencia de estudio como información motivacional, sin convertirla en una barrera educativa.

### Definir qué cuenta como día activo

No basta abrir la TUI.

Debe existir actividad significativa, por ejemplo:

```text
N minutos activos mínimos
o
una actividad educativa completada
```

Definir política versionada/configurable.

### Streak data

```text
current
longest
last_active_local_date
total_active_days
```

### Timezone

El día del streak se calcula en timezone del estudiante.

### Reglas

1. No penalizar mastery por perder streak.
2. No bloquear nada.
3. No enviar guilt messaging.
4. Cambiar timezone no debe permitir explotar fácilmente duplicados; documentar comportamiento.
5. Recalcular desde Study History si state inconsistente.

### Tests

- same day;
- next day;
- skipped day;
- timezone;
- DST;
- longest streak.

### CLI/TUI

Mostrar:

```text
Streak: 6 days
```

sin exagerar.

### Commit sugerido

```text
feat(streak): add non-punitive study streak tracking
```

---

## Paso 22 — Implementar Achievement & Milestone Framework

- [ ] Paso 22 completado

### Objetivo

Reconocer progreso real mediante hitos deterministas.

### No usar IA

Achievement definitions son datos.

### Ejemplos Foundation

```text
first_session
first_concept_mastered
seven_active_days
ten_hours_studied
first_review_completed
module_mastered fixture
```

### Achievement Definition

```text
id
title
description
criteria_type
criteria_config
hidden
version
```

### Student Achievement

```text
achievement_id
unlocked_at
context
```

### Requisitos

1. Idempotente.
2. No desbloquear dos veces.
3. Recalcular desde historial cuando sea posible.
4. No almacenar achievements como única fuente de verdad.
5. Mensajes profesionales.
6. No convertirlo en economía de puntos todavía.

### TUI

Mensaje sutil:

```text
Milestone unlocked
7 active study days
```

### Tests

- unlock;
- no duplicate;
- historical recalculation;
- multiple conditions.

### Commit sugerido

```text
feat(achievement): add learning milestone framework
```

---

## Paso 23 — Implementar Learning Analytics v1

- [ ] Paso 23 completado

### Objetivo

Convertir el estado del estudiante en información útil y explicable.

### Métricas iniciales

#### Progress

```text
concepts introduced
concepts learning
concepts mastered
reviews due
```

#### Time

```text
today
week
month
total
```

#### Mastery

```text
average known mastery
strongest concepts
weakest concepts
```

No promediar `unknown` como 0.

#### Retention

```text
fresh
due
overdue
```

#### Activity

```text
active days
current streak
longest streak
```

#### Pace

```text
concepts mastered per week
study time per week
```

No prometer fecha de “graduación” todavía salvo algoritmo bien definido.

### Analytics Service

Debe poder generar snapshot desde fuentes primarias.

Evitar una tabla de analytics como única verdad.

### Cache

Solo si hace falta por rendimiento.

### Explicabilidad

Cada métrica debe tener descripción.

Ejemplo:

```text
Known mastery average excludes concepts you have not studied yet.
```

### Tests

- unknown exclusion;
- empty profile;
- date ranges;
- deterministic sorting;
- large fixture.

### Commit sugerido

```text
feat(analytics): add explainable learning analytics v1
```

---

## Paso 24 — Implementar Adaptive Daily Plan v1

- [ ] Paso 24 completado

### Objetivo

Decidir qué debería hacer el estudiante hoy usando reglas deterministas.

### Input

```text
active goal
curriculum instance
prerequisites
concept mastery
reviews due
mistakes
student time budget
study history
```

### Output

```text
DailyPlan
├── warm-up
├── reviews
├── new learning candidate
└── optional reinforcement
```

### Importante

I-02 no genera aún contenido completo.

Daily Plan selecciona **qué** trabajar.

I-05 construirá `lesson/practice/assessment`.

### Policy v1

Orden conceptual:

```text
1. Critical overdue prerequisites
2. Important due reviews
3. Reinforcement for blocking weaknesses
4. Next eligible new concept/topic
5. Optional extra practice
```

### Time budget

Ejemplo:

```text
45 min available

5 min warm-up
10 min reviews
25 min new lesson
5 min buffer
```

No necesita optimización perfecta.

### Reglas

1. No introducir concepto bloqueado.
2. No exceder agresivamente time budget.
3. Explicar cada selección.
4. Puede producir “review-only day”.
5. Puede producir “nothing urgent” si corresponde.
6. Plan versionado:
   - `daily-plan-v1`.
7. Guardar snapshot diario para historial.
8. Regenerar solo bajo reglas explícitas.
9. Cambios de state pueden invalidarlo.
10. No depender de IA.

### Tests

- brand-new student;
- due reviews;
- blocked next lesson;
- all current content mastered;
- tiny time budget;
- no active goal;
- deterministic output.

### Documentación

```text
docs/architecture/daily-plan-v1.md
```

### Commit sugerido

```text
feat(planning): add adaptive daily learning plan v1
```

---

## Paso 25 — Construir Progress Dashboard application service

- [ ] Paso 25 completado

### Objetivo

Crear una única vista de lectura coherente para TUI/CLI sin hacer múltiples queries desordenadas desde UI.

### Dashboard model

Debe incluir:

```text
goal
curriculum
overall progress
current phase/module/lesson
mastery summary
reviews due
today plan
study time
streak
recent milestone
weak concepts
```

### Requisitos

1. Query/read model separado de entidades de escritura si ayuda.
2. No incluir datos no disponibles.
3. Empty states correctos.
4. Ninguna query de UI directa a SQLite.
5. Rendimiento razonable con miles de concepts.
6. Dashboard puede refrescarse después de evidence/session.

### Tests

- new student;
- partial progress;
- many concepts;
- due reviews;
- empty goal.

### Commit sugerido

```text
feat(progress): add Student Core dashboard read model
```

---

## Paso 26 — Integrar el Student Core en la TUI de Kelyro

- [ ] Paso 26 completado

### Objetivo

Convertir la Foundation TUI en la primera experiencia educativa persistente real.

### Home después de onboarding

Ejemplo:

```text
╭──────────────── Kelyro ────────────────────╮
│                                            │
│ Backend Engineering with Go                │
│                                            │
│ Progress                                   │
│ ███████░░░░░░░░░░░  18%                    │
│                                            │
│ Today                                      │
│ → 2 reviews                                │
│ → Next: Variables / Initialization         │
│                                            │
│ Mastery required                     85%    │
│ Streak                              6 days  │
│ Study this week                       4h12  │
│                                            │
│ [Enter] Today     [r] Roadmap               │
│ [p] Progress      [h] History               │
│ [q] Quit                                   │
╰────────────────────────────────────────────╯
```

El texto exacto puede variar.

### Pantallas

Agregar:

```text
Onboarding
Today
Progress
Concept detail
Reviews
History
Goal
Profile
```

### Roadmap

Usar curriculum fixture para mostrar:

```text
Phase
  Module
    Lesson
      Topic
```

Estados:

```text
completed/mastered
current
available
locked
review due
```

### UX

1. Mostrar por qué algo está locked.
2. Mostrar diferencia entre completion y mastery.
3. Mostrar unknown sin llamarlo 0%.
4. No saturar con métricas.
5. Funcionar sin color.
6. Accesible en terminal estrecha.
7. No usar caracteres Unicode indispensables para comprender.
8. Mantener mascota/logo fuera de lógica funcional; branding puede añadirse después sin afectar core.

### Tests

- navigation;
- state transitions;
- onboarding → home;
- resize;
- empty;
- dashboard refresh.

### Commit sugerido

```text
feat(tui): integrate Student and Learning Core
```

---

## Paso 27 — Añadir CLI del Student & Learning Core

- [ ] Paso 27 completado

### Objetivo

Permitir consultar y operar Student Core sin entrar a la TUI.

### Comandos

Como mínimo:

```bash
kelyro profile
kelyro goal
kelyro status
kelyro progress
kelyro roadmap
kelyro history
kelyro time
kelyro reviews
kelyro mistakes
kelyro today
kelyro mastery
```

### Ejemplos

```bash
kelyro status
```

```text
Goal: Backend Engineering with Go
Current: Foundations / Variables
Mastery threshold: 85%

Concepts
Mastered: 12
Learning: 4
Review due: 2
```

### Requisitos

1. Human-readable default.
2. Exit codes coherentes.
3. `--help`.
4. No lanzar TUI desde subcommands.
5. `--workspace` Foundation sigue funcionando.
6. No agregar JSON como interfaz principal.
7. Si se necesita output machine-readable futuro, diseñar flag separado sin comprometer UX.

### Tests

- command routing;
- output;
- errors;
- workspace not initialized;
- onboarding incomplete.

### Commit sugerido

```text
feat(cli): expose Student Core commands
```

---

## Paso 28 — Generar artefactos Markdown humanos de progreso

- [ ] Paso 28 completado

### Objetivo

Mantener la filosofía de Kelyro: máquina en SQLite, humano en Markdown.

### Actualizar/generar

```text
LEARNING.md

00-roadmap/
├── ROADMAP.md
└── PROGRESS.md
```

Estos son artifacts de workspace, no confundir con:

```text
docs/implementation/.../PROGRESS.md
```

### `LEARNING.md`

Debe poder incluir:

```md
# Backend Engineering with Go

## Current
...

## Today
...

## Mastery
Required: 85%

## Reviews
...
```

### `ROADMAP.md`

Representación legible del curriculum instance.

### `00-roadmap/PROGRESS.md`

Snapshot humano:

```text
study time
mastered concepts
reviews due
streak
recent milestones
```

### Reglas

1. SQLite sigue siendo source of truth.
2. Markdown es view.
3. Respetar Artifact Ownership de I-01.
4. Si usuario modifica artifact generado:
   - no sobrescribir silenciosamente.
5. No volcar datos internos/IDs innecesarios.
6. No incluir información sensible.
7. Golden tests.
8. Regeneración explícita/event-driven, no en cada keypress.

### CLI

```bash
kelyro progress export
```

o comando equivalente para regenerar artifacts.

### Commit sugerido

```text
feat(artifacts): render human-readable learning progress
```

---

## Paso 29 — Implementar compatibilidad, recalculación y migración de Student Algorithms

- [ ] Paso 29 completado

### Objetivo

Preparar el sistema para que mastery/retention/daily-plan evolucionen sin destruir progreso histórico.

### Problema

Hoy:

```text
mastery-v1
retention-v1
daily-plan-v1
```

Mañana existirán:

```text
mastery-v2
retention-v2
```

No se debe mutar evidence histórico.

### Requisitos

1. Evidence es append-only salvo corrección explícita.
2. Algoritmo actual configurable por versión interna.
3. Recalculation command/service.
4. Registrar algoritmo usado para derived state.
5. Si cambia algoritmo:
   - recalcular concept state;
   - recalcular retention;
   - reprogramar reviews si corresponde;
   - mantener audit.
6. Backup antes de migraciones que cambien estado masivo.
7. Dry-run.
8. Mostrar impacto.

### CLI interna/avanzada

```bash
kelyro maintenance recalculate --dry-run
```

Puede quedar hidden/advanced.

### Tests

- v1 state;
- simulated v2 fake;
- recalculation;
- rollback/backup;
- evidence unchanged.

### Commit sugerido

```text
feat(maintenance): add versioned learning-state recalculation
```

---

## Paso 30 — Hardening de integridad, privacidad y rendimiento del Student Core

- [ ] Paso 30 completado

### Objetivo

Atacar fallos que solo aparecen cuando los subsistemas se conectan.

### Integridad

Revisar:

```text
orphan evidence
orphan mistakes
duplicate active goal
duplicate active session
invalid mastery
dangling curriculum state
review duplicates
timezone inconsistencies
achievement duplicates
```

### Privacy

Comprobar que:

- profile no incluye datos innecesarios;
- export respeta privacy;
- logs no imprimen respuestas sensibles completas;
- diagnostic answers no aparecen en audit técnico sin razón;
- Student Core sigue local-first.

### Rendimiento

Crear fixture grande, por ejemplo:

```text
50 phases
150 modules
500 lessons
2,000 concepts
varios miles de evidence records
```

No porque un curriculum real necesariamente tenga esa forma, sino para evitar algoritmos O(n³) accidentales.

### Targets

No imponer benchmarks arbitrarios irreales.

Sí detectar:

- queries N+1;
- graph traversal repetitivo;
- TUI bloqueada por queries innecesarias;
- indexes faltantes.

### Concurrencia

Aunque la TUI sea single-user:

- evitar doble write simultáneo desde commands/background future;
- manejar SQLite busy/locked correctamente;
- transaction boundaries.

### Tests

- corruption scenarios;
- large graph;
- large evidence set;
- race tests aplicables;
- fuzzing para parsers/validators donde aporte valor.

### Commits

Separar fixes:

```text
fix(mastery): ...
fix(review): ...
perf(progress): ...
```

### Criterios de aceptación

- no findings críticos conocidos;
- tests de regresión añadidos;
- no degradación de I-01.

---

## Paso 31 — Añadir E2E completo del Student & Learning Core

- [ ] Paso 31 completado

### Objetivo

Simular la experiencia real desde workspace Foundation hasta un Student Core con progreso.

### Escenario 1 — Nuevo estudiante

```text
init workspace
launch kelyro
complete onboarding
set goal
select mastery threshold
skip diagnostic
initialize fixture curriculum
view Today
quit
reopen
```

Verificar persistencia.

### Escenario 2 — Diagnóstico

```text
new workspace
complete diagnostic fixture
verify estimated evidence
verify initial concept states
```

### Escenario 3 — Mastery

Inyectar evidence mediante test harness/application service:

```text
record evidence
recalculate
reach threshold
verify dependent unlock
```

### Escenario 4 — Mistake

```text
record repeated mistake
verify occurrences
verify warm-up candidate
```

### Escenario 5 — Retention

Con clock fake:

```text
master concept
advance time
verify review due
complete successful review
verify reschedule
```

### Escenario 6 — Daily plan

```text
due review + weak prerequisite + next lesson
verify ordering
verify time budget
```

### Escenario 7 — History/streak

Simular varios días.

### Escenario 8 — Markdown

Generar artifacts, modificarlos manualmente, validar protection.

### Escenario 9 — Migration from I-01

Abrir workspace Foundation real/fixture y migrar.

### Escenario 10 — Offline

Todo I-02 debe funcionar sin network.

### CI

E2E debe ejecutarse donde sea viable en:

```text
Linux
Windows
macOS
```

### Commit sugerido

```text
test(e2e): cover Student and Learning Core lifecycle
```

---

## Paso 32 — Dogfooding controlado de I-02

- [ ] Paso 32 completado

### Objetivo

Usar Kelyro como usuario real antes de comenzar I-03.

### Importante

Todavía no hay pack real investigado.

Usar curriculum fixture suficientemente grande como para probar UX, no para “aprender Backend Go” en producción.

### Probar manualmente

Durante varias sesiones:

1. crear un workspace limpio;
2. onboarding;
3. abandonar onboarding y retomarlo;
4. cambiar threshold;
5. ejecutar diagnóstico;
6. navegar roadmap;
7. inspeccionar locked reasons;
8. simular/add evidence usando herramientas dev;
9. revisar cambios de mastery;
10. generar mistakes;
11. avanzar clock/test profile para reviews;
12. revisar Today;
13. revisar History;
14. revisar analytics;
15. cerrar/reabrir;
16. modificar Markdown;
17. backup/restore;
18. export/import;
19. modo offline;
20. terminal pequeña;
21. Windows/macOS/Linux cuando estén disponibles.

### Bug workflow

Cada bug:

```text
reproduce
↓
regression test
↓
fix
↓
full relevant suite
↓
Conventional Commit
↓
PATCH release si el bug afecta una release publicada
```

### No avanzar a I-03 si existen

- pérdida de progreso;
- corruption DB;
- mastery inconsistente;
- unlock incorrecto;
- reviews duplicadas;
- onboarding que no puede retomarse;
- crashes frecuentes;
- problemas multiplataforma bloqueantes.

### Commit

No existe un único commit “dogfooding”.

Cada bug tiene su propio fix.

Al final:

```text
docs(roadmap): record I-02 dogfooding results
```

si corresponde.

---

## Paso 33 — Cerrar formalmente I-02 Student & Learning Core

- [ ] Paso 33 completado

### Objetivo

Declarar estable la capa educativa base antes de que I-03 empiece a investigar fuentes reales.

### Verificación completa

```bash
go test ./...
go vet ./...
go test -race ./...
```

Además:

- CI Linux verde;
- CI Windows verde;
- CI macOS verde;
- E2E Foundation verde;
- E2E Student Core verde;
- migration I-01 → I-02;
- backup/restore;
- export/import;
- offline;
- large curriculum fixture;
- large evidence fixture;
- privacy review;
- no secrets;
- no network calls ocultas;
- working tree limpio.

### Revisar arquitectura

Confirmar:

```text
Student Core
    ✗ no importa Bubble Tea
    ✗ no importa SQLite
    ✗ no importa AI providers
    ✗ no hace web research
    ✗ no conoce GitHub
    ✗ no genera curriculum con LLM

Student Core
    ✓ consume curriculum contracts
    ✓ calcula mastery
    ✓ guarda evidence
    ✓ conoce student state
    ✓ programa reviews
    ✓ produce daily plan
```

### Revisar documentación

Actualizar:

```text
README.md
AGENTS.md
docs/architecture/student-learning-domain.md
docs/architecture/curriculum-consumption-contract.md
docs/architecture/mastery-v1.md
docs/architecture/retention-v1.md
docs/architecture/daily-plan-v1.md
docs/implementation/I-02-student-learning-core/PLAN.md
docs/implementation/I-02-student-learning-core/PROGRESS.md
```

### Completion record

Añadir:

```md
## I-02 Student & Learning Core Completion

Status: completed
Release: <versión real>
Completed steps: 0-33

Algorithms:
- mastery-v1
- retention-v1
- review-scheduler-v1
- daily-plan-v1

Known limitations:
- No Research Engine yet.
- No production Learning Packs yet.
- No full Exercise/Assessment Engine yet.
- No AI Runtime yet.

Ready for:
I-03 Research & Source Intelligence
```

### Release

No asumir número.

Usar el historial real de SemVer.

Si I-02 agrega gran cantidad de funcionalidad compatible durante `0.x`, el resultado puede ser una nueva MINOR.

Ejemplo únicamente ilustrativo:

```text
v0.12.0
```

No usar ese número si el repositorio real está en otra secuencia.

### Commit sugerido

```text
docs(roadmap): mark I-02 Student Core complete
```

Crear annotated tag únicamente si corresponde a release real.

---

# Checklist final — I-02 Student & Learning Core

## Ejecución

- [ ] Paso 0 — Apertura formal de I-02
- [ ] Paso 1 — Modelo de dominio educativo
- [ ] Paso 2 — Repositories y application services
- [ ] Paso 3 — SQLite schema y migrations
- [ ] Paso 4 — Student Profile
- [ ] Paso 5 — Learning Goals
- [ ] Paso 6 — Onboarding resumible
- [ ] Paso 7 — Mastery Threshold
- [ ] Paso 8 — Curriculum consumption model
- [ ] Paso 9 — Knowledge Graph y prerequisites
- [ ] Paso 10 — Curriculum Instance
- [ ] Paso 11 — Diagnostic framework
- [ ] Paso 12 — Integración onboarding + diagnóstico
- [ ] Paso 13 — Evidence + Mastery Engine v1
- [ ] Paso 14 — Progression Policy
- [ ] Paso 15 — Mistake Memory
- [ ] Paso 16 — Study Sessions
- [ ] Paso 17 — Study History + Time Tracking
- [ ] Paso 18 — Retention Model v1
- [ ] Paso 19 — Spaced Repetition Scheduler v1
- [ ] Paso 20 — Warm-up Selector
- [ ] Paso 21 — Streaks
- [ ] Paso 22 — Achievements
- [ ] Paso 23 — Learning Analytics v1
- [ ] Paso 24 — Adaptive Daily Plan v1
- [ ] Paso 25 — Progress Dashboard service
- [ ] Paso 26 — TUI Student Core
- [ ] Paso 27 — CLI Student Core
- [ ] Paso 28 — Markdown de progreso
- [ ] Paso 29 — Algorithm migration/recalculation
- [ ] Paso 30 — Integrity/privacy/performance hardening
- [ ] Paso 31 — E2E Student Core
- [ ] Paso 32 — Dogfooding
- [ ] Paso 33 — Cierre formal de I-02

---

# Checklist de capacidades entregadas

## Student

- [ ] Perfil persistente
- [ ] Preferencias de estudio
- [ ] Timezone
- [ ] Disponibilidad diaria
- [ ] Goal persistente
- [ ] Goal lifecycle
- [ ] Onboarding resumible
- [ ] Diagnóstico opcional
- [ ] Mastery threshold configurable

## Curriculum consumption

- [ ] Curriculum determinista
- [ ] Phases
- [ ] Modules
- [ ] Lessons
- [ ] Topics
- [ ] Concepts
- [ ] Stable IDs
- [ ] Knowledge graph
- [ ] Prerequisites
- [ ] Validation
- [ ] Large curriculum support
- [ ] Curriculum Instance
- [ ] Student Concept State

## Mastery

- [ ] Evidence append/persistence
- [ ] Evidence confidence
- [ ] Evidence independence
- [ ] Unknown ≠ 0
- [ ] Mastery Engine v1
- [ ] Mastery explainability
- [ ] Threshold progression
- [ ] Concept state transitions
- [ ] Dependent unlock evaluation

## Learning memory

- [ ] Mistake Memory
- [ ] Mistake dedupe
- [ ] Mistake recurrence
- [ ] Study Sessions
- [ ] Study History
- [ ] Time Tracking

## Retention

- [ ] Retention Model v1
- [ ] Due state
- [ ] Overdue state
- [ ] Spaced repetition schedule
- [ ] Review priority
- [ ] Review time budgeting
- [ ] Warm-up selection

## Motivation / analytics

- [ ] Streaks
- [ ] Longest streak
- [ ] Achievements
- [ ] Milestones
- [ ] Progress analytics
- [ ] Time analytics
- [ ] Mastery analytics
- [ ] Retention analytics
- [ ] Pace analytics

## Daily experience

- [ ] Daily Plan v1
- [ ] Reviews before new content when necessary
- [ ] Blocking weaknesses prioritized
- [ ] Next eligible concept selection
- [ ] Time-budget-aware planning
- [ ] Dashboard
- [ ] Today TUI
- [ ] Progress TUI
- [ ] Roadmap TUI
- [ ] History TUI
- [ ] Student Core CLI
- [ ] Human-readable Markdown progress

---

# Definition of Done — I-02

- [ ] I-01 Foundation sigue sin regresiones críticas
- [ ] Student Core funciona sin Internet
- [ ] Student Core funciona sin proveedor de IA
- [ ] Domain no importa Bubble Tea
- [ ] Domain no importa SQLite
- [ ] Domain no importa GitHub
- [ ] No existe Research Engine implementado prematuramente
- [ ] No existe Curriculum Compiler real implementado prematuramente
- [ ] No existe Exercise Engine completo implementado prematuramente
- [ ] Todos los curriculums usados son fixtures deterministas
- [ ] Onboarding puede interrumpirse y retomarse
- [ ] Goal persiste
- [ ] Diagnostic persiste
- [ ] Threshold persiste
- [ ] Curriculum Instance persiste
- [ ] Mastery es reproducible
- [ ] Mastery es explicable
- [ ] Unknown no se trata como 0
- [ ] Unlock depende de prerequisites + policy
- [ ] Mistakes persisten
- [ ] Sessions persisten
- [ ] Time Tracking es consistente
- [ ] Retention usa clock injectable
- [ ] Reviews no se duplican
- [ ] Streak usa timezone del estudiante
- [ ] Achievements son idempotentes
- [ ] Analytics excluye datos desconocidos correctamente
- [ ] Daily Plan es determinista
- [ ] Daily Plan respeta prerequisitos
- [ ] Daily Plan respeta time budget razonablemente
- [ ] Markdown respeta Artifact Ownership
- [ ] Migrations I-01 → I-02 pasan
- [ ] Recalculation no altera evidence histórico
- [ ] Backups siguen funcionando
- [ ] Export/import sigue funcionando
- [ ] Privacy review pasa
- [ ] Large fixture no degrada inaceptablemente
- [ ] `go test ./...` pasa
- [ ] `go vet ./...` pasa
- [ ] race tests aplicables pasan
- [ ] CI Linux pasa
- [ ] CI Windows pasa
- [ ] CI macOS pasa
- [ ] E2E I-01 pasa
- [ ] E2E I-02 pasa
- [ ] Dogfooding completado
- [ ] No bugs críticos/bloqueantes conocidos
- [ ] Todos los pasos completados están marcados `[x]`
- [ ] PROGRESS.md actualizado por cada paso
- [ ] Cada paso tiene commits Conventional Commit coherentes
- [ ] Working tree limpio
- [ ] Release final respeta SemVer
- [ ] Annotated tag creado si se publicó release
- [ ] PROGRESS.md declara I-02 listo para I-03

---

# Resultado esperado al terminar I-02

Kelyro debe poder hacer este flujo **sin IA y sin Internet**:

```text
$ kelyro

New learner
    ↓
Onboarding
    ↓
Learning Goal
    ↓
Mastery Threshold
    ↓
Optional Diagnostic
    ↓
Deterministic Curriculum Fixture
    ↓
Student Knowledge State
    ↓
Today
```

Después de registrar evidencia de aprendizaje mediante los mecanismos de desarrollo/test de I-02:

```text
Evidence
   ↓
Mastery Engine
   ↓
Concept State
   ↓
Prerequisite Engine
   ↓
Unlock / Reinforcement
   ↓
Retention
   ↓
Reviews
   ↓
Daily Plan
```

Y el estudiante debe poder consultar:

```text
kelyro status
kelyro progress
kelyro roadmap
kelyro today
kelyro reviews
kelyro history
kelyro time
kelyro mistakes
```

mientras Kelyro mantiene:

```text
SQLite             → machine state / source of truth
Markdown           → human-readable views
TUI                → daily experience
CLI                → direct/power-user access
```

La siguiente implementación, **I-03 Research & Source Intelligence**, podrá entonces concentrarse exclusivamente en investigar fuentes actuales y producir evidencia curricular confiable, porque el cerebro que sabe representar al estudiante, medir progreso y consumir un curriculum ya estará funcionando.
