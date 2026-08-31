# Kelyro — Plan de Implementación I-03C: Live Research Closure

> Corrección acotada de I-03 — Research & Source Intelligence.
>
> Objetivo: cerrar el gap funcional detectado en la auditoría sin reescribir ni desestabilizar la arquitectura ya implementada.
>
> Al terminar I-03C, este flujo debe funcionar desde el binario real:
>
> ```text
> kelyro research topic "<tema>"
>         ↓
> Query Planner
>         ↓
> Production SearchProvider
>         ↓
> Search results / URLs
>         ↓
> Source Registration
>         ↓
> HTTP Fetch
>         ↓
> Snapshot
>         ↓
> Normalize
>         ↓
> Evidence / Claim Extraction
>         ↓
> Trust / Verification
>         ↓
> Source Bundle
>         ↓
> completed
> ```
>
> I-03C NO reemplaza I-03.
> I-03C NO implementa I-04.
> I-03C NO introduce IA.
> La prioridad es: **reuse > wiring > small adapters > new abstractions**.

---

# Motivo de esta corrección

La auditoría técnica determinó:

```text
I-01 Foundation                      COMPLETE
I-02 Student & Learning Core         COMPLETE
I-03 Research & Source Intelligence  PARTIAL
```

Gap real:

```text
Query Planner               IMPLEMENTED
SearchProvider interface    IMPLEMENTED
Production SearchProvider   MISSING
URL Discovery               MISSING
HTTP Fetch                  IMPLEMENTED
Snapshot                    IMPLEMENTED
Normalization               IMPLEMENTED
Claims                      PARTIAL
Trust Verification          IMPLEMENTED
Source Bundle               PARTIAL in production flow
```

Actualmente:

```text
kelyro research topic "<tema>"
→ crea request
→ crea run(status=planned)
→ crea queue item
→ DiscoveryPending=true
→ FIN
```

I-03C debe completar exclusivamente lo que falta entre ese `planned` y un `Source Bundle` real.

---

# Objetivo exacto

Al finalizar:

```bash
kelyro research topic "Go interfaces"
```

debe poder:

1. planificar consultas;
2. ejecutar búsqueda pública real mediante un provider configurado;
3. obtener URLs;
4. registrar fuentes;
5. descargar contenido;
6. crear snapshots;
7. normalizar;
8. extraer evidence/claims de forma determinista y conservadora;
9. aplicar trust/authority/freshness/verification;
10. construir Source Bundle;
11. persistir provenance/audit/cost;
12. finalizar ResearchRun;
13. respetar privacy/network policy;
14. degradar correctamente sin red/provider;
15. conservar comportamiento offline.

---

# No objetivos

I-03C NO debe implementar:

```text
AI research reasoning
LLM claim extraction
AI query generation
AI summarization
I-04 Curriculum Compiler
Learning Pack generation
plugin platform
marketplace
automation
background cloud service
general web crawler
recursive crawling
browser rendering
```

---

# Principios de mínimo cambio

1. No cambiar interfaces existentes si pueden reutilizarse.
2. No renombrar paquetes estables por estética.
3. No mover archivos sin necesidad funcional.
4. No sustituir repositories existentes.
5. No sustituir QueryPlannerV1.
6. No sustituir privacy/network policy.
7. No sustituir HTTP fetcher.
8. No sustituir snapshot/cache.
9. No sustituir normalizers.
10. No sustituir trust/authority.
11. No sustituir verification/conflict/freshness.
12. No sustituir SourceBundle domain.
13. No introducir una segunda queue.
14. Consumir la queue ya existente.
15. No introducir un segundo ResearchRun lifecycle.
16. Completar el lifecycle existente.
17. No introducir IA para cerrar este gap.
18. No hacer scraping arbitrario de motores.
19. Usar provider/API explícito y documentado.
20. Toda red respeta `privacy.allow_network`.
21. Toda credencial usa Foundation Secrets.
22. No guardar API keys en TOML/SQLite/logs.
23. Search y Fetch son operaciones diferentes.
24. Search results son candidates, no evidence.
25. Search ranking no equivale a authority.
26. Preservar provenance query→bundle.
27. Preservar cost control.
28. Queries/results/fetches siempre bounded.
29. Live tests opt-in.
30. Tests normales sin Internet.
31. No romper offline.
32. No avanzar I-04 para ocultar gaps.

---

# Arquitectura actual a preservar

```text
internal/research
├── authority
├── trust
├── queryplanner
├── freshness
├── conflict
├── verification
├── bundle
└── application
    └── SearchProvider interface

internal/infra
├── researchhttp
├── researchfetch
├── researchnormalize
├── researchcachefs
└── researchrelease
```

Objetivo:

```text
SearchProvider
      ↓
Production Adapter
      ↓
Research Orchestrator
      ↓
existing services
```

---

# Naming

```text
I-03C — Live Research Closure
```

Repositorio:

```text
docs/implementation/I-03C-live-research-closure/
├── PLAN.md
└── PROGRESS.md
```

No renumerar I-04.

---

## Paso 0 — Abrir formalmente I-03C

- [x] Paso 0 completado

### Objetivo

Registrar la corrección sin cambiar comportamiento.

### Antes de tocar código

```bash
git status
git log --oneline --decorate -n 30
go test ./...
go vet ./...
```

### Crear

```text
docs/implementation/I-03C-live-research-closure/
├── PLAN.md
└── PROGRESS.md
```

### PROGRESS.md

```md
# I-03C Live Research Closure — Progress Log

Current step: 0
Last completed step: none
Baseline commit: <real commit>
I-03 status before correction: PARTIAL

Gaps:
- production SearchProvider missing
- URL discovery missing
- queue worker/orchestrator missing
- query-to-bundle wiring missing
- generic deterministic evidence/claim extraction incomplete
```

### AGENTS.md

Agregar:

```text
- Do not refactor completed I-03 components unless required.
- Prefer existing interfaces/services/repositories.
- No I-04.
- No AI dependency.
- No search-engine scraping.
- All network access passes privacy policy.
- All live tests are opt-in.
```

### Commit sugerido

```text
docs(research): open I-03C live research closure
```

---

## Paso 1 — Congelar baseline funcional

- [x] Paso 1 completado

Documentar componentes existentes y confirmar qué se reutiliza:

```text
QueryPlannerV1
DiscoveryService
SearchProvider
ResearchRequest
ResearchRun
queue
Source Registry
Fetcher
Snapshot
Normalizer
Evidence
Claims
Verification
Bundle
Cost control
Privacy
```

Crear:

```text
docs/implementation/I-03C-live-research-closure/BASELINE.md
```

Regla:

```text
existing + working = reuse
```

### Commit

```text
test(research): freeze I-03 live research baseline
```

---

## Paso 2 — Definir acceptance contract query-to-bundle

- [x] Paso 2 completado

Pipeline obligatorio:

```text
topic
→ query plan
→ search
→ URLs
→ fetch
→ snapshot
→ normalize
→ evidence
→ claims
→ verify
→ bundle
→ completed run
```

Success mínimo:

```text
status=completed
discovery_pending=false
searches_performed > 0
sources_discovered > 0
sources_fetched >= 1
bundle_id != empty
```

Failure classes:

```text
network_disabled
provider_unconfigured
provider_auth_failed
provider_rate_limited
search_failed
no_results
fetch_failed_partial
verification_insufficient
bundle_not_buildable
cancelled
```

### Commit

```text
docs(research): define query-to-bundle acceptance contract
```

---

## Paso 3 — Revisar y congelar SearchProvider contract

- [x] Paso 3 completado

Revisar:

```text
internal/research/application/contracts.go
```

Confirmar si soporta:

```text
query
limit
results
title
URL/locator
snippet optional
provider metadata
```

Si falta metadata imprescindible, hacer cambio mínimo backward-compatible.

No añadir:

```text
crawler
browser
LLM
ranking engine
```

Crear contract tests.

### Commit

```text
test(research): stabilize SearchProvider contract
```

---

## Paso 4 — Search Provider configuration v1

- [x] Paso 4 completado

Config conceptual:

```toml
[research.search]
provider = "..."
max_results_per_query = 8
max_queries_per_run = 4
```

Credenciales vía Foundation Secrets:

```text
research.search.<provider>.api_key
```

Estados:

```text
configured
missing_credentials
disabled
unavailable
```

### Commit

```text
feat(research): add production search provider configuration
```

---

## Paso 5 — Seleccionar reference production SearchProvider

- [x] Paso 5 completado

Elegir un único adapter inicial evaluando:

```text
documented API
structured JSON results
TLS
authentication
rate limits
cost visibility
result URL/title/snippet
testability
```

Crear ADR:

```text
docs/architecture/adr/ADR-XXX-research-search-provider.md
```

No elegir scraping HTML si existe API formal viable.

Vendor solo en infra/config/doctor, no domain.

### Commit

```text
docs(research): select reference production search provider
```

---

## Paso 6 — Implement production SearchProvider adapter

- [x] Paso 6 completado

Ubicación sugerida:

```text
internal/infra/researchsearch/
```

Implementar:

```text
Search(ctx, query, limit)
```

Responsabilidades:

```text
request
auth
response bounds
JSON decode
result normalization
provider error mapping
rate-limit metadata
```

No responsabilidades:

```text
trust
authority
claim extraction
bundle generation
```

### Commit

```text
feat(research): add production web search adapter
```

---

## Paso 7 — Harden Search HTTP transport

- [x] Paso 7 completado

Verificar:

```text
timeouts
TLS
redirect rules
body limits
status mapping
cancellation
user agent
rate limits
no secret logging
```

Documentar diferencia entre:

```text
search endpoint
vs
fetched result URL
```

### Commit

```text
fix(security): harden research search transport
```

---

## Paso 8 — Secrets integration

- [x] Paso 8 completado

Resolver credenciales con Foundation Secrets.

Mostrar únicamente:

```text
Provider: configured
Credential: available
```

Nunca valor.

### Commit

```text
feat(research): load search credentials from secret store
```

---

## Paso 9 — Privacy/network gate antes de Search

- [x] Paso 9 completado

Antes de invocar provider:

```text
privacy.allow_network
```

Si false:

```text
network_disabled
```

y cero requests.

### Commit

```text
fix(privacy): gate live research search behind network policy
```

---

## Paso 10 — Cost Control sobre búsquedas reales

- [x] Paso 10 completado

Reusar I-03 cost-control:

```text
max searches/run
max results/query
max provider calls
provider cost estimate if available
```

Nunca trabajo de red no bounded.

### Commit

```text
feat(research): enforce budgets on production search calls
```

---

## Paso 11 — Production provider wiring

- [x] Paso 11 completado

Composition root:

```text
config
→ secrets
→ privacy
→ provider adapter
→ SearchProvider
```

Preferir wiring en:

```text
cmd/kelyro/main.go
```

o factory existente.

Nunca usar silenciosamente StaticSearchProvider en producción.

### Commit

```text
feat(research): wire production search provider
```

---

## Paso 12 — Doctor readiness

- [x] Paso 12 completado

Agregar diagnóstico:

```text
Research Search
✓ network policy enabled
✓ provider configured
✓ credential available
```

o estado claro si no.

No ejecutar búsquedas costosas en doctor.

### Commit

```text
feat(doctor): report research search readiness
```

---

## Paso 13 — Research Orchestrator v1

- [x] Paso 13 completado

Crear coordinador sobre servicios existentes:

```text
load queue item
load run
search
register candidates
fetch
snapshot
normalize
extract
verify
bundle
finalize
```

No implementar directamente:

```text
HTTP
trust
normalization
verification
SQLite
```

Ubicación sugerida:

```text
internal/research/application/orchestrator.go
```

### Commit

```text
feat(research): define live research orchestrator
```

---

## Paso 14 — Completar ResearchRun lifecycle

- [x] Paso 14 completado

Reusar state machine.

Estados conceptuales si hacen falta:

```text
planned
discovering
fetching
extracting
verifying
bundling
completed
failed
cancelled
```

No crear segunda state machine.

### Commit

```text
feat(research): complete live research run lifecycle
```

---

## Paso 15 — Consumir queue existente

- [x] Paso 15 completado

Worker semantics:

```text
claim item
execute
ack
retry transient
fail permanent
```

No crear otra queue.

Idempotency obligatoria.

### Commit

```text
feat(research): consume pending research work
```

---

## Paso 16 — Execution path inicial sin daemon

- [x] Paso 16 completado

Para minimizar código:

```text
kelyro research topic
→ enqueue
→ bounded orchestrator execution
→ result
```

o command explícito compatible con UX existente.

No introducir background daemon. I-09 lo hará después.

### Commit

```text
feat(research): execute queued research without background daemon
```

---

## Paso 17 — Search Result → Source Candidate

- [ ] Paso 17 completado

Mapear:

```text
URL
title
snippet
query
provider
rank
discovered_at
```

Candidate ≠ trusted source.

### Commit

```text
feat(research): convert web search results into source candidates
```

---

## Paso 18 — Candidate deduplication

- [ ] Paso 18 completado

Dedup por:

```text
canonical URL
normalized locator
existing registry
```

Preservar qué queries descubrieron cada fuente.

### Commit

```text
feat(research): deduplicate discovered source candidates
```

---

## Paso 19 — Source Registry ingestion

- [ ] Paso 19 completado

Reusar Source Registry.

Persistir discovery metadata sin asignar trust automático.

### Commit

```text
feat(research): register live discovered sources
```

---

## Paso 20 — Wiring del HTTP Fetcher existente

- [ ] Paso 20 completado

Usar fetcher ya probado con:

```text
privacy
SSRF
redirects
size limits
content types
timeouts
```

Permitir partial source failure.

### Commit

```text
feat(research): fetch discovered sources through existing adapter
```

---

## Paso 21 — Snapshot + Cache wiring

- [ ] Paso 21 completado

Reusar:

```text
snapshot
researchcachefs
hashing
metadata
```

Provenance:

```text
query → result → source → snapshot
```

### Commit

```text
feat(research): snapshot live discovered sources
```

---

## Paso 22 — Normalizer wiring

- [ ] Paso 22 completado

Reusar normalizadores de:

```text
HTML
Markdown
JSON
text
```

### Commit

```text
feat(research): normalize live fetched research content
```

---

## Paso 23 — Diseñar Deterministic Evidence Extractor v1

- [ ] Paso 23 completado

No intentar NLP general.

Extraer evidencia conservadora desde:

```text
headings
bounded sections
structured metadata
release/version facts
explicit deprecation markers
topic-relevant passages
```

Output:

```text
EvidenceCandidate
```

Version:

```text
evidence-extractor-v1
```

Crear:

```text
docs/architecture/research-evidence-extractor-v1.md
```

### Commit

```text
feat(research): define deterministic evidence extractor v1
```

---

## Paso 24 — Implement Evidence Extractor v1

- [ ] Paso 24 completado

Requisitos:

```text
deterministic
bounded excerpts
citation locator
no LLM
no invented facts
stable scoring
```

Tests con docs oficiales/releases/spec/community/irrelevant.

### Commit

```text
feat(research): extract bounded evidence from normalized sources
```

---

## Paso 25 — Diseñar Conservative Claim Extractor v1

- [ ] Paso 25 completado

Claim families iniciales:

```text
explicit definition
version/release fact
deprecation statement
availability/support statement
explicit requirement
explicit recommendation
```

Claim siempre referencia Evidence.

Ambigüedad:

```text
do not create claim
```

Version:

```text
claim-extractor-v1
```

### Commit

```text
feat(research): define conservative claim extraction v1
```

---

## Paso 26 — Implement Claim Extractor v1

- [ ] Paso 26 completado

Requisitos:

```text
deterministic
evidence-backed
citation required
bounded
dedupe
confidence explicit
no unsupported inference
```

### Commit

```text
feat(research): derive evidence-backed claims deterministically
```

---

## Paso 27 — Authority / Trust integration

- [ ] Paso 27 completado

Reusar authority/trust actuales.

Search provider rank no modifica trust directamente.

### Commit

```text
feat(research): apply existing trust policy to discovered sources
```

---

## Paso 28 — Freshness / Temporal Scope integration

- [ ] Paso 28 completado

Reusar:

```text
freshness
release intelligence
deprecation
historical evidence
```

Capturar metadata temporal disponible.

### Commit

```text
feat(research): evaluate freshness in live research runs
```

---

## Paso 29 — Multi-source Verification integration

- [ ] Paso 29 completado

Reusar verification/diversity.

Resultados:

```text
verified
supported
conflicted
insufficient_evidence
```

### Commit

```text
feat(research): verify live claims across discovered sources
```

---

## Paso 30 — Source Bundle desde run real

- [ ] Paso 30 completado

Reusar bundle service/domain.

Bundle referencia:

```text
request
run
queries
sources
snapshots
evidence
claims
verification
conflicts
policy versions
```

### Commit

```text
feat(research): build Source Bundles from live research runs
```

---

## Paso 31 — Finalizar run y queue

- [ ] Paso 31 completado

Success:

```text
run.status=completed
DiscoveryPending=false
queue=acked/completed
bundle_id persisted
```

Failure: structured safe reason.

### Commit

```text
feat(research): finalize live research runs transactionally
```

---

## Paso 32 — Provenance end-to-end

- [ ] Paso 32 completado

Verificar:

```text
User Topic
→ ResearchRequest
→ Query
→ Search Call
→ Search Result
→ Source
→ Snapshot
→ Evidence
→ Claim
→ Verification
→ Bundle
```

### Commit

```text
feat(research): preserve query-to-bundle provenance
```

---

## Paso 33 — Audit + Cost accounting

- [ ] Paso 33 completado

Registrar:

```text
provider ID
adapter version
query count
result count
fetch count
bytes
success/failure
bundle ID
policy versions
```

Nunca credentials.

### Commit

```text
feat(audit): record live research execution metadata
```

---

## Paso 34 — Completar UX de `research topic`

- [ ] Paso 34 completado

El comando ya no termina en `DiscoveryPending=true`.

Output conceptual:

```text
Researching: Go interfaces

Queries planned: 4
Sources discovered: 12
Sources fetched: 7
Evidence items: 26
Claims: 14
Verified: 9
Conflicts: 1

Source Bundle:
<bundle-id>
```

### Commit

```text
feat(cli): complete live research topic workflow
```

---

## Paso 35 — Mejorar `research status/show`

- [ ] Paso 35 completado

Mostrar:

```text
status
phase
queries
provider
sources
warnings
bundle
failure reason
```

### Commit

```text
feat(cli): expose live research run progress
```

---

## Paso 36 — Unit tests del SearchProvider real

- [ ] Paso 36 completado

Sin Internet, con `httptest.Server`.

Casos:

```text
success
empty
malformed
401
403
429
500
timeout
oversized response
cancellation
```

### Commit

```text
test(research): cover production search provider adapter
```

---

## Paso 37 — SearchProvider conformance tests

- [ ] Paso 37 completado

Mismo suite para:

```text
StaticSearchProvider
production adapter
future adapters
```

### Commit

```text
test(research): add SearchProvider conformance suite
```

---

## Paso 38 — E2E query-to-bundle sin Internet

- [ ] Paso 38 completado

Fake Search API + local content server:

```text
query
→ URLs
→ fetch
→ normalize
→ evidence
→ claims
→ verify
→ bundle
```

Verificar:

```text
run completed
DiscoveryPending=false
bundle persisted
provenance complete
```

### Commit

```text
test(e2e): cover research query-to-bundle workflow
```

---

## Paso 39 — E2E privacy disabled

- [ ] Paso 39 completado

Con:

```text
privacy.allow_network=false
```

verificar cero provider calls y cero fetches.

### Commit

```text
test(e2e): enforce offline research privacy boundary
```

---

## Paso 40 — E2E partial source failure

- [ ] Paso 40 completado

Fixture:

```text
A success
B timeout
C 404
D success
```

Permitir completion con warnings si policy lo permite.

### Commit

```text
test(e2e): tolerate bounded source failures
```

---

## Paso 41 — E2E retry/idempotency

- [ ] Paso 41 completado

Interrumpir después de fetch/snapshot y reintentar.

No duplicar lógicamente:

```text
source
snapshot
claim
bundle
```

### Commit

```text
test(e2e): make research orchestration retry-safe
```

---

## Paso 42 — Live SearchProvider smoke

- [ ] Paso 42 completado

Opt-in:

```text
KELYRO_LIVE_RESEARCH_SEARCH_TESTS=1
```

Debe probar realmente:

```text
query → external provider → result URL
```

No URL hardcodeada.

### Commit

```text
test(research): add opt-in live web search smoke
```

---

## Paso 43 — Live query-to-bundle smoke

- [ ] Paso 43 completado

Flujo real opt-in:

```text
query
→ search
→ URL
→ fetch
→ normalize
→ evidence
→ claim
→ verify
→ bundle
```

Assert invariants, no ranking exacto.

### Commit

```text
test(research): add opt-in live query-to-bundle smoke
```

---

## Paso 44 — Security review

- [ ] Paso 44 completado

Threats:

```text
secret leakage
provider redirect abuse
malformed results
javascript/data/file URLs
oversized URLs
query/log injection
rate-limit abuse
response bombs
```

Actualizar:

```text
docs/security/research-threat-model.md
```

### Commit

```text
fix(security): harden live research discovery
```

---

## Paso 45 — Performance / bounds

- [ ] Paso 45 completado

Mantener límites conservadores:

```text
queries/run bounded
results/query bounded
fetch concurrency bounded
bytes bounded
claims/source bounded
```

### Commit

```text
perf(research): bound live research orchestration
```

---

## Paso 46 — Offline regression

- [ ] Paso 46 completado

Sin provider/red deben seguir funcionando:

```text
existing sources
existing bundles
cached evidence
research status
I-01
I-02
```

### Commit

```text
test(research): preserve offline-first research behavior
```

---

## Paso 47 — Regression I-01/I-02/I-03

- [ ] Paso 47 completado

```bash
go test ./...
go vet ./...
go test -tags=e2e ./tests/e2e
```

No I-04.

### Commit

```text
test(research): verify I-03C regression safety
```

---

## Paso 48 — Dogfooding live research

- [ ] Paso 48 completado

Probar:

```text
official docs topic
release/version topic
deprecation topic
multi-source topic
noisy/irrelevant results
```

Inspeccionar:

```text
queries
sources
authority
evidence
claims
verification
conflicts
bundle
cost
audit
```

### Commit

```text
docs(research): record I-03C dogfooding results
```

---

## Paso 49 — Corregir estado administrativo de I-03

- [ ] Paso 49 completado

Añadir al PROGRESS original:

```text
Post-closure corrective implementation:
I-03C Live Research Closure

Reason:
Production SearchProvider and query-to-bundle wiring were missing.

Resolved in:
<commit/release>
```

No reescribir historia.

### Commit

```text
docs(research): link I-03 closure to corrective implementation
```

---

## Paso 50 — Actualizar architecture docs

- [ ] Paso 50 completado

Actualizar:

```text
docs/architecture/research-application.md
docs/architecture/research-cli-v1.md
```

Eliminar limitaciones ya resueltas y documentar flow real.

### Commit

```text
docs(research): document production live research pipeline
```

---

## Paso 51 — Formal closure audit

- [ ] Paso 51 completado

Confirmar:

```text
Production SearchProvider      YES
URL discovery                  YES
HTTP fetch                     YES
Normalization                  YES
Evidence extraction            YES
Claim extraction               YES
Trust/verification             YES
Source Bundle                  YES
CLI query-to-bundle            YES
Offline mode                   YES
Privacy gate                   YES
Cost control                   YES
Live query smoke               YES
```

Pregunta final:

```text
Can Kelyro currently search the public Internet
from a user/research request?
```

Expected:

```text
YES, when configured and network is allowed.
```

### Commit

```text
docs(research): complete I-03C closure audit
```

---

## Paso 52 — Cierre formal I-03C

- [ ] Paso 52 completado

Gates:

```bash
go test ./...
go vet ./...
go test -race ./...
go test -tags=e2e ./tests/e2e
git diff --check
```

Live opt-in:

```bash
KELYRO_LIVE_RESEARCH_SEARCH_TESTS=1 go test ./tests/live -count=1 -v
```

Completion record:

```md
## I-03C Live Research Closure

Status: completed
Baseline: <commit>
Completion: <commit/release>

Closed gaps:
- production SearchProvider
- public URL discovery
- queue consumption
- research orchestration
- deterministic evidence extraction
- conservative claim extraction
- query-to-Source-Bundle wiring
- live query search test

I-03 status after correction:
COMPLETE

Ready for:
I-04 Curriculum Compiler & Learning Packs
```

### Commit

```text
docs(roadmap): mark I-03C live research closure complete
```

---

# Checklist resumido

- [ ] Paso 0 — Apertura
- [ ] Paso 1 — Baseline
- [ ] Paso 2 — Acceptance contract
- [ ] Paso 3 — SearchProvider contract
- [ ] Paso 4 — Search config
- [ ] Paso 5 — Provider selection
- [ ] Paso 6 — Production SearchProvider
- [ ] Paso 7 — Transport hardening
- [ ] Paso 8 — Secrets
- [ ] Paso 9 — Privacy
- [ ] Paso 10 — Cost control
- [ ] Paso 11 — Wiring
- [ ] Paso 12 — Doctor
- [ ] Paso 13 — Orchestrator
- [ ] Paso 14 — Run lifecycle
- [x] Paso 15 — Queue consumer
- [x] Paso 16 — Execution path
- [ ] Paso 17 — Search candidates
- [ ] Paso 18 — Deduplication
- [ ] Paso 19 — Source Registry
- [ ] Paso 20 — Fetcher
- [ ] Paso 21 — Snapshot/cache
- [ ] Paso 22 — Normalizer
- [ ] Paso 23 — Evidence design
- [ ] Paso 24 — Evidence implementation
- [ ] Paso 25 — Claim design
- [ ] Paso 26 — Claim implementation
- [ ] Paso 27 — Trust
- [ ] Paso 28 — Freshness
- [ ] Paso 29 — Verification
- [ ] Paso 30 — Source Bundle
- [ ] Paso 31 — Finalize run
- [ ] Paso 32 — Provenance
- [ ] Paso 33 — Audit/cost
- [ ] Paso 34 — CLI topic
- [ ] Paso 35 — CLI status/show
- [ ] Paso 36 — Provider unit tests
- [ ] Paso 37 — Contract tests
- [ ] Paso 38 — E2E query-to-bundle
- [ ] Paso 39 — E2E privacy
- [ ] Paso 40 — E2E partial failure
- [ ] Paso 41 — E2E idempotency
- [ ] Paso 42 — Live search
- [ ] Paso 43 — Live query-to-bundle
- [ ] Paso 44 — Security
- [ ] Paso 45 — Performance
- [ ] Paso 46 — Offline regression
- [ ] Paso 47 — Full regression
- [ ] Paso 48 — Dogfooding
- [ ] Paso 49 — I-03 progress correction
- [ ] Paso 50 — Architecture docs
- [ ] Paso 51 — Closure audit
- [ ] Paso 52 — Formal closure

---

# Definition of Done

- [ ] Existing I-03 architecture preserved
- [ ] Existing QueryPlannerV1 preserved
- [ ] Existing SearchProvider reused
- [ ] Production SearchProvider exists
- [ ] Search credentials use Foundation Secrets
- [ ] No credential in TOML/SQLite/logs
- [ ] Network policy blocks search
- [ ] Cost control limits search calls
- [ ] Search results are candidates, not trusted evidence
- [ ] URL schemes validated
- [ ] Existing SSRF protection reused
- [ ] Existing Source Registry reused
- [ ] Existing Fetcher reused
- [ ] Existing Snapshot/Cache reused
- [ ] Existing Normalizers reused
- [ ] Evidence extraction deterministic
- [ ] Claims evidence-backed
- [ ] No AI required
- [ ] Search rank does not determine authority
- [ ] Existing Trust/Verification reused
- [ ] Existing SourceBundle reused
- [ ] Existing queue reused
- [ ] Queue has consumer
- [ ] ResearchRun reaches completed
- [ ] DiscoveryPending clears
- [ ] Provenance query→bundle complete
- [ ] Audit/cost complete
- [ ] `kelyro research topic` performs real discovery when configured
- [ ] E2E query-to-bundle passes offline
- [ ] Live search proves query→URL
- [ ] Live test proves query→bundle
- [ ] Offline behavior preserved
- [ ] I-01/I-02/I-03 regressions pass
- [ ] No I-04 implementation
- [ ] No I-07 dependency
- [ ] Security review complete
- [ ] Dogfooding complete
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] race tests applicable pass
- [ ] working tree clean
- [ ] Final audit Internet Search = YES
- [ ] I-03 can be marked COMPLETE
- [ ] Ready for I-04

---

# Esfuerzo recomendado para Codex

**High por defecto.**

Usar **xhigh** solamente en:

```text
Paso 3   SearchProvider contract
Paso 7   HTTP/security
Paso 13  Orchestrator design
Paso 15  Queue/idempotency
Paso 23  Evidence Extractor design
Paso 25  Claim Extractor design
Paso 31  transactional finalization
Paso 41  retry/idempotency E2E
Paso 44  security review
Paso 51  closure audit
```

---

# Prompt reutilizable por paso

```text
Trabaja únicamente en el Paso XX de:

docs/implementation/I-03C-live-research-closure/PLAN.md

Esta es una corrección acotada de I-03.

Reglas obligatorias:

1. Lee AGENTS.md.
2. Lee completo el Paso XX.
3. Lee I-03C PROGRESS.md.
4. Lee solo las partes relevantes del PLAN/PROGRESS original de I-03.
5. Revisa git status y commits recientes.
6. Antes de modificar, identifica qué componentes existentes pueden reutilizarse.
7. En Plan Mode, propone el cambio mínimo.
8. No reescribas componentes I-03 que ya funcionan.
9. No implementes I-04.
10. No introduzcas IA/I-07.
11. No crees una segunda SearchProvider interface.
12. No crees una segunda queue.
13. Toda red pasa privacy/network policy.
14. Toda credencial pasa Foundation Secrets.
15. Ejecuta criterios de verificación.
16. Si todo pasa:
    - marca checkbox;
    - actualiza PROGRESS.md;
    - crea Conventional Commit;
    - deja working tree limpio.

Si detectas que el paso exige cambiar una arquitectura estable de I-03,
DETENTE y explícame por qué antes de hacerlo.
```

---

# Resultado esperado

Antes:

```text
$ kelyro research topic "Go interfaces"

Query plan created.
Discovery pending.

→ FIN
```

Después:

```text
$ kelyro research topic "Go interfaces"

Researching: Go interfaces

Queries planned: 4
Sources discovered: 11
Sources fetched: 6
Evidence items: 24
Claims: 13
Verified: 9
Conflicts: 1

Source Bundle:
<bundle-id>

Research completed.
```

Arquitectura final:

```text
User Topic
    ↓
QueryPlannerV1                 EXISTING
    ↓
Production SearchProvider      I-03C
    ↓
Source Candidates              I-03C wiring
    ↓
Source Registry                EXISTING
    ↓
HTTP Fetcher                   EXISTING
    ↓
Snapshot / Cache               EXISTING
    ↓
Normalizer                     EXISTING
    ↓
Evidence Extractor v1          I-03C minimal
    ↓
Claim Extractor v1             I-03C minimal
    ↓
Trust / Verification           EXISTING
    ↓
Source Bundle                  EXISTING
```

Entonces sí queda cerrado:

```text
I-03
Research → Evidence → Claims → Verification → Source Bundles
```

y recién después:

```text
I-04
Source Bundles → Curriculum Compiler → Learning Packs
```
