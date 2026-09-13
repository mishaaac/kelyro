# I-04 Curriculum Compiler & Learning Packs — Progress Log

## Estado general

Current step: 22
Last completed step: 21
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
