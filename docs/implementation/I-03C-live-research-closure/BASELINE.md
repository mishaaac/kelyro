# I-03C — Baseline funcional de Live Research

## Identidad del baseline

- Baseline de producto: `acbfc63` (`docs(release): record v0.2.0-alpha.1 publication`).
- Release publicada: `v0.2.0-alpha.1`, cuyo tag apunta a `361adac`.
- Apertura documental de I-03C: `0116a08`.
- Regla de la corrección: `existing + working = reuse`.

El Paso 0 solo añadió documentación. Por tanto, el comportamiento congelado
en este documento es el de `acbfc63`.

## Comportamiento observable actual

`kelyro research topic "<tema>"` hace lo siguiente:

1. construye un `ResearchTopic` y un plan determinista con `PlannerV1`;
2. resuelve `privacy.allow_network`;
3. crea o reutiliza el `ResearchRequest` deduplicado por la queue;
4. crea un `ResearchRun` en estado `planned` con presupuesto v1;
5. persiste un `ResearchQueueItem` en estado `queued`;
6. persiste el audit inicial;
7. devuelve `DiscoveryPending=true` y termina.

No consume la queue, no invoca un `SearchProvider`, no descubre URLs, no
registra ni descarga fuentes y no produce evidence, claims, verification ni
Source Bundle desde ese comando.

Esto está cubierto explícitamente por
`TestServicePlansAndInspectsManualResearchTopic` en
`internal/app/research_cache_test.go`.

## Inventario y decisión de reutilización

| Componente | Implementación existente | Evidencia funcional | Decisión I-03C |
|---|---|---|---|
| QueryPlannerV1 | `internal/research/queryplanner/planner_v1.go`; `PlannerV1`, `query-planner-v1`, máximo 8 queries | tests deterministas para propósito, versión, autoridad, bounds e input inmutable | Reutilizar sin sustituir. |
| DiscoveryService | `internal/research/application/services.go`; normaliza query/resultados, deduplica locators, limita output y separa live/cache | `discovery_test.go`, `network_test.go` y E2E controlado | Reutilizar. El orchestration futuro debe llamarlo. |
| SearchProvider | puerto en `internal/research/application/contracts.go`; acepta query + request ID, kind/version/limit y retorna title, locator, snippet opcional, provider, rank y published hint | contract behavior cubierto por DiscoveryService; `memory.StaticSearchProvider` es determinista y sin red | Reutilizar el contrato. Falta un adapter de producción; no hay que introducir otro puerto. |
| ResearchRequest | value/domain model validado en `internal/research/research.go`; persistencia SQLite mediante `ResearchRunRepository` | domain, application y SQLite repository tests | Reutilizar identidad y semántica inmutable. |
| ResearchRun | lifecycle `planned/running/completed/failed/cancelled`, timestamps terminales y cost metadata en `internal/research/research.go` | service/repository/audit tests y E2E | Reutilizar y completar el lifecycle existente; no crear un segundo lifecycle. |
| queue | `ResearchQueueItem`, `research-trigger-v1`, `ResearchTriggerService` y repository SQLite ordenado/deduplicado | `trigger_policy_test.go`, application tests y `research_trigger_queue_repository_test.go` | Reutilizar la queue. Falta consumo/worker; no crear una segunda queue. |
| Source Registry | `SourceService`, `SourceRegistryService`, domain registry y repositories SQLite | registry, application, SQLite y `researchdb` roundtrip tests | Reutilizar para registrar candidates aceptados y conservar identidad estable. |
| Fetcher | hardened client `researchhttp`, adapter `researchfetch.Fetcher` y `FetchService` privacy-gated con fallback offline | HTTP/fetch/network/live/E2E tests | Reutilizar. Falta wiring desde el flujo `research topic`. |
| Snapshot | `SnapshotCaptureService` + `SnapshotRepository`; hashes, revalidation y body policies acotadas | `snapshot_capture_test.go`, cache tests y E2E | Reutilizar; no persistir bodies web sin límites. |
| Normalizer | `researchnormalize.Normalizer`, versión `source-normalization-v1`, para HTML, texto, JSON y Markdown | golden tests, live opt-in y E2E | Reutilizar. Falta wiring después del snapshot. |
| Evidence | domain/repository bounded y trazable a source + snapshot; ingestion específica de release notes | domain, repository, release discovery y E2E tests | Reutilizar modelo y storage. Falta extractor genérico determinista para contenido normalizado. |
| Claims | domain/repository con source IDs, evidence IDs y scope temporal/versionado; ingestion específica de release notes | domain, repository, verification/bundle y E2E tests | Reutilizar modelo y storage. Falta extractor genérico conservador; los fixtures/manual builders no son producción. |
| Verification | `verification-v1` y `NewVerificationService`, basados en claims, trust, registry y conflictos | policy/application/E2E tests | Reutilizar. Falta integrarlo en el flujo productivo. |
| Bundle | `bundle.AssembleV1` y `NewSourceBundleService`; repository y export canónico | domain/policy/application/SQLite/E2E tests | Reutilizar. El store CLI actual lo construye con dependencias de assembly en `nil`, por lo que solo puede leer/exportar bundles ya persistidos. |
| Cost control | presupuesto y metadata `research-cost-control-v1`, servicio de reservas y repository SQLite | domain/application/SQLite tests | Reutilizar y aplicarlo antes de búsquedas/fetches reales. Hoy el CLI solo inicializa metadata. |
| Privacy | Foundation `privacy.NetworkGate`, deny-by-default `privacy.allow_network` y `NetworkResearchAccess` para discovery/fetch/release/update | privacy, application, CLI y E2E tests | Reutilizar como gate obligatorio para toda red y conservar cache offline. |

## Infraestructura auxiliar reutilizable

`ResearchProcessingService` ya aporta concurrencia y límites v1 para lotes de
discovery/fetch preparados por un caller. No es un orchestrator query-to-bundle:
recibe fetches explícitos y no registra sources, captura snapshots, normaliza,
extrae evidence/claims, verifica, ensambla bundle ni cambia el run. I-03C puede
reutilizar sus límites o su ejecución acotada sin presentarlo como el flujo que
falta.

`researchcachefs.NewOfflineAdapter` implementa los fallbacks separados de
search y fetch. Debe conservarse la regla de que el cache offline nunca invoca
el provider live.

## Qué demuestra y qué no demuestra el E2E existente

`TestResearchEngineEvidencePipelineEndToEnd` demuestra que los componentes
pueden interoperar con SQLite, `httptest`, privacy, fetch, snapshots,
normalización, trust, verification y bundle.

El test no demuestra el cierre live de producción porque:

- instancia un `fixtureSearchProvider`, no un provider configurado por el binario;
- registra sources mediante el repository desde el test;
- construye Evidence a partir de un segmento seleccionado por el test;
- construye Claims mediante helpers y statements escritos en el fixture;
- ensambla servicios con repositories directamente, fuera del wiring del CLI;
- marca el run como `completed` desde el test.

Por tanto, ese E2E es una garantía de reutilización y una protección de
regresión, no el acceptance test query-to-bundle que corresponde a pasos
posteriores.

## Gaps congelados

1. No existe implementación de producción de `SearchProvider`.
2. No existe consumer/worker de la queue persistida.
3. No existe orchestrator que conecte el plan con discovery, registration,
   fetch, snapshot, normalize, evidence, claims, verification y bundle.
4. No existe extracción genérica determinista de Evidence desde
   `NormalizedSource`.
5. No existe extracción genérica conservadora de Claims desde Evidence.
6. El wiring productivo no construye los servicios completos necesarios para
   verification y bundle assembly.
7. El comando real no completa ni falla el `ResearchRun`; permanece `planned`.

Estos gaps delimitan I-03C. No autorizan I-04, IA, crawling, browser rendering,
scraping de motores ni reemplazos estéticos de componentes ya probados.
