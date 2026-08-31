# I-03C Live Research Closure — Progress Log

## Estado general

Current step: 4
Last completed step: 3
Baseline commit: acbfc63
I-03 status before correction: PARTIAL

## Gaps

- production SearchProvider missing
- URL discovery missing
- queue worker/orchestrator missing
- query-to-bundle wiring missing
- generic deterministic evidence/claim extraction incomplete

## Registro

## Step 00 — Apertura formal de I-03C

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- Plan I-03C incorporado como memoria persistente en
  `docs/implementation/I-03C-live-research-closure/PLAN.md`.
- Registro de progreso inicializado con el commit real desde el que se abre la
  corrección y con los gaps funcionales confirmados por la auditoría.
- `AGENTS.md` actualizado con los límites de mínimo cambio, privacidad, red,
  providers, tests live, IA e I-04 aplicables a I-03C.

### Decisions

- `acbfc63` es el baseline funcional y documental de I-03C; contiene la
  publicación de `v0.2.0-alpha.1` y precede cualquier cambio de esta corrección.
- I-03C es una corrección acotada del flujo live ya especificado. Reutilizará
  componentes I-03 existentes y no reabre otras áreas cerradas.
- El archivo de plan aportado en la raíz se trasladó a la ruta canónica de
  implementación para evitar dos copias divergentes.
- Este paso no cambia comportamiento, dependencias, schema ni código Go.

### Verification

- `git status --short --branch` y revisión de los 30 commits más recientes.
- `go test ./...`.
- `go vet ./...`.
- Revisión del cierre de I-03, su publicación `v0.2.0-alpha.1` y sus límites
  conocidos antes de abrir esta corrección.

### Notes for next session

- El Paso 1 debe congelar el baseline funcional existente y declarar qué se
  reutiliza, sin implementar todavía el pipeline query-to-bundle.
- No comenzar el Paso 2 ni cambios de producción antes de cerrar el Paso 1.

## Step 01 — Baseline funcional congelado

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- `BASELINE.md` creado con el comportamiento observable real de
  `kelyro research topic`, el inventario de los 16 componentes exigidos y la
  decisión explícita de reutilización para cada uno.
- Contratos, implementations y cobertura existente vinculados para planner,
  discovery, provider, requests/runs, queue, registry, fetch, snapshot,
  normalization, evidence, claims, verification, bundle, cost y privacy.
- Alcance del E2E existente aclarado: prueba interoperabilidad con provider y
  contenido fixture, pero no ejecuta el query-to-bundle desde el binario.
- Gaps productivos congelados sin cambiar código, schema, interfaces ni
  comportamiento.

### Decisions

- `existing + working = reuse`: I-03C conectará los puertos, servicios,
  repositories, adapters, algoritmos y lifecycle existentes.
- `SearchProvider` ya contiene la metadata necesaria para el baseline; su
  implementación de producción falta, pero no se justifica otro contrato.
- La queue persistida es la única queue autorizada. Su consumer falta y deberá
  completar las transiciones existentes.
- `ResearchProcessingService` es infraestructura acotada reutilizable, pero no
  se considera un orchestrator porque recibe fetches preparados y no produce
  snapshots, evidence, claims, verification, bundles ni transiciones del run.
- Evidence/Claims específicos de releases y los builders de fixtures no cubren
  extracción genérica; esa carencia permanece explícita para pasos posteriores.
- El wiring CLI de bundle permite inspeccionar bundles persistidos, pero sus
  dependencias de assembly son `nil`; no es un pipeline productivo completo.

### Verification

- Tests dirigidos de planner, application, memory provider, privacy, hardened
  HTTP/fetch, normalizer, researchdb, SQLite y app/CLI.
- E2E controlado `TestResearchEngineEvidencePipelineEndToEnd` con `httptest` y
  provider fixture, sin Internet público.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 2 es el siguiente paso pendiente y requiere autorización explícita.
- El Paso 2 debe definir el acceptance contract query-to-bundle; no implementar
  aún provider, worker, orchestrator ni extractors.

## Step 02 — Acceptance contract query-to-bundle

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- `ACCEPTANCE.md` creado con el pipeline obligatorio desde topic hasta bundle y
  `ResearchRun` completed.
- Invariantes de aceptación definidas para planning, discovery, candidates,
  registration, fetch, snapshots, normalization, evidence, claims, trust,
  verification, bundle, audit y cost.
- Success mínimo congelado con métricas durables y roundtrip del workspace.
- Las diez failure classes requeridas documentadas con condición, terminalidad,
  privacidad, fallback y comportamiento parcial.
- Ocho escenarios mínimos definidos para futuros acceptance tests sin Internet
  público.

### Decisions

- Search results siguen siendo candidates y no Evidence; el contrato exige la
  cadena durable source → snapshot → evidence antes de aceptar Claims.
- Un run offline puede completar mediante cache suficiente, pero audit y
  métricas deben distinguirlo de red live.
- `fetch_failed_partial` puede coexistir con success cuando otra fuente permite
  verification y bundle; todas las demás causas terminales impiden completed.
- `completed` se persiste únicamente después de un bundle durable `ready` o
  `ready_with_caveats`.
- Este paso estabiliza nombres y resultados observables, no introduce todavía
  tipos Go, provider configuration, error mapping ni orchestration.

### Verification

- Reconciliación directa con los modelos y servicios I-03 existentes.
- Revisión contra el baseline funcional congelado en `BASELINE.md`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 3 debe revisar y congelar el contrato `SearchProvider` existente.
- No implementar configuración, selección o adapter de producción reservados a
  los Pasos 4–6.

## Step 03 — SearchProvider contract estabilizado

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- Contrato `SearchProvider` revisado contra query, limit, results, title,
  locator, snippet opcional y provider metadata requeridos por el plan.
- Comentarios del port y sus DTOs ampliados para congelar provenance,
  normalización, bounds, optionality, cancellation y la separación candidate
  vs Evidence/trust/ranking.
- Contract tests nuevos para campos requeridos y opcionales, inputs/results
  inválidos y el cruce completo de la frontera `DiscoveryService`.
- Cobertura explícita de `Provider`, `Rank` y `PublishedHint` como metadata
  neutral suficiente para el adapter inicial.

### Decisions

- No se cambió la firma ni se añadió metadata: `SearchQuery`, `SearchOptions` y
  `SearchResult` ya contienen todo lo imprescindible del Paso 3.
- `RequestID` preserva el enlace de provenance; `DesiredKind` y
  `TargetVersion` son hints, mientras `Limit` es obligatorio y bounded.
- `Provider` es un identificador estable no secreto, `Rank` conserva el orden
  observado del provider y nunca equivale a authority.
- `Snippet` y `PublishedHint` permanecen opcionales y nunca son Evidence.
- `DiscoveryService` normaliza inputs/outputs, clona pointers, elimina
  fragments de locator, limita/deduplica candidates y limpia metadata cache en
  resultados live.
- No se añadieron crawler, browser, LLM, ranking engine, configuración ni
  adapter de producción.

### Verification

- `go test ./internal/research/application -run 'SearchProviderContract|Discovery' -count=1`.
- `go vet ./internal/research/application`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 4 es el siguiente paso pendiente y requiere autorización explícita.
- El Paso 4 debe definir configuración v1 sin seleccionar ni implementar aún el
  provider de referencia reservado a los Pasos 5–6.
