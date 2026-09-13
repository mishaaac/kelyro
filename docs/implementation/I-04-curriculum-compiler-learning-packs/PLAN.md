# Kelyro — Plan de Implementación I-04: Curriculum Compiler & Learning Packs

> Objetivo de esta implementación: construir el compilador curricular de Kelyro y el formato de Learning Packs. Al terminar I-04, Kelyro debe poder tomar Source Bundles verificados provenientes de I-03, descomponer un objetivo profesional/técnico en competencias, conceptos y prerequisitos, expandirlo hasta granularidad suficiente, detectar huecos, validar cobertura, producir un grafo curricular determinista y versionado, empaquetarlo como Learning Pack, instalar/activar versiones de packs y generar una instancia curricular consumible por I-02.
>
> I-04 **NO** implementa todavía el motor completo de lecciones, ejercicios, assessments o proyectos ejecutables. Esa responsabilidad pertenece a I-05 — Lesson, Practice & Assessment Engine. I-04 define **qué debe aprenderse, en qué estructura y con qué evidencia**, no ejecuta todavía el ciclo pedagógico completo.
>
> Este documento está diseñado para Spec-Driven Development con Codex en sesiones independientes. Cada paso debe poder planearse, implementarse, verificarse, registrarse y comitearse sin depender del historial del chat.

---

# Cobertura funcional de I-04

Esta implementación cubre:

```text
#119–123
Learning Packs
Pack Marketplace / Pack Catalog
Pack Versions
Pack Prerequisites
Environment Packs
```

y:

```text
#210–240
Coverage
Gap Scanner
Atomizer
Granularity Guard
No Module Limit
No Lesson Limit
Atomic Concept
Expansion Pass
Prerequisite Expansion
Zero-Assumption Audit
Definition-before-use
Vocabulary Graph
First-principles Coverage
Goal Decomposition
Competency Matrix
Theory Coverage
Practice Coverage
Production Reality
Toolchain Research Integration
Security Coverage
Experimental Flag
Legacy Flag
Best Practice vs Historical
Source Review Contract
Curriculum Reviewer
Beginner Simulation
Expert Coverage Review
Reproducibility
Evidence Report
Copyright-aware Curriculum
```

---

# Frontera de responsabilidad de I-04

I-04 debe construir:

```text
Goal Decomposition
Competency Model
Curriculum Specification
Concept Atomization
Knowledge Graph Compilation
Prerequisite Expansion
Vocabulary Graph
Coverage Analysis
Gap Detection
Zero-Assumption Audit
Definition-before-use Audit
First-principles Expansion
Production Reality Coverage
Security Coverage
Toolchain Coverage
Curriculum Review
Source Bundle Consumption
Curriculum Versioning
Learning Pack Format
Pack Validation
Pack Installation
Pack Dependency Resolution
Environment Packs
Pack Catalog
Compiled Curriculum Artifact
Research/Evidence Report
Student-safe Curriculum Instance Hand-off
```

I-04 no debe construir:

```text
Generated lesson prose final
Interactive lesson renderer completo
Exercise runtime
Assessment runtime
Project runner
AI Tutor runtime
Git project workflow
Plugin runtime
Notifications
Student mastery logic
Research web fetching
```

---

# Principios obligatorios de I-04

1. **El curriculum no se genera desde cero.**
2. **El curriculum se compila desde evidencia verificada.**
3. **Source Bundles de I-03 son input; no hacer research live directamente desde el compiler.**
4. **El objetivo determina el tamaño; no existe límite artificial de módulos o lecciones.**
5. **Granularidad suficiente antes de compactación visual.**
6. **Conceptos tienen IDs estables y versionados.**
7. **Jerarquía visible y grafo de prerequisitos son estructuras distintas.**
8. **Definition-before-use es una regla verificable.**
9. **Zero-assumption es una auditoría, no un slogan.**
10. **Todo conocimiento nuevo debe tener prerequisitos explícitos o quedar marcado como foundational/root.**
11. **Un concepto atómico debe ser evaluable de forma independiente.**
12. **No mezclar una competencia profesional completa con un solo concepto.**
13. **Theory, practice, production y security son coberturas distintas.**
14. **Una ruta profesional debe cubrir herramientas reales, no solo sintaxis.**
15. **No enseñar prácticas obsoletas como recomendación actual.**
16. **Historical/legacy puede conservarse, pero etiquetado.**
17. **Experimental/preview debe estar explícitamente marcado.**
18. **Curriculum Definition ≠ Personalized Learning Path.**
19. **Learning Pack ≠ Plugin.**
20. **Pack version ≠ app version.**
21. **Pack dependency ≠ concept prerequisite.**
22. **Environment Pack describe herramientas/entorno; no debe contener secretos.**
23. **El compiler debe ser determinista con el mismo input/version/config.**
24. **Todo output debe tener reproducibility metadata.**
25. **Los auditorías deben producir razones, no solo pass/fail.**
26. **No usar IA como requisito para compilar.**
27. **Future AI reviewers pueden conectarse por interfaces, pero no escribir estructura sin validación.**
28. **No eliminar progreso del estudiante por una actualización de pack.**
29. **Los cambios curriculares deben clasificarse antes de migrar.**
30. **I-04 entrega a I-05 contratos pedagógicos estructurados, no contenido improvisado.**

---

# Arquitectura conceptual objetivo

```text
Learning Goal
      ↓
Goal Decomposer
      ↓
Competency Matrix
      ↓
Source Bundles (I-03)
      ↓
Concept Extraction
      ↓
Atomizer
      ↓
Prerequisite Expansion
      ↓
Vocabulary Graph
      ↓
Coverage / Gap Scanner
      ↓
Zero-Assumption / Definition-Before-Use Audits
      ↓
Production / Toolchain / Security Coverage
      ↓
Curriculum Reviewer
      ↓
Curriculum Compiler
      ↓
Versioned Learning Pack
      ↓
Compiled Curriculum
      ↓
Curriculum Instance (I-02)
      ↓
I-05 Lesson / Practice / Assessment
```

---

## Paso 0 — Abrir formalmente I-04 y registrar baseline de I-03

- [x] Paso 0 completado

### Objetivo

Abrir I-04 con trazabilidad completa y sin mezclarla con I-03.

### Precondiciones

I-03 debe estar:

```text
implemented
tested
dogfooded
sin bugs críticos conocidos
```

Antes de modificar código:

```bash
git status
git log --oneline --decorate -n 20
go test ./...
go vet ./...
```

Registrar tag/commit real de I-03.

### Crear

```text
docs/
└── implementation/
    └── I-04-curriculum-compiler-learning-packs/
        ├── PLAN.md
        └── PROGRESS.md
```

### `PROGRESS.md`

```md
# I-04 Curriculum Compiler & Learning Packs — Progress Log

## Estado general

Current step: 0
Last completed step: none
Current release: <real current version>
Research baseline: <tag/commit real>

## Registro
```

### Actualizar `AGENTS.md`

Agregar reglas:

1. Leer PLAN/PROGRESS de I-04.
2. No hacer research web desde compiler.
3. Solo consumir Source Bundles/Claims verificados de I-03.
4. No implementar I-05.
5. No modificar mastery del estudiante desde compiler.
6. No imponer límites artificiales de curriculum.
7. Todo compiler pass debe ser determinista y testeable.
8. Todo cambio de pack debe tener clasificación/versionado.
9. No modificar packs publicados in-place.
10. No convertir TUI/CLI en fuente de reglas de dominio.

### Prompt reutilizable

```text
Trabaja únicamente en el Paso XX de
docs/implementation/I-04-curriculum-compiler-learning-packs/PLAN.md.

Antes de modificar código:
1. Lee AGENTS.md.
2. Lee el Paso XX completo.
3. Lee PROGRESS.md.
4. Revisa git status.
5. Revisa commits recientes.
6. Inspecciona solo paquetes necesarios.
7. En Plan Mode, propone el cambio mínimo de este paso.
8. No implementes I-05 ni pasos posteriores.
9. Ejecuta criterios de verificación.
10. Si todo pasa, marca checkbox, actualiza PROGRESS.md y crea commit Conventional Commit.
```

### Esfuerzo recomendado

**High por defecto.**

Usar **xhigh** especialmente en:

```text
Paso 1   Domain model
Paso 4   Learning Pack schema
Paso 7   Goal Decomposition
Paso 9   Competency Matrix
Paso 12  Atomizer
Paso 14  Prerequisite Expansion
Paso 16  Vocabulary Graph
Paso 18  Coverage Engine
Paso 20  Zero-Assumption Audit
Paso 21  Definition-before-use
Paso 29  Curriculum Compiler
Paso 32  Pack versioning
Paso 35  Pack dependency resolution
Paso 40  Curriculum migration intelligence
Paso 44  Final hardening
```

### Commit sugerido

```text
docs(roadmap): open I-04 Curriculum Compiler
```

---

## Paso 1 — Diseñar el modelo de dominio curricular

- [x] Paso 1 completado

### Objetivo

Definir el lenguaje del compiler antes del schema de packs.

### Paquetes sugeridos

```text
internal/
└── curriculum/
    ├── goal/
    ├── competency/
    ├── concept/
    ├── graph/
    ├── compiler/
    ├── coverage/
    ├── audit/
    ├── pack/
    ├── version/
    ├── environment/
    └── catalog/
```

### Entidades/value objects

```text
CurriculumDefinition
CurriculumVersion
CurriculumID

LearningGoalSpec
GoalOutcome
ProfessionalRole

Competency
CompetencyLevel
CompetencyMatrix

Concept
ConceptID
Atomicity
Difficulty
ConceptStatus

Prerequisite
PrerequisiteKind
VocabularyTerm
VocabularyGraph

Phase
Module
LessonSpec
TopicSpec

CoverageRequirement
CoverageResult
Gap

CompilationInput
CompilationConfig
CompilationResult
CompilationPass

LearningPack
PackManifest
PackVersion
PackDependency

EnvironmentPack
ToolRequirement

CurriculumChange
MigrationClass
```

### Estados

```text
current
experimental
preview
legacy
historical
deprecated
```

### Invariantes

- IDs estables;
- no duplicate IDs;
- concept no prereq itself;
- no dangling hierarchy;
- no dangling graph edges;
- status válido;
- source bundle refs obligatorios según policy;
- timestamps UTC;
- pack version válida según Pack Version policy.

### Documentación

```text
docs/architecture/curriculum-domain.md
```

### Tests

Value objects/invariants.

### Commit sugerido

```text
feat(curriculum): define compiler domain model
```

---

## Paso 2 — Definir repositories y application services

- [x] Paso 2 completado

### Objetivo

Separar compiler de SQLite, filesystem, YAML y TUI.

### Interfaces

```text
CurriculumRepository
PackRepository
PackCatalogRepository
EnvironmentPackRepository
CompilationRepository
```

### Services

```text
CurriculumCompilerService
PackService
PackValidationService
PackInstallService
PackUpgradeService
CoverageService
CurriculumAuditService
```

### Inputs externos

Adapters/contracts para:

```text
ResearchBundleProvider
Clock
Filesystem
PackArchiveReader
SignatureVerifier future
```

### Tests

Fakes in-memory.

### Commit sugerido

```text
refactor(curriculum): define compiler service boundaries
```

---

## Paso 3 — Añadir persistence schema y migrations de I-04

- [x] Paso 3 completado

### Objetivo

Persistir packs, compilations y curriculums instalados.

### Tablas sugeridas

```text
learning_packs
learning_pack_versions
pack_dependencies
pack_installations

curriculum_definitions
curriculum_versions
curriculum_nodes
curriculum_edges
curriculum_sources

competencies
competency_concepts

curriculum_compilations
compilation_passes
coverage_results
curriculum_gaps
curriculum_audit_results

environment_packs
environment_tool_requirements
```

No duplicar I-02 Student Concept State.

### Reglas

- migration forward-only;
- pack version immutable;
- installed version separada de available version;
- indexes;
- references a I-03 bundle IDs;
- no secrets.

### Tests

I-03 → I-04 migration.

### Commit sugerido

```text
feat(storage): add curriculum and pack persistence schema
```

---

## Paso 4 — Diseñar Learning Pack Format v1

- [x] Paso 4 completado

### Objetivo

Definir el formato portable/versionado de un Learning Pack.

### Estructura sugerida

```text
my-pack/
├── pack.yaml
├── curriculum/
│   ├── curriculum.yaml
│   ├── competencies.yaml
│   ├── concepts.yaml
│   ├── prerequisites.yaml
│   └── hierarchy.yaml
├── sources/
│   └── evidence-report.json
├── environment/
│   └── environment.yaml
├── README.md
├── LICENSE
└── checksums.txt
```

No es obligatorio que todos los archivos sean YAML; elegir formatos coherentes.

### `pack.yaml`

Campos mínimos:

```text
id
name
description
version
schema_version
domain
target
authors/maintainers
license
created_at
minimum_kelyro_version
dependencies
environment_pack
curriculum_entry
source_evidence_entry
status
```

### Reglas

1. Stable pack ID.
2. Version immutable.
3. Schema version separada de pack version.
4. No scripts ejecutables arbitrarios en v1.
5. No secrets.
6. Relative paths seguros.
7. Checksums.

### Documentar

```text
docs/specs/learning-pack-v1.md
```

### Tests

Manifest parser/validator.

### Commit sugerido

```text
feat(pack): define Learning Pack format v1
```

---

## Paso 5 — Implementar Pack Loader y Validator

- [x] Paso 5 completado

### Objetivo

Cargar packs desde filesystem/archive de forma segura.

### Validaciones

```text
manifest
schema version
IDs
paths
checksums
dependencies
curriculum files
source evidence refs
environment ref
```

### Security

- path traversal;
- symlink escape;
- duplicate entries;
- archive bomb;
- oversized files;
- invalid UTF-8 donde aplique.

### Output

```text
ValidatedPack
Warnings
Errors
```

### CLI

```bash
kelyro packs validate <path>
```

### Tests

Good/bad fixtures.

### Commit sugerido

```text
feat(pack): add secure pack loading and validation
```

---

## Paso 6 — Implementar Source Bundle ingestion desde I-03

- [x] Paso 6 completado

### Objetivo

Convertir Source Bundles verificados en input curricular.

### Reglas

Aceptar:

```text
ready_for_compile
ready_with_caveats
```

Rechazar por defecto:

```text
not_ready
conflicted critical claims
```

### Input representation

```text
CurriculumEvidenceSet
- claims
- version scopes
- source authority
- freshness
- conflicts
- historical/current flags
```

### No network

Compiler no re-fetch.

### Tests

- ready;
- caveat;
- stale;
- conflicted;
- missing bundle.

### Commit sugerido

```text
feat(curriculum): ingest verified research bundles
```

---

## Paso 7 — Implementar Goal Decomposition v1

- [x] Paso 7 completado

### Objetivo

Descomponer un objetivo amplio en outcomes y competency areas.

### Ejemplo

```text
Backend Engineer with Go
```

puede producir áreas como:

```text
computing foundations
programming foundations
Go language
networking
HTTP
databases
testing
security
containers
observability
deployment
architecture
professional tooling
```

La lista real debe provenir de evidencia/pack specification, no hardcodeada en core.

### Inputs

```text
LearningGoalSpec
target role/outcome
evidence set
domain profile
```

### Output

```text
GoalDecomposition
- outcomes
- competency areas
- scope
- exclusions
```

### Version

```text
goal-decomposer-v1
```

### Tests

- narrow goal;
- professional role;
- no evidence;
- unsupported scope.

### Commit sugerido

```text
feat(curriculum): add goal decomposition v1
```

---

## Paso 8 — Implementar Professional Outcome Model

- [x] Paso 8 completado

### Objetivo

Evitar curriculums que enseñen sintaxis pero no preparación profesional.

### Outcome categories

```text
knowledge
application
debugging
design
tool usage
production
security
communication/documentation
maintenance
```

### Reglas

Un goal profesional debe declarar:

```text
what learner can explain
what learner can build
what learner can debug
what learner can operate
what learner can maintain
```

### Tests

Completeness validation.

### Commit sugerido

```text
feat(curriculum): model professional learning outcomes
```

---

## Paso 9 — Implementar Competency Matrix v1

- [x] Paso 9 completado

### Objetivo

Representar competencias requeridas y profundidad esperada.

### Matrix

```text
Competency
- ID
- area
- outcome
- expected level
- evidence refs
- concept refs future
```

### Levels

No usar necesariamente beginner/intermediate/advanced.

Ejemplo:

```text
awareness
understand
apply
analyze
design
operate
teach/explain
```

Puede haber múltiples dimensions.

### Reglas

- goal coverage;
- no competency without evidence;
- stable IDs;
- hierarchy opcional.

### Version

```text
competency-matrix-v1
```

### Tests

- completeness;
- duplicate;
- unsupported competency.

### Commit sugerido

```text
feat(curriculum): add competency matrix v1
```

---

## Paso 10 — Implementar Claim-to-Concept candidate extraction

- [x] Paso 10 completado

### Objetivo

Derivar candidatos de conceptos desde claims estructurados.

### Importante

No significa que cada claim sea un concept.

Pipeline:

```text
claims
→ group by semantic subject
→ concept candidates
→ atomizer
```

### Candidate metadata

```text
candidate_id
title
scope
claim refs
version scope
status
```

### Tests

- duplicate claims;
- definition + behavior;
- historical claim;
- version-specific.

### Commit sugerido

```text
feat(curriculum): derive concept candidates from evidence
```

---

## Paso 11 — Definir Atomic Concept Criteria

- [x] Paso 11 completado

### Objetivo

Formalizar cuándo una unidad de conocimiento es suficientemente atómica.

### Criterios

Un concept debería idealmente poder:

```text
be named
be defined
have prerequisites
be explained
be practiced
be assessed
have evidence
```

### Anti-patterns

No aceptar como concept único:

```text
"Go Programming"
"Backend Development"
"Databases"
```

si contienen múltiples ideas evaluables.

### AtomicityResult

```text
atomic
needs_split
too_fragmented
unknown
```

### Documentar

```text
docs/architecture/atomic-concept-criteria-v1.md
```

### Commit sugerido

```text
feat(curriculum): define atomic concept criteria v1
```

---

## Paso 12 — Implementar Atomizer v1

- [x] Paso 12 completado

### Objetivo

Dividir candidatos demasiado amplios en conceptos atómicos.

### Ejemplo

```text
Variables
```

puede expandirse a:

```text
storage/value concept
identifier
declaration
explicit type
initialization
type inference
short declaration
zero value
assignment
scope introduction
```

No hardcodear ese ejemplo.

### Inputs

```text
candidate
claims
atomicity policy
domain hints
```

### Output

```text
AtomicConceptSet
split reasons
claim mapping
```

### Determinismo

Same input → same IDs/output order.

### Version

```text
atomizer-v1
```

### Tests

- already atomic;
- broad concept;
- overlapping split;
- no evidence to split.

### Commit sugerido

```text
feat(curriculum): add concept atomizer v1
```

---

## Paso 13 — Implementar Granularity Guard

- [x] Paso 13 completado

### Objetivo

Evitar que el compiler compacte excesivamente por estética.

### Rules

1. No max modules.
2. No max lessons.
3. No max concepts.
4. No “roadmap must fit one screen”.
5. Merge solo si atomicity/evaluability se preserva.
6. Tiny concepts pueden agruparse visualmente en LessonSpec sin perder IDs.

### Result

```text
granularity warnings
forced split
safe visual grouping
```

### Tests

Large fixture.

### Commit sugerido

```text
feat(curriculum): enforce curriculum granularity guard
```

---

## Paso 14 — Implementar Prerequisite Extraction

- [x] Paso 14 completado

### Objetivo

Derivar prerequisitos directos desde evidence y concept semantics.

### Types

```text
hard
recommended
exposure_only
tool_dependency
vocabulary
```

### No usar ordering visual como prereq implícito.

### Tests

Chains/diamonds.

### Commit sugerido

```text
feat(curriculum): derive concept prerequisites
```

---

## Paso 15 — Implementar Prerequisite Expansion Pass v1

- [x] Paso 15 completado

### Objetivo

Detectar prerequisitos implícitos ausentes y expandir el curriculum.

### Example conceptual

Si se requiere:

```text
HTTP handlers
```

pero faltan:

```text
functions
interfaces
network basics
request/response model
```

el compiler debe reportar/insertar conceptos fundacionales según evidence disponible.

### Output

```text
added prerequisite concepts
unresolved prerequisite gaps
reasons
```

### Version

```text
prerequisite-expansion-v1
```

### Tests

- missing root;
- recursive expansion;
- cycle prevention.

### Commit sugerido

```text
feat(curriculum): add prerequisite expansion pass v1
```

---

## Paso 16 — Implementar Knowledge Graph Compiler

- [x] Paso 16 completado

### Objetivo

Compilar conceptos/prereqs a grafo validado consumible por I-02.

### Operations

- cycle detection;
- topological order;
- unreachable nodes;
- disconnected components;
- root concepts;
- critical path metadata.

### Output

Compatible con I-02 Curriculum Consumption Contract.

### Tests

Large graph.

### Commit sugerido

```text
feat(curriculum): compile validated prerequisite graph
```

---

## Paso 17 — Implementar Vocabulary Graph

- [x] Paso 17 completado

### Objetivo

Rastrear términos que deben definirse antes de usarse.

### VocabularyTerm

```text
term
canonical concept
introduced_by
used_by
aliases
scope
```

### Reglas

- alias resolution;
- acronym expansion;
- term cannot be “known” without introduction/prereq unless foundational assumption explicitly allowed.

### Tests

- acronym;
- alias;
- first use.

### Commit sugerido

```text
feat(curriculum): add vocabulary dependency graph
```

---

## Paso 18 — Implementar Definition-before-use Audit

- [x] Paso 18 completado

### Objetivo

Detectar lecciones/conceptos que usan vocabulario aún no introducido.

### Output

```text
violation
term
used_at
expected introduction
suggested prerequisite
severity
```

### Exceptions

Permitir términos comunes definidos por domain baseline explícito.

No una lista mágica global.

### Tests

- valid;
- invalid;
- alias;
- same lesson order.

### Commit sugerido

```text
feat(audit): enforce definition-before-use
```

---

## Paso 19 — Implementar Coverage Engine v1

- [x] Paso 19 completado

### Objetivo

Medir cobertura contra goal + competency matrix.

### Dimensions

```text
competency coverage
concept coverage
evidence coverage
theory coverage
practice contract coverage
production coverage
security coverage
toolchain coverage
```

### Importante

No reducir a porcentaje único.

### Output

```text
CoverageReport
per dimension
missing
partial
covered
```

### Version

```text
coverage-v1
```

### Commit sugerido

```text
feat(coverage): add multidimensional curriculum coverage v1
```

---

## Paso 20 — Implementar Gap Scanner

- [x] Paso 20 completado

### Objetivo

Convertir coverage incompleta en gaps accionables.

### Gap types

```text
missing competency
missing concept
missing prerequisite
missing evidence
missing theory
missing practice contract
missing production reality
missing toolchain
missing security
missing current guidance
```

### Severity

```text
blocking
important
recommended
informational
```

### Tests

Fixtures.

### Commit sugerido

```text
feat(coverage): add curriculum gap scanner
```

---

## Paso 21 — Implementar Zero-Assumption Audit v1

- [x] Paso 21 completado

### Objetivo

Verificar que una ruta marcada como “from zero” realmente empiece desde primeros principios.

### Profile

```text
zero
some experience
domain experienced
```

La auditoría se ejecuta contra baseline declarado.

### Para `zero`

Detectar supuestos como:

```text
terminal
files/directories
process
program
compiler/interpreter
network basics
editor
version control
```

cuando sean relevantes al goal.

No hardcodear todos como universales; vienen de competency/evidence/domain profile.

### Version

```text
zero-assumption-v1
```

### Documentar

```text
docs/architecture/zero-assumption-audit-v1.md
```

### Commit sugerido

```text
feat(audit): add zero-assumption curriculum audit
```

---

## Paso 22 — Implementar First-Principles Expansion

- [x] Paso 22 completado

### Objetivo

Añadir conceptos base cuando zero-assumption detecta huecos.

### Rule

No añadir automáticamente sin evidence.

Si evidence insuficiente:

```text
Gap: research required
```

para futura recompilación con I-03.

### Output

```text
expanded root concepts
unresolved research needs
```

### Commit sugerido

```text
feat(curriculum): expand first-principles foundations
```

---

## Paso 23 — Implementar Theory Coverage

- [ ] Paso 23 completado

### Objetivo

Garantizar que cada competencia importante tenga suficiente conocimiento conceptual.

### Theory contract

Concept puede declarar:

```text
definition required
mental model required
mechanism required
tradeoffs required
failure modes required
```

I-05 generará/mostrará contenido.

### Tests

Missing definition/mental model.

### Commit sugerido

```text
feat(coverage): validate theory coverage
```

---

## Paso 24 — Implementar Practice Coverage Contract

- [ ] Paso 24 completado

### Objetivo

Definir qué debe poder practicar/evaluar I-05.

### PracticeExpectation

```text
recall
recognize
apply
debug
design
build
compare
explain
```

### Reglas

Todo concept importante debe tener al menos una práctica compatible con expected competency.

No generar ejercicios todavía.

### Commit sugerido

```text
feat(curriculum): define practice expectations for I-05
```

---

## Paso 25 — Implementar Production Reality Coverage

- [ ] Paso 25 completado

### Objetivo

Evitar curriculums que solo cubren tutorial-level knowledge.

### Categories

```text
failure modes
performance
observability
deployment
configuration
maintenance
debugging
reliability
tradeoffs
operational concerns
```

dependiendo del domain.

### Evidence required

Production claims deben tener Source Bundles adecuados.

### Commit sugerido

```text
feat(coverage): add production reality coverage
```

---

## Paso 26 — Implementar Toolchain Coverage

- [ ] Paso 26 completado

### Objetivo

Integrar herramientas reales necesarias para lograr el objetivo.

### ToolRequirement

```text
tool id
purpose
when introduced
required/recommended/optional
platform notes
evidence refs
environment pack ref
```

### Ejemplos futuros

```text
Git
compiler/runtime
editor
container runtime
database
debugger
formatter
linter
```

No hardcodear en core.

### Commit sugerido

```text
feat(coverage): add professional toolchain coverage
```

---

## Paso 27 — Implementar Security Coverage

- [ ] Paso 27 completado

### Objetivo

Hacer security parte del curriculum, no apéndice opcional.

### Categories

Según domain:

```text
input validation
authn/authz
secrets
dependency security
data protection
secure defaults
threat awareness
supply chain
```

### Security-sensitive evidence

Debe requerir I-03 verification apropiada.

### Commit sugerido

```text
feat(coverage): add curriculum security coverage
```

---

## Paso 28 — Implementar Current / Experimental / Legacy classification

- [ ] Paso 28 completado

### Objetivo

Transferir inteligencia temporal de I-03 al curriculum.

### Concept/lesson status

```text
current
preview
experimental
legacy
historical
deprecated
```

### Rules

- current default recommendation;
- experimental separated;
- historical cannot silently appear as current best practice;
- legacy may be taught for maintenance context.

### UI metadata ready.

### Commit sugerido

```text
feat(curriculum): classify current experimental and legacy guidance
```

---

## Paso 29 — Implementar Best Practice vs Historical Guidance

- [ ] Paso 29 completado

### Objetivo

Permitir enseñar “cómo se hacía” sin confundirlo con “cómo conviene hacerlo hoy”.

### GuidanceType

```text
recommended_current
acceptable_current
legacy_maintenance
historical_context
avoid
experimental
```

### Evidence mapping

Cada classification debe tener claim refs.

### Commit sugerido

```text
feat(curriculum): distinguish current and historical guidance
```

---

## Paso 30 — Implementar Curriculum Hierarchy Builder

- [ ] Paso 30 completado

### Objetivo

Construir Phase → Module → Lesson → Topic a partir del graph sin perder granularidad.

### Importante

Hierarchy es UX.

Prerequisite graph sigue siendo verdad pedagógica.

### Heuristics

Agrupar por:

```text
competency area
concept cohesion
prerequisite locality
difficulty
practice context
```

### No limits artificiales.

### Tests

- 2,000 concepts;
- stable deterministic grouping;
- graph edges cross modules.

### Commit sugerido

```text
feat(curriculum): build deterministic learning hierarchy
```

---

## Paso 31 — Implementar Curriculum Compiler Pipeline v1

- [ ] Paso 31 completado

### Objetivo

Orquestar todos los passes.

### Pipeline

```text
validate input
→ goal decomposition
→ competency matrix
→ concept candidates
→ atomize
→ prereq extraction
→ prereq expansion
→ vocabulary graph
→ graph compile
→ hierarchy build
→ coverage
→ gaps
→ audits
→ temporal classification
→ final review
→ compiled artifact
```

### CompilationPass

Cada pass registra:

```text
name
version
input hash
output hash
warnings
errors
duration
```

### Determinismo

Same input/config → same output hash.

### Version

```text
curriculum-compiler-v1
```

### Documentar

```text
docs/architecture/curriculum-compiler-v1.md
```

### Commit sugerido

```text
feat(compiler): add curriculum compiler pipeline v1
```

---

## Paso 32 — Implementar Curriculum Reviewer v1

- [ ] Paso 32 completado

### Objetivo

Realizar revisión determinista antes de publicar un pack.

### Review dimensions

```text
coverage
granularity
prerequisites
definition-before-use
zero-assumption
source readiness
freshness
security
production
toolchain
temporal status
```

### Result

```text
approved
approved_with_warnings
rejected
```

### Future AI reviewer

Definir interface opcional:

```text
CurriculumReviewAdvisor
```

No requerir IA.

### Commit sugerido

```text
feat(review): add curriculum reviewer v1
```

---

## Paso 33 — Implementar Beginner Simulation

- [ ] Paso 33 completado

### Objetivo

Simular recorrido de alguien sin conocimiento previo y detectar saltos.

### Simulation

Walk topological order and track:

```text
introduced concepts
vocabulary
tools
assumptions
```

Cuando aparece una necesidad no conocida:

```text
BeginnerGap
```

### No LLM obligatorio.

### Tests

Known broken curriculum fixtures.

### Commit sugerido

```text
feat(review): add deterministic beginner simulation
```

---

## Paso 34 — Implementar Expert Coverage Review

- [ ] Paso 34 completado

### Objetivo

Detectar que una ruta llegue al nivel profesional esperado, no solo a fundamentos.

### Inputs

```text
professional outcomes
competency matrix
production/security/tool coverage
```

### Output

```text
missing advanced competency
insufficient depth
missing production capability
```

### Future expert AI adapter contract

Opcional, no required.

### Commit sugerido

```text
feat(review): add expert coverage review
```

---

## Paso 35 — Implementar Pack Versioning Policy v1

- [ ] Paso 35 completado

### Objetivo

Versionar packs independientemente de Kelyro.

### SemVer-like policy

Ejemplo:

```text
PATCH
- source refresh
- typo/metadata
- non-structural clarification

MINOR
- new concepts
- new lessons
- expanded coverage compatible

MAJOR
- breaking concept IDs
- incompatible curriculum contract
- restructuring requiring migration
```

Durante `0.x`, documentar prerelease semantics.

### No mover tags/version publicados.

### Documentar

```text
docs/specs/pack-versioning-v1.md
```

### Commit sugerido

```text
feat(pack): add Learning Pack version policy
```

---

## Paso 36 — Implementar Pack Dependency Resolver

- [ ] Paso 36 completado

### Objetivo

Resolver dependencias entre packs.

### Ejemplo

```text
backend-go
depends on
computing-foundations >=x
```

si diseño real lo requiere.

### Importante

Pack dependency ≠ concept prerequisite.

### Requirements

- version constraints;
- cycles;
- missing;
- incompatible;
- optional dependency future.

### Tests

Graph.

### Commit sugerido

```text
feat(pack): resolve Learning Pack dependencies
```

---

## Paso 37 — Implementar Environment Pack Format v1

- [ ] Paso 37 completado

### Objetivo

Separar requirements de herramientas/entorno del curriculum conceptual.

### `environment.yaml`

```text
id
version
platform support
tools
minimum versions
required/recommended/optional
install guidance refs
when_needed
```

### Reglas

- no auto-install arbitrario;
- no secrets;
- platform-specific guidance;
- integrates with `kelyro doctor`.

### Documentar

```text
docs/specs/environment-pack-v1.md
```

### Commit sugerido

```text
feat(environment): define Environment Pack format v1
```

---

## Paso 38 — Integrar Environment Packs con Doctor

- [ ] Paso 38 completado

### Objetivo

Hacer doctor context-aware según curriculum/phase.

### Ejemplo

Al inicio:

```text
Git — recommended
Go — required
Docker — not needed yet
PostgreSQL — future
```

Más adelante:

```text
Docker — required for current module
```

### Reglas

- no exigir herramientas antes de enseñarlas;
- explain why;
- official install source metadata.

### Commit sugerido

```text
feat(doctor): make diagnostics curriculum-aware
```

---

## Paso 39 — Implementar Pack Installation y Activation

- [ ] Paso 39 completado

### Objetivo

Instalar un Learning Pack validado y activarlo para un workspace.

### Global storage

Usar Foundation config/data dirs.

No copiar pack completo dentro de cada workspace si puede referenciarse por ID/version de forma segura.

### Commands

```bash
kelyro packs install <path>
kelyro packs list
kelyro packs show <id>
kelyro packs activate <id>@<version>
```

### Reglas

- validate first;
- immutable installed version;
- checksums;
- dependency resolution;
- no scripts.

### Commit sugerido

```text
feat(pack): install and activate Learning Packs
```

---

## Paso 40 — Implementar Pack Catalog / Marketplace v1

- [ ] Paso 40 completado

### Objetivo

Permitir descubrir packs sin mezclarlo con Plugin Marketplace.

### I-04 scope

Implementar:

```text
catalog model
catalog source abstraction
search/list metadata
version availability
trust/status metadata
```

No necesita ecosistema completo de publicación comunitaria todavía.

### Commands

```bash
kelyro packs search <query>
kelyro packs catalog
```

### Catalog entry

```text
pack id
name
description
versions
maintainer
status
compatibility
source
```

### Offline

Catalog cache.

### Security

No instalar automáticamente.

### Commit sugerido

```text
feat(pack): add Learning Pack catalog v1
```

---

## Paso 41 — Implementar Curriculum Change Classification

- [ ] Paso 41 completado

### Objetivo

Clasificar diferencias entre pack versions.

### Change types

```text
metadata_only
source_refresh
concept_added
concept_removed
concept_split
concept_merged
prerequisite_changed
hierarchy_changed
status_changed
environment_changed
```

### Severity/migration class

```text
safe
requires_recompile
requires_student_review
breaking
```

### Inputs

I-03 Drift/Impact + old/new curriculum.

### Commit sugerido

```text
feat(curriculum): classify curriculum version changes
```

---

## Paso 42 — Implementar Student-safe Curriculum Migration Plan

- [ ] Paso 42 completado

### Objetivo

Preparar actualización de pack sin destruir Student State de I-02.

### Rules

- unchanged stable concept ID → preserve mastery/evidence;
- added concept → new unknown state;
- split concept → migration mapping required;
- merged concept → do not invent mastery;
- removed/deprecated → preserve historical evidence;
- changed prerequisite → recalc unlock eligibility;
- hierarchy movement → no mastery loss.

### Output

```text
CurriculumMigrationPlan
```

I-02 aplica mediante application service, no direct SQL.

### Dry-run

```bash
kelyro packs upgrade <id> --dry-run
```

### Commit sugerido

```text
feat(pack): plan student-safe curriculum migrations
```

---

## Paso 43 — Implementar Pack Upgrade

- [ ] Paso 43 completado

### Objetivo

Aplicar una versión nueva con backup y migration plan.

### Flow

```text
discover version
→ validate
→ diff
→ migration plan
→ backup
→ confirmation
→ install
→ migrate curriculum instance
→ integrity check
```

### Requirements

- rollback/recovery;
- never mutate old pack;
- audit;
- no silent breaking migration.

### Tests

- patch;
- added concept;
- split;
- failure rollback.

### Commit sugerido

```text
feat(pack): add safe Learning Pack upgrades
```

---

## Paso 44 — Implementar Reproducibility Metadata

- [ ] Paso 44 completado

### Objetivo

Reproducir cómo se compiló un curriculum.

### Store

```text
compiler version
all pass versions
source bundle IDs/hashes
compilation config
pack schema version
input hash
output hash
timestamp
```

### Command

```bash
kelyro curriculum build-info
```

### Commit sugerido

```text
feat(compiler): record reproducible curriculum builds
```

---

## Paso 45 — Implementar Curriculum Evidence Report

- [ ] Paso 45 completado

### Objetivo

Producir reporte humano/machine-readable de la evidencia del pack.

### Report

```text
goal
competencies
concept count
source bundle count
primary source coverage
freshness
conflicts/caveats
historical content
experimental content
gaps
compiler versions
```

### Artifact

```text
EVIDENCE.md
```

dentro del pack build/export, más JSON interno.

### Copyright

Solo excerpts mínimos/citations.

### Commit sugerido

```text
feat(curriculum): generate curriculum evidence report
```

---

## Paso 46 — Implementar Copyright-aware Pack Builder

- [ ] Paso 46 completado

### Objetivo

Construir packs distribuibles sin copiar material externo de forma indebida.

### Rules

- include original Kelyro-authored structured metadata;
- citations/URLs;
- minimal bounded excerpts;
- source hashes;
- licenses for included original/code assets;
- no cached web bodies;
- no full article transcripts.

### Checks

Pack validation debe rechazar known forbidden retention patterns si detectable.

### Commit sugerido

```text
fix(pack): enforce copyright-aware pack builds
```

---

## Paso 47 — Implementar CLI `kelyro curriculum` y completar `kelyro packs`

- [ ] Paso 47 completado

### Objetivo

Hacer compiler y pack lifecycle inspeccionables.

### Commands

```bash
kelyro curriculum compile
kelyro curriculum validate
kelyro curriculum coverage
kelyro curriculum gaps
kelyro curriculum audit
kelyro curriculum build-info

kelyro packs list
kelyro packs show
kelyro packs validate
kelyro packs install
kelyro packs activate
kelyro packs search
kelyro packs upgrade --dry-run
```

### Human-readable

Default.

### Machine-readable

`--json` si convención existe.

### Commit sugerido

```text
feat(cli): expose Curriculum Compiler and pack management
```

---

## Paso 48 — Integrar Curriculum/Pack views en TUI

- [ ] Paso 48 completado

### Objetivo

Mostrar estructura real compilada.

### Views

```text
Pack
Curriculum Overview
Coverage
Gaps
Audit
Sources/Evidence link
Update available
Migration preview
```

### Roadmap

Ahora el placeholder de I-01 se reemplaza por curriculum real.

Mostrar:

```text
Phase
Module
Lesson
Topic
Concept progress from I-02
```

### Important

TUI reads application services.

No compile en cada render.

### Commit sugerido

```text
feat(tui): integrate compiled curriculum and pack views
```

---

## Paso 49 — Crear primer Reference Learning Pack de desarrollo

- [ ] Paso 49 completado

### Objetivo

Construir un pack de referencia suficientemente real para probar el compiler.

### Nombre sugerido

```text
backend-go-reference
```

pero no declararlo todavía “production complete” si I-03 research coverage no lo soporta totalmente.

### Scope

Debe demostrar:

- goal decomposition;
- competency matrix;
- hundreds of concepts si evidence lo justifica;
- prerequisites;
- zero-assumption foundations;
- production/security/toolchain;
- current/legacy classification;
- evidence report;
- environment pack.

### Importante

No imponer cantidad.

Si el compiler produce 350 conceptos, aceptar.

Si produce 900 con evidencia y atomicidad válida, aceptar.

### Commit sugerido

```text
feat(pack): add reference Backend Go Learning Pack
```

---

## Paso 50 — E2E Curriculum Compiler

- [ ] Paso 50 completado

### Objetivo

Probar pipeline completo desde Source Bundles hasta Curriculum Instance.

### E2E

```text
I-03 source bundles fixture
→ compiler
→ pack build
→ pack validate
→ install
→ activate
→ I-02 curriculum instance
→ roadmap
```

### Scenarios

1. Fresh valid bundle.
2. Missing evidence.
3. Conflict blocks compile.
4. Historical source.
5. Experimental concept.
6. Missing prerequisite expanded.
7. Definition-before-use failure.
8. Zero-assumption failure.
9. Production gap.
10. Security gap.
11. Pack dependency.
12. Environment pack.
13. Upgrade adds concept.
14. Upgrade splits concept.
15. Migration preserves mastery.
16. Copyright-safe export.

### CI

Linux/Windows/macOS.

### Commit sugerido

```text
test(e2e): cover curriculum compilation and pack lifecycle
```

---

## Paso 51 — Performance y scale hardening

- [ ] Paso 51 completado

### Objetivo

Asegurar que gran curriculums no rompan compiler/TUI.

### Fixture

```text
10,000 concepts
20,000 prerequisite edges
1,000 competencies
many source bundles
```

No porque un pack real deba ser así, sino para detectar O(n³).

### Measure

- graph compile;
- audits;
- coverage;
- serialization;
- pack install;
- roadmap read.

### Requirements

- bounded memory;
- deterministic outputs;
- no recursion stack issues;
- indexes.

### Commit sugerido

```text
perf(curriculum): harden compiler for large learning graphs
```

---

## Paso 52 — Security hardening de packs/compiler

- [ ] Paso 52 completado

### Objetivo

Tratar packs externos como input no confiable.

### Review

```text
archive traversal
symlink escape
malicious YAML/JSON
oversized manifests
dependency cycles
checksum spoofing
path injection
HTML/Markdown unsafe content
unexpected executable files
resource exhaustion
```

### Future signature hook

Definir interface para signatures sin requerir marketplace signing completo.

### Commit sugerido

```text
fix(security): harden Learning Pack ingestion
```

---

## Paso 53 — Dogfooding de I-04

- [ ] Paso 53 completado

### Objetivo

Usar el compiler y reference pack como usuario/reviewer real antes de I-05.

### Revisar manualmente

1. goal decomposition;
2. competency matrix;
3. concept atomicity;
4. prerequisites;
5. hierarchy;
6. roadmap;
7. zero-assumption;
8. definition-before-use;
9. vocabulary;
10. production coverage;
11. toolchain;
12. security;
13. current/experimental/legacy;
14. source evidence;
15. pack installation;
16. activation;
17. upgrade dry-run;
18. environment doctor integration;
19. performance;
20. offline behavior.

### Human review critical

Leer muestras reales de:

```text
first 20 concepts
random 20 concepts
advanced module
security module
production module
```

y validar contra evidence.

### Bugs

```text
reproduce
→ regression test
→ fix
→ targeted suite
→ full suite
→ commit
```

### No avanzar a I-05 si existen

- missing foundational concepts;
- invalid prerequisite cycles;
- concepts without evidence;
- historical guidance marked current;
- student mastery lost during pack upgrade;
- compiler nondeterminism;
- pack validation bypass;
- serious security/copyright issues.

### Commit final

```text
docs(roadmap): record I-04 dogfooding results
```

---

## Paso 54 — Cierre formal de I-04

- [ ] Paso 54 completado

### Objetivo

Declarar lista la capa curricular antes de implementar lessons/practice/assessment.

### Gates

```bash
go test ./...
go vet ./...
go test -race ./...
git diff --check
```

Además:

- CI Linux;
- CI Windows;
- CI macOS;
- E2E I-01;
- E2E I-02;
- E2E I-03;
- E2E I-04;
- large curriculum fixture;
- pack security;
- migration safety;
- offline;
- copyright pack report;
- working tree limpio.

### Arquitectura

Confirmar:

```text
Curriculum Domain
    ✗ no importa Bubble Tea
    ✗ no importa SQLite
    ✗ no hace HTTP
    ✗ no modifica mastery directamente
    ✗ no genera ejercicios finales

Compiler
    ✓ consume Source Bundles
    ✓ descompone goals
    ✓ genera competency matrix
    ✓ atomiza concepts
    ✓ expande prerequisitos
    ✓ compila graph
    ✓ audita coverage
    ✓ audita zero-assumption
    ✓ audita definition-before-use
    ✓ cubre production/toolchain/security
    ✓ versiona packs
    ✓ produce reproducibility/evidence report
```

### Documentación obligatoria

Actualizar:

```text
README.md
AGENTS.md

docs/architecture/curriculum-domain.md
docs/architecture/atomic-concept-criteria-v1.md
docs/architecture/zero-assumption-audit-v1.md
docs/architecture/curriculum-compiler-v1.md

docs/specs/learning-pack-v1.md
docs/specs/pack-versioning-v1.md
docs/specs/environment-pack-v1.md

docs/implementation/I-04-curriculum-compiler-learning-packs/PLAN.md
docs/implementation/I-04-curriculum-compiler-learning-packs/PROGRESS.md
```

### Completion record

```md
## I-04 Curriculum Compiler & Learning Packs Completion

Status: completed
Release: <real version>
Completed steps: 0-54

Algorithms/contracts:
- goal-decomposer-v1
- competency-matrix-v1
- atomizer-v1
- prerequisite-expansion-v1
- coverage-v1
- zero-assumption-v1
- curriculum-compiler-v1
- Learning Pack v1
- Environment Pack v1

Known limitations:
- Lesson content runtime not implemented.
- Practice/Assessment runtime not implemented.
- Projects not implemented.
- AI Tutor not required.

Ready for:
I-05 Lesson, Practice & Assessment Engine
```

### Commit sugerido

```text
docs(roadmap): mark I-04 Curriculum Compiler complete
```

### Release

Usar SemVer real.

No asumir número.

---

# Checklist final — I-04 Curriculum Compiler & Learning Packs

## Ejecución

- [x] Paso 0 — Apertura formal
- [x] Paso 1 — Curriculum domain
- [x] Paso 2 — Service boundaries
- [x] Paso 3 — Persistence
- [ ] Paso 4 — Learning Pack v1
- [ ] Paso 5 — Pack Loader/Validator
- [ ] Paso 6 — I-03 Source Bundle ingestion
- [ ] Paso 7 — Goal Decomposition
- [ ] Paso 8 — Professional Outcomes
- [ ] Paso 9 — Competency Matrix
- [x] Paso 10 — Concept candidates
- [x] Paso 11 — Atomic Concept Criteria
- [x] Paso 12 — Atomizer
- [x] Paso 13 — Granularity Guard
- [x] Paso 14 — Prerequisite extraction
- [x] Paso 15 — Prerequisite expansion
- [x] Paso 16 — Knowledge Graph Compiler
- [x] Paso 17 — Vocabulary Graph
- [x] Paso 18 — Definition-before-use
- [x] Paso 19 — Coverage Engine
- [x] Paso 20 — Gap Scanner
- [ ] Paso 21 — Zero-Assumption Audit
- [ ] Paso 22 — First-Principles Expansion
- [ ] Paso 23 — Theory Coverage
- [ ] Paso 24 — Practice Coverage Contract
- [ ] Paso 25 — Production Reality
- [ ] Paso 26 — Toolchain Coverage
- [ ] Paso 27 — Security Coverage
- [ ] Paso 28 — Current/Experimental/Legacy
- [ ] Paso 29 — Best Practice vs Historical
- [ ] Paso 30 — Hierarchy Builder
- [ ] Paso 31 — Compiler Pipeline
- [ ] Paso 32 — Curriculum Reviewer
- [ ] Paso 33 — Beginner Simulation
- [ ] Paso 34 — Expert Coverage
- [ ] Paso 35 — Pack Versioning
- [ ] Paso 36 — Pack Dependencies
- [ ] Paso 37 — Environment Pack
- [ ] Paso 38 — Doctor integration
- [ ] Paso 39 — Pack Installation
- [ ] Paso 40 — Pack Catalog
- [ ] Paso 41 — Change Classification
- [ ] Paso 42 — Student-safe Migration Plan
- [ ] Paso 43 — Pack Upgrade
- [ ] Paso 44 — Reproducibility
- [ ] Paso 45 — Evidence Report
- [ ] Paso 46 — Copyright-aware Pack Builder
- [ ] Paso 47 — CLI
- [ ] Paso 48 — TUI
- [ ] Paso 49 — Reference Pack
- [ ] Paso 50 — E2E
- [ ] Paso 51 — Performance
- [ ] Paso 52 — Security
- [ ] Paso 53 — Dogfooding
- [ ] Paso 54 — Cierre formal

---

# Checklist de capacidades entregadas

## Compiler

- [ ] Goal Decomposition
- [ ] Professional Outcomes
- [ ] Competency Matrix
- [ ] Concept Candidate Extraction
- [ ] Atomizer
- [ ] Granularity Guard
- [ ] Prerequisite Extraction
- [ ] Prerequisite Expansion
- [ ] Knowledge Graph
- [ ] Vocabulary Graph
- [ ] Hierarchy Builder
- [ ] Deterministic Compiler Pipeline

## Audits

- [ ] Definition-before-use
- [ ] Coverage Engine
- [ ] Gap Scanner
- [ ] Zero-Assumption
- [ ] Beginner Simulation
- [ ] Expert Coverage
- [ ] Curriculum Reviewer

## Professional depth

- [ ] Theory Coverage
- [ ] Practice Expectations
- [ ] Production Reality
- [ ] Toolchain Coverage
- [ ] Security Coverage
- [ ] Current Guidance
- [ ] Experimental flags
- [ ] Legacy/Historical flags

## Learning Packs

- [ ] Pack Manifest
- [ ] Pack schema version
- [ ] Pack versions
- [ ] Pack validation
- [ ] Pack checksums
- [ ] Pack dependencies
- [ ] Pack install
- [ ] Pack activate
- [ ] Pack catalog
- [ ] Pack upgrade
- [ ] Environment Pack

## Updates

- [ ] Curriculum diff
- [ ] Change classification
- [ ] Migration plan
- [ ] Mastery preservation
- [ ] Added concept handling
- [ ] Split concept handling
- [ ] Removed/deprecated concept handling
- [ ] Backup before upgrade

## Transparency

- [ ] Reproducibility metadata
- [ ] Evidence Report
- [ ] Source Bundle refs
- [ ] Freshness/caveat propagation
- [ ] Copyright-aware pack build
- [ ] CLI
- [ ] TUI

---

# Definition of Done — I-04

- [ ] I-01–I-03 sin regresiones críticas
- [ ] Compiler no hace live web research
- [ ] Compiler consume Source Bundles I-03
- [ ] Curriculum output determinista
- [ ] Concept IDs estables
- [ ] No module limit
- [ ] No lesson limit
- [ ] No concept limit artificial
- [ ] Atomizer testeado
- [ ] Granularity Guard funciona
- [ ] Knowledge Graph sin ciclos
- [ ] Prerequisite expansion funciona
- [ ] Vocabulary Graph funciona
- [ ] Definition-before-use audit funciona
- [ ] Coverage multidimensional
- [ ] Gap Scanner funciona
- [ ] Zero-Assumption audit funciona
- [ ] First-principles expansion funciona
- [ ] Theory Coverage funciona
- [ ] Practice contracts existen
- [ ] Production Reality cubierta
- [ ] Toolchain cubierta
- [ ] Security cubierta
- [ ] Experimental/legacy/historical distinguido
- [ ] Curriculum Reviewer funciona
- [ ] Beginner Simulation funciona
- [ ] Expert Coverage Review funciona
- [ ] Learning Pack v1 documentado
- [ ] Packs inmutables por versión
- [ ] Pack dependencies resueltas
- [ ] Environment Pack funciona con Doctor
- [ ] Pack install/activate funciona
- [ ] Pack catalog funciona
- [ ] Pack upgrade preserva Student State
- [ ] Curriculum migration nunca inventa mastery
- [ ] Reproducibility metadata completa
- [ ] Evidence Report generado
- [ ] Copyright-aware build pasa
- [ ] Security review de packs pasa
- [ ] Large graph performance aceptable
- [ ] Reference Pack compilable
- [ ] Reference Pack revisado manualmente
- [ ] `go test ./...` pasa
- [ ] `go vet ./...` pasa
- [ ] race tests aplicables pasan
- [ ] CI Linux pasa
- [ ] CI Windows pasa
- [ ] CI macOS pasa
- [ ] E2E I-04 pasa
- [ ] Dogfooding realizado
- [ ] No bugs críticos conocidos
- [ ] Todos los pasos completados `[x]`
- [ ] PROGRESS.md actualizado
- [ ] Commits Conventional Commit coherentes
- [ ] Working tree limpio
- [ ] Release respeta SemVer
- [ ] Ready for I-05

---

# Resultado esperado al terminar I-04

Kelyro debe poder tomar evidencia investigada por I-03:

```text
Source Bundles
Claims
Versions
Freshness
Conflicts
Deprecations
```

y compilar:

```text
Learning Goal
    ↓
Professional Outcomes
    ↓
Competency Matrix
    ↓
Atomic Concepts
    ↓
Prerequisite Graph
    ↓
Vocabulary Graph
    ↓
Coverage / Gap Analysis
    ↓
Zero-Assumption
    ↓
Definition-before-use
    ↓
Production / Toolchain / Security
    ↓
Compiled Curriculum
    ↓
Learning Pack v1
```

Luego:

```text
$ kelyro packs install backend-go.kelyro-pack
$ kelyro packs activate backend-go
$ kelyro roadmap
```

debe mostrar un curriculum real y versionado.

La relación queda:

```text
I-03
Evidence about reality
        ↓
I-04
What must be learned
        ↓
I-02
What this student already knows
        ↓
I-05
What the student does next:
lesson / practice / assessment
```

La siguiente implementación, **I-05 — Lesson, Practice & Assessment Engine**, tomará `LessonSpec`, `Concept`, `PracticeExpectation`, `Mastery`, `DailyPlan` y el estado del estudiante para convertir el curriculum compilado en sesiones educativas reales.
