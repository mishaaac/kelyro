# I-04 Curriculum Compiler & Learning Packs — Progress Log

## Estado general

Current step: 5
Last completed step: 4
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
