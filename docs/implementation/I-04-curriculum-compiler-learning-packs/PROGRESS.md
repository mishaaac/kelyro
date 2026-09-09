# I-04 Curriculum Compiler & Learning Packs — Progress Log

## Estado general

Current step: 2
Last completed step: 1
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
