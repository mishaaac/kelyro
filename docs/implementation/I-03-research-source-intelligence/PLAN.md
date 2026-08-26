# Kelyro — Plan de Implementación I-03: Research & Source Intelligence

> Objetivo de esta implementación: construir la capa de investigación y evidencia de Kelyro. Al terminar I-03, Kelyro debe poder descubrir fuentes actuales, clasificarlas por autoridad y confianza, investigar temas mediante fuentes web/live respetando privacidad y políticas de red, almacenar evidencia y provenance, detectar versiones/releases/deprecaciones, verificar múltiples fuentes, resolver conflictos de forma explícita, calcular freshness y calidad, producir source bundles auditables y detectar drift entre conocimiento previamente verificado y fuentes actuales.
>
> I-03 **NO** compila todavía un Learning Pack de producción ni decide la estructura final del curriculum. Esa responsabilidad pertenece a I-04 — Curriculum Compiler & Learning Packs. I-03 entrega a I-04 evidencia estructurada, confiable, versionada y trazable.
>
> Este documento está diseñado para Spec-Driven Development con Codex en sesiones independientes. Cada paso debe poder planearse, implementarse, verificarse, registrarse y comitearse sin depender del historial del chat.

---

# Cobertura funcional de I-03

Esta implementación cubre las capacidades maestras:

```text
#167–209
Research Engine
Primary Sources First
Trust Policy
Trusted Registry
Tech Source Profile
Live Web Research
Discovery
Release Awareness
Stable/Preview/Experimental
Version-bound Curriculum
Version-aware Lessons
Release Notes
Deprecation
Historical Sources
Freshness Score
Last Verified
Curriculum Freshness
Provenance
Concept Citations
Lesson Citations
Deep Links
Source Bundle
Further Reading
Playground
Package Reference
Tutorial Discovery
Standards
Cross-Tech Sources
Per-topic Authority
Conflict Resolver
Multi-source Verification
Evidence Store
Research Audit
sources command
sources lesson
research command
Update Scan
Drift
Impact
Selective Migration contract
Student-safe Updates contract
Changelog evidence
Source-driven Compiler contract
```

y:

```text
#241–250
Source fallback
Offline Research Cache
Cost Control
Trigger Policies
Resource Quality
Community Resources
YouTube Supplement
Diversity
Real Source Code
Transparency
```

---

# Frontera de responsabilidad de I-03

I-03 debe construir:

```text
Research Query
Research Topic
Source
Source Registry
Authority Profile
Trust Policy
Discovery
Fetch
Normalization
Source Snapshot
Evidence
Evidence Claim
Provenance
Citation
Source Bundle
Freshness
Release Intelligence
Deprecation Intelligence
Conflict Detection
Verification
Research Cache
Research Audit
Update Scan
Drift Report
Impact Report
Research Commands
Contracts para I-04
```

I-03 no debe construir:

```text
Curriculum final
Lesson final
Personalized Learning Path
Full Curriculum Compiler
Learning Pack Marketplace
Exercise Engine
AI Tutor
Project Engine
Plugin Marketplace
External notifications
Automatic destructive curriculum migration
```

---

# Principios obligatorios de I-03

1. **La evidencia manda.**
2. **El LLM transforma evidencia; no reemplaza la evidencia.**
3. **Primary Sources First.**
4. **Official ≠ automáticamente correcto para todo contexto.**
5. **La autoridad depende del tema.**
6. **Cada afirmación importante debe poder rastrearse a evidencia.**
7. **La investigación debe ser reproducible en la medida razonable.**
8. **Las fuentes pueden cambiar; los snapshots deben conservarse.**
9. **Freshness y authority son dimensiones distintas.**
10. **Una fuente histórica puede ser útil aunque no sea current guidance.**
11. **No mezclar discovery con trust decision.**
12. **No mezclar fetch con parsing con evidence extraction.**
13. **No mezclar Research Engine con Curriculum Compiler.**
14. **No permitir que una fuente de baja autoridad sobrescriba silenciosamente una de alta autoridad.**
15. **Los conflictos se muestran; no se ocultan.**
16. **Network access respeta Foundation privacy gates.**
17. **Offline mode debe seguir pudiendo consultar evidencia cacheada.**
18. **No almacenar contenido completo innecesariamente por copyright.**
19. **Preferir metadata, excerpts mínimos y hashes sobre duplicación masiva.**
20. **URLs no son identidad suficiente; las fuentes necesitan IDs estables.**
21. **Los research runs deben ser auditables.**
22. **Los algoritmos de score deben ser explícitos y versionados.**
23. **No depender de un único motor de búsqueda/proveedor.**
24. **No asumir que todos los dominios son programación.**
25. **Los perfiles de autoridad son extensibles por dominio.**
26. **No inventar datos cuando una fuente no responde.**
27. **“No encontrado” ≠ “no existe”.**
28. **Cada dato temporal importante debe incluir “last verified”.**
29. **Release status debe distinguir stable/preview/experimental/legacy.**
30. **I-03 produce contratos para migración; I-04 decide cómo aplicar al curriculum.**

---

# Arquitectura conceptual objetivo

```text
Research Request
      ↓
Research Planner
      ↓
Discovery
      ↓
Candidate Sources
      ↓
Authority / Trust Evaluation
      ↓
Fetcher
      ↓
Normalizer
      ↓
Source Snapshot
      ↓
Evidence Extraction / Claims
      ↓
Verification / Conflict Resolution
      ↓
Source Bundle
      ↓
Evidence Store
      ↓
Freshness / Drift / Impact
      ↓
I-04 Curriculum Compiler
```

No todas las etapas necesitan IA.

En I-03, la infraestructura principal debe funcionar determinísticamente usando metadata, parsers, reglas y adapters. Un future AI Research Reviewer puede integrarse después a través de contratos explícitos.

---

## Paso 0 — Abrir formalmente I-03 y registrar baseline de I-02

- [x] Paso 0 completado

### Objetivo

Abrir I-03 sin perder trazabilidad de I-02 y establecer la nueva memoria persistente del repositorio.

### Precondiciones

I-02 debe estar:

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

Si existe release/tag final de I-02, registrarlo como baseline.

### Crear

```text
docs/
└── implementation/
    └── I-03-research-source-intelligence/
        ├── PLAN.md
        └── PROGRESS.md
```

### `PROGRESS.md`

```md
# I-03 Research & Source Intelligence — Progress Log

## Estado general

Current step: 0
Last completed step: none
Current release: <real current version>
Student Core baseline: <tag/commit real>

## Registro
```

### Actualizar `AGENTS.md`

Agregar reglas:

1. Leer PLAN/PROGRESS de I-03 antes de trabajar.
2. No construir I-04 prematuramente.
3. Toda fuente externa debe pasar por adapters.
4. No hacer network calls desde domain.
5. No escribir contenido web bruto ilimitado a SQLite.
6. No inventar claims.
7. Toda puntuación de trust/freshness/quality debe tener algoritmo versionado.
8. Respetar `privacy.allow_network`.
9. Tests unitarios no dependen de Internet real.
10. Integration tests de red deben ser explícitos/opt-in o usar fixtures/httptest.
11. No romper offline core de I-01/I-02.

### Prompt reutilizable

```text
Trabaja únicamente en el Paso XX de
docs/implementation/I-03-research-source-intelligence/PLAN.md.

Antes de modificar código:
1. Lee AGENTS.md.
2. Lee el Paso XX completo.
3. Lee PROGRESS.md.
4. Revisa git status.
5. Revisa commits recientes relevantes.
6. Inspecciona solo paquetes necesarios.
7. En Plan Mode, propone cómo ejecutar únicamente este paso.
8. No implementes I-04 ni pasos posteriores.
9. Ejecuta todos los criterios de verificación.
10. Si todo pasa, marca checkbox, actualiza PROGRESS.md y crea commit Conventional Commit.
```

### Esfuerzo recomendado

**High por defecto.**

Usar **xhigh** especialmente en:

```text
Paso 1   Domain model
Paso 4   Trust Policy
Paso 5   Authority Profiles
Paso 9   Fetch/normalization boundaries
Paso 13  Evidence/Claim model
Paso 16  Freshness
Paso 19  Release intelligence
Paso 23  Conflict resolution
Paso 24  Multi-source verification
Paso 38  Drift detection
Paso 39  Impact analysis
Paso 44  Security hardening
Paso 45  Performance/concurrency
Paso 49  Final closure
```

### Commit sugerido

```text
docs(roadmap): open I-03 Research and Source Intelligence
```

---

## Paso 1 — Diseñar el modelo de dominio de Research & Source Intelligence

- [x] Paso 1 completado

### Objetivo

Definir el lenguaje del dominio antes de diseñar tablas o adapters web.

### Paquetes sugeridos

```text
internal/
└── research/
    ├── source/
    ├── authority/
    ├── trust/
    ├── discovery/
    ├── fetch/
    ├── evidence/
    ├── provenance/
    ├── citation/
    ├── verification/
    ├── freshness/
    ├── release/
    ├── drift/
    ├── cache/
    └── audit/
```

Simplificar si demasiados paquetes dañan cohesión.

### Entidades/value objects

Como mínimo:

```text
ResearchRequest
ResearchRun
ResearchTopic
ResearchPurpose
Source
SourceID
SourceKind
SourceLocator
SourceVersion
AuthorityProfile
AuthorityTier
TrustDecision
TrustReason
DiscoveredSource
SourceSnapshot
SourceMetadata
FetchMetadata
Evidence
Claim
ClaimID
ClaimType
ClaimConfidence
Provenance
Citation
DeepLink
SourceBundle
FreshnessState
FreshnessScore
ReleaseRecord
ReleaseChannel
ReleaseStatus
DeprecationRecord
Conflict
VerificationResult
DriftReport
ImpactReport
```

### Tipos de source iniciales

```text
official_documentation
specification
standard
release_notes
official_blog
package_reference
official_tutorial
source_code
issue_tracker
community_article
community_forum
video
paper
book_reference
other
```

No encerrar dominio futuro en tecnología.

### Invariantes

- IDs no vacíos;
- URL/locator validado;
- snapshot tiene `fetched_at`;
- evidence referencia snapshot;
- claim no puede existir sin source/evidence;
- citations apuntan a evidence/source snapshot;
- confidence dentro de rango;
- freshness score dentro de rango;
- release version no puede ser vacía;
- timestamps UTC;
- status enums válidos.

### Documentación

Crear:

```text
docs/architecture/research-domain.md
```

### Tests

- IDs;
- locators;
- confidence;
- source kinds;
- invalid timestamps;
- provenance relationships.

### Commit sugerido

```text
feat(research): define research and source domain model
```

---

## Paso 2 — Definir repositories y application services de Research

- [x] Paso 2 completado

### Objetivo

Separar dominio de SQLite, HTTP y CLI.

### Repositories

Diseñar interfaces pequeñas:

```text
SourceRepository
SnapshotRepository
EvidenceRepository
ResearchRunRepository
TrustRegistryRepository
ReleaseRepository
FreshnessRepository
DriftRepository
ResearchCacheRepository
```

### Application services

```text
ResearchService
DiscoveryService
SourceService
VerificationService
FreshnessService
ReleaseIntelligenceService
DriftService
ImpactService
```

### Adapters futuros

Contratos para:

```text
SearchProvider
SourceFetcher
SourceNormalizer
MetadataExtractor
Clock
```

### No hacer

Una mega-interface:

```go
type ResearchRepository interface { /* everything */ }
```

### Tests

Fakes in-memory y tests de error mapping.

### Commit sugerido

```text
refactor(research): define Research Engine service boundaries
```

---

## Paso 3 — Añadir persistence schema y migrations de I-03

- [x] Paso 3 completado

### Objetivo

Persistir research state utilizando SQLite de Foundation.

### Tablas sugeridas

```text
research_runs
research_topics
sources
source_aliases
source_snapshots
trust_registry
authority_profiles
evidence
claims
claim_sources
citations
source_bundles
source_bundle_items
release_records
deprecation_records
freshness_state
verification_results
source_conflicts
research_cache_entries
drift_reports
impact_reports
```

Puede omitirse almacenamiento derivado si se recalcula eficientemente.

### Reglas

1. Forward-only migrations.
2. No modificar migration previa.
3. Foreign keys.
4. Índices para source locator, latest snapshot, claim topic, last verified, release/version y research run.
5. No almacenar secretos.
6. No almacenar cuerpos web completos sin política.
7. Hash de contenido cuando aporte reproducibilidad.
8. Metadata separada de excerpts.

### Tests

- I-02 → I-03 migration;
- new DB;
- roundtrip repositories;
- constraints;
- duplicate source handling.

### Commit sugerido

```text
feat(storage): add Research Engine persistence schema
```

---

## Paso 4 — Implementar Trust Policy v1

- [x] Paso 4 completado

### Objetivo

Definir cómo Kelyro decide si una fuente es apropiada para sustentar conocimiento.

### Dimensiones

No usar un único booleano `trusted`.

Modelar como mínimo:

```text
authority
freshness
relevance
directness
stability
corroboration
```

### Authority tiers ejemplo

```text
A — normative/official primary
B — official supporting
C — strong secondary expert
D — community supplementary
E — unverified/low confidence
```

### Policy rules

Ejemplo:

```text
language specification
→ specification > official docs > trusted expert > community

security advisory
→ vendor advisory + recognized security authority

package API
→ official package reference/source > tutorials

historical behavior
→ archived docs/release notes may outrank current docs
```

### Output

```text
TrustDecision
- accepted
- accepted_as_supplement
- requires_verification
- rejected
- reason codes
```

### Version

```text
trust-policy-v1
```

### Documentar

```text
docs/architecture/trust-policy-v1.md
```

### Tests

- normative source;
- community only;
- stale official;
- conflicting official/historical;
- low quality;
- missing metadata.

### Commit sugerido

```text
feat(trust): add versioned source trust policy v1
```

---

## Paso 5 — Implementar Authority Profiles por dominio y tópico

- [x] Paso 5 completado

### Objetivo

Evitar una lista global de “sitios buenos” incapaz de modelar autoridad contextual.

### Concepto

```text
AuthorityProfile
domain
topic pattern
preferred source kinds
preferred domains/organizations
minimum corroboration
allowed supplementary kinds
```

### Primer perfil de referencia

Como fixture, crear un **Technology / Software** profile.

Para un futuro pack de Go podría reconocer fuentes como:

```text
go.dev/doc
go.dev/ref/spec
pkg.go.dev
go.dev/dl
go.dev/doc/devel/release
github.com/golang/go
```

Pero no hardcodear Go dentro del core.

### Profiles data-driven

```text
assets/
└── research/
    └── authority-profiles/
        └── technology-software.yaml
```

### Validación

- duplicate IDs;
- invalid domain patterns;
- unknown source kinds;
- contradictory rules.

### Tests

- topic matching;
- precedence;
- fallback;
- custom future domain.

### Commit sugerido

```text
feat(authority): add topic-aware authority profiles
```

---

## Paso 6 — Construir Trusted Source Registry

- [x] Paso 6 completado

### Objetivo

Mantener fuentes/organizaciones conocidas con metadata y razones de confianza.

### Registry entry

```text
id
organization
canonical_domains
source kinds
authority hints
domains/topics
notes
status
added_at
last_reviewed_at
```

### Status

```text
trusted
conditional
historical
deprecated
blocked
```

### Importante

Registry ≠ verdad absoluta.

Trust Policy decide usando:

```text
registry + topic + source kind + freshness + corroboration
```

### CLI administrativa inicial

```bash
kelyro sources registry list
kelyro sources registry show <id>
```

### Tests

- domain normalization;
- subdomain rules;
- blocked source;
- historical source;
- duplicate domain.

### Commit sugerido

```text
feat(sources): add trusted source registry
```

---

## Paso 7 — Integrar privacy/network gate con Research Engine

- [x] Paso 7 completado

### Objetivo

Garantizar que ninguna investigación live bypassée las políticas de Foundation.

### Regla

Si:

```text
privacy.allow_network = false
```

entonces:

```text
discovery live       → blocked
fetch live           → blocked
release lookup live  → blocked
```

pero:

```text
offline cache
stored evidence
source registry
freshness metadata
```

siguen disponibles.

### Error model

No usar error genérico. Crear un error categorizable equivalente a:

```text
NetworkResearchBlocked
```

### Research mode

```text
offline
online
auto
```

`auto` respeta config.

### Tests

- blocked;
- allowed;
- cached fallback;
- no accidental provider call.

### Commit sugerido

```text
feat(research): enforce privacy gate for live research
```

---

## Paso 8 — Implementar Research HTTP Client seguro y configurable

- [x] Paso 8 completado

### Objetivo

Crear infraestructura de fetch robusta sin usar `http.Get` disperso.

### Requisitos

1. Timeouts.
2. Redirect limit.
3. User-Agent de Kelyro.
4. Max response size.
5. Content-Type validation.
6. TLS defaults seguros.
7. Context cancellation.
8. Retry solo en errores transitorios.
9. Backoff limitado.
10. No retry indiscriminado de 4xx.
11. Rate limit hooks.
12. Redaction en logs.
13. No guardar auth headers.
14. SSRF protection para URLs no confiables: loopback/private metadata endpoints bloqueados por defecto.
15. Proxy respetando comportamiento estándar cuando aplique.
16. Compresión soportada de forma segura.

### Tests

Usar `httptest.Server`.

Cubrir timeout, redirect, oversize, 404, 429, 500 y cancellation.

### Commit sugerido

```text
feat(research): add hardened source HTTP client
```

---

## Paso 9 — Implementar Source Fetcher y Source Snapshot

- [x] Paso 9 completado

### Objetivo

Descargar una fuente y conservar metadata reproducible.

### Snapshot fields

```text
snapshot_id
source_id
locator
fetched_at
status_code
content_type
etag
last_modified
content_hash
content_length
fetch_version
```

### Body policy

No asumir que siempre se persiste body completo.

Definir:

```text
metadata only
normalized excerpt
bounded cached body
```

según source policy.

### Conditional requests

Soportar `ETag`, `If-Modified-Since` y `304` cuando corresponda.

### Requisitos

- immutable snapshot metadata;
- new fetch → new snapshot o referencia 304;
- canonical content hash;
- no overwrite histórico.

### Tests

- changed body;
- unchanged ETag;
- 304;
- invalid content type;
- size limits.

### Commit sugerido

```text
feat(fetch): add immutable source snapshots
```

---

## Paso 10 — Implementar Source Normalization pipeline

- [x] Paso 10 completado

### Objetivo

Transformar HTML/text/JSON/documentation responses a representación investigable sin conservar ruido innecesario.

### NormalizedSource

Puede incluir:

```text
title
canonical URL
language
headings
plain text segments
code blocks metadata
links
published/updated date
version hints
```

### Parsers iniciales

```text
text/html
text/plain
application/json
text/markdown cuando la fuente sea directa
```

PDF puede quedar detrás de adapter posterior; no bloquear I-03 por un parser PDF completo.

### Sanitización

- remover scripts/styles;
- normalize whitespace;
- preserve heading hierarchy;
- preserve code fences útiles;
- no ejecutar HTML;
- canonicalize links.

### Tests

Golden fixtures.

### Commit sugerido

```text
feat(research): normalize fetched source documents
```

---

## Paso 11 — Implementar Source Discovery abstraction

- [x] Paso 11 completado

### Objetivo

Descubrir candidatos sin atarse a un buscador específico.

### SearchProvider

```text
Search(ctx, query, options) []SearchResult
```

### SearchResult

```text
title
url
snippet
provider
rank
published hint
```

### Importante

Search result no es evidence.

Debe pasar por:

```text
discover
→ classify
→ fetch
→ verify
```

### Provider inicial

Puede ser fake/stub primero y adapter real después. Nunca hardcodear API keys.

### Tests

- provider error;
- duplicate URLs;
- normalization;
- deterministic rank preservation.

### Commit sugerido

```text
feat(discovery): add pluggable source discovery
```

---

## Paso 12 — Implementar Research Query Planner v1

- [x] Paso 12 completado

### Objetivo

Convertir un tópico en consultas de investigación deterministas.

### Sin IA obligatoria

Planner v1 puede generar variantes para:

```text
official docs
specification
release notes
deprecation
API reference
tutorial
source code
security
```

según `ResearchPurpose`.

### ResearchPurpose

```text
concept_definition
current_usage
version_behavior
release_status
deprecation_check
prerequisite_research
production_practice
security_guidance
```

### Inputs

```text
topic
domain
technology
target version
purpose
authority profile
```

### Output

```text
ResearchQueryPlan
- query
- desired source kind
- required authority
- priority
```

### Version

```text
query-planner-v1
```

### Tests

- definition;
- release;
- security;
- non-tech generic topic.

### Commit sugerido

```text
feat(research): add deterministic research query planner v1
```

---

## Paso 13 — Implementar Evidence y Claim Model

- [x] Paso 13 completado

### Objetivo

Representar lo que una fuente realmente soporta.

### Diferencia

```text
Source = documento/recurso
Evidence = fragmento/observación obtenida
Claim = afirmación estructurada respaldada por evidence
```

### Claim fields

```text
claim_id
topic
statement
claim_type
scope
version_scope
status_scope
confidence
created_at
```

### Evidence fields

```text
evidence_id
snapshot_id
location
excerpt
excerpt_hash
context_before/after bounded
extracted_at
extractor_version
```

### Copyright-aware

Excerpt debe ser **mínimo necesario**. No duplicar artículos enteros.

### Claim types

```text
definition
requirement
behavior
version_change
deprecation
recommendation
warning
example
compatibility
security
historical
```

### Tests

- claim without evidence rejected;
- excerpt bounds;
- multiple evidence per claim;
- version scope.

### Commit sugerido

```text
feat(evidence): add structured claims and evidence
```

---

## Paso 14 — Implementar Provenance Graph

- [x] Paso 14 completado

### Objetivo

Poder contestar:

```text
¿De dónde salió esta afirmación?
```

### Graph

```text
ResearchRun
   ↓
Query
   ↓
DiscoveredSource
   ↓
Source
   ↓
Snapshot
   ↓
Evidence
   ↓
Claim
   ↓
SourceBundle
   ↓
future Curriculum Concept
```

### Requisitos

1. IDs estables.
2. No ciclos inválidos.
3. Timestamps.
4. Algorithm/tool versions.
5. Human-readable explain.
6. Exportable.

### CLI interna

```bash
kelyro sources trace <claim-id>
```

### Tests

- full chain;
- missing node;
- multiple sources;
- historical snapshot.

### Commit sugerido

```text
feat(provenance): add research provenance graph
```

---

## Paso 15 — Implementar Citations y Deep Links

- [x] Paso 15 completado

### Objetivo

Generar referencias que lleven al estudiante/reviewer al lugar más específico posible.

### Citation model

```text
source title
canonical URL
deep link
section/heading
snapshot date
version scope
last verified
```

### Deep-link strategies

Según fuente:

```text
URL anchors
package symbols
spec sections
release heading
source-host file + line/commit permalink
```

### Fallback

Si no existe deep link estable:

```text
canonical URL + heading/path hint
```

### Tests

- anchor;
- no anchor;
- source-code permalink;
- invalid URL.

### Commit sugerido

```text
feat(citation): add stable source citations and deep links
```

---

## Paso 16 — Implementar Freshness Model v1

- [x] Paso 16 completado

### Objetivo

Medir qué tan reciente/confiablemente verificada está una evidencia.

### No confundir

```text
source publication date
snapshot fetched date
last verified date
freshness score
```

### Inputs

```text
last_verified_at
source_updated_at
technology release cadence
claim type
source kind
known new release
```

### Output

```text
fresh
aging
stale
unknown
```

más score.

### Version

```text
freshness-v1
```

### Configuración

Authority profile puede definir TTL hints por claim/source type.

### Tests

Clock injectable; límites de TTL; release trigger.

### Documentación

```text
docs/architecture/freshness-v1.md
```

### Commit sugerido

```text
feat(freshness): add versioned evidence freshness model
```

---

## Paso 17 — Implementar Last Verified y Refresh Scheduling

- [x] Paso 17 completado

### Objetivo

Saber cuándo revisar otra vez una fuente/claim.

### Scheduling metadata

```text
last_verified_at
next_verify_at
verification_reason
priority
```

### Triggers

```text
TTL expired
new release detected
source changed
conflict unresolved
security-sensitive
manual request
```

### No background daemon aún

I-03 crea scheduling state. Automatizaciones reales pueden venir después.

### CLI

```bash
kelyro sources stale
```

### Tests

- due;
- not due;
- release trigger;
- manual trigger.

### Commit sugerido

```text
feat(freshness): schedule source reverification
```

---

## Paso 18 — Implementar Resource Quality Model v1

- [x] Paso 18 completado

### Objetivo

Evaluar utilidad pedagógica/técnica sin confundirla con autoridad.

### Dimensiones

```text
accuracy confidence
clarity
specificity
depth
maintainability
examples
accessibility
noise
```

### Distinción importante

```text
high authority + low pedagogy
```

puede ser excelente como evidence pero no como Further Reading.

### Output

```text
quality score
quality reasons
recommended use:
- evidence
- further reading
- example
- supplementary
- reject
```

### Version

```text
resource-quality-v1
```

### Commit sugerido

```text
feat(sources): add resource quality scoring v1
```

---

## Paso 19 — Implementar Release Intelligence Model

- [x] Paso 19 completado

### Objetivo

Representar versiones/releases de tecnologías y su estado.

### Entity

```text
TechnologyRelease
technology_id
version
released_at
channel
status
source_ids
verified_at
```

### Channels/status

```text
stable
preview
beta
rc
experimental
nightly
legacy
eol
unknown
```

No imponer SemVer a tecnologías que no lo usan.

### Version identity

Crear abstraction equivalente a:

```text
VersionIdentifier
```

que soporte SemVer cuando aplique y otros esquemas cuando no.

### Tests

- semantic;
- date-based;
- non-semver;
- preview;
- stable.

### Commit sugerido

```text
feat(releases): add technology release intelligence model
```

---

## Paso 20 — Implementar Release Discovery y Release Notes ingestion

- [ ] Paso 20 completado

### Objetivo

Descubrir releases actuales desde fuentes autorizadas.

### Sources prioritarias

Por Authority Profile:

```text
official release pages
official changelogs
official repositories/tags
official package registries
```

### Requisitos

1. Provider-specific adapters detrás de interfaces.
2. No depender solo de GitHub.
3. Snapshot release notes.
4. Claims de cambio version-scoped.
5. Current stable detectable.
6. Preview separado.
7. No auto-upgrade curriculum.

### Tests

- new stable;
- preview;
- no releases;
- malformed release feed;
- duplicate release.

### Commit sugerido

```text
feat(releases): discover releases and release notes
```

---

## Paso 21 — Implementar Deprecation & Legacy Intelligence

- [ ] Paso 21 completado

### Objetivo

Detectar cuando una práctica/API/versión quedó deprecated, superseded o legacy.

### DeprecationRecord

```text
subject
introduced/version
deprecated/version
removed/version
replacement
status
evidence
```

### Status

```text
deprecated
removed
legacy
historical_only
superseded
```

### Requisitos

1. No inferir de ausencia en docs.
2. Requerir evidence explícita o multi-source strong inference marcada.
3. Replacement opcional.
4. Conservar historical guidance.

### Commit sugerido

```text
feat(research): add deprecation and legacy intelligence
```

---

## Paso 22 — Implementar Historical Source handling

- [ ] Paso 22 completado

### Objetivo

No descartar fuentes antiguas cuando son necesarias para entender versiones previas.

### Source temporal scope

```text
current
historical
version_bound
archived
```

### Reglas

- historical no se recomienda como current guidance sin warning;
- puede ser autoridad principal para comportamiento de versión antigua;
- citations indican scope;
- source bundles distinguen current/historical.

### Tests

- archived docs;
- old release notes;
- conflicting current vs historical.

### Commit sugerido

```text
feat(sources): support historical and version-bound sources
```

---

## Paso 23 — Implementar Conflict Detection & Resolver v1

- [ ] Paso 23 completado

### Objetivo

Detectar claims incompatibles y producir una resolución explicable.

### Conflict types

```text
direct contradiction
version mismatch
temporal mismatch
scope mismatch
recommendation disagreement
authority mismatch
```

### Resolver

No hacer:

```text
pick highest score and hide conflict
```

Debe producir:

```text
resolution
confidence
reason
winning scope/source if applicable
unresolved flag
```

### Rules examples

```text
current official vs old tutorial
→ likely temporal conflict

spec vs community blog
→ normative claim favors spec

two official docs same version conflict
→ unresolved/escalate
```

### Version

```text
conflict-resolver-v1
```

### Tests

- clear authority;
- temporal;
- version;
- unresolved official conflict.

### Documentación

```text
docs/architecture/conflict-resolver-v1.md
```

### Commit sugerido

```text
feat(verification): add source conflict resolver v1
```

---

## Paso 24 — Implementar Multi-Source Verification

- [ ] Paso 24 completado

### Objetivo

Definir cuándo un claim necesita corroboración.

### Policy

Por claim type:

```text
normative definition
→ primary source may suffice

production recommendation
→ prefer 2+ strong sources

security-sensitive claim
→ authoritative security source required

community technique
→ corroboration required
```

### VerificationResult

```text
verified
verified_with_caveat
insufficient_evidence
conflicted
rejected
```

### Metrics

- source count;
- independent organization count;
- authority distribution;
- scope consistency.

### Regla

Mirrors o páginas de la misma organización no cuentan automáticamente como fuentes independientes.

### Commit sugerido

```text
feat(verification): add multi-source claim verification
```

---

## Paso 25 — Implementar Source Bundle

- [ ] Paso 25 completado

### Objetivo

Empaquetar evidencia suficiente para que I-04 pueda compilar un concepto/lesson sin volver a investigar todo desde cero.

### Bundle

```text
bundle_id
research_topic
purpose
target version
claims
primary sources
supporting sources
historical sources
conflicts
freshness
verified_at
research_run
```

### Requisitos

1. Immutable/versioned bundle.
2. Hash reproducible.
3. No incluir contenido completo innecesario.
4. Human-readable summary.
5. Machine-readable representation.
6. Bundle state:
   - ready
   - ready_with_caveats
   - incomplete
   - conflicted

### Tests

- deterministic serialization;
- bundle hash;
- missing required evidence.

### Commit sugerido

```text
feat(research): add versioned source bundles
```

---

## Paso 26 — Implementar Further Reading Selection

- [ ] Paso 26 completado

### Objetivo

Seleccionar recursos útiles para el estudiante aparte de las fuentes estrictamente probatorias.

### Categorías

```text
official deep dive
tutorial
interactive resource
reference
community explanation
video supplement
source code
```

### Selection

Considerar quality, authority, reading level, freshness, duplication y diversity.

### Reglas

1. Primary evidence no necesariamente es mejor reading material.
2. Community sources siempre etiquetadas.
3. No esconder paywall.
4. No recomendar stale tutorial sin warning.
5. Limitar cantidad razonable.

### Commit sugerido

```text
feat(sources): select curated further reading
```

---

## Paso 27 — Añadir source kinds especializados: Playground, Package Reference y Standards

- [ ] Paso 27 completado

### Objetivo

Representar recursos técnicos especialmente útiles.

### Playground

```text
interactive
language/runtime
version
official/community
shareable URL
```

### Package Reference

```text
package/module
symbol
version
canonical docs
source code link
```

### Standards

```text
standard body
standard ID
revision
status
official locator
```

### No hardcodear Go.

### Commit sugerido

```text
feat(sources): support playgrounds package references and standards
```

---

## Paso 28 — Implementar Community Resource Policy

- [ ] Paso 28 completado

### Objetivo

Permitir fuentes comunitarias sin tratarlas como equivalentes a documentación normativa.

### Resource types

```text
blog
forum
Q&A
conference talk
community tutorial
repository example
```

### Reglas

1. `supplementary` por defecto.
2. Puede elevarse según authority profile.
3. Autor/organización opcionalmente evaluable.
4. Freshness importante.
5. No usar votos/popularity como verdad.
6. Comments no son evidence fuerte.
7. Attribution clara.

### Commit sugerido

```text
feat(sources): add community resource trust policy
```

---

## Paso 29 — Implementar Video Supplement metadata

- [ ] Paso 29 completado

### Objetivo

Permitir videos como recurso suplementario sin convertir transcripts en fuente primaria por defecto.

### Metadata

```text
video URL
channel
publisher
published_at
duration
title
description
official/community
transcript availability
```

### Reglas

1. Video supplementary por defecto.
2. Official conference/video puede tener autoridad superior según topic.
3. Transcript no debe almacenarse completo sin necesidad.
4. Deep links a timestamp si se conoce.
5. No depender de YouTube específicamente en el domain.

### Commit sugerido

```text
feat(sources): support video learning resources
```

---

## Paso 30 — Implementar Source Diversity policy

- [ ] Paso 30 completado

### Objetivo

Evitar bundles aparentemente corroborados que en realidad dependen de la misma fuente/organización.

### Diversity dimensions

```text
organization
source kind
perspective
implementation/reference
geography/language future
```

### Importante

No perseguir diversidad por sí misma si existe una fuente normativa única.

### Output

```text
diversity assessment
independent source count
warnings
```

### Commit sugerido

```text
feat(verification): add source diversity assessment
```

---

## Paso 31 — Implementar Real Source Code Evidence

- [ ] Paso 31 completado

### Objetivo

Permitir que comportamiento técnico pueda verificarse contra implementación real cuando sea apropiado.

### SourceCodeLocator

```text
repository
commit
path
line range
symbol
```

### Requisitos

1. Preferir permalink por commit.
2. No apuntar solo a `main` si evidence debe ser reproducible.
3. Source code no reemplaza spec cuando spec es normativa.
4. Version scope.
5. Bounded excerpt.
6. License metadata si disponible.

### Adapters

GitHub puede ser primer adapter, pero interface host-neutral.

### Commit sugerido

```text
feat(evidence): support reproducible source-code evidence
```

---

## Paso 32 — Implementar Research Cache y Offline Research Mode

- [ ] Paso 32 completado

### Objetivo

Permitir usar investigación ya realizada sin Internet.

### Cache layers

```text
discovery cache
fetch metadata cache
bounded source cache
normalized source cache
source bundle cache
```

### Requisitos

1. TTL por tipo.
2. Cache hit explícito.
3. Stale cache usable en offline con warning.
4. No llamar red en offline.
5. Size limits.
6. Eviction.
7. Corruption detection.
8. Foundation cache dir conventions.
9. Cache no es source of truth de historical evidence.
10. Stored evidence/snapshots importantes sobreviven eviction.

### CLI

```bash
kelyro research cache status
kelyro research cache clear
```

Clear nunca debe borrar evidence persistente sin confirmación explícita.

### Commit sugerido

```text
feat(cache): add offline research cache
```

---

## Paso 33 — Implementar Research Cost Control

- [ ] Paso 33 completado

### Objetivo

Evitar búsquedas/fetches innecesarios y preparar uso futuro de APIs pagadas.

### Cost dimensions

```text
search requests
fetch requests
bytes
provider API calls
future model calls
```

### Budget

```text
per run
per topic
daily optional
```

### Reglas

1. Cache before network when valid.
2. Stop when verification requirements satisfied.
3. No descargar 50 fuentes si 3 primarias bastan.
4. User-visible reason cuando budget detiene research.
5. Cost metadata en ResearchRun.
6. Sin dinero real obligatorio en I-03; unidades genéricas.

### CLI

```bash
kelyro research stats
```

### Commit sugerido

```text
feat(research): add bounded research cost controls
```

---

## Paso 34 — Implementar Research Trigger Policies

- [ ] Paso 34 completado

### Objetivo

Decidir cuándo investigar/reinvestigar.

### Triggers

```text
manual
missing evidence
freshness expired
new technology release
deprecation detected
conflict unresolved
curriculum compile request future
security-sensitive refresh
```

### Policy

```text
research-trigger-v1
```

### No scheduler externo

Solo decisión/queue metadata.

### Commit sugerido

```text
feat(research): add research trigger policy v1
```

---

## Paso 35 — Implementar `kelyro research` y `kelyro sources`

- [ ] Paso 35 completado

### Objetivo

Hacer Research Engine inspeccionable desde CLI.

### Comandos mínimos

```bash
kelyro research topic <topic>
kelyro research status <run-id>
kelyro research stats

kelyro sources
kelyro sources list
kelyro sources show <source-id>
kelyro sources trace <claim-id>
kelyro sources stale
kelyro sources conflicts
kelyro sources registry list
```

### `research topic`

En I-03 puede:

1. construir query plan;
2. usar discovery;
3. fetch;
4. classify;
5. producir claims/bundle.

Debe respetar network privacy.

### Output

Humano primero.

```text
Research: Go range-over-func

Status: ready with caveats
Primary sources: 3
Supporting sources: 2
Conflicts: 0
Last verified: <timestamp>
```

### Tests

CLI routing/output/errors.

### Commit sugerido

```text
feat(cli): expose Research and Source Intelligence commands
```

---

## Paso 36 — Implementar Source Transparency views en TUI

- [ ] Paso 36 completado

### Objetivo

Mostrar al usuario/reviewer qué sabe Kelyro y de dónde sale.

### Pantallas

```text
Research
Sources
Source detail
Claim detail
Conflicts
Freshness
```

### Ejemplo conceptual

```text
Sources · Go interfaces

✓ Go Language Specification        Primary
✓ Official documentation           Primary
○ Community article                Supplement

Last verified: 2 days ago
Conflicts: none
```

### Requisitos

1. Mostrar source kind.
2. Mostrar authority.
3. Mostrar freshness.
4. Mostrar historical warning.
5. Mostrar version scope.
6. No mostrar URLs enormes sin formato.
7. Open in browser via Platform abstraction de I-01.
8. No hacer network fetch por cada render.

### Commit sugerido

```text
feat(tui): add research source transparency views
```

---

## Paso 37 — Implementar Update Scan

- [ ] Paso 37 completado

### Objetivo

Comparar evidencia almacenada con señales actuales de cambio.

### Scan

```text
known technologies
known releases
tracked sources
freshness due
```

### Output

```text
UpdateScan
- new release
- changed source
- stale evidence
- deprecated subject
- unresolved conflict
```

### No modificar curriculum

Solo reporta.

### CLI

```bash
kelyro research update-scan
```

### Offline

Puede reportar scan incompleto por network disabled y analizar metadata local.

### Commit sugerido

```text
feat(research): add source update scanning
```

---

## Paso 38 — Implementar Drift Detection v1

- [ ] Paso 38 completado

### Objetivo

Detectar cuando claims previamente verificadas ya no coinciden con evidencia actual.

### Drift types

```text
source_changed
claim_invalidated
version_superseded
recommendation_changed
deprecation_introduced
scope_changed
```

### Inputs

```text
old source bundle
new snapshots/claims
release data
```

### Output

```text
DriftReport
severity
affected claims
old evidence
new evidence
confidence
```

### Severity

```text
informational
minor
important
critical
```

### Version

```text
drift-v1
```

### Tests

- wording change no semantic drift;
- version change;
- deprecation;
- source gone;
- unresolved.

### Documentar

```text
docs/architecture/drift-v1.md
```

### Commit sugerido

```text
feat(drift): add evidence drift detection v1
```

---

## Paso 39 — Implementar Impact Analysis

- [ ] Paso 39 completado

### Objetivo

Traducir drift a “qué podría verse afectado” sin modificar todavía contenido.

### Impact targets

I-03 conoce referencias/contracts, no curriculum final.

Puede reportar:

```text
source bundle IDs
claim IDs
future concept refs
future lesson refs
technology version refs
```

### Output

```text
ImpactReport
affected evidence
affected bundles
severity
recommended action
```

### Recommended actions

```text
no_action
reverify
review_curriculum
recompile_future
manual_review
```

### Commit sugerido

```text
feat(drift): add research impact analysis
```

---

## Paso 40 — Definir Selective Migration y Student-safe Update contracts para I-04

- [ ] Paso 40 completado

### Objetivo

Diseñar la frontera para que I-04 pueda actualizar curriculum sin destruir progreso.

### I-03 produce

```text
DriftReport
ImpactReport
SourceBundle old/new
ChangeClassification
```

### I-04 decidirá

```text
recompile concept
add concept
deprecate concept
update lesson
migrate curriculum version
protect student progress
```

### Contrato

Crear:

```text
docs/architecture/research-to-curriculum-update-contract.md
```

Debe cubrir stable concept IDs, source bundle version, breaking/non-breaking knowledge change, suggested migration class y student state untouched by I-03.

### No implementar

Ninguna migration curricular real.

### Commit sugerido

```text
feat(research): define curriculum update intelligence contract
```

---

## Paso 41 — Definir Source-driven Compiler contract para I-04

- [ ] Paso 41 completado

### Objetivo

Entregar a I-04 una API clara para consumir investigación.

### I-04 debe poder pedir

```text
GetReadyBundles(topic)
GetClaims(topic/version)
GetPrimarySources(topic)
GetConflicts(topic)
GetFreshness(topic)
RequireVerification(topic)
```

### Compiler eligibility

Research bundle puede declarar:

```text
ready_for_compile
ready_with_caveats
not_ready
```

### Reasons

```text
missing primary source
stale
conflicted
insufficient corroboration
version unknown
```

### Regla crítica

I-04 no debe compilar silenciosamente un concepto crítico desde bundle `not_ready`.

### Commit sugerido

```text
feat(research): define source-driven compiler contract
```

---

## Paso 42 — Implementar Research Audit Trail y reproducibility metadata

- [ ] Paso 42 completado

### Objetivo

Permitir auditar cómo se produjo un bundle.

### ResearchRun metadata

```text
run_id
started_at
completed_at
query planner version
trust policy version
freshness version
conflict resolver version
providers used
network mode
cache hits
source count
bytes fetched
outcome
```

### Reproducibility

Guardar queries, source locators, snapshot hashes, algorithm versions y target technology/version.

No prometer que Internet futuro devolverá exactamente lo mismo.

### CLI

```bash
kelyro research show <run-id>
```

### Commit sugerido

```text
feat(audit): add reproducible research run metadata
```

---

## Paso 43 — Copyright, licensing y content retention hardening

- [ ] Paso 43 completado

### Objetivo

Evitar convertir Kelyro en un mirror indiscriminado de contenido externo.

### Política

1. Guardar URLs + metadata + hashes.
2. Excerpts mínimos para evidencia.
3. Full body cache bounded/temporary cuando sea necesario.
4. No exportar bodies externos por defecto.
5. License metadata para source code cuando disponible.
6. No incluir videos/transcripts completos.
7. Source bundle export usa claims/citations/excerpts mínimos.
8. Cache eviction configurable.
9. Respetar `no-store` cuando técnicamente/políticamente apropiado.
10. No fabricar licencias.

### Documentación

```text
docs/architecture/source-content-retention-policy.md
```

### Commit sugerido

```text
fix(research): harden external content retention policy
```

---

## Paso 44 — Security hardening de Research Engine

- [ ] Paso 44 completado

### Objetivo

Tratar Internet como input no confiable.

### Revisar

```text
SSRF
redirect abuse
oversized responses
decompression bombs
malformed HTML
URL normalization
path traversal en cache
header injection
log injection
credential leakage
malicious filenames
content-type spoofing
```

### Prompt injection

Aunque I-03 no use LLM obligatoriamente, contenido web puede contener instrucciones maliciosas.

Definir desde ya:

> External source content is data, never instructions.

Si future AI extractor se conecta:

- source text delimitado;
- tool permissions separadas;
- no obedecer instrucciones dentro de fuente;
- no secrets in context salvo necesidad explícita.

### Tests/fuzz

- URLs;
- parser;
- malicious HTML;
- cache paths.

### Commit sugerido

```text
fix(security): harden Research Engine inputs
```

---

## Paso 45 — Performance, concurrency y rate-limit hardening

- [ ] Paso 45 completado

### Objetivo

Permitir research runs con múltiples fuentes sin saturar red ni SQLite.

### Concurrency

Bounded worker pool. Nunca goroutine ilimitada por URL.

### Limits

```text
max concurrent discovery
max concurrent fetch
per-host concurrency
rate limit
global run budget
```

### SQLite

Batch writes/transacciones.

### Bench fixture

Simular:

```text
100 queries
500 candidate URLs
200 fetched sources
5,000 claims
```

sin Internet real.

### Tests

- cancellation;
- worker leaks;
- rate limit;
- deterministic ordering donde aplique;
- race tests.

### Commit sugerido

```text
perf(research): bound concurrent source processing
```

---

## Paso 46 — E2E Research Engine con fixture HTTP controlado

- [ ] Paso 46 completado

### Objetivo

Probar todo el pipeline sin depender de Internet público.

### Crear test server con

```text
official docs
release notes
historical page
community article
conflicting page
changed page
ETag/304
rate limit endpoint
```

### E2E

```text
research request
→ query plan
→ discovery fake
→ authority classify
→ fetch
→ normalize
→ evidence
→ claims
→ verification
→ source bundle
→ persistence
→ CLI inspect
```

### Escenarios

1. Primary source sufficient.
2. Needs corroboration.
3. Conflict.
4. Historical vs current.
5. New release.
6. Deprecation.
7. Offline cache.
8. Stale cache.
9. Privacy blocks live.
10. Source changes → drift.

### CI

Linux/Windows/macOS donde corresponda.

### Commit sugerido

```text
test(e2e): cover Research Engine evidence pipeline
```

---

## Paso 47 — Controlled live-web integration tests

- [ ] Paso 47 completado

### Objetivo

Verificar adapters reales sin convertir CI en flaky.

### Reglas

1. Opt-in:
   ```text
   KELYRO_LIVE_RESEARCH_TESTS=1
   ```
2. No ejecutar por defecto en CI.
3. Usar pocas fuentes estables.
4. Timeouts estrictos.
5. No assertions sobre texto exacto frágil.
6. Verificar reachable, classification, metadata, snapshot hash y privacy gate.
7. No requerir API pagada para suite básica.

### Commit sugerido

```text
test(research): add opt-in live source integration checks
```

---

## Paso 48 — Dogfooding de Research & Source Intelligence

- [ ] Paso 48 completado

### Objetivo

Usar I-03 sobre temas tecnológicos reales antes de I-04.

### Topics sugeridos

Elegir varios perfiles:

```text
1. concepto estable
2. API/version behavior
3. release reciente
4. feature experimental
5. deprecated API
6. security guidance
7. historical behavior
8. community-heavy topic
```

### Para cada uno revisar

- discovery quality;
- authority ranking;
- primary source selection;
- source snapshots;
- citations;
- freshness;
- conflicts;
- release state;
- source bundle;
- CLI/TUI transparency;
- offline access;
- update scan.

### Importantísimo

Comparar manualmente los claims contra las fuentes.

I-03 no está completo si produce bundles bonitos pero incorrectos.

### Bugs

```text
reproduce
→ regression test
→ fix
→ targeted suite
→ full suite
→ commit
```

### No avanzar a I-04 si existen

- network privacy bypass;
- evidence sin provenance;
- claims inventados;
- conflict hidden;
- snapshots no reproducibles;
- source cache corruptible;
- current/historical mezclados;
- release status incorrecto sistemáticamente.

### Commit final de registro

```text
docs(roadmap): record I-03 dogfooding results
```

---

## Paso 49 — Cierre formal de I-03

- [ ] Paso 49 completado

### Objetivo

Declarar lista la capa de research para ser consumida por I-04.

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
- offline tests;
- privacy tests;
- cache tests;
- large source fixture;
- security tests;
- opt-in live smoke realizado manualmente;
- no secrets;
- working tree limpio.

### Arquitectura

Confirmar:

```text
Research Domain
    ✗ no importa Bubble Tea
    ✗ no importa SQLite
    ✗ no importa net/http
    ✗ no compila curriculum
    ✗ no modifica student mastery
    ✗ no usa source web como instrucciones

Research Engine
    ✓ descubre
    ✓ clasifica autoridad
    ✓ descarga vía adapter
    ✓ normaliza
    ✓ guarda snapshots
    ✓ crea evidence/claims
    ✓ verifica
    ✓ detecta conflictos
    ✓ versiona releases
    ✓ calcula freshness
    ✓ crea source bundles
    ✓ detecta drift
    ✓ produce impact reports
```

### Documentación obligatoria

Actualizar:

```text
README.md
AGENTS.md

docs/architecture/research-domain.md
docs/architecture/trust-policy-v1.md
docs/architecture/freshness-v1.md
docs/architecture/conflict-resolver-v1.md
docs/architecture/drift-v1.md
docs/architecture/research-to-curriculum-update-contract.md
docs/architecture/source-content-retention-policy.md

docs/implementation/I-03-research-source-intelligence/PLAN.md
docs/implementation/I-03-research-source-intelligence/PROGRESS.md
```

### Completion record

```md
## I-03 Research & Source Intelligence Completion

Status: completed
Release: <real version>
Completed steps: 0-49

Algorithms:
- trust-policy-v1
- query-planner-v1
- freshness-v1
- resource-quality-v1
- conflict-resolver-v1
- research-trigger-v1
- drift-v1

Known limitations:
- No production Curriculum Compiler yet.
- No production Learning Packs yet.
- No AI Research Reviewer required yet.
- No automatic curriculum migration.

Ready for:
I-04 Curriculum Compiler & Learning Packs
```

### Commit sugerido

```text
docs(roadmap): mark I-03 Research Engine complete
```

### Release

No asumir número de versión. Usar SemVer real del repositorio.

---

# Checklist final — I-03 Research & Source Intelligence

## Ejecución

- [x] Paso 0 — Apertura formal
- [x] Paso 1 — Research domain
- [x] Paso 2 — Service boundaries
- [ ] Paso 3 — Persistence
- [ ] Paso 4 — Trust Policy
- [ ] Paso 5 — Authority Profiles
- [ ] Paso 6 — Trusted Registry
- [ ] Paso 7 — Privacy/network gate
- [ ] Paso 8 — HTTP client
- [ ] Paso 9 — Fetcher/Snapshots
- [x] Paso 10 — Normalization
- [ ] Paso 11 — Discovery abstraction
- [x] Paso 12 — Query Planner
- [x] Paso 13 — Evidence/Claims
- [x] Paso 14 — Provenance
- [x] Paso 15 — Citations/Deep Links
- [x] Paso 16 — Freshness
- [x] Paso 17 — Verification scheduling
- [ ] Paso 18 — Resource Quality
- [ ] Paso 19 — Release model
- [ ] Paso 20 — Release discovery
- [ ] Paso 21 — Deprecation
- [ ] Paso 22 — Historical sources
- [ ] Paso 23 — Conflict Resolver
- [ ] Paso 24 — Multi-source Verification
- [ ] Paso 25 — Source Bundle
- [ ] Paso 26 — Further Reading
- [ ] Paso 27 — Playground/Package/Standards
- [ ] Paso 28 — Community Resources
- [ ] Paso 29 — Video Supplements
- [ ] Paso 30 — Source Diversity
- [ ] Paso 31 — Real Source Code
- [ ] Paso 32 — Offline Research Cache
- [ ] Paso 33 — Cost Control
- [ ] Paso 34 — Trigger Policies
- [ ] Paso 35 — Research/Sources CLI
- [ ] Paso 36 — TUI Transparency
- [ ] Paso 37 — Update Scan
- [ ] Paso 38 — Drift Detection
- [ ] Paso 39 — Impact Analysis
- [ ] Paso 40 — Migration contracts
- [ ] Paso 41 — Compiler contract
- [ ] Paso 42 — Research Audit
- [ ] Paso 43 — Copyright/retention
- [ ] Paso 44 — Security hardening
- [ ] Paso 45 — Performance/concurrency
- [ ] Paso 46 — E2E controlled
- [ ] Paso 47 — Live integration opt-in
- [ ] Paso 48 — Dogfooding
- [ ] Paso 49 — Cierre formal

---

# Checklist de capacidades entregadas

## Sources

- [ ] Source identity
- [ ] Source kinds
- [ ] Canonical locators
- [ ] Trusted registry
- [ ] Authority profiles
- [ ] Historical status
- [ ] Version scope
- [ ] Snapshots
- [ ] Content hashes
- [ ] Conditional fetch

## Trust

- [ ] Trust Policy v1
- [ ] Topic-aware authority
- [ ] Primary Sources First
- [ ] Supplementary resources
- [ ] Blocked sources
- [ ] Trust explanations

## Discovery

- [ ] SearchProvider abstraction
- [x] Query Planner v1
- [ ] Deduplication
- [ ] Cost-aware discovery
- [ ] Offline behavior

## Evidence

- [x] Evidence
- [x] Claims
- [x] Bounded excerpts
- [x] Provenance
- [ ] Citations
- [ ] Deep links
- [ ] Source bundles
- [ ] Reproducible hashes

## Verification

- [ ] Multi-source verification
- [ ] Source diversity
- [ ] Conflict detection
- [ ] Conflict Resolver v1
- [ ] Unresolved conflict state
- [ ] Claim confidence

## Freshness

- [ ] Last verified
- [ ] Freshness v1
- [ ] Stale detection
- [ ] Refresh scheduling
- [ ] Trigger policies

## Releases

- [ ] Release records
- [ ] Stable
- [ ] Preview/beta/rc
- [ ] Experimental
- [ ] Legacy/EOL
- [ ] Release notes
- [ ] Deprecation
- [ ] Historical guidance

## Resource intelligence

- [ ] Resource Quality v1
- [ ] Further Reading
- [ ] Package References
- [ ] Playgrounds
- [ ] Standards
- [ ] Community Resources
- [ ] Video Supplements
- [ ] Real Source Code

## Research operations

- [ ] Research runs
- [ ] Research audit
- [ ] Cost stats
- [ ] Offline cache
- [ ] Update scan
- [ ] Drift report
- [ ] Impact report
- [ ] CLI
- [ ] TUI source transparency

## I-04 contracts

- [ ] Ready-for-compile status
- [ ] Source Bundle API
- [ ] Drift contract
- [ ] Impact contract
- [ ] Selective migration metadata
- [ ] Student-safe update boundary

---

# Definition of Done — I-03

- [ ] I-01/I-02 sin regresiones críticas
- [ ] Research domain independiente de HTTP/SQLite/UI
- [ ] `privacy.allow_network=false` bloquea todo live research
- [ ] Offline cache funciona
- [ ] Offline evidence permanece consultable
- [ ] Search result nunca se trata directamente como evidence
- [ ] Primary Sources First implementado
- [ ] Trust decisions son explicables
- [ ] Authority depende del tópico
- [ ] Source snapshots son inmutables
- [ ] Content hashes son reproducibles
- [x] Claims requieren evidence
- [x] Evidence tiene provenance
- [ ] Citations tienen source/snapshot
- [x] Excerpts respetan política bounded
- [ ] No se exportan bodies externos por defecto
- [ ] Freshness usa clock injectable
- [ ] Historical ≠ current
- [ ] Release status distingue stable/preview/experimental/legacy
- [ ] Deprecation requiere evidence
- [ ] Conflict no se oculta
- [ ] Multi-source verification funciona
- [ ] Same organization no cuenta automáticamente como múltiples fuentes independientes
- [ ] Resource quality separado de authority
- [ ] Community resources etiquetados
- [ ] Video es supplementary por defecto
- [ ] Source code usa permalinks cuando se usa como evidence
- [ ] Research cost bounded
- [ ] Update Scan funciona
- [ ] Drift detection funciona
- [ ] Impact report funciona
- [ ] I-03 no modifica curriculum final
- [ ] I-03 no modifica Student Mastery
- [ ] I-04 contracts documentados
- [ ] Research audit registra algoritmos/providers/snapshots
- [ ] Security review pasa
- [ ] SSRF protections pasan
- [ ] No secrets en logs
- [ ] No prompt instructions desde fuentes externas
- [ ] Large fixture razonable
- [ ] Concurrency bounded
- [ ] `go test ./...` pasa
- [ ] `go vet ./...` pasa
- [ ] race tests aplicables pasan
- [ ] CI Linux pasa
- [ ] CI Windows pasa
- [ ] CI macOS pasa
- [ ] E2E I-03 pasa
- [ ] Live opt-in smoke realizado
- [ ] Dogfooding realizado
- [ ] Bundles revisados manualmente contra fuentes
- [ ] No bugs críticos/bloqueantes conocidos
- [ ] Todos los pasos completados marcados `[x]`
- [ ] PROGRESS.md actualizado por paso
- [ ] Commits Conventional Commit coherentes
- [ ] Working tree limpio
- [ ] Release final respeta SemVer
- [ ] Ready for I-04

---

# Resultado esperado al terminar I-03

Kelyro debe poder ejecutar un flujo conceptual como:

```text
$ kelyro research topic "Go interfaces"
```

y producir:

```text
Research Request
      ↓
Query Plan
      ↓
Discovery
      ↓
Primary Sources
      ↓
Fetch + Snapshot
      ↓
Normalize
      ↓
Evidence
      ↓
Claims
      ↓
Multi-source Verification
      ↓
Source Bundle
```

con transparencia:

```text
Topic: Go interfaces
Status: ready
Target version: current stable

Primary sources:
✓ Language Specification
✓ Official documentation

Supporting:
✓ Package/source reference

Historical:
○ Historical official guide

Conflicts: 0
Last verified: <timestamp>
Freshness: fresh
```

Después, cuando una tecnología cambie:

```text
Release detected
      ↓
Update Scan
      ↓
Re-fetch affected sources
      ↓
New evidence
      ↓
Drift Report
      ↓
Impact Report
      ↓
I-04 receives change intelligence
```

I-03 termina cuando Kelyro ya puede responder de manera fiable:

```text
¿Qué fuentes sustentan esto?
¿Qué versión cubren?
¿Cuándo se verificaron?
¿Qué tan autoritativas son?
¿Hay conflicto?
¿Cambió algo?
¿Qué podría verse afectado?
```

sin todavía decidir cómo reescribir el curriculum.

La siguiente implementación, **I-04 — Curriculum Compiler & Learning Packs**, tomará estos Source Bundles, Claims, Freshness y Drift Reports para construir curriculums granulares, versionados, completos y actualizables sin depender de la memoria del modelo.
