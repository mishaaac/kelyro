# I-04 Curriculum Compiler & Learning Packs — Progress Log

## Estado general

Current step: 53
Last completed step: 52
Current release: v0.2.0-alpha.3
Research baseline: v0.2.0-alpha.2 (`743cafecd383eff64ed325be674ba983f289bfa3`)
Branch baseline: `8658a7a`

## Registro

## Step 00 — Apertura formal de I-04

Status: completed
Date: 2026-09-09
Release: unreleased

### Delivered

- El plan I-04 quedó incorporado como memoria persistente en
  `docs/implementation/I-04-curriculum-compiler-learning-packs/PLAN.md`.
- El registro de progreso se inicializó con la release actual, el tag/commit
  publicado que cerró I-03 y el commit real desde el que se abrió la rama I-04.
- `AGENTS.md` quedó actualizado con el flujo de sesiones y las fronteras de
  evidencia, red, Student Core, I-05, determinismo, versionado y capas de UI.

### Decisions

- `v0.2.0-alpha.2` en
  `743cafecd383eff64ed325be674ba983f289bfa3` es el baseline publicado e
  inmutable que cerró formalmente I-03C y declaró Research listo para I-04.
- `v0.2.0-alpha.3` es la release actual al abrir I-04; `8658a7a` es la base
  efectiva de la rama `feat/i-04-curriculum-compiler-learning-packs`.
- El plan aportado en la raíz se trasladó a la ruta canónica para evitar dos
  copias divergentes.
- Este paso no cambia comportamiento, dependencias, schema ni código Go.

### Verification

- `git status --short --branch` y revisión de los 20 commits más recientes.
- Revisión del cierre formal y la publicación de I-03C.
- `go test ./...`.
- `go vet ./...`.

### Notes for next session

- El Paso 1 es el siguiente paso pendiente: diseñar el modelo de dominio
  curricular y sus invariantes antes de schema, repositories o services.
- No implementar interfaces de aplicación, persistence, formatos de packs ni
  ninguna parte de I-05 durante el Paso 1.

## Step 01 — Modelo de dominio curricular

Status: completed
Date: 2026-09-09
Release: unreleased

### Delivered

- Paquete cohesivo `internal/curriculum`, separado por archivos de goal,
  competency, concept, graph, hierarchy, coverage, compilation, pack,
  environment y change, sin dependencias externas ni infraestructura/UI.
- Value objects validados para IDs generales, `CurriculumID`, `ConceptID`,
  timestamps UTC, versiones curriculares opacas y `PackVersion` con sintaxis
  SemVer 2.0 estricta.
- Vocabulario completo del Paso 1 para definitions, goals/outcomes/roles,
  matrices de competencias, conceptos, prerequisitos, vocabulario, hierarchy,
  coverage/gaps, compilations, Learning Packs, Environment Packs y cambios.
- Estados cerrados para guidance temporal, atomicity, difficulty, competency,
  prerequisites, coverage, gaps, tool requirements, changes y migrations.
- Referencia inmutable a Source Bundles mediante ID, content hash, algorithm
  version y verified-at, más referencias Claim/bundle sin importar Research ni
  habilitar red.
- Validación relacional de duplicados, autorreferencias, outcomes/concepts,
  edges, vocabulary, hierarchy, sibling order, coverage targets y evidence.
- Separación explícita entre jerarquía visible y grafo pedagógico, sin límites
  artificiales de nodos o edges.
- Tests deterministas de value objects, taxonomías cerradas, timestamps,
  versiones, Source Bundle refs y agregados válidos/inválidos.
- Contrato y límites documentados en
  `docs/architecture/curriculum-domain.md` y enlazados desde el índice
  de arquitectura.

### Decisions

- Mantener un único paquete `internal/curriculum` por cohesión, siguiendo el
  patrón probado por Learning y Research; extraer subpaquetes solo cuando los
  application services revelen seams útiles.
- No reutilizar `internal/learning.Curriculum` como modelo de compilación: I-04
  conserva un modelo learner-neutral propio y más rico, y un paso posterior
  producirá el contrato de consumo I-02.
- Mantener `CurriculumVersion` opaca y `PackVersion` SemVer estricta porque son
  identidades distintas; la clasificación compatible/breaking se reserva al
  Paso 35.
- Hacer `required` la policy de evidencia productiva y conservar
  `optional_for_fixture` como excepción explícita para fixtures deterministas.
- El modelo base valida estructura y referencias, pero no implementa cycle
  detection, atomization, coverage, audit o compiler algorithms reservados a
  sus pasos versionados.
- No se añadieron repositories, services, SQLite, YAML/JSON, CLI/TUI, network,
  AI, generación I-05 ni mutaciones de Student Core.

### Verification

- `go test ./internal/curriculum -count=1`.
- `go vet ./internal/curriculum`.
- `go test ./... -count=1`.
- `go vet ./...`.
- Auditoría de imports: `internal/curriculum` importa únicamente `errors`,
  `fmt`, `regexp`, `strings`, `time` y `unicode` de la biblioteca estándar.
- `git diff --check`.

### Notes for next session

- El Paso 2 es el siguiente paso pendiente: repositories, external ports,
  application services y fakes in-memory sobre este vocabulario.
- No implementar persistence, schema de Learning Pack, loaders, compiler
  algorithms, CLI/TUI ni pasos posteriores durante el Paso 2.

## Step 02 — Repositories y application service boundaries

Status: completed
Date: 2026-09-09
Release: unreleased

### Delivered

- Paquete `internal/curriculum/application` con puertos separados para
  curriculums, packs/activation, catalog metadata, environment packs y
  compilation records.
- Contratos transport-neutral para `CurriculumCompilerService`, `PackService`,
  validation, install, upgrade, coverage y curriculum audits, sin implementar
  todavía sus algoritmos o side effects futuros.
- `ResearchBundleProvider` de solo lectura sobre Source Bundles/Claims durables
  I-03, explícitamente sin discovery, fetch, refresh ni network.
- Puertos `Clock`, `Filesystem`, `PackArchiveReader` y el hook opcional futuro
  `SignatureVerifier`, sin acoplar dominio a OS, archive format o signing.
- Taxonomía causal de errores `not_found`, `conflict`, `invalid_state`,
  `unavailable`, `persistence_failure` y `external_failure`, preservando causas
  y mapeando cancellation/deadline a unavailable.
- Fake `internal/curriculum/application/memory` para los cinco repositories,
  con mutex, validación de writes, versiones inmutables, orden estable, context
  cancellation y copias defensivas profundas.
- Tests de error mapping, roundtrip de todos los repository families,
  activation, catalog, compilation records, conflictos, ownership de slices y
  orden determinista.
- Límites y semántica documentados en
  `docs/architecture/curriculum-application.md` y enlazados desde el índice.

### Decisions

- Usar `Add`/`Append`, sin update de definitions o pack versions publicados,
  para representar inmutabilidad desde la frontera de persistence.
- Separar catalog metadata de pack installation y activation: aparecer en el
  catálogo nunca equivale a trust ni autorización para instalar.
- Permitir que application importe el dominio `internal/research` únicamente
  en el read port; `internal/curriculum` sigue sin depender de Research.
- Definir interfaces de los siete services ahora, pero reservar compiler,
  validation, install, upgrade, coverage y audit behavior para sus pasos.
- Exponer wrappers repository estrechos sobre un store fake compartido porque
  Go no permite sobrecargar `Add/Get/List` para agregados distintos.
- No se añadieron SQLite, persistence productiva, migrations, filesystem/archive
  implementations, YAML/JSON, network, CLI/TUI, AI ni Student Core writes.

### Verification

- `go test ./internal/curriculum/application/... -count=1`.
- `go vet ./internal/curriculum/application/...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `go test ./... -count=1`.
- `go vet ./...`.
- Auditoría de imports: application usa stdlib, Curriculum y el read-only
  Research boundary; memory usa stdlib, Curriculum y application únicamente.
- `git diff --check`.

### Notes for next session

- El Paso 3 es el siguiente paso pendiente: migration forward-only y schema
  SQLite para packs, compilations y curriculums instalados.
- No definir Learning Pack v1, loader, ingestion, compiler algorithms, CLI/TUI
  ni pasos posteriores durante el Paso 3.

## Step 04 — Learning Pack Format v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- Contrato portable `learning-pack/v1` documentado para directorios y ZIP,
  con layout, checksums, límites, seguridad y fronteras explícitas.
- `PackManifest` ampliado con domain, target, authors/maintainers, license,
  minimum Kelyro version y entries de curriculum, evidencia y environment.
- Parser YAML estricto, bounded y UTF-8 para un único `pack.yaml`, con rechazo
  de campos desconocidos, schema no soportado y timestamps que no sean UTC Z.
- Validación de IDs portables estables, SemVer 2.0, constraints AND simples,
  dependencias duplicadas/autorreferentes y rutas relativas canónicas.
- Tests positivos y negativos del manifest parser, incluidos traversal,
  rutas absolutas/Windows, schema, constraints, documentos múltiples y tamaño.

### Decisions

- V1 usa un único curriculum YAML agregado y un evidence report JSON; dividir
  esos documentos internamente queda permitido para una versión futura del
  schema, no como interpretación implícita de v1.
- ZIP es el único archive format de v1; un directorio conserva el mismo root
  lógico y el mismo archivo de checksums.
- `curriculum_id` es requerido además de los mínimos del plan para comprobar
  identidad cruzada antes de construir un `LearningPack` de dominio.
- Los constraints soportan conjunción de comparadores SemVer sin OR, rangos
  abreviados ni tags flotantes; resolverlos pertenece a pasos posteriores.
- No se implementaron todavía filesystem/archive loading, CLI, instalación,
  ingestion I-03, compiler passes, red, scripts ni Student Core writes.

### Verification

- `go test ./internal/curriculum/... ./internal/infra/learningpack -count=1`.
- `go vet ./internal/curriculum/... ./internal/infra/learningpack`.
- `git diff --check`.

### Notes for next session

- El Paso 5 es el siguiente paso: loader seguro de directory/ZIP, validación
  integral y `kelyro packs validate <path>`.
- No implementar instalación, dependency resolution, ingestion I-03 ni
  compiler algorithms durante el Paso 5.

## Step 05 — Secure Pack Loader and Validator

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `PackValidationService` productivo read-only para directorios y ZIP v1, con
  `ValidatedPack` representado por `PackValidationResult.Pack`, warnings y
  errores estructurados.
- Enumeración bounded con rechazo de root/file symlinks, escapes, paths no
  portables, entries duplicadas, special files, executable bits y extensiones
  de script/binario.
- Límites de 1,024 entries, 4 MiB por archivo, 32 MiB totales y ratio ZIP
  100:1, comprobados antes y durante lectura.
- Inventario SHA-256 completo, ordenado e inmutable mediante `checksums.txt`,
  más UTF-8 obligatorio para el contenido textual v1.
- Decoders estrictos para curriculum YAML, evidence report JSON y environment
  YAML, con validación de agregados y referencias entre documentos.
- CLI `kelyro packs validate <path>` cableada en composition root, con salida
  quiet segura, exit code no-cero y razones para packs inválidos.
- Tests de good directory/ZIP y bad checksum, UTF-8, evidence, schema, symlink,
  traversal, duplicate archive entry, executable mode y cancellation.
- Contrato operativo documentado en
  `docs/architecture/learning-pack-loader-v1.md`.

### Decisions

- El único archive soportado en v1 es ZIP; un archivo regular no-ZIP falla sin
  intentar inferir tar u otros formatos.
- Todos los archivos v1 son textuales y UTF-8. Esto mantiene fuera binarios,
  assets y contenido ejecutable hasta que un schema futuro defina su política.
- El loader valida constraints pero no resuelve disponibilidad de dependencies;
  install/resolution sigue reservado para pasos posteriores.
- Un status no-current válido produce warning; no se reetiqueta ni se trata
  como current.
- La CLI usa directamente el application boundary de validación y no abre un
  workspace, base de datos ni servicio de Research.
- No se implementaron instalación/activación, ingestion I-03, compiler passes,
  red, plugin runtime, I-05 ni Student Core writes.

### Verification

- `go test ./... -count=1`.
- `go vet ./...`.
- `go test -race ./internal/infra/learningpack ./internal/cli -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 6 es el siguiente paso: consumir bundles/claims I-03 durables y
  producir `CurriculumEvidenceSet` con eligibility determinista y sin red.
- No implementar Goal Decomposition ni pasos posteriores durante el Paso 6.

## Step 06 — Source Bundle ingestion from I-03

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- Modelo learner-neutral `CurriculumEvidenceSet` con Claim statements/scopes,
  version scopes, source authority roles, freshness, conflicts, temporal flags,
  exact Source Bundle ref, caveats, eligibility y algorithm version.
- `SourceBundleIngestionService` y `SourceBundleIngester` sobre el port I-03
  read-only ampliado a bundles, claims y conflicts durables.
- Adapter `infra/curriculumresearch` que compone los tres readers I-03 ya
  existentes sin exponer discovery, fetch ni operaciones de red.
- Mapping conservador `ready -> ready_for_compile`, caveats preservadas como
  `ready_with_caveats`, e incomplete/conflicted como `not_ready` no aceptado.
- Gate explícito para Claim IDs críticos: stale/unknown freshness y conflictos
  unresolved producen `not_ready` con razones estables.
- Validación de hashes canónicos, identidad/topic de Claims, conflicts dentro
  del bundle, source refs, version/status flags y algoritmos soportados.
- Orden determinista de Claims/Conflicts desde el bundle y de source IDs,
  version scopes y razones derivadas.
- Tests de ready, caveat, stale normal/crítico, incomplete, conflicted critical,
  missing bundle, historical/version flags y repetibilidad.
- Contrato documentado en
  `docs/architecture/curriculum-evidence-ingestion-v1.md`.

### Decisions

- La autoridad consumida es el role/temporal scope ya congelado en el Source
  Bundle; el ingestor no consulta ni reinterpreta trust mutable actual.
- Los rechazos de policy retornan `Accepted=false` con evidencia `not_ready` y
  razones; missing/corrupt/inconsistent durable data permanece error causal.
- `ready_with_caveats` es input aceptable para la siguiente etapa, pero no
  equivale a aceptación silenciosa para compilar; el futuro compiler deberá
  registrar su decisión.
- Historical Claim type y preview/experimental/legacy status scope se preservan
  en el vocabulario temporal curricular, nunca como recomendación current.
- No se añadieron discovery, fetch, refresh, queueing, red, Goal Decomposition,
  compiler passes, pack installation, I-05 ni Student Core writes.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test -race ./internal/curriculum/... ./internal/infra/curriculumresearch -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 7 es el siguiente paso pendiente: Goal Decomposition v1 sobre un
  goal/evidence set aceptado, sin hardcodear dominios en core.
- No implementar todavía Professional Outcome Model ni pasos posteriores.

## Step 07 — Goal Decomposition v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- Modelo `DomainProfile` con domain/scope vocabulary y áreas de competencia
  versionadas, evidenciadas y declaradas por el pack.
- Output `GoalDecomposition` con outcomes, áreas, scope/exclusions, profile ref
  y `goal-decomposer-v1`.
- `GoalDecomposerService` y algoritmo determinista que selecciona áreas por
  scope y por marker profesional explícito, preservando el orden del profile.
- Validación de dominio, scopes soportados, evidence eligibility, exact
  bundle/Claim refs, outcome mapping y cobertura completa del goal.
- Tests de goal estrecho, role profesional, ausencia de evidencia, scope no
  soportado y repetibilidad.
- Contrato documentado en `docs/architecture/goal-decomposition-v1.md`.

### Decisions

- Core no contiene taxonomías por profesión o tecnología; `DomainProfile` es
  la especificación declarativa que un pack aporta y evidencia.
- Los outcomes ya declarados por el `LearningGoalSpec` se preservan; este paso
  estructura su cobertura y no inventa nuevos resultados desde texto libre.
- Áreas generales aplican sin selector; áreas profesionales requieren role;
  las demás se seleccionan por intersección exacta de scopes.
- Evidence `ready_with_caveats` puede alimentar la descomposición sin perder
  sus caveats; `not_ready` bloquea.
- No se implementaron Professional Outcome Model, Competency Matrix, concept
  extraction, compiler orchestration, red, I-05 ni Student Core writes.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test -race ./internal/curriculum/... -count=1`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 8 es el siguiente: categorizar outcomes y exigir explain/build/debug/
  operate/maintain para goals profesionales.
- No implementar Competency Matrix v1 ni pasos posteriores durante el Paso 8.

## Step 08 — Professional Outcome Model

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- Taxonomía cerrada de outcome categories: knowledge, application, debugging,
  design, tool usage, production, security, communication/documentation y
  maintenance.
- Taxonomía cerrada de capabilities: explain, build, debug, operate y maintain.
- `GoalOutcome` ampliado con category obligatoria y capability explícita,
  opcional únicamente para goals no profesionales.
- Invariante de `LearningGoalSpec`: un goal con `ProfessionalRole` debe cubrir
  las cinco capabilities, sin inferirlas desde title/statement.
- Learning Pack curriculum decoder actualizado para rechazar categories o
  capabilities ausentes/desconocidas según el contrato.
- Tests de cobertura profesional completa/incompleta, enums desconocidos y
  compatibilidad de goals estrechos no profesionales.
- Contrato documentado en
  `docs/architecture/professional-outcomes-v1.md`.

### Decisions

- Category describe dimensión de cobertura; capability describe lo que el
  learner podrá hacer. No se fuerza una relación rígida entre ambas.
- Cada outcome declara una category y como máximo una capability; múltiples
  outcomes pueden cubrir la misma capability.
- La presencia de `ProfessionalRole`, no palabras del texto, activa la regla
  exhaustiva explain/build/debug/operate/maintain.
- No se generó outcome prose ni se implementaron Competency Matrix, practice,
  assessment, projects, I-05 o mutaciones de Student Core.

### Verification

- `go test ./internal/curriculum/... ./internal/infra/learningpack -count=1`.
- `go vet ./internal/curriculum/... ./internal/infra/learningpack`.
- `go test -race ./internal/curriculum/... ./internal/infra/learningpack -count=1`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 9 es el siguiente: construir `competency-matrix-v1` desde una
  descomposición, specs del pack y evidencia aceptada.
- No implementar Claim-to-Concept extraction ni pasos posteriores.

## Step 09 — Competency Matrix v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `Competency` ampliada con stable area ID, dimensiones de profundidad
  opcionales y parent ID opcional, además del area label, outcome, nivel,
  evidence refs y futuros concept refs.
- `CompetencyMatrixBuilderService` y builder determinista
  `competency-matrix-v1` sobre Goal Decomposition, declarations y evidence.
- Cobertura obligatoria de todos los goal outcomes y todas las áreas
  seleccionadas por la descomposición.
- Rechazo de competencias sin evidencia, IDs duplicados, area/outcome no
  soportados, area label inconsistente y evidence refs ausentes.
- Jerarquía opcional validada: parent existente, misma área, no self-parent y
  cycle detection determinista.
- Profundidad multidimensional con IDs declarativos del pack y niveles cerrados
  independientes del nivel principal.
- Decoder Learning Pack v1 actualizado para `area_id`, `dimensions` y
  `parent_id`.
- Tests de completeness, duplicate, unsupported competency, missing evidence,
  dimensiones, ownership/repetibilidad y jerarquía válida/cíclica.
- Contrato documentado en `docs/architecture/competency-matrix-v1.md`.

### Decisions

- Core no hardcodea dimensiones de competencia; cada pack declara IDs estables
  como theory, communication u otros apropiados a su dominio.
- Una competencia v1 se vincula a un outcome; múltiples competencias pueden
  cubrir el mismo outcome y cada outcome debe tener al menos una.
- `concept_refs` permanece opcional porque el Paso 10 aún no extrajo conceptos;
  `CurriculumDefinition` seguirá validando su existencia al finalizar.
- La excepción `optional_for_fixture` se conserva en el modelo base, pero el
  builder productivo v1 nunca emite una competencia sin evidencia.
- No se implementaron Claim-to-Concept extraction, atomization, compiler
  orchestration, I-05, red ni Student Core writes.

### Verification

- `go test ./internal/curriculum/... ./internal/infra/learningpack -count=1`.
- `go vet ./internal/curriculum/... ./internal/infra/learningpack`.
- `go test -race ./internal/curriculum/... ./internal/infra/learningpack -count=1`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 10 es el siguiente paso pendiente: derivar concept candidates desde
  Claims estructurados sin asumir que cada Claim equivale a un concepto.
- No implementar todavía atomization ni pasos posteriores.

## Step 10 — Claim-to-Concept candidate extraction

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- Familia cerrada `EvidenceClaimKind` que preserva el tipo estructurado del
  Claim I-03 sin acoplar el dominio curricular al paquete Research.
- `ConceptCandidate`, `ConceptCandidateSet` y servicio de extracción
  `claim-to-concept-candidates-v1` sobre evidence sets aceptados.
- Agrupación por scope semántico normalizado, manteniendo separados version
  scope y status temporal y conservando todos los Claim refs exactos.
- IDs SHA-256 estables, orden determinista y selección de spelling independiente
  del orden de entrada.
- Tests de claims duplicados, definition + behavior, historical,
  version-specific y repetibilidad con Claims reordenados.
- Contrato documentado en
  `docs/architecture/claim-to-concept-candidates-v1.md`.

### Decisions

- Usar el `scope` estructurado de I-03 como semantic subject v1; no aplicar NLP,
  taxonomías hardcodeadas ni inferencias desde prose no estructurada.
- Agrupar familias de Claim distintas cuando respaldan el mismo subject, pero
  nunca mezclar estados temporales ni version scopes distintos.
- Mantener candidatos separados de `Concept`: la atomicidad y cualquier split
  pertenecen a los Pasos 11 y 12.
- No se implementaron atomization, granularity guard, prerequisites, red, I-05
  ni mutaciones de Student Core.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 11 es el siguiente: formalizar Atomic Concept Criteria v1 y sus
  resultados cerrados antes de dividir candidatos.
- No implementar Atomizer v1 ni pasos posteriores durante el Paso 11.

## Step 11 — Atomic Concept Criteria v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- Política pura `atomic-concept-criteria-v1` con señales explícitas para name,
  definition, prerequisite boundary, explanation, practice, assessment,
  evidence y valor standalone.
- Estados tri-state `satisfied`, `unsatisfied` y `unknown` para impedir que
  metadata ausente se interprete como aprobación.
- Resultados cerrados `atomic`, `needs_split`, `too_fragmented` y `unknown`
  con razones machine-stable y precedencia determinista.
- Detección declarativa de múltiples partes independientemente evaluables sin
  nombres de dominio hardcodeados.
- Tests de los cuatro resultados, anti-patterns amplios, estados inválidos y
  partes duplicadas.
- Contrato documentado en
  `docs/architecture/atomic-concept-criteria-v1.md`.

### Decisions

- La policy clasifica señales aportadas explícitamente; no intenta extraer
  semántica desde títulos o prose mediante heurísticas frágiles.
- Una frontera de prerequisitos explícitamente foundational satisface el mismo
  criterio; los prerequisitos concretos pertenecen a los Pasos 14 y 15.
- `needs_split` tiene precedencia cuando hay varias partes evaluables;
  `too_fragmented` representa ausencia explícita de valor autónomo o de
  explain/practice/assess independiente; lo incompleto queda `unknown`.
- No se implementaron splits, concepts, granularity guard, prerequisites, I-05
  ni mutaciones de Student Core.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 12 es el siguiente: Atomizer v1 debe consumir candidatos, Claims,
  esta policy y domain hints para emitir concepts atómicos con claim mapping.
- No implementar Granularity Guard ni prerequisites durante el Paso 12.

## Step 12 — Atomizer v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `ConceptAtomizerService` y `atomizer-v1` con Atomicity Policy inyectada,
  candidatos validados, evidence sets aceptados y domain hints declarativos.
- Modelo `ConceptAtomizationHint`, `AtomicConceptSet` y mapping one-to-many de
  Claims a Concepts con invariantes bidireccionales.
- Preservación de candidatos ya atómicos mediante un único hint y split de
  candidatos amplios únicamente con dos o más hijos evidenciados y atómicos.
- Rechazo conservador de candidatos unknown/too_fragmented, hijos no atómicos,
  Claims externos o sin mapear y ausencia de evidencia suficiente para split.
- Soporte explícito de evidence overlap: un Claim puede respaldar más de un
  Concept sin fusionar sus identidades.
- Orden estable por Concept ID y bundle/Claim ID, copias defensivas, status
  heredado y version scope comprobado.
- Tests de already atomic, broad concept, overlapping split, no evidence to
  split, child no atómico, Claim sin mapear y repetibilidad con hints reordenados.
- Contrato documentado en `docs/architecture/concept-atomizer-v1.md`.

### Decisions

- Los domain hints aportan semántica de split y stable Concept IDs; el core
  valida evidencia y atomicidad pero nunca hardcodea el ejemplo Variables ni
  otra taxonomía de dominio.
- Un split sin hints evidenciados es un error explícito, no una oportunidad de
  inventar conceptos desde prose.
- Permitir mapping Claim-to-Concept one-to-many porque una misma afirmación
  puede evidenciar varias unidades evaluables; la relación siempre es explícita.
- No se implementaron Granularity Guard, prerequisite extraction/expansion,
  graph compiler, I-05 ni mutaciones de Student Core.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 13 es el siguiente: Granularity Guard debe impedir límites o merges
  estéticos sin borrar stable Concept IDs.
- No implementar prerequisite extraction ni pasos posteriores durante el Paso 13.

## Step 13 — Granularity Guard v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `granularity-guard-v1` sobre Concepts, merge proposals y visual groups, con
  Atomicity Policy v1 inyectada.
- Forced splits para concepts `needs_split` y warnings estructurados para
  unidades amplias, demasiado fragmentadas o con atomicity desconocida.
- Merge decisions explícitas: solo se permite un merge cuya unidad resultante
  conserve atomicidad, explicación, práctica y evaluación independientes.
- Agrupación visual segura por LessonSpec ID que conserva todos los stable
  Concept IDs y rechaza referencias inexistentes o asignación visual múltiple.
- Fixture de 5.000 Concepts que prueba la ausencia de límites artificiales,
  además de tests de merge seguro/inseguro, forced split y determinismo.
- Contrato documentado en `docs/architecture/granularity-guard-v1.md`.

### Decisions

- El guard reporta decisiones y warnings pero no modifica ni fusiona Concepts.
- La compactación visual es metadata separada de la identidad y de la
  granularidad semántica.
- No existe configuración de máximos para Concepts, lessons o modules; el
  tamaño lo determina el objetivo y la evidencia.
- No se implementaron prerequisite extraction/expansion, graph compiler, I-05
  ni mutaciones de Student Core.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 14 es el siguiente: derivar prerequisites directos desde evidencia y
  semántica declarada, sin usar orden de phases/modules/lessons/topics.
- No implementar prerequisite expansion ni Knowledge Graph Compiler todavía.

## Step 14 — Prerequisite Extraction v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `prerequisite-extractor-v1` y service boundary para derivar aristas directas
  entre Concepts atómicos.
- `ConceptPrerequisiteSemantic` evidenciado con dependent/required IDs, kind,
  Claim refs exactos y razón explícita.
- Soporte de `hard`, `recommended`, `exposure_only`, `tool_dependency` y
  `vocabulary` mediante la taxonomía cerrada existente.
- `PrerequisiteExtraction` con derivaciones ordenadas y proyección defensiva a
  las aristas `Prerequisite` del dominio.
- Validación de Concepts, self-reference, evidencia disponible y duplicados,
  sin recibir ni consultar orden visual alguno.
- Tests deterministas de chains, diamonds, los cinco tipos, Concept ausente y
  Claim ref no disponible.
- Contrato documentado en
  `docs/architecture/prerequisite-extraction-v1.md`.

### Decisions

- La semántica viene declarada por el dominio/pack y respaldada por Claims; no
  se infiere desde títulos, prose ni posición en la jerarquía.
- Este pass acepta solo aristas cuyos dos Concepts ya existen. Los fundamentos
  ausentes pertenecen al Paso 15.
- Permitir múltiples kinds entre el mismo par si cada arista exacta es única,
  siguiendo la identidad relacional del dominio existente.
- No se implementaron prerequisite expansion, cycle/topological graph compile,
  I-05 ni mutaciones de Student Core.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 15 es el siguiente: expandir fundamentos disponibles recursivamente,
  reportar gaps no resueltos y prevenir ciclos.
- No implementar el Knowledge Graph Compiler del Paso 16.

## Step 15 — Prerequisite Expansion v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `prerequisite-expansion-v1` y service boundary sobre Concepts actuales,
  direct prerequisites, semántica declarada, catálogo evidenciado y evidence sets.
- Inserción recursiva de Concepts prerequisite disponibles, conservando stable
  IDs, metadata, Claim refs y las aristas directas ya existentes.
- Gaps estructurados para missing root boundary, prerequisite Concept no
  disponible y arista descartada por cycle prevention.
- Rechazo de ciclos de entrada, prevención incremental de nuevos ciclos y
  validación acíclica del resultado sin adelantar el graph compiler completo.
- Orden determinista de added Concepts, expanded prerequisites, gaps y reasons,
  con copias defensivas y sin inferencias desde jerarquía visual.
- Tests de missing root, recursive expansion, unavailable prerequisite, cycle
  prevention y repetibilidad con catálogos/semántica reordenados.
- Contrato documentado en
  `docs/architecture/prerequisite-expansion-v1.md`.

### Decisions

- Solo se inserta un Concept ausente cuando el catálogo lo aporta atómico y con
  evidencia verificada; un ID mencionado no basta para inventarlo.
- Un Concept no foundational sin boundary declarada produce un gap, mientras
  que un prerequisite requerido pero ausente del catálogo usa un código distinto.
- Una arista que cerraría un ciclo se omite y queda explicada como gap; topology,
  reachability, components y critical path pertenecen al Paso 16.
- No se implementaron Knowledge Graph Compiler, Vocabulary Graph, I-05 ni
  mutaciones de Student Core.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 16 es el siguiente: compilar el grafo validado con cycle detection,
  topological order, roots, components, unreachable nodes y critical path.
- No implementar Vocabulary Graph ni pasos posteriores durante el Paso 16.

## Step 16 — Knowledge Graph Compiler v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `knowledge-graph-compiler-v1` y service boundary sobre Concepts atómicos y
  prerequisites tipados, sin dependencia de hierarchy, persistence o red.
- Rechazo de Concepts ausentes/no atómicos, edges duplicadas y ciclos, con
  orden topológico estable mediante priority queue lexicográfica.
- Roots, componentes débilmente conectados y Concepts no alcanzables desde un
  root marcado explícitamente como foundational.
- Critical path global y por componente, con desempate determinista por stable
  Concept ID y metadata de cantidad de edges.
- Proyección declarada `curriculum-consumption/v1`: hard requiere mastery;
  exposure/tool/vocabulary requieren introduction; recommended no bloquea.
- Fixture de cadena de 5.000 Concepts y tests de componentes, unreachable,
  cycles, proyección I-02 y repetibilidad con inputs reordenados.
- Contrato documentado en
  `docs/architecture/knowledge-graph-compiler-v1.md`.

### Decisions

- Distinguir root estructural de foundational root: un componente sin
  fundamento explícito se conserva pero sus Concepts quedan en unreachable.
- Mantener todos los prerequisite kinds en el DAG; la proyección I-02 omite
  recommended porque el contrato Student Core solo representa gates.
- Resolver múltiples edge kinds del mismo par con `mastered` por encima de
  `introduced`, sin perder las edges tipadas del artifact curricular.
- Este paso no construye todavía hierarchy ni el `learning.Curriculum`
  completo, y no crea instancias ni modifica mastery.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 17 es el siguiente: construir Vocabulary Graph con resolución
  explícita de aliases/acrónimos y domain baseline declarado.
- No implementar Definition-before-use Audit ni pasos posteriores durante el
  Paso 17.

## Step 17 — Vocabulary Graph v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `vocabulary-graph-v1` y service boundary sobre Concepts, definiciones de
  términos, uses observados y domain baseline explícito.
- `VocabularyDefinition` con term, canonical Concept, introduced-by, aliases y
  scope; `VocabularyUse` conserva el término observado y su Concept.
- Resolución case-insensitive de términos, aliases y acrónimos declarados, con
  namespace único y rechazo de mappings ambiguos.
- `VocabularyGraph` canónico con `UsedBy` estable y
  `ResolvedVocabularyUse` que conserva spelling observado/canónico.
- `DomainVocabularyBaselineTerm` con scope y reason obligatorios; no existe una
  lista global mágica de vocabulario supuesto.
- Rechazo de uses sin declaración/baseline, referencias a Concepts ausentes y
  colisiones entre términos, aliases o baseline.
- Tests de acronym, alias, first use, baseline explícito, término no declarado,
  colisión ambigua y repetibilidad con input reordenado.
- Contrato documentado en `docs/architecture/vocabulary-graph-v1.md`.

### Decisions

- Tratar acronym expansion como alias resolution explícita; el builder nunca
  adivina expansiones desde las letras o el contexto.
- Conservar baseline y uses resueltos en el artifact para que el audit siguiente
  pueda explicar excepciones y el spelling realmente usado.
- Coalescer uses del mismo canonical term en el mismo Concept, conservando un
  output set-like estable y evitando violations duplicadas.
- La precedencia pedagógica de introduced-by frente a used-at pertenece al Paso
  18; este paso valida identidad y resolución, no aprueba el orden.

### Verification

- `go test ./internal/curriculum/... ./internal/infra/learningpack -count=1`.
- `go vet ./internal/curriculum/... ./internal/infra/learningpack`.
- `git diff --check`.

### Notes for next session

- El Paso 18 es el siguiente: auditar definition-before-use con prerequisite
  reachability, aliases resueltos, baseline explícito y same-lesson order.
- No implementar Coverage Engine ni pasos posteriores durante el Paso 18.

## Step 18 — Definition-before-use Audit v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `definition-before-use-v1` y service boundary sobre Knowledge Graph y
  Vocabulary Graph compilados, con Topic positions opcionales para orden local.
- Aceptación de introducción en el mismo Concept, como prerequisite transitivo
  o en una posición anterior de la misma lesson.
- Violations estructuradas con code, canonical/observed term, used-at, expected
  introduction, safe suggested prerequisite, severity y reason.
- Distinción entre missing vocabulary prerequisite, introduction posterior en
  el DAG y same-lesson order incompatible.
- Aliases/acrónimos auditados por su canonical term sin perder el spelling
  observado que explica la violation.
- Exención únicamente para uses resueltos contra el domain baseline explícito,
  con conteos separados y sin lista global de términos comunes.
- Tests de curriculum válido, inválido, alias, same-lesson order en ambos
  sentidos y baseline explícito.
- Contrato documentado en
  `docs/architecture/definition-before-use-v1.md`.

### Decisions

- Dar precedencia al prerequisite DAG: si el use Concept es prerequisite del
  introducer, el orden visual no puede declarar válida la secuencia.
- Permitir introducción anterior dentro de una lesson sin exigir una edge solo
  para expresar orden intralección; fuera de ella sí se requiere reachability.
- Sugerir una vocabulary prerequisite únicamente si falta y es segura; no
  sugerir una edge ya existente o que cerraría un ciclo.
- Todo hallazgo es error y hace fallar este audit v1; severity queda explícita
  para composición futura con el reviewer.
- No se modifican graph/hierarchy, no se implementa Coverage, I-05 ni mastery.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 19 es el siguiente: Coverage Engine v1 debe medir las dimensiones
  independientes definidas por el plan sobre el curriculum compilado.
- No implementar Gap Scanner ni pasos posteriores durante el Paso 19.

## Step 19 — Coverage Engine v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `coverage-v1` y `CoverageService` sobre Curriculum ID, goal, competency
  matrix, Concepts, requirements, supports y evidence sets verificadas.
- `CoverageReport` con las ocho dimensiones independientes del plan, cada una
  con status missing/partial/covered, resultados y particiones por requirement.
- Medición estructural de goal-outcome-to-competency,
  competency-to-Concept y evidence refs disponibles en Claims aceptadas.
- `CoverageSupport` trazable para los contracts especializados posteriores,
  con Concept, Claim y artifact refs explícitas y reason obligatorio.
- Dimensiones sin requirements reportadas como missing; no se calcula ni se
  expone un porcentaje o score global.
- Rechazo de target curriculum/goal incompatible, evidence no disponible,
  support huérfano y referencias de soporte a Concepts ausentes.
- Tests de las ocho dimensiones, covered/partial/missing, soporte explícito,
  ausencia de requirements, evidence inválida y determinismo por input order.
- Contrato documentado en `docs/architecture/curriculum-coverage-v1.md`.

### Decisions

- Tratar CoverageRequirement como unidad atómica; partial surge de una unidad
  estructural incompleta o de una dimensión con resultados mixtos.
- Medir directamente competency/concept/evidence y reservar las reglas
  especializadas de theory/practice/production/security/toolchain para los
  Pasos 23–27, sin adelantarlas.
- Exigir soportes declarados para esas cinco dimensiones en lugar de inferir
  cobertura desde nombres, descripciones o prose.
- Mantener las ocho dimensiones visibles aunque una no tenga requirements, de
  modo que una omisión no quede escondida.
- No se implementaron Gap Scanner, reglas de los Pasos 23–27, I-05 ni mastery.

### Verification

- `go test ./internal/curriculum/... ./internal/infra/learningpack -count=1`.
- `go vet ./internal/curriculum/... ./internal/infra/learningpack`.
- `git diff --check`.

### Notes for next session

- El Paso 20 es el siguiente: convertir cada resultado incompleto y findings
  prerequisite/current-guidance explícitos en gaps estables y accionables.
- No implementar Zero-Assumption Audit ni pasos posteriores durante el Paso 20.

## Step 20 — Curriculum Gap Scanner v1

Status: completed
Date: 2026-09-11
Release: unreleased

### Delivered

- `gap-scanner-v1` y service boundary sobre Coverage Report, requirements,
  prerequisite expansion gaps y current-guidance findings explícitos.
- Mapping completo de las ocho coverage dimensions a missing competency,
  concept, evidence, theory, practice, production, security y toolchain.
- Conversión adicional a missing prerequisite y missing current guidance, sin
  ejecutar todavía clasificación temporal de los Pasos 28–29.
- Gap goal-targeted para cada dimensión sin requirements, evitando que una
  omisión declarativa desaparezca del reporte.
- Política explícita blocking/important/recommended para missing y partial;
  informational queda reservado para policies advisory posteriores.
- IDs SHA-256 deterministas, deduplicación exacta, evidence refs canónicas y
  orden por severidad, kind, target e ID.
- Validación de correspondencia exacta entre Coverage Report y requirements.
- Fixtures que cubren los diez Gap kinds, severidades, partial coverage,
  mismatch inválido y repetibilidad con inputs reordenados.
- Contrato documentado en `docs/architecture/curriculum-gap-scanner-v1.md`.

### Decisions

- Tratar una dimensión no declarada como gap accionable en el goal, no como un
  estado neutro o implícitamente cubierto.
- Mantener los gaps atómicos por requirement; el scanner no produce un score ni
  permite que una dimensión compense otra.
- Recibir current-guidance findings por boundary explícita para no adelantar la
  inteligencia temporal y best-practice de los Pasos 28–29.
- Derivar identidad desde el payload causal completo para que findings distintos
  sobre el mismo target no se sobrescriban según input order.
- No se implementaron Zero-Assumption Audit, specialized coverage, I-05 ni
  mutaciones de Student Core.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 21 es el siguiente: Zero-Assumption Audit v1 debe usar un baseline y
  Domain Profile declarados, nunca una lista universal hardcodeada.
- No implementar First-Principles Expansion ni pasos posteriores durante el
  Paso 21.

## Step 03 — Persistence schema y migration I-04

Status: completed
Date: 2026-09-09
Release: unreleased

### Delivered

- Migration SQLite forward-only v48 sobre el cierre I-03 v47, sin modificar
  ninguna de las 47 migrations publicadas anteriores.
- Schema para definitions/versions y Source Bundle refs, competencies/mapping,
  packs/versions/dependencies/installations, compilations/passes, coverage,
  gaps/audits y environment packs/tool requirements.
- Reutilización explícita de `curriculum_instances`, `curriculum_nodes`,
  `curriculum_edges` y `concept_registry` del contrato I-02, evitando tablas
  duplicadas o una segunda verdad para el hand-off curricular.
- Versiones de curriculum, Learning Pack, compilations y Environment Pack
  protegidas por identidad compuesta y triggers de inmutabilidad.
- Estado available separado de installation/activation; content hash se fija
  en la instalación y un índice parcial permite un solo pack activo por
  workspace database.
- Foreign keys desde curriculum evidence hacia Source Bundles I-03 y entre
  definitions, versions, competencies, nodes, packs y environment metadata.
- Índices para lookup por curriculum, source bundle, competency/concept,
  installation, compilation, coverage, gap severity y tool introduction.
- JSON metadata validada y acotada entre 1 MiB y 64 MiB según artifact; no se
  añadieron campos para raw web bodies, scripts, credentials o secrets.
- Test de migración real desde schema 47 que conserva estado/Source Bundles,
  valida schema 48, FK de evidencia e inmutabilidad de versiones.
- Contrato documentado en `docs/architecture/curriculum-persistence.md` y
  enlazado desde el índice de arquitectura.

### Decisions

- Tratar las tablas `curriculum_*` I-02 existentes como la proyección durable
  del consumption contract, no como Student State; los learner instances viven
  en tablas separadas y no se modifican en este paso.
- Conservar el agregado completo en JSON bounded de version/compilation y
  projections normalizadas queryables, sin definir todavía el formato portable
  Learning Pack v1.
- No agregar FK de `pack_dependencies.dependency_pack_id` a una versión local:
  una dependencia puede estar declarada aunque aún no esté disponible o
  instalada; el resolver futuro decidirá ese estado.
- Mantener installation mutable como lifecycle de workspace, pero hacer las
  versions y compilation history inmutables.
- Los workflows que proyectan los repositories del Paso 2 a estas tablas se
  implementarán junto con compilation/install lifecycle; este paso congela el
  schema durable y sus constraints sin adelantar esos comportamientos.
- No se duplicó Student Concept State ni se añadieron format loaders, compiler
  algorithms, CLI/TUI, network, AI o I-05.

### Verification

- `go test ./internal/storage/sqlite -run 'TestResearchSchemaMigratesToCurriculumPersistence|TestOpenCreatesAndMigratesNewDatabase' -count=1`.
- `go test ./internal/storage/sqlite -count=1`.
- `go vet ./internal/storage/sqlite`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 4 es el siguiente paso pendiente: especificar Learning Pack Format
  v1 y su manifest parser/validator sin implementar todavía el loader seguro.
- No implementar filesystem/archive loading, I-03 ingestion, compiler passes,
  installation, CLI/TUI ni pasos posteriores durante el Paso 4.

## Step 21 — Zero-Assumption Audit v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `zero-assumption-v1` con perfiles `zero`, `some_experience` y
  `domain_experienced`, separados del estado mutable del estudiante.
- `AssumptionBaseline` ligado a una versión exacta de Domain Profile, con
  fundamentos relevantes declarados por competencia y respaldados por Claims.
- Prohibición de assumptions explícitos para `zero`; los perfiles con
  experiencia pueden declarar Concept IDs asumidos sin una lista global oculta.
- Auditoría de existencia, clasificación foundational y reachability como
  prerequisite de cada Concept objetivo de la competencia.
- Findings estructurados para foundation ausente, foundation no-root y edge de
  prerequisite ausente, con target, evidencia, severidad y razón.
- Validación cruzada de profile, baseline, matrix, Concepts, Knowledge Graph y
  evidencia I-03 aceptada, con output estable ante input reordenado.
- Contrato documentado en
  `docs/architecture/zero-assumption-audit-v1.md`.

### Decisions

- Hacer que el baseline enumere únicamente fundamentos relevantes al goal; el
  audit no hardcodea terminal, filesystem, process, editor, VCS o network como
  universales.
- Auditar contra el DAG compilado y exigir una ruta pedagógica real desde el
  foundation hasta cada Concept de la competencia, no solo presencia nominal.
- Mantener el learner profile como input curricular declarado, no inferirlo del
  Student Core ni mutar mastery.
- Reservar la inserción de roots y edges para el Paso 22; este paso solo emite
  diagnósticos deterministas.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 22 es el siguiente: expandir foundations detectados únicamente cuando
  exista un candidato atómico con evidencia verificada.
- La expansión debe producir research needs explícitos cuando falte evidencia y
  no debe adelantar Theory Coverage ni Practice Coverage.

## Step 22 — First-Principles Expansion v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `first-principles-expansion-v1` sobre findings validados de
  `zero-assumption-v1` para el perfil `zero`.
- Candidates pack-authored con Concept atómico/foundational, tipo de edge,
  evidencia separada para Concept y relación, y razón explícita.
- Inserción determinista de roots ausentes y prerequisite edges hacia todos los
  Concepts afectados, conservando inputs y publicadas como inmutables.
- Research needs estructurados para candidate ausente, evidencia insuficiente,
  conflicto con Concept existente y prevención de ciclos.
- Rechazo de cualquier candidate cuya identidad no coincida con el foundation
  detectado, más validación de Concepts, edges y evidencia disponible.
- Tests de expansión válida, ausencia de candidate, falta de evidencia,
  conflicto inmutable y repetibilidad.
- Contrato documentado en
  `docs/architecture/first-principles-expansion-v1.md`.

### Decisions

- Exigir evidencia verificada tanto para el root como para la relación; una
  definición respaldada no demuestra por sí sola el prerequisite pedagógico.
- Emitir solo additions en el artifact para que el pipeline futuro recompile el
  DAG explícitamente, sin mutar el graph de entrada in-place.
- Convertir evidencia ausente/unavailable en `research required`, nunca en una
  inferencia automática ni una llamada live a I-03.
- No reescribir Concepts existentes que no sean foundational; se reporta el
  conflicto para una nueva versión curricular.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 23 es el siguiente: declarar y medir definition, mental model,
  mechanism, tradeoffs y failure modes por competencia importante.
- No implementar expectations de práctica ni demás coberturas especializadas.

## Step 23 — Theory Coverage v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `theory-coverage-v1` con facets independientes `definition`, `mental_model`,
  `mechanism`, `tradeoffs` y `failure_modes`.
- Importancia declarada mediante IDs de competencias; no se infiere desde
  nombres, prose ni un nivel de competencia hardcodeado.
- `TheoryContract` evidence-backed por competencia y `TheoryFacetSupport`
  trazable a Concepts de esa misma competencia y Claims aceptadas.
- Estados missing/covered por facet y missing/partial/covered agregados por
  competencia, con ausencia de contrato tratada como hallazgo explícito.
- Bridge al Coverage Engine mediante un requirement atómico por facet y support
  correspondiente; un facet cubierto no oculta definition o mental model.
- IDs deterministas, orden canónico de facets y output estable ante input
  reordenado.
- Tests de definition/mental model ausentes, contrato completo, contrato
  ausente, soporte cross-competency inválido y repetibilidad.
- Contrato documentado en `docs/architecture/theory-coverage-v1.md`.

### Decisions

- Hacer que cada contrato seleccione sus facets requeridos según evidencia; los
  cinco tipos existen en v1 pero no se fuerzan universalmente.
- Exigir soporte explícito y evidence-backed; no considerar que el campo
  `Concept.Definition` cubra automáticamente todo el contrato conceptual.
- Emitir requirements separados por facet para conservar la independencia y
  permitir que Coverage/Gap Scanner reporten faltantes accionables.
- Mantener el output como contrato estructurado de I-04; I-05 será quien genere
  o muestre contenido y prácticas.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 24 es el siguiente: definir el Practice Coverage Contract que I-05
  podrá consumir sin implementar todavía su runtime.
- No adelantar Production Reality, Security o Toolchain Coverage.

## Step 24 — Practice Coverage Contract v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `practice-coverage-v1` y `practice-compatibility-v1` con expectations
  `recall`, `recognize`, `apply`, `debug`, `design`, `build`, `compare` y
  `explain`.
- Concepts importantes declarados explícitamente y requirement único que los
  liga a una competencia que realmente contiene el Concept.
- Matriz versionada de compatibilidad contra `ExpectedLevel`; una actividad de
  menor demanda no satisface por accidente una capacidad superior.
- Resultados con expectations compatibles e incompatibles separados y estado
  missing/covered por Concept importante.
- Bridge al Coverage Engine mediante requirements `practice_contract` y
  supports únicamente para expectations compatibles.
- Contratos y expectations respaldados por Claims aceptadas, IDs estables y
  resultados deterministas ante input reordenado.
- Tests de expectativa compatible, expectativa insuficiente, mapping inválido
  y repetibilidad.
- Contrato documentado en `docs/architecture/practice-coverage-v1.md`.

### Decisions

- Separar la declaración de importancia del requirement para poder verificar
  que ningún Concept importante queda omitido silenciosamente.
- Tratar PracticeExpectation como hand-off estructurado para I-05, no como
  ejercicio, assessment, solución, score o runtime.
- Mantener la compatibilidad como política explícita/versionada en lugar de
  comparar strings o asumir una jerarquía universal de actividades.
- Conservar expectations incompatibles en el diagnóstico aunque no cuenten
  como coverage.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 25 es el siguiente: medir categorías de Production Reality declaradas
  por el dominio y exigir Source Bundles adecuados.
- No implementar Toolchain ni Security Coverage durante el Paso 25.

## Step 25 — Production Reality Coverage v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `production-coverage-v1` con diez categorías cerradas de realidad productiva,
  seleccionadas explícitamente por el dominio y no impuestas universalmente.
- Requirements atómicos sobre curriculum, goal, competency o Concept, y
  supports trazables a Concepts y Claims exactas.
- `production-evidence-v1` para distinguir Claims operativas adecuadas de
  definiciones/tutoriales y fuentes históricas.
- Exigencia de autoridad primary/supporting current/version-bound y tipos de
  Claim operativos; evidence conocida pero inadecuada queda visible y no cuenta.
- Resultados missing/covered por requirement con supports adecuados y
  rechazados separados.
- Bridge directo a la dimensión `production` del Coverage Engine, que recibe
  solo supports que pasaron la policy.
- Tests de categorías declaradas, evidencia adecuada, definition-only,
  autoridad histórica, integración con Coverage y repetibilidad.
- Contrato documentado en
  `docs/architecture/production-reality-coverage-v1.md`.

### Decisions

- No exigir las diez categorías a todo dominio: la ausencia relevante debe ser
  declarada por el Domain Profile y no adivinada por core.
- Evaluar evidence inadecuada como coverage missing, reservando invalid input
  para referencias inexistentes o estructuras corruptas.
- Excluir definition/example-only y autoridad histórica para evitar que un
  tutorial o material archivado demuestre preparación productiva actual.
- Mantener el análisis learner-neutral y sin ejecutar despliegues, herramientas
  ni observabilidad real.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 26 es el siguiente: comprobar tools declaradas contra Environment
  Packs, punto de introducción, nivel, plataformas y evidencia.
- No implementar Security Coverage ni Environment Pack installation.

## Step 26 — Toolchain Coverage v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `toolchain-coverage-v1` sobre `ToolRequirement` y `EnvironmentPack`
  existentes, sin duplicar el modelo portable del pack.
- Requirements goal-scoped con tool ID, Environment Pack ID/version exactos,
  minimum level, propósito indirecto, razón y evidencia.
- Resolución exacta de pack/version y tool, sin sustitución implícita entre
  versiones disponibles.
- Verificación de nivel required/recommended/optional, Concept de introducción,
  notas por plataforma y evidencia aceptada.
- Estados missing para pack/tool ausente, partial con missing fields estables y
  covered únicamente para metadata completa.
- Bridge a la dimensión `toolchain` del Coverage Engine solo para tools
  completamente resueltas.
- Tests con tools arbitrarias no hardcodeadas, metadata incompleta, nivel débil,
  pack/tool ausente y repetibilidad ante inputs reordenados.
- Contrato documentado en `docs/architecture/toolchain-coverage-v1.md`.

### Decisions

- Reutilizar `ToolRequirement` como dueño de purpose, level, introduction,
  platforms y evidence; el nuevo requirement declara la necesidad y su pack.
- Considerar required más fuerte que recommended y recommended más fuerte que
  optional mediante una policy cerrada y testeada.
- Interpretar `Platforms` como notas portables por plataforma sin imponer una
  lista universal de sistemas operativos.
- No inspeccionar ni modificar el host, instalar tools/packs, ejecutar comandos
  o guardar secretos durante coverage.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 27 es el siguiente: Security Coverage debe cubrir categorías de
  seguridad explícitas y relevantes al dominio.
- No adelantar clasificación experimental/legacy ni compiler orchestration.

## Step 27 — Security Coverage v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `security-coverage-v1` con categorías explícitas para input validation,
  authentication, authorization, secrets, dependencies, data protection,
  secure defaults, threat awareness y supply chain.
- Requirements seleccionados por el dominio y supports limitados a Concepts
  compatibles con el target curricular.
- `security-evidence-v1` con exigencia de bundle ready sin caveats, freshness,
  Claim security/current, confidence mínima, ausencia de conflictos y respaldo
  multi-source con al menos una fuente primary.
- Evidence conocida pero insuficientemente verificada queda como support
  rechazado y nunca satisface la dimensión blocking de security.
- Bridge al Coverage Engine únicamente para supports que pasan la policy.
- Tests de categorías domain-specific, multi-source verificado, single-source,
  bundle caveated, integración con Coverage y repetibilidad.
- Contrato documentado en `docs/architecture/security-coverage-v1.md`.

### Decisions

- Mantener las categorías domain-driven; security es obligatorio cuando es
  relevante, no una checklist universal aplicada sin contexto.
- Dar a security una policy más estricta que production: fresh, sin caveats,
  current, confidence >= 0.8 y corroboración por dos fuentes.
- Consumir exclusivamente verification metadata congelada por I-03, sin
  ejecutar discovery, fetch o verificación live desde el compiler.
- Preservar evidence rechazada en el diagnóstico en vez de descartarla o
  convertirla en coverage.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 28 es el siguiente: transferir status temporal de Claims a metadata
  separada para Concepts y lessons.
- No implementar GuidanceType ni hierarchy building durante el Paso 28.

## Step 28 — Current / Experimental / Legacy Classification v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `temporal-classification-v1` con status current, preview, experimental,
  legacy, historical y deprecated derivados de Claims y autoridad temporal.
- Precedencia conservadora para evidence mixta y override explícito cuando el
  status declarado contradice la evidencia verificada.
- Metadata UI-ready para primary recommendation, separación experimental y
  uso exclusivamente contextual, sin acoplar UI al algoritmo.
- Clasificación de Concepts por evidence propia y de lessons mediante Concepts
  incluidos más evidence directa opcional.
- Reconocimiento de deprecation Claims y de evidence sostenida únicamente por
  fuentes historical/archived, aunque el status declarado fuera current.
- Declaración contextual explícita para material legacy/historical y warning
  estructurado cuando falta.
- Tests de los seis status, lesson mixta, no promoción silenciosa de historical
  y determinismo ante input reordenado.
- Contrato documentado en `docs/architecture/temporal-classification-v1.md`.

### Decisions

- Emitir un artifact separado en vez de mutar Concepts o añadir prematuramente
  status durable a LessonSpec antes del hierarchy builder.
- Usar precedencia `deprecated > historical > legacy > experimental > preview
  > current` para que evidence riesgosa nunca se presente como default.
- Tratar current como primary recommendation, experimental/preview como
  separado y legacy/historical como context-only.
- Reservar GuidanceType y la distinción recommended/acceptable para el Paso 29.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 29 es el siguiente: mapear status y Claims a GuidanceType con evidence
  refs en cada clasificación.
- No implementar hierarchy building ni compiler pipeline.

## Step 29 — Best Practice vs Historical Guidance v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `guidance-classifier-v1` con GuidanceType `recommended_current`,
  `acceptable_current`, `legacy_maintenance`, `historical_context`, `avoid` y
  `experimental`.
- Mapping cerrado desde el artifact temporal del Paso 28, sin inferencias por
  keywords ni relectura de contenido externo.
- Distinción entre current respaldado por Claim recommendation y behavior
  current meramente aceptable.
- Evidence refs obligatorias y preservadas en cada clasificación para Concepts
  y lessons.
- CurrentGuidanceFindings para deprecated y para legacy/historical que carecen
  de declaración contextual, compatibles con Gap Scanner.
- Tests de los seis GuidanceType, material contextual, ausencia de alternativa
  actual, evidence mapping y determinismo por orden de input.
- Contrato documentado en
  `docs/architecture/current-vs-historical-guidance-v1.md`.

### Decisions

- Exigir un Claim `recommendation` explícito para `recommended_current`; status
  current por sí solo produce `acceptable_current`.
- Mapear preview y experimental al mismo GuidanceType separado conservando su
  status temporal original en la clasificación.
- No emitir missing-current para historia/mantenimiento declarados
  intencionalmente como contextuales; sí hacerlo para deprecated siempre.
- Entregar CurrentGuidanceFindings al scanner existente sin ejecutar todavía
  el compiler pipeline del Paso 31.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 30 es el siguiente: construir la hierarchy visible de forma
  determinista sin convertirla en la verdad pedagógica.
- No adelantar compiler orchestration ni reviewer.

## Step 30 — Curriculum Hierarchy Builder v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `curriculum-hierarchy-builder-v1` como proyección UX determinista desde el
  DAG atómico hacia `Phase -> Module -> Lesson -> Topic`.
- Agrupación por dificultad, área de competencia, competencia primaria y
  contexto de práctica opcional, con orden topológico estable de Concepts.
- IDs de nodos derivados de las claves semánticas completas mediante SHA-256,
  independientes del orden de entrada y de los títulos visibles.
- Preservación explícita de Concepts sin competencia bajo un grupo de soporte,
  sin eliminar ni duplicar unidades granulares.
- Separación contractual entre hierarchy y prerequisite graph; los edges
  pueden cruzar cualquier límite visible sin ser copiados ni reinterpretados.
- Tests de determinismo ante inputs reordenados, edges cross-module y una
  fixture de 2.000 Concepts sin límites artificiales.
- Contrato documentado en
  `docs/architecture/curriculum-hierarchy-builder-v1.md`.

### Decisions

- Usar dificultad solo para la fase visible y mantener el orden pedagógico de
  cada Topic según la posición topológica del grafo.
- Elegir de forma estable la primera pareja área/competencia cuando un Concept
  pertenece a varias competencias; la decisión es únicamente de presentación.
- Tratar el contexto de práctica como hint inerte del pack, sin generar prose,
  ejercicios, assessments ni runtime I-05.
- No imponer máximos de nodos ni tamaño de Topic.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `git diff --check`.

### Notes for next session

- El Paso 31 es el siguiente: orquestar los passes existentes y registrar
  hashes, versiones, diagnósticos y duración sin volverlos dependientes de UI.
- No implementar todavía la decisión de publicación del reviewer del Paso 32.

## Step 31 — Curriculum Compiler Pipeline v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `curriculum-compiler-v1` como orquestador application-layer de 19 trazas
  desde validación de input hasta el artifact compilado.
- Ejecución de los servicios reales para goal decomposition, competency
  matrix, candidates, atomization, granularity, prerequisites, vocabulary,
  graph, hierarchy, coverage, audits, temporal guidance y gaps.
- `CurriculumCompileRequest` explícito para identidad de output, evidence
  congelada y todos los inputs domain/pack-authored necesarios por los passes.
- Comprobación uno-a-uno entre Source Bundle refs y evidence sets, sin ningún
  puerto de discovery, fetch o refresh.
- `CompilationPass` con nombre, versión, hashes SHA-256, warnings, errors y
  duración; los fallos retornan la traza parcial con causa preservada.
- Serialización textual validada de IDs, versiones y timestamps para hashes
  canónicos JSON que preservan identidad en lugar de campos opacos vacíos.
- `CompilationDiagnostics` tipado con artifacts intermedios para revisión y
  copia defensiva completa en el repository in-memory.
- Tests de orden completo de passes, hashes repetibles, artifact estable y
  registro del pass fallido.
- Contrato documentado en
  `docs/architecture/curriculum-compiler-v1.md`.

### Decisions

- Ejecutar gap scanning después de temporal/guidance classification porque el
  scanner consume `CurrentGuidanceFindings`; la dependencia queda explícita.
- Mantener la duración como observación fuera del hash de contenido para que
  no rompa reproducibilidad.
- Reservar inicialmente `final-review` como integración tipada; el Paso 32 lo
  conectó al reviewer real y retiene su decisión dentro de los diagnósticos.
- Permitir que coverage y audits produzcan diagnósticos sin abortar la
  compilación: el reviewer es quien decide approved/warnings/rejected.

### Verification

- `go test ./... -count=1`.
- `go vet ./...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 32 es el siguiente: evaluar todos los diagnósticos de publicación con
  severidades y una decisión determinista.
- El advisor futuro debe ser opcional y no puede cambiar la decisión core.

## Step 32 — Curriculum Reviewer v1

Status: completed
Date: 2026-09-12
Release: unreleased

### Delivered

- `curriculum-reviewer-v1` como gate determinista previo a publicación con
  resultados `approved`, `approved_with_warnings` y `rejected`.
- Las once dimensiones obligatorias: coverage, granularity, prerequisites,
  definition-before-use, zero-assumption, source readiness, freshness,
  security, production, toolchain y temporal status.
- Findings estructurados con severity, code, target y reason, ordenados de
  forma canónica y agregados por dimensión.
- Rechazo para coverage missing/partial, Concepts no atómicos o inalcanzables,
  prerequisitos faltantes, audits bloqueantes, sources no ready/conflicted,
  freshness stale/unknown y guidance temporal insegura.
- Warnings para diagnósticos de granularidad, Sources ready con caveats o
  aging, y contenido preview/experimental correctamente separado.
- Puerto opcional `CurriculumReviewAdvisor` que solo adjunta notas y no puede
  cambiar findings ni la decisión core; ausencia o fallo no bloquea review.
- Integración del reviewer core sin advisor en el pass `final-review` del
  compiler, conservando decisiones rejected como artifacts inspeccionables.
- Tests de aprobación completa, warnings, rechazo, repetibilidad y aislamiento
  de autoridad del advisor.
- Contrato documentado en
  `docs/architecture/curriculum-reviewer-v1.md`.

### Decisions

- Evaluar coverage general separada de las dimensiones dedicadas production,
  security y toolchain para que cada contrato tenga visibilidad propia.
- Tratar partial coverage como bloqueante para publicación, no como un promedio
  aceptable entre dimensiones.
- Confiar en los artifacts versionados del compiler y metadata congelada de
  I-03 sin reejecutar trust/freshness ni hacer network.
- Mantener advisor notes fuera del cálculo de decisión para preservar
  determinismo y evitar una dependencia de IA.

### Verification

- `go test ./... -count=1`.
- `go vet ./...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 33 es el siguiente: implementar Beginner Simulation sobre el graph y
  la hierarchy compilados.
- No implementar Expert Review ni Source Review Contract durante el Paso 33.

## Step 33 — Beginner Simulation v1

Status: completed
Date: 2026-09-13
Release: unreleased

### Delivered

- `beginner-simulation-v1` como recorrido determinista de una persona sin
  conocimiento implícito sobre el orden topológico del graph compilado.
- Estado explícito de Concepts, vocabulario, herramientas y supuestos ya
  introducidos, con trazabilidad de cada Concept hacia su Topic visible.
- `BeginnerGap` tipado para prerequisitos, vocabulario, herramientas y
  supuestos usados antes de su introducción o resolución.
- Inputs pack-authored explícitos para tool uses y assumptions, sin inferencia
  por keywords, ejecución de herramientas ni dependencia de LLM.
- Integración como pass previo a final review y dimensión bloqueante
  `beginner_simulation` del Curriculum Reviewer.
- Copia defensiva del artifact en el repository in-memory y tests con
  curriculum intencionalmente roto, curriculum corregido y orden remezclado.
- Contrato documentado en
  `docs/architecture/beginner-simulation-v1.md`.

### Decisions

- Recorrer siempre `KnowledgeGraphCompilation.TopologicalOrder`; la hierarchy
  solo aporta ubicación UX y nunca redefine el orden pedagógico.
- Comprobar necesidades antes de introducir el Concept actual, por lo que un
  Concept no puede resolver silenciosamente una necesidad que ya usa.
- Tratar el vocabulary baseline como la única excepción explícita de
  conocimiento previo y exigir declaraciones equivalentes para tools y
  assumptions.
- Rechazar cualquier beginner gap en publicación, conservando el artifact
  compilado y sus razones para inspección.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 34 es el siguiente: Expert Coverage Review contra outcomes
  profesionales, competency depth y coberturas production/security/toolchain.
- No implementar todavía Pack Versioning Policy ni Source Review Contract.

## Step 34 — Expert Coverage Review v1

Status: completed
Date: 2026-09-13
Release: unreleased

### Delivered

- `expert-coverage-review-v1` como revisión determinista de que la ruta alcance
  los outcomes profesionales declarados y no termine solo en fundamentos.
- Policy capability-aware para explain, build, debug, operate y maintain, sin
  asumir que todos los niveles avanzados son intercambiables.
- Verificación de profundidad Concept/competency: introductory, foundational,
  intermediate o advanced según el nivel esperado.
- Findings tipados `missing_advanced_competency`, `insufficient_depth` y
  `missing_production_capability` con dimensión production/security/toolchain.
- Gate obligatorio de las tres coberturas de realidad profesional cuando el
  goal declara un `ProfessionalRole`.
- Puerto opcional `ExpertCoverageAdvisor` cuyas notas no pueden modificar el
  resultado core, más tests de ruta completa, rota y determinismo.
- Integración como pass del compiler, dimensión bloqueante del reviewer y
  artifact copiado defensivamente por el repository in-memory.
- Contrato documentado en
  `docs/architecture/expert-coverage-review-v1.md`.

### Decisions

- Exigir `operate` explícito para un outcome operativo; design o explain no lo
  satisfacen por una falsa ordenación numérica.
- Pedir al menos un Concept con profundidad suficiente por competency sin
  imponer cantidad de Concepts, módulos o lessons.
- Aplicar el análisis de outcome/depth a todo goal y reservar el gate conjunto
  production/security/toolchain para rutas que declaran rol profesional.
- Mantener el adapter experto sin autoridad sobre findings o publicación y sin
  volver IA un requisito.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 35 es el siguiente: Pack Versioning Policy v1 independiente de la
  versión de Kelyro, incluida la semántica 0.x/prerelease.
- No implementar todavía change detector, migration planner ni pack upgrade.

## Step 35 — Learning Pack Versioning Policy v1

Status: completed
Date: 2026-09-13
Release: unreleased

### Delivered

- `pack-versioning-policy-v1` como servicio determinista que clasifica todos
  los `CurriculumChange` antes de aceptar una nueva versión del pack.
- Impactos cerrados PATCH, MINOR y MAJOR separados de la transición SemVer real
  y agregados conservadoramente por el cambio de mayor impacto.
- Mapping de source/metadata, expansión compatible, Concept identity breaks y
  migration classes hacia una clasificación explicable por cambio.
- Validación del incremento exacto siguiente, reset de componentes inferiores,
  precedencia creciente y rechazo de versiones iguales, regresivas, saltadas,
  sobre-versionadas o diferenciadas solo por build metadata.
- Semántica explícita para breaking changes durante `0.x`: impacto MAJOR
  conservado y transición al siguiente minor, con `1.0.0` permitido como nueva
  línea estable cuando corresponda.
- Iteraciones prerelease inmutables y crecientes sobre una core release line ya
  clasificada, incluidas promotion a beta/RC/stable.
- Tests de PATCH/MINOR/MAJOR, overrides por migration, `0.x`, prereleases,
  decisiones inválidas, mayor impacto y determinismo por input reordenado.
- Especificación publicada en `docs/specs/pack-versioning-v1.md`.

### Decisions

- Mantener pack version, Kelyro version, curriculum definition version, schema
  version y Concept IDs como identidades independientes.
- Rechazar over-versioning además de under-versioning para que el número de
  versión comunique fielmente la clasificación completa del release.
- Tratar `requires_student_review` como MAJOR porque la compatibilidad no puede
  suponerse aunque la forma estructural aislada parezca menor.
- No integrar todavía el policy con change detection, migration planning,
  upgrade, catalog, filesystem o Git tags; pertenecen a pasos posteriores.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 36 es el siguiente: Pack Dependency Resolver sobre manifests locales
  y constraints versionados.
- No implementar todavía instalación, activación ni dependencias opcionales.

## Step 36 — Learning Pack Dependency Resolver v1

Status: completed
Date: 2026-09-13
Release: unreleased

### Delivered

- `pack-dependency-resolver-v1` para resolver el graph transitivo requerido
  desde un root manifest y un conjunto local de versiones disponibles.
- Parser y evaluación de constraints AND con SemVer exacto, `=`, `<`, `<=`,
  `>` y `>=`, incluida precedencia correcta de prereleases.
- Selección determinista de la versión compatible más alta con backtracking
  cuando constraints compartidos o cycles invalidan una elección previa.
- Diagnósticos tipados `missing`, `incompatible` y `cycle`, con depender,
  constraint, versiones disponibles y path explicable.
- `InstallOrder` dependency-first que contiene cada pack una sola vez y deja el
  root al final, sin mezclar estas edges con concept prerequisites.
- Tests de graph transitivo, highest-compatible, constraints compartidos,
  backtracking, missing, incompatible, cycle y estabilidad ante input reorder.
- Contrato documentado en
  `docs/architecture/learning-pack-dependency-resolver-v1.md`.

### Decisions

- Tratar todas las dependencies v1 como requeridas; optional dependency queda
  reservado para un schema futuro y nunca se infiere por ausencia.
- Resolver solo manifests ya disponibles: catalog discovery, network,
  instalación y activación no pertenecen a este servicio puro.
- No devolver una selección parcial en failure para evitar que un installer
  futuro confunda un graph incompleto con un plan ejecutable.
- Usar el texto completo de versión únicamente como tie-break determinista
  entre anomalías de igual precedencia SemVer por build metadata.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 37 es el siguiente: completar Environment Pack Format v1 y su parser
  estricto dentro de `learning-pack/v1`.
- No integrar todavía Doctor ni auto-instalar herramientas.

## Step 37 — Environment Pack Format v1

Status: completed
Date: 2026-09-13
Release: unreleased

### Delivered

- Schema estricto `environment-pack/v1` con ID/version independientes,
  platform support y tool requirements separados del curriculum conceptual.
- Metadata de tool para display name, minimum version, nivel
  required/recommended/optional, `introduced_at`, `when_needed`, plataformas,
  evidence e install guidance refs.
- Registros de instalación por plataforma con source name, instrucciones,
  query-free HTTPS official URL y evidence refs.
- Validación cross-document de Concepts, Source Bundles, plataformas y guidance
  refs desde el loader seguro de `learning-pack/v1`.
- Separación entre validación básica y portable estricta para que Toolchain
  Coverage pueda reportar metadata incompleta sin aceptar packs publicables
  incompletos.
- Toolchain Coverage ampliado para display, minimum version, when-needed y
  guidance, con copias defensivas de los nuevos campos.
- Tests positivos y negativos de unknown auto-install fields, URL con
  credentials/query, plataforma inválida, timing faltante y guidance incompleta.
- Especificación en `docs/specs/environment-pack-v1.md`.

### Decisions

- Usar `linux`, `darwin` y `windows` como vocabulario portable cerrado; macOS
  se representa con el identificador runtime de Go `darwin`.
- No aceptar command candidates, version args, package-manager commands ni
  secrets desde un pack; Doctor solo podrá usar su registry confiable.
- Exigir evidence tanto para el tool requirement como para la metadata de su
  fuente oficial de instalación.
- Mantener URLs como metadata inerte que ninguna capa abre o ejecuta
  automáticamente.

### Verification

- `go test ./internal/curriculum/... ./internal/infra/learningpack -count=1`.
- `go vet ./internal/curriculum/... ./internal/infra/learningpack`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 38 es el siguiente: proyectar Environment Pack y posición curricular
  a un Doctor context-aware con reasons y official guidance.
- No implementar instalación automática, pack activation ni Student writes.

## Step 38 — Curriculum-aware Doctor v1

Status: completed
Date: 2026-09-13
Release: unreleased

### Delivered

- `environment-doctor-planner-v1` como proyección determinista de Environment
  Pack, plataforma, graph, hierarchy y Concept actual a un plan diagnóstico.
- Estados tipados `current`, `future` y `not_needed_yet`, con rechazo de tools
  requeridos antes de su `introduced_at` y orden canónico por tool ID.
- Contexto con nivel required/recommended/optional, versión mínima, explicación,
  módulo/fase de necesidad y guidance oficial específico de plataforma.
- Doctor ampliado con estado no bloqueante `deferred`: tools futuros permanecen
  visibles, pero no se resuelven ni ejecutan antes de ser necesarios.
- Validación de minimum version para tools actuales con precedencia SemVer,
  incluida semántica prerelease y failure cuando la versión no puede probarse.
- Frontera segura para tools desconocidos: un requirement actual falla como
  diagnóstico no disponible y uno futuro queda deferred; ninguno ejecuta datos
  del pack.
- Adaptador de aplicación desde `EnvironmentDoctorPlan` hacia `doctor.Context`
  y render de reason, source e install guidance en CLI/TUI.
- Contrato documentado en
  `docs/architecture/curriculum-aware-doctor-v1.md`.

### Decisions

- Usar `KnowledgeGraphCompilation.TopologicalOrder` para timing pedagógico y la
  hierarchy solo para nombres de phase/module mostrados al usuario.
- Tratar `introduced_at` y `when_needed` como hitos distintos: una tool puede
  estar enseñada pero seguir siendo futura antes del módulo que la necesita.
- No permitir command candidates, version args, scripts o package-manager
  commands desde Environment Packs; solo el registry confiable puede probar.
- Mantener official URLs e instrucciones como metadata inerte, nunca abierta o
  ejecutada automáticamente.
- Recibir un plan explícito en application; la resolución del pack activo se
  reserva al Paso 39 y no se anticipa aquí.

### Verification

- `go test ./internal/curriculum/... ./internal/doctor ./internal/app ./internal/cli ./internal/tui -count=1`.
- `go vet ./internal/curriculum/... ./internal/doctor ./internal/app ./internal/cli ./internal/tui`.
- `go test -race ./internal/curriculum/application/... ./internal/doctor -count=1`.
- `go test ./... -count=1`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 39 es el siguiente: instalación inmutable y activación por workspace
  de packs ya validados.
- No implementar todavía catalog, upgrade, auto-install de tools ni I-05.

## Step 39 — Learning Pack Installation and Activation v1

Status: completed
Date: 2026-09-14
Release: unreleased

### Delivered

- `pack-installation-policy-v1` con validación previa, resolución completa de
  dependencias locales e instalación idempotente por hash.
- Snapshot ZIP canónico generado desde las entradas ya validadas y content hash
  estable derivado del `checksums.txt` estricto del pack.
- Repositorio filesystem global con staging privado, revalidación antes del
  rename atómico, versiones inmutables y detección de corrupción en cada read.
- Activación por workspace mediante una referencia ID/version/hash en
  `.kelyro/state/active-pack.json`, sin duplicar el archive global ni tocar
  Student Core.
- Comandos `packs install`, `packs list`, `packs show` y `packs activate`, con
  discovery Foundation para la activación y soporte de `--workspace`.
- Helpers de paths cross-platform y contrato documentado en
  `docs/architecture/learning-pack-installation-v1.md`.

### Decisions

- Usar el directorio global de configuración de Foundation para datos
  persistentes de packs y reservar el global cache para el catálogo del Paso
  40; ninguna versión instalada es disposable.
- Requerir que dependencias v1 ya estén instaladas y sean compatibles; el
  installer no consulta catálogos, no descarga y no auto-instala.
- Persistir una instantánea canónica producida durante validation para cerrar
  la ventana TOCTOU entre leer un source directory/ZIP y almacenarlo.
- Mantener activación fuera de SQLite como referencia workspace-local pequeña;
  el pack completo permanece global y la referencia se escribe atómicamente.
- No ejecutar archivos, scripts o install guidance y no modificar mastery,
  evidence ni curriculum instances del estudiante.

### Verification

- `go test ./... -count=1`.
- `go vet ./...`.
- Tests de installer para dependencias, idempotencia, conflicto inmutable,
  validación fallida, orden de versiones y aislamiento por workspace.
- Tests de CLI para install/list/show/activate y de paths cross-platform.
- `git diff --check`.

### Notes for next session

- El Paso 40 es el siguiente: modelo, source abstraction, cache offline y
  búsqueda/listado de metadata del Pack Catalog v1.
- Catalog metadata nunca autoriza instalación ni activa packs automáticamente.

## Step 40 — Learning Pack Catalog / Marketplace v1

Status: completed
Date: 2026-09-14
Release: unreleased

### Delivered

- Modelo `pack-catalog/v1` con identidad, descripción, domain/target,
  maintainer, versiones disponibles, temporal status, compatibility, source y
  trust metadata cerrada.
- `PackCatalogSource` separado de `PackCatalogCache`; metadata disponible nunca
  equivale a pack validado, instalado o confiable para ejecución.
- Compatibilidad recalculada localmente contra la versión SemVer de Kelyro y
  estado `unknown` explícito para builds development/unknown.
- Servicio cache-first/fallback que conserva el último snapshot válido y usa
  modo offline ante source ausente, inválido o no disponible.
- Búsqueda determinista, case-insensitive y multi-token sobre ID, nombre,
  descripción, domain, target, maintainer y source, con ranking explicable.
- Adaptador JSON estricto/bounded y cache global privado con writes atómicos.
- Comandos `packs catalog` y `packs search <query>` sin descarga ni instalación
  automática.
- Contrato documentado en `docs/specs/pack-catalog-v1.md`.

### Decisions

- No definir un protocolo remoto ni ecosistema comunitario en v1; el source
  transport-neutral permite conectarlo después sin mezclarlo con plugins.
- Tratar locator de source y artifact como metadata inerte: ni CLI ni service
  lo abre, descarga o ejecuta.
- No confiar en la compatibility publicada; se deriva de
  `minimum_kelyro_version` y la versión local.
- Una cache inexistente representa un catálogo offline vacío válido, no un
  error que impida usar otros comandos de packs.
- Mantener trust status independiente de pack temporal status y de validación
  criptográfica futura.

### Verification

- `go test ./internal/curriculum/... ./internal/infra/packcatalogfs ./internal/cli -count=1`.
- `go vet ./internal/curriculum/... ./internal/infra/packcatalogfs ./internal/cli`.
- Tests de refresh/cache, fallback offline, compatibilidad, search ranking,
  roundtrip atómico, documento desconocido/trailing y CLI catalog/search.
- `git diff --check`.

### Notes for next session

- El Paso 41 es el siguiente: clasificar old/new curriculum con señales
  explícitas de I-03 Drift/Impact y mappings declarados para split/merge.
- El clasificador no aplicará migraciones ni modificará Student Core.

## Step 41 — Curriculum Change Classification v1

Status: completed
Date: 2026-09-14
Release: unreleased

### Delivered

- `curriculum-change-classifier-v1` sobre definitions old/new válidas del mismo
  curriculum, con output versionado, orden estable y change IDs deterministas.
- Detección de `metadata_only`, `source_refresh`, Concept add/remove,
  prerequisite, hierarchy, status y environment changes.
- `ConceptIdentityMapping` obligatorio para clasificar split/merge; sin mapping
  se conservan add/remove independientes y no se inventa continuidad.
- Clases `safe`, `requires_recompile`, `requires_student_review` y `breaking`
  asignadas conservadoramente por dimensión.
- Integración explícita con Drift/Impact I-03 validados, incluida pertenencia de
  Source Bundles, relación Impact→Drift, affected Concept refs y elevación de
  severidad por recommended action.
- Separación entre prerequisite structure y evidence refresh para no reportar
  cambios de graph por una actualización de evidencia únicamente.
- Contrato documentado en
  `docs/architecture/curriculum-change-classifier-v1.md`.

### Decisions

- Nunca inferir split/merge por similitud textual o evidence overlap; una
  mapping declarada y razonada es necesaria para cambios de identidad.
- Agregar un record por change kind con Concept IDs ordenados, evitando que el
  input order produzca diagnósticos o IDs distintos.
- Clasificar removals y status changes como student review sin tocar mastery;
  Step 42 decidirá las acciones de migration.
- Tratar hierarchy y environment como recompilación compatible, no como pérdida
  de progreso, y conservar source refresh como safe salvo señales I-03 más
  severas.
- No aplicar Pack SemVer aquí: el policy del Paso 35 consume los changes ya
  clasificados y mantiene ambas responsabilidades separadas.

### Verification

- `go test ./internal/curriculum/... -count=1`.
- `go vet ./internal/curriculum/...`.
- `go test -race ./internal/curriculum/application/... -count=1`.
- Tests de todas las dimensiones, add/remove sin inferencia, split/merge
  explícitos, input reorder, Drift/Impact, severidad y metadata-only safe.
- `git diff --check`.

### Notes for next session

- El Paso 42 es el siguiente: producir un Student-safe Curriculum Migration
  Plan a partir de estos changes, sin aplicar todavía upgrade ni Student writes.
- Split/merge requerirán mappings explícitos y nunca derivarán mastery.

## Step 42 — Student-safe Curriculum Migration Plan v1

Status: completed
Date: 2026-09-14
Release: unreleased

### Delivered

- `CurriculumMigrationPlan` versionado, validado y con ID determinista para dos
  versiones inmutables del mismo curriculum.
- Acciones explícitas `preserve_state`, `initialize_unknown`,
  `preserve_historical`, `split_no_transfer` y `merge_no_transfer`.
- Preservación de mastery/evidence para stable Concept IDs, incluida la
  conservación histórica de conceptos deprecated, legacy o historical.
- Inicialización unknown para conceptos añadidos, sin fabricar exposición,
  mastery ni evidencia.
- Split/merge fail-closed: requieren exactamente los mappings declarados al
  clasificador, preservan evidencia histórica y no transfieren mastery.
- Señal explícita para recalcular unlock eligibility cuando cambia el grafo de
  prerequisitos; los cambios de jerarquía conservan el estado por stable ID.
- Planner read-only de upgrades locales: descubre la versión instalada más
  reciente, vuelve a validarla mediante el repository, clasifica el diff,
  verifica SemVer y construye el migration plan.
- Comando `kelyro packs upgrade <id> --dry-run`, con target opcional
  `<id>@<version>` y resumen explícito de impacto sin writes.
- Contrato documentado en
  `docs/architecture/curriculum-migration-plan-v1.md`.

### Decisions

- No inferir continuidad por título, texto, orden o similitud: solo el stable
  Concept ID o un mapping split/merge explícito participan en el plan.
- Conservar el plan learner-neutral; I-02 será el único dueño de su aplicación
  mediante un application service en el Paso 43.
- Descubrir solo versiones ya instaladas e inmutables. El catálogo sigue siendo
  metadata no confiable y nunca dispara instalación automática.
- Rechazar una transición SemVer que no corresponde al impacto clasificado
  antes de ofrecer cualquier aplicación.

### Verification

- `go test ./internal/curriculum/... ./internal/cli ./cmd/kelyro -count=1`.
- Tests de stable IDs, additions, deprecation, removal, hierarchy,
  prerequisite recalculation, split explícito, ausencia de mapping,
  determinismo, discovery local, SemVer y CLI dry-run.
- `git diff --check`.

### Notes for next session

- El Paso 43 aplicará el plan con backup, confirmación, un application service
  I-02 transaccional, integrity check, rollback/recovery y audit.
- La instancia anterior seguirá siendo histórica; no se reescribirán evidence
  facts ni packs publicados.

## Step 43 — Safe Learning Pack Upgrade v1

Status: completed
Date: 2026-09-14
Release: unreleased

### Delivered

- `pack-upgrade-policy-v1` compone discovery local, revalidación del artifact,
  diff, SemVer, migration plan, backup, confirmación, aplicación I-02,
  activación, integrity check y audit.
- El CLI permite `kelyro packs upgrade <id>` con confirmación interactiva o
  `--yes`; `--dry-run` conserva el preview sin backup ni writes.
- Backup Foundation obligatorio con `backup.retention` resuelto desde la
  configuración efectiva del workspace antes de cualquier aplicación.
- `CurriculumInstanceService.Migrate` de I-02 instala la definición nueva y,
  dentro de una única Unit of Work, crea la instancia target, transfiere solo
  estados de stable IDs, inicializa nuevos targets unknown y archiva la
  instancia source sin borrar su estado histórico.
- Adapter `learningmigration` explícito entre I-04 e I-02, con proyección
  determinista al contrato `curriculum-consumption/v1`; no existe SQL en los
  servicios de curriculum o aprendizaje.
- Unlock eligibility se recalcula naturalmente contra el grafo de la nueva
  definición; no se copia ningún unlock cache, retention o review schedule.
- Chequeo posterior mediante el validador SQLite read-only: `quick_check`,
  migrations, foreign keys e invariantes Student Core.
- Eventos auditables `pack.upgrade.completed` y `pack.upgrade.failed` con
  versiones, plan, backup, conteos, etapa fallida y estado de recovery.
- Restore automático del backup ante fallos de migration, activation,
  integrity o audit; el backup contiene tanto `learning.db` como la referencia
  `state/active-pack.json`.
- Contrato documentado en `docs/architecture/pack-upgrade-v1.md`.

### Decisions

- V1 descubre únicamente versiones inmutables ya instaladas. Un catalog entry
  no es contenido confiable y el upgrade nunca instala automáticamente desde
  red; `packs install` sigue siendo el paso explícito de adquisición.
- No rebind de una instancia existente: la antigua queda archived y auditable,
  y la nueva tiene identidad propia. Los facts append-only no se reasignan.
- Un target preexistente para el mismo goal hace fail closed en lugar de
  asumir que una migración anterior fue completa.
- Toda aplicación requiere confirmación, incluso patch; breaking split/merge
  exige además mappings explícitos en el request y jamás transfiere mastery.
- La proyección de conceptos reutiliza definición/dificultad/estado del pack y
  colapsa prerequisitos al contrato I-02, prefiriendo `mastered` sobre
  `introduced`; no genera lesson, practice, assessment ni project runtimes.

### Verification

- `go test ./... -count=1`.
- `go vet ./...`.
- `go test -race ./internal/curriculum/application/... ./internal/learning/application/... ./internal/infra/learningmigration ./internal/cli -count=1`.
- Tests de patch, addition, split explícito sin mastery, confirmación, backup,
  activation, audit, projection I-04→I-02, migración transaccional, state
  preservation, unknown initialization y rollback por fallo inyectado.
- `git diff --check`.

### Notes for next session

- El Paso 44 es el siguiente: persistir reproducibility metadata completa del
  compiler y pack sin reabrir la semántica de upgrade.
- No implementar I-05 ni añadir descarga automática de packs.

## Step 44 — Curriculum Build Reproducibility Metadata v1

Status: completed
Date: 2026-09-14
Release: unreleased

### Delivered

- Contrato `curriculum-build-info/v1` con versión del compiler, lista ordenada
  de pass/version, Source Bundle IDs/hashes, config completa, pack schema,
  input/output hashes y timestamp UTC determinista.
- El compiler adjunta metadata sólo al resultado exitoso y la valida contra
  los hashes y versiones reales de su trace; los records persistibles la
  requieren explícitamente.
- Learning Pack v1 declara `build_info_entry`; el loader JSON estricto valida
  el documento y cruza schema y Source Bundles con manifest/curriculum.
- `kelyro curriculum build-info` inspecciona el pack activo del workspace sin
  compilar, usar red o modificar Student Core.
- Contrato documentado en
  `docs/architecture/curriculum-reproducibility-v1.md` y enlazado desde el
  índice y las especificaciones relacionadas.

### Decisions

- Reutilizar el timestamp inmutable de build en vez del reloj de ejecución para
  conservar determinismo completo.
- Excluir durations observacionales de los hashes; input/output y pass
  versions sí quedan vinculados y validados.
- Mantener build info opcional en valores de dominio parciales o fixtures, pero
  obligatorio para resultados persistidos y packs portables validados.
- Limitar la CLI de este paso a `build-info`; compile/validate/coverage/gaps y
  audit siguen reservados al Paso 47.

### Verification

- `go test ./internal/curriculum/... ./internal/infra/learningpack ./internal/cli ./cmd/kelyro -count=1`.
- Tests de determinismo del compiler, decode JSON estricto, cruce de bundles y
  render del build info del pack activo.
- `git diff --check`.

### Notes for next session

- El Paso 45 es el siguiente: generar el reporte humano/machine-readable de
  evidencia a partir del resultado y evidence sets ya congelados.
- El reporte conservará citations/identidades y no copiará cuerpos externos.

## Step 45 — Curriculum Evidence Report v1

Status: completed
Date: 2026-09-14
Release: unreleased

### Delivered

- Modelo validado `curriculum-evidence-report/v1` para goal, competencias,
  conteos, bundles/Claims, cobertura primaria, freshness, conflictos/caveats,
  contenido temporal, gaps y versiones del compiler.
- Reporter determinista que consume exclusivamente el `CompilationResult` y
  los `CurriculumEvidenceSet` congelados que corresponden a sus Source Bundles.
- Policy `primary-source-coverage-v1`, calculada por Claim referenciado con al
  menos un Source ID de rol primario dentro del bundle exacto.
- Proyección Markdown determinista destinada a `EVIDENCE.md`, sin statements
  de Claims, excerpts, cuerpos web ni transcripciones externas.
- Evidence JSON portable ampliado y decodificado estrictamente; el loader lo
  cruza con curriculum, build info y referencias de evidencia utilizadas.
- Contrato documentado en
  `docs/architecture/curriculum-evidence-report-v1.md`.

### Decisions

- Representar evidencia externa mediante IDs/hashes/roles/freshness y no
  duplicar prosa externa en el reporte.
- Medir cobertura primaria sobre Claims únicos realmente referenciados por el
  curriculum, no sobre el número bruto de sources del bundle.
- Mantener el reporter separado del compiler: recibe un resultado ya válido y
  puede generar JSON/Markdown para build/export sin alterar sus hashes.
- Reservar la inclusión física de `EVIDENCE.md` y los checks copyright del
  archive para el Pack Builder del Paso 46.

### Verification

- `go test ./internal/curriculum/... ./internal/infra/learningpack -count=1`.
- Tests de salida repetible, cobertura primaria, ausencia de Claim statements,
  evidencia no coincidente y carga estricta del reporte portable.
- `git diff --check`.

### Notes for next session

- El Paso 46 construirá el archive distribuible, incorporará JSON y
  `EVIDENCE.md`, y rechazará patrones de retención externa prohibidos.
- No añadir scripts, contenido I-05, cache bodies ni descarga de sources.

## Step 46 — Copyright-aware Learning Pack Builder v1

Status: completed
Date: 2026-09-14
Release: unreleased

### Delivered

- `PackBuildService` y adapter in-memory determinista que serializa manifest,
  curriculum, environment opcional, build info, evidence JSON/Markdown,
  README/licencia, assets autorizados, checksums y ZIP canónico.
- Una citation HTTP(S) obligatoria por source del evidence set congelado;
  excerpts opcionales limitados a 512 bytes y ligados por SHA-256.
- Assets limitados a contenido original declarado Kelyro-authored bajo
  `assets/`, con licencia, copyright holder y ledger hash-bound
  `pack-asset-licenses/v1`.
- `EVIDENCE.md` obligatorio y byte-identical a la proyección canónica del
  reporte machine-readable, evitando agregar texto externo fuera del modelo.
- Pack validation rechaza paths cache/raw/body/snapshot/transcript, formatos de
  documentos completos, filenames de transcript/full article, campos
  estructurados conocidos de retención y assets sin licencia detectable.
- Contrato documentado en
  `docs/architecture/copyright-aware-pack-builder-v1.md`.

### Decisions

- Construir el archive en memoria y devolver bytes portables; escritura a disco
  y comandos build/export quedan fuera de este paso y de las reglas de dominio.
- No copiar automáticamente statements de Claims ni Evidence excerpts. Sólo
  una citation declarada puede incluir un excerpt mínimo, bounded y hasheado.
- Generar todos los documentos core desde valores validados en lugar de aceptar
  blobs YAML/JSON suministrados por el caller.
- Tratar los checks por patrones como una barrera conservadora detectable, no
  como una afirmación automática sobre titularidad o fair use.

### Verification

- `go test ./... -count=1`.
- `go vet ./...`.
- Tests de build repetible, roundtrip por el loader, ausencia de Claim text,
  citation faltante/excerpt oversized, asset externo/sin licencia y patrones
  prohibidos de cache/transcript/body.
- `git diff --check`.

### Notes for next session

- El Paso 47 es el siguiente: exponer compile/validate/coverage/gaps/audit y
  completar la CLI de packs, reutilizando los services ya implementados.
- No implementar todavía I-05 ni publicación/descarga automática.

## Step 47 — Curriculum and Learning Pack CLI

Status: completed
Date: 2026-09-15
Release: unreleased

### Delivered

- Comandos `curriculum compile`, `validate`, `coverage`, `gaps`, `audit` y
  `build-info` sobre el Learning Pack activo del workspace.
- `CurriculumInspectorV1` read-only para estructura portable, receta de
  compilación, cobertura declarada, gaps retenidos, evidencia y grafo de
  prerequisitos.
- Salida humana determinista con identidad pack/curriculum, conteos de
  jerarquía, dimensiones de cobertura, razones de auditoría y links de fuentes.
- Los comandos existentes `packs validate/install/list/show/activate/search`
  y `packs upgrade --dry-run` permanecen conectados a sus application services.

### Decisions

- `curriculum compile` inspecciona y verifica el artefacto compilado inmutable
  activo. No inventa un formato de autoría CLI ni reconstruye inputs de
  atomización, competencias o soportes que el pack portable no conserva.
- Coverage muestra requirements y gaps retenidos; no presenta una
  recomputación falsa sin los `CoverageSupport` usados por el compiler.
- Mantener salida human-readable únicamente: el CLI vigente rechaza `--json`,
  por lo que no existe la convención opcional mencionada por el plan.
- La inspección no usa red, no compila en cada render y no toca Student Core.

### Verification

- `go test ./internal/curriculum/application ./internal/cli ./cmd/kelyro -count=1`.
- `go vet ./internal/curriculum/application ./internal/cli ./cmd/kelyro`.
- Tests de los cinco nuevos renderers, build info, validación de pack, grafo,
  cobertura, gaps, auditorías y evidencia.
- `git diff --check`.

### Notes for next session

- El Paso 48 integrará Pack, Curriculum Overview, Coverage, Gaps, Audit,
  Evidence y estado de upgrade en TUI mediante lecturas explícitas.
- La vista Roadmap seguirá tomando progreso conceptual exclusivamente de I-02.

## Step 48 — Compiled Curriculum and Pack TUI Views

Status: completed
Date: 2026-09-15
Release: unreleased

### Delivered

- Vista TUI `Curriculum & Learning Pack`, accesible con `u`, para Pack,
  Curriculum Overview, Coverage, Gaps, Audit, Sources/Evidence y
  Update/Migration Preview.
- `CurriculumWorkspaceViewV1` compone el pack activo, su inspección y un preview
  de upgrade exclusivamente `dry-run`, degradando de forma explícita cuando no
  hay update o el preview no está disponible.
- Carga y refresh asíncronos fuera del render; `View()` sólo proyecta estado ya
  leído y mantiene viewport/scroll para curriculums extensos.
- El home anuncia la nueva vista y sus golden files cubren terminales small,
  normal y large.
- Roadmap continúa proyectando Phase, Module, Lesson, Topic y concept mastery
  desde el dashboard I-02, sin duplicar ni recalcular estado estudiantil.

### Decisions

- No ejecutar el compiler desde la TUI ni desde cada render. El pack activo es
  un artefacto compilado inmutable y la inspección es read-only.
- Detectar updates sólo entre versiones instaladas localmente; el catálogo no
  descarga, instala ni activa contenido.
- Un fallo del preview no oculta pack/curriculum/evidence ya disponibles; se
  muestra como estado `unavailable` con razón.
- Mantener una única vista scrolleable para el conjunto relacionado en vez de
  introducir navegación profunda prematura antes del reference pack.

### Verification

- `go test ./internal/curriculum/application ./internal/tui ./cmd/kelyro -count=1`.
- `go vet ./internal/curriculum/application ./internal/tui ./cmd/kelyro`.
- Tests de composición, ausencia de update, navegación, workspace resolution,
  carga asíncrona, overview, coverage, gaps, evidence y migration preview.
- Golden tests TUI para anchos 32, 80 y 120.
- `git diff --check`.

### Notes for next session

- El Paso 49 creará `backend-go-reference` como pack de desarrollo verificable,
  con scope limitado por la evidencia fixture disponible y sin claim de
  completitud productiva.
- No imponer un número de conceptos ni introducir contenido I-05.

## Step 49 — Backend Go Reference Learning Pack

Status: completed
Date: 2026-09-15
Release: unreleased

### Delivered

- Artefacto portable `reference-packs/backend-go-reference.zip`, válido como
  `learning-pack/v1`, más fuente/generador determinista en
  `internal/infra/referencepack`.
- Goal profesional con outcomes explain/build/debug/operate/maintain, cuatro
  competency areas y cinco competencias.
- Nueve conceptos atómicos respaldados por la evidencia fixture disponible,
  sin mínimo, máximo ni target artificial de tamaño.
- Root fundacional explícito, ocho edges de prerequisitos, vocabulary graph,
  hierarchy, baseline zero-assumption y beginner simulation.
- Coverage declarada para las ocho dimensiones, incluidos production,
  security y toolchain; el evidence report no retiene bodies ni Claim text.
- Clasificación temporal con ocho conceptos current y un concepto GOPATH
  legacy limitado a contexto histórico.
- Environment Pack declarativo para Linux, macOS y Windows con Go toolchain,
  punto de introducción/uso y guidance oficial sin instalar herramientas.
- Documentación de límites y regeneración en
  `docs/architecture/backend-go-reference-pack.md` y `reference-packs/README.md`.

### Decisions

- Marcar pack `preview` y source policy `optional_for_fixture`: la fixture
  determinista tiene caveat visible y no sustituye Source Bundles productivos
  verificados de I-03.
- No inflar el curriculum a cientos de conceptos sin evidencia. Los nueve
  conceptos existen para cubrir seams reales del compiler; producción puede
  crecer sin límites cuando I-03 lo justifique.
- Hacer que el build falle si el reviewer determinista rechaza el curriculum;
  el caveat esperado produce approval con warnings, no una publicación falsa.
- Usar el copyright-aware builder productivo para generar README, licencia,
  ledger de assets, checksums, build info, evidence JSON/Markdown y ZIP
  canónico.
- Context7 confirmó `/golang/go` como fuente oficial de alta reputación y las
  rutas `https://go.dev/dl/` y `https://go.dev/doc/install`; el pack conserva
  sólo el link oficial y metadata propia.

### Verification

- `go test ./... -count=1`.
- `go vet ./...`.
- `go test ./internal/infra/referencepack -count=1` compara bytes regenerados
  con el ZIP committed, comprueba determinismo/cancelación y valida el archive.
- `go run ./cmd/kelyro packs validate reference-packs/backend-go-reference.zip`
  devuelve `Status: valid` y el warning esperado por status `preview`.
- Context7 `resolve-library-id` y `query-docs` contra `/golang/go` para la ruta
  oficial de instalación del Environment Pack.
- `git diff --check`.

### Notes for next session

- El Paso 50 es el siguiente: E2E desde Source Bundle hasta Curriculum Instance.
- Reemplazar la fixture por evidencia productiva I-03 será trabajo separado;
  no promover este pack preview a production-complete in-place.

## Step 50 — E2E Curriculum Compiler

Status: completed
Date: 2026-09-15
Release: unreleased

### Delivered

- Suite E2E offline que atraviesa el hand-off I-03, ingestión de evidencia,
  compiler versionado, pack build copyright-aware, validación, instalación,
  activación, proyección I-02, Curriculum Instance y roadmap.
- Cobertura explícita de los 16 escenarios del plan: bundle válido, evidencia
  ausente, conflicto crítico, fuente histórica, concepto experimental,
  expansión de prerrequisitos, fallos definition-before-use y zero-assumption,
  gaps production/security, dependencia de pack, Environment Pack, add/split
  upgrades, preservación de mastery y export seguro.
- Fixture I-03 construida sólo con records de dominio deterministas; ninguna
  prueba accede a Internet ni retiene bodies de fuentes.
- `BackendGoDevelopmentFixture` permite al E2E reutilizar el input de referencia
  y sustituir exclusivamente su evidencia sintética por el Source Bundle I-03.
- El gate existente `go run ./tools/quality e2e` incorpora automáticamente la
  suite en Linux, macOS y Windows; los nombres de CI/release ya incluyen
  Curriculum Compiler.

### Bug fixed

- El pipeline enviaba todas las semánticas de prerrequisitos al extractor,
  incluido el caso válido cuyo `RequiredConceptID` sólo estaba disponible en
  `AvailableConcepts`. El extractor lo rechazaba antes de que el pass de
  expansión pudiera incorporarlo.
- El compiler ahora extrae sólo edges entre conceptos ya atomizados y deja las
  semánticas de conceptos disponibles al pass de expansión. La regresión está
  cubierta por unit test y por el escenario E2E 6.
- Commit independiente: `fix(compiler): allow verified prerequisite expansion`
  (`12831e3`).

### Decisions

- Mantener la suite bajo el build tag `e2e` y el gate multi-OS ya existente, sin
  introducir dependencias ni un segundo runner.
- Usar el validator y builder productivos con ZIP real; el repositorio de
  instalación del test es in-memory para no contaminar el store global del
  usuario ni depender del sistema operativo.
- Probar add/split mediante classifier + migration planner productivos y la
  preservación de mastery mediante el application service I-02; no mutar
  mastery desde el compiler.
- Tratar claims, URLs y metadata como inputs; el evidence report exportado no
  incluye statements, bodies ni campos típicos de contenido íntegro.

### Verification

- `go test ./... -count=1`.
- `go test -tags=e2e ./tests/e2e -count=1`.
- `go vet ./...`.
- `go test -race ./internal/curriculum/... ./internal/infra/referencepack ./internal/infra/learningpack ./internal/infra/learningmigration -count=1`.
- `go test -race -tags=e2e ./tests/e2e -run TestCurriculumCompilerAndPackLifecycleEndToEnd -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 51 es el siguiente: performance y scale hardening con la fixture de
  10,000 conceptos y 20,000 edges.
- La fixture E2E continúa siendo deliberadamente offline y no convierte el
  reference pack preview en contenido productivo.

## Step 51 — Performance y scale hardening

Status: completed
Date: 2026-09-15
Release: unreleased

### Delivered

- Fixture determinista y offline con 10,000 conceptos, 20,000 edges
  acíclicos, 1,000 competencias, 100 Source Bundles y las ocho dimensiones de
  coverage.
- Gate integral que compila dos veces, compara curriculum/diagnostics/hashes,
  serializa dos packs byte-identical, valida e instala el ZIP real y comprueba
  sus cardinalidades.
- Índice de critical paths O(V), con un predecesor y longitud por concepto, en
  lugar de retener todos los prefijos de ruta O(V²).
- Validación única O(V+E) del grafo de prerrequisitos ya extraído, expansión
  profunda con stack explícito y regresiones para ciclos y cadenas de 10,000
  conceptos.
- Índice de evidencia construido una vez por compilación y reutilizado por los
  10,000 candidatos de atomización.
- Hashes de trazabilidad enviados por streaming a SHA-256, compatibles byte a
  byte con el hash JSON anterior, sin conservar un segundo buffer completo por
  pass.
- Lectura I-02 de dashboard/roadmap elevada a 10,000 conceptos, sin trasladar
  reglas ni trabajo del compiler a la TUI.
- Benchmarks de compiler y serialización con `-benchmem`, más tiempos por pass
  y contrato operativo en
  `docs/architecture/curriculum-scale-hardening-v1.md`.

### Bug fixed

- El builder podía devolver un Learning Pack que el validator del mismo
  paquete rechazaba: la fixture serializa un curriculum YAML de 9,038,778
  bytes, por encima del antiguo límite por entrada de 4 MiB.
- El límite por entrada sube a 16 MiB, manteniendo el techo total de 32 MiB,
  1,024 entries y ratio ZIP 100:1. `validateEntries` ahora aplica los mismos
  límites en memoria que los loaders directory/ZIP, por lo que builder e
  installer no pueden divergir de nuevo por este motivo.

### Measurements

- Linux `amd64`, Intel i7-12650H: compiler completo ~1.15 s; knowledge graph
  ~80 ms; coverage ~21 ms; definition-before-use ~46 ms; zero-assumption
  ~21 ms; final review ~134 ms.
- Pack serialization ~2.04 s para 10,677,994 bytes; validación + instalación
  ~1.64 s; roadmap/dashboard read ~20 ms.
- Benchmark de una iteración: compiler ~1.14 s y 884,622,928 B/op;
  serialization ~2.05 s y 1,411,015,800 B/op. Son bytes acumulados asignados,
  no memoria residente ni budgets portables; quedan registrados para detectar
  regresiones futuras.

### Decisions

- No fijar un máximo de conceptos, competencias o edges. Las cardinalidades
  son probes de regresión; los límites de bytes/entries sólo protegen el
  boundary de archivos no confiables.
- Preservar el digest existente de `json.Marshal` al cambiar a hashing por
  streaming, evitando invalidar builds reproducibles o packs ya generados.
- Mantener `pathExists` únicamente para edges nuevos de expansión, donde se
  necesita decidir si una propuesta introduce ciclo; el DAG existente se
  valida una sola vez.
- Medir tiempos sin convertir resultados dependientes de la máquina en
  thresholds frágiles de CI.

### Verification

- `go test ./... -count=1`.
- `go test -tags=e2e ./tests/e2e -count=1`.
- `go vet ./...`.
- `go test -race ./internal/curriculum/... ./internal/infra/curriculumscale ./internal/infra/learningpack ./internal/learning/application -count=1`.
- `go test ./internal/infra/curriculumscale -run '^$' -bench 'BenchmarkLargeCurriculum' -benchtime=1x -benchmem -count=1`.
- `git diff --check`.

### Notes for next session

- El Paso 52 es el siguiente: security hardening de packs/compiler.
- Mantener los límites resource-bound del loader y no convertir el fixture de
  escala en un tamaño recomendado o en un límite artificial del dominio.

## Step 52 — Security hardening de packs/compiler

Status: completed
Date: 2026-09-16
Release: unreleased

### Delivered

- Threat model explícito para packs externos y compiler, con controles para
  traversal, symlinks, parsers, checksums, dependencias, contenido activo,
  agotamiento de recursos, terminales y la frontera sin ejecución/red/IA.
- Paths portables limitados a 1,024 bytes y válidos también en Windows: se
  rechazan controles, caracteres inválidos, device names y sufijos punto/space.
- El budget de 1,024 entradas ahora cuenta directorios vacíos además de archivos;
  las lecturas comparan tamaño real/declarado, identidad de archivo y root para
  detectar cambios o sustituciones durante validación.
- Ratio ZIP exacto de 100:1 sin división truncada; inventario de checksums
  escaneado incrementalmente con tamaño de línea y cantidad esperada bounded.
- JSON duplicate-key scan iterativo, sin recursión proporcional a nesting, más
  fuzz targets offline para JSON y manifest YAML.
- Rechazo de controles terminales aun cuando YAML/JSON los codifique con escape,
  extensiones ejecutables/activas adicionales y Markdown con raw HTML, esquemas
  activos o imágenes que puedan iniciar requests externas.
- El hook futuro `SignatureVerifier` queda confirmado como frontera opcional:
  checksums aportan integridad, no identidad de publisher, y v1 no inventa una
  PKI ni requiere marketplace signing.

### Bugs fixed

- Un árbol de pack podía contener una cantidad no acotada de directorios vacíos
  porque el límite contaba sólo archivos; ahora toda entrada del filesystem
  consume budget.
- Campos YAML/JSON con controles escapados podían superar el filtro UTF-8 y
  llegar a vistas terminales; las invariantes de IDs/texto y excerpts los
  rechazan después de decodificar.
- Markdown checksum-valid podía incluir HTML activo, enlaces `javascript:` o
  imágenes remotas; el loader ahora aplica un perfil Markdown pasivo.
- Citation URLs podían retener userinfo o parámetros con tokens/secrets; ahora
  son bounded y rechazan credenciales en userinfo, query y fragment.
- El cálculo entero del ratio ZIP podía aceptar valores ligeramente superiores
  a 100:1; ahora compara contra el mínimo comprimido exacto.
- El detector de claves JSON duplicadas descendía recursivamente y exponía el
  stack a nesting hostil; ahora usa un stack explícito bounded por el input.

### Decisions

- Mantener límites sólo en el container no impone un máximo curricular: no se
  añadió límite de conceptos, módulos, lecciones, competencias o edges.
- Rechazar contenido activo en vez de intentar sanearlo de forma dependiente de
  un renderer futuro; fenced code permanece dato y nunca se ejecuta.
- No implementar firmas, trust roots, descarga, plugins, AI ni behavior de I-05.

### Verification

- `go test ./... -count=1`.
- `go test -race ./internal/infra/learningpack ./internal/curriculum/... -count=1`.
- `go test ./internal/infra/learningpack -run '^$' -fuzz '^FuzzParseManifest$' -fuzztime=3s` (35,559 executions).
- `go test ./internal/infra/learningpack -run '^$' -fuzz '^FuzzRejectDuplicateJSONKeys$' -fuzztime=3s` (296,043 executions).
- Regresiones deterministas para controles escapados, raw HTML, active links,
  imágenes, ZIP bombs, paths Windows/oversized, extensiones activas y exceso de
  directorios.
- `git diff --check`.

### Notes for next session

- El Paso 53 es el siguiente: dogfooding manual del compiler y reference pack.
- No promover el pack preview ni comenzar I-05 durante el dogfooding.
