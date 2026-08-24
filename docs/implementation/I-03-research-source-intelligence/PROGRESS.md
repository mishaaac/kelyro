# I-03 Research & Source Intelligence — Progress Log

## Estado general

Current step: 2
Last completed step: 1
Current release: v0.1.0-alpha.3 (published prerelease)
Student Core baseline: v0.1.0-alpha.3 (751f6b9); I-03 branch base 498b9fb

## Registro

## Step 00 — Apertura formal de I-03

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Plan I-03 incorporado como memoria persistente en
  `docs/implementation/I-03-research-source-intelligence/PLAN.md`.
- Registro de progreso inicializado con el tag publicado de Student & Learning
  Core y el commit real desde el que se abre I-03.
- `AGENTS.md` actualizado con el flujo de sesiones, los límites de I-03, la
  política de red y las reglas de evidencia, testing y versionado.

### Decisions

- `v0.1.0-alpha.3` en `751f6b9` es el baseline publicado e inmutable de I-02;
  `498b9fb` es el commit de documentación posterior a la publicación y la base
  efectiva de la rama I-03.
- Cada paso de I-03 se autoriza, implementa, verifica, documenta y comitea de
  forma independiente.
- I-03 producirá evidencia, provenance, Source Bundles e inteligencia de cambio;
  no compilará curriculum de producción ni modificará el estado de aprendizaje.
- La investigación live respetará `privacy.allow_network`; los tests ordinarios
  serán deterministas y no dependerán de Internet público.

### Verification

- Revisión de `git status` y de los 20 commits más recientes.
- Revisión del cierre y el registro de publicación de I-02.
- `GOCACHE=/tmp/kelyro-i03-step0-test-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step0-vet-gocache go vet ./...`.

### Notes for next session

- El Paso 1 es el siguiente paso pendiente y requiere autorización explícita.
- No implementar persistencia, adapters web, políticas de trust ni ninguna parte
  de I-04 antes de sus pasos correspondientes.

## Step 01 — Modelo de dominio de Research & Source Intelligence

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Paquete cohesivo `internal/research`, separado por archivos de source,
  research, evidence y change intelligence, sin dependencias externas ni de
  infraestructura/presentación.
- Value objects validados para IDs generales, `SourceID`, `ClaimID`, timestamps
  UTC, locators HTTP(S), versiones opacas, tópicos generales, confidence y
  freshness scores.
- Vocabulario completo del Paso 1 para requests/runs, fuentes y snapshots,
  authority/trust, discovery candidates, evidence/claims, provenance,
  citations/deep links, bundles, freshness, releases, deprecations, conflicts,
  verification, drift e impact.
- Enums cerrados para source kinds, purposes, lifecycle/status, authority,
  claims, freshness, releases, conflicts, verification, drift, severidad y
  acciones recomendadas.
- Validadores relacionales que comprueban la cadena real
  `request → run → source → snapshot → evidence → claim` y las referencias de
  citations, además de rechazar IDs duplicados.
- Tests deterministas de IDs, locators, scores, source kinds, timestamps UTC,
  timelines, estados inválidos y relaciones de provenance/citations.
- Contrato y límites del dominio documentados en
  `docs/architecture/research-domain.md` y enlazados desde el índice de
  arquitectura.

### Decisions

- Se usa un único paquete `internal/research` para mantener cohesión y evitar
  micro-paquetes/ciclos prematuros; futuros límites solo se extraerán cuando los
  application services revelen dependencias reales.
- `SourceID` no deriva de URL: la identidad sobrevive aliases y cambios de
  locator, mientras snapshots conservan la URL exacta usada en cada fetch.
- Los locators del dominio aceptan HTTP(S) absoluto y rechazan credenciales;
  resolución DNS, SSRF, redirects y networking pertenecen a adapters futuros.
- `SourceVersion` es opaca y no exige SemVer para soportar revisiones, fechas,
  ediciones y otros esquemas de versión.
- Discovery candidates nunca son evidence. Claims requieren source y evidence;
  citations requieren source, snapshot y evidence.
- El Paso 1 define shapes y estados, no implementa todavía Trust Policy,
  authority matching, freshness, verification, conflict, drift ni impact
  algorithms.
- No se añadieron repositories, SQLite, HTTP, parsing, CLI/TUI, Curriculum
  Compiler ni mutaciones de Student Core.

### Verification

- `GOCACHE=/tmp/kelyro-i03-step1-target3-gocache go test ./internal/research`.
- `GOCACHE=/tmp/kelyro-i03-step1-target3-gocache go vet ./internal/research`.
- `GOCACHE=/tmp/kelyro-i03-step1-full-test-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step1-full-vet-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step1-quality-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go test -race ./...`, build y smokes
  de CLI.
- Auditoría de imports: `internal/research` depende únicamente de la librería
  estándar y no importa HTTP, SQLite, UI, Student Core ni providers.
- `git diff --check`.

### Notes for next session

- El Paso 2 es el siguiente paso pendiente y requiere autorización explícita.
- Repositories y application services deberán consumir este vocabulario sin
  exponer SQLite, HTTP o UI al dominio.
- No implementar persistence schema, adapters web, Trust Policy ni pasos
  posteriores antes de su autorización independiente.
