# I-03 Research & Source Intelligence — Progress Log

## Estado general

Current step: 43
Last completed step: 42
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

## Step 02 — Repositories y application services de Research

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Paquete `internal/research/application` con puertos pequeños para sources,
  snapshots, evidence, research runs, trust registry, releases, freshness,
  verification, drift, impact y research cache.
- Contratos transport-neutral para `SearchProvider`, `SourceFetcher`,
  `SourceNormalizer`, `MetadataExtractor` y `Clock`, con DTOs validados y fetch
  explícitamente limitado por bytes.
- Ocho servicios de aplicación delgados: Research, Discovery, Source,
  Verification, Freshness, Release Intelligence, Drift e Impact.
- Taxonomía causal de errores `not_found`, `conflict`, `invalid_state`,
  `unavailable`, `persistence_failure` y `external_failure`, preservando causas
  y separando fallos de storage de fallos de providers.
- Fake determinista `internal/research/application/memory` para todos los
  repositories, con mutex, orden estable, cancelación, checks relacionales y
  copias defensivas de pointers, slices y payloads.
- Soporte de múltiples runs para un mismo `ResearchRequest` inmutable, sin
  duplicar o mutar la identidad del request.
- Tests black-box de servicios, fakes, error mapping, provider failures,
  dependencias ausentes, context cancellation, ownership de datos y contratos
  externos.
- Límites, semántica de repositories, puertos, servicios, errores y fake
  documentados en `docs/architecture/research-application.md`.

### Decisions

- `Repositories` es solo un wiring bundle; no es una mega-interface ni se
  expone como dependencia de los servicios.
- Cada servicio recibe únicamente los ports que utiliza. Verification e Impact
  tienen repositories propios para no mezclar agregados con Drift.
- No se introdujo `UnitOfWork`: las operaciones actuales escriben un solo
  registro y aún no existe un caso de uso atómico que justifique ese contrato.
- `ResearchRunRepository` conserva un request y admite múltiples runs; reutilizar
  el mismo request ID con contenido diferente produce conflict.
- Repository failures desconocidos mapean a `persistence_failure`; fallos de
  adapters externos mapean a `external_failure`; cancelación y deadlines siempre
  mapean a `unavailable`.
- Freshness y cache records son outputs/DTOs de application. No implementan
  todavía fórmulas, eviction, TTL policy ni algoritmos reservados a pasos
  posteriores.
- No se añadieron SQLite, HTTP real, search providers, parsers, Trust Policy,
  CLI/TUI, Curriculum Compiler ni cambios al Student Core.

### Verification

- `GOCACHE=/tmp/kelyro-i03-step2-target2-gocache go test ./internal/research/application/...`.
- `GOCACHE=/tmp/kelyro-i03-step2-target2-gocache go vet ./internal/research/application/...`.
- `GOCACHE=/tmp/kelyro-i03-step2-full-test-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step2-full-vet-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step2-quality2-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go test -race ./...`, build y smokes
  de CLI.
- Auditoría de imports: application y memory dependen únicamente de la librería
  estándar y de los paquetes `internal/research` correspondientes; no importan
  HTTP, SQLite, UI, Student Core ni providers concretos.
- `git diff --check`.

### Notes for next session

- El Paso 3 es el siguiente paso pendiente y requiere autorización explícita.
- El adapter SQLite deberá implementar estos ports, respetar la semántica de
  conflictos/not-found y conservar las relaciones source/snapshot/evidence.
- No implementar Trust Policy, network access ni pasos posteriores durante la
  persistence de Step 03.

## Step 03 — Persistence schema y migrations de Research

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Migration SQLite forward-only v23 sobre el schema publicado de Student Core,
  sin modificar ninguna de las 22 migrations previas.
- Schema para requests/topics y runs, sources/aliases/snapshots, authority y
  trust, evidence/claims/citations/bundles, release/deprecation/freshness,
  verification/conflicts, cache, drift e impact.
- Claves foráneas y constraint compuesto que preservan la cadena
  `source → snapshot → evidence`, además de relaciones run/request,
  trust/source, claim/source, bundle/run, citation/evidence e impact/drift.
- Índices para locator, aliases, latest snapshot, research run, claim topic,
  last verified, release/version, verification por claim y vencimientos de
  freshness/cache.
- Adapter de producción expuesto como `Database.Repositories().Research` para
  los once repository ports definidos en Step 02, también disponible dentro de
  las transacciones Foundation existentes.
- Error mapping estable para invalid state, not found, conflict, unavailable y
  persistence failure, con orden determinista y semántica alineada con el fake
  en memoria.
- Retención acotada: snapshots sin body web, excerpts separados de metadata y
  limitados a 8 KiB, cache opaco limitado a 1 MiB, y hashes conservados para
  reproducibilidad.
- Contrato de persistencia documentado en
  `docs/architecture/research-persistence.md` y enlazado desde el índice de
  arquitectura.

### Decisions

- `research_topics.request_id` representa la identidad inmutable del request y
  puede ser referenciado por múltiples runs, sin duplicar el request.
- Identidad estable de source y canonical locator son constraints únicos
  independientes; aliases quedan preparados en schema para pasos posteriores.
- Colecciones pequeñas sin consultas independientes se persisten como arrays
  JSON validados; relaciones consultables de claim/source y bundle items usan
  tablas dedicadas.
- Los adapters comprueban relaciones que sus ports pueden observar antes de
  escribir, para conservar las clasificaciones `not_found` e `invalid_state`
  en vez de filtrar errores internos de SQLite.
- No se añadió un Unit of Work nuevo: los repositorios Research reutilizan el
  transaction boundary ya provisto por Foundation.
- No se implementaron Trust Policy, red, fetchers, parsers, algoritmos de
  freshness/verification/drift/impact, CLI/TUI, Curriculum Compiler ni cambios
  de Student Core.

### Verification

- Migration test determinista desde schema v22 (I-02) a v23, preservando estado
  Foundation/Student Core.
- New database test actualizado con todas las tablas v23 y foreign keys activas.
- Roundtrips de sources, snapshots, evidence, requests/runs, authority profiles,
  trust decisions, releases, freshness, verification, drift, impact y cache.
- Tests de duplicate source ID/locator, ownership source/snapshot/evidence,
  foreign keys, excerpt máximo y cache máximo.
- `GOCACHE=/tmp/kelyro-i03-step3-target2-gocache go test ./internal/storage/sqlite`.
- `GOCACHE=/tmp/kelyro-i03-step3-target2-vet-gocache go vet ./internal/storage/sqlite`.
- `GOCACHE=/tmp/kelyro-i03-step3-fulltest-gocache go test ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step3-quality-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go vet ./...`, `go test -race ./...`,
  build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 4 es el siguiente paso pendiente y requiere autorización explícita.
- Trust Policy v1 deberá consumir los authority profiles y append-only trust
  decisions persistidos aquí sin introducir un booleano global `trusted`.
- No implementar discovery live, networking ni pasos posteriores durante el
  Paso 4.

## Step 04 — Trust Policy v1

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Subpaquete puro `internal/research/trust` con política stateless y
  determinista identificada de forma inmutable como `trust-policy-v1`.
- Input validado para source, topic, purpose, use case, evaluation timestamp y
  las dimensiones independientes freshness, relevance, directness, stability y
  corroboration; authority se clasifica contextualmente como tier A–E.
- Contextos explícitos general, language specification, security advisory,
  package API e historical behavior, sin hardcodear tecnologías, dominios ni
  organizaciones concretas.
- Precedencia determinista de decisiones para `accepted`,
  `accepted_as_supplement`, `requires_verification` y `rejected`, sin booleano
  global de trusted ni score numérico que oculte dimensiones.
- Reason codes ordenados para cada dimensión, metadata, reglas contextuales y
  outcome final, preservados dentro del `TrustDecision` existente.
- Reglas conservadoras para metadata incompleta, official stale, community-only,
  preview/experimental/legacy, evidencia ausente y conflictos explícitos.
- Security guidance requiere evidence authority A/B y corroboración
  independiente; historical release notes reciben precedencia A pero un
  conflicto sigue visible como `requires_verification`.
- Política y fronteras documentadas en
  `docs/architecture/trust-policy-v1.md`, enlazadas desde el índice y reflejadas
  en el documento del dominio Research.

### Decisions

- Trust Policy v1 es lógica de dominio pura y no un application service: no
  necesita repositories, clocks implícitos, SQLite, HTTP ni UI.
- Authority v1 usa source kind + use case como baseline contextual. El matching
  data-driven de profiles, organizations y domains permanece reservado al Paso
  5 y el Trusted Source Registry al Paso 6.
- Freshness no reduce authority: una fuente official stale conserva su tier y
  cambia a `requires_verification`.
- Una source normative single-source puede aceptarse fuera de security; una
  community source sin corroboración independiente nunca sostiene conocimiento
  por sí sola.
- Historical precedence no resuelve conflictos. Release notes pueden tener
  mayor tier que current docs para historical behavior, pero corroboration
  `conflicted` siempre exige verificación explícita.
- Metadata mínima requiere publisher; contextos y purposes time-sensitive
  requieren además `published_at` o `updated_at`.
- No se implementaron Authority Profile matching/fixtures, Trusted Registry,
  freshness calculation, verification, Conflict Resolver, network access,
  discovery, Curriculum Compiler ni cambios de Student Core.

### Verification

- Tests de normative source, community-only, stale official,
  official/historical conflict, low quality y missing metadata.
- Tests adicionales de security independent corroboration, package API
  precedence, determinismo e inputs inválidos.
- `GOCACHE=/tmp/kelyro-i03-step4-research-gocache go test ./internal/research/...`.
- `GOCACHE=/tmp/kelyro-i03-step4-research-vet-gocache go vet ./internal/research/...`.
- `GOCACHE=/tmp/kelyro-i03-step4-fulltest-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step4-fullvet-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step4-final-quality-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go vet ./...`, `go test -race ./...`,
  build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 5 es el siguiente paso pendiente y requiere autorización explícita.
- Authority Profiles deberá producir matching data-driven compatible con los
  tiers y use cases de Trust Policy v1, sin hardcodear Go dentro del core.
- No implementar Trusted Source Registry, network access ni pasos posteriores
  durante el Paso 5.

## Step 05 — Authority Profiles por dominio y tópico

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- `AuthorityProfile` enriquecido con dominios y organizaciones preferidos,
  corroboración mínima y source kinds suplementarios, manteniendo authority
  tier, version e identidad explícitos.
- Subpaquete puro `internal/research/authority` con catálogo inmutable, matching
  case-insensitive y precedencia determinista por dominio y especificidad del
  patrón de tópico.
- Topic key general `<technology>/<subject>` cuando existe tecnología, sin
  branches ni constantes Go dentro del matcher.
- Contrato data-driven `authority-profiles/v1` y loader YAML estricto en
  `internal/infra/authorityyaml`, con rechazo de campos desconocidos, múltiples
  documentos, versiones incompatibles y catálogos inválidos.
- Fixture `assets/research/authority-profiles/technology-software.yaml` con
  fallback Software y perfil Go más específico; dominios y organizaciones Go
  viven únicamente en datos.
- Validación de duplicate IDs, selectors contradictorios, patterns inválidos,
  source kinds desconocidos, corroboración inválida y kinds simultáneamente
  preferred/supplementary.
- Migration SQLite forward-only v24 para persistir el contrato completo sin
  modificar v23, con defaults compatibles para perfiles preexistentes.
- Tests de topic matching, precedence, fallback, dominio futuro custom,
  strict YAML, copias defensivas, roundtrip SQLite y upgrade v23 → v24.
- Contrato, algoritmo, fixture, persistence y límites documentados en
  `docs/architecture/authority-profiles-v1.md` y enlazados desde el índice.

### Decisions

- El dominio del perfil es exacto o `*`; el fallback siempre es explícito y el
  matcher no inventa un perfil cuando no existe coincidencia.
- `topic_pattern` soporta únicamente `*`. Con tecnología, compara contra
  `<technology>/<subject>`; sin tecnología, compara contra `<subject>`.
- Un dominio exacto precede al global; dentro de igual especificidad de dominio,
  vence el patrón con más caracteres literales; los empates conservan orden por
  ID estable.
- Un catálogo rechaza selectors normalizados duplicados en vez de permitir que
  reglas equivalentes se sobrescriban silenciosamente.
- Preferred domains son host patterns DNS exactos o con wildcard inicial
  `*.`; matching contra locators/sources queda reservado a pasos posteriores.
- Authority Profile es preferencia contextual, no evidencia ni TrustDecision.
  Trust Policy v1 y el catálogo permanecen componentes puros separados.
- Se reutiliza `go.yaml.in/yaml/v3`, ya pinneado por el repositorio; no se añadió
  ninguna dependencia externa nueva.
- No se implementaron Trusted Source Registry, discovery, networking,
  clasificación URL/source, freshness, verification, Curriculum Compiler ni
  cambios de Student Core.

### Verification

- Documentación actual de `go.yaml.in/yaml/v3` consultada mediante Context7 para
  confirmar `yaml.NewDecoder`, `KnownFields` y detección de trailing documents.
- Tests dirigidos de research, authority YAML y SQLite.
- `GOCACHE=/tmp/kelyro-i03-step5-fulltest-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step5-fullvet-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step5-quality-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go test -race ./...`, build y smokes
  de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 6 es el siguiente paso pendiente y requiere autorización explícita.
- Trusted Source Registry podrá consumir los profile preferences y añadir
  metadata razonada de sources/organizations, sin convertir perfiles en un
  booleano global de confianza.
- No implementar discovery live, network access ni pasos posteriores durante
  el Paso 6.

## Step 06 — Trusted Source Registry

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Modelo de dominio `SourceRegistryEntry` con organización, dominios canónicos,
  source kinds, authority hints, ámbitos de dominio/tópico, notas, estado y
  timestamps de alta/revisión; todos los invariantes se validan al construir.
- Estados explícitos `trusted`, `conditional`, `historical`, `deprecated` y
  `blocked`, conservados por catálogo, application services y persistence.
- Canonicalización DNS case-insensitive con eliminación de trailing dot,
  soporte exacto y wildcard inicial `*.`, y reglas de subdominio sin incluir el
  apex para wildcards.
- Catálogo puro con rechazo de IDs/domains duplicados, matching determinista y
  precedencia exacta sobre wildcard; el contexto se evalúa con research domain,
  topic key y source kind.
- Integración conservadora con Trust Policy v1: el registry aporta contexto y
  restricciones, pero nunca convierte una candidata en evidencia ni promueve
  el tier base; `blocked` rechaza y los estados no plenamente vigentes exigen
  verificación según el use case.
- Repository port, service y adapters memory/SQLite con migration forward-only
  v25, persistencia JSON acotada y protección transaccional contra dominios
  canónicos duplicados.
- Factory de research store por workspace y wiring de aplicación/CLI para
  `kelyro sources registry list` y `kelyro sources registry show <id>`.
- Tests de normalización, reglas de subdominio, blocked/historical, precedencia,
  duplicados, copias defensivas, roundtrip/upsert SQLite, reapertura del store,
  aplicación, CLI e integración con Trust Policy.
- Contrato, matching, persistence, límites y semántica documentados en
  `docs/architecture/trusted-source-registry.md` y documentos Research
  relacionados.

### Decisions

- Un dominio canónico es un host DNS, no una URL; solo admite coincidencia
  exacta o wildcard inicial `*.` para mantener reglas auditables.
- Una entrada puede declarar varios source kinds, pero cada authority hint debe
  referirse a uno de ellos y solo puede volver más conservadora la evaluación
  base de Trust Policy v1.
- El registry no es verdad absoluta: además del match de host, su aplicabilidad
  depende de tópico, source kind, frescura y corroboración evaluados por Trust
  Policy.
- `historical` es utilizable para investigación histórica; fuera de ese use
  case requiere verificación. `conditional` y `deprecated` también requieren
  verificación, mientras `blocked` fuerza rechazo.
- SQLite impide que dos entries posean el mismo dominio canónico, incluido el
  wildcard normalizado; el upsert del mismo ID permanece permitido.
- La CLI de este paso es de inspección y no añade comandos de mutación,
  discovery live, fetch ni acceso de red.
- No se implementaron privacy/network gate, HTTP client, discovery, freshness,
  verification scheduling, Curriculum Compiler ni cambios de Student Core.

### Verification

- Tests dirigidos de research, persistence, application, CLI y research store.
- `GOCACHE=/tmp/kelyro-i03-step6-fulltest-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step6-fullvet-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step6-quality-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go vet ./...`,
  `go test -race ./...`, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 7 es el siguiente paso pendiente y requiere autorización explícita.
- El privacy/network gate deberá cubrir toda operación live sin inutilizar
  evidencia persistida ni caché offline.
- No implementar HTTP client, discovery ni pasos posteriores durante el Paso 7.

## Step 07 — Privacy/network gate para Research Engine

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Modos de investigación cerrados `offline`, `online` y `auto`, validados en
  application y sin capacidad de sobreescribir la política resuelta de
  Foundation.
- Gate obligatorio `privacy.NetworkGate` en los tres límites live actuales:
  discovery (`research.discovery`), fetch (`research.fetch`) y release lookup
  (`research.release_lookup`).
- `DiscoveryService`, `FetchService` y `ReleaseLookupService` con autorización
  previa al adapter live y validación de identidad/estructura de toda salida.
- Puertos offline explícitos y separados para discovery, fetched sources y
  releases, evitando que un fallback de cache pueda confundirse con un
  provider de red.
- Fallback determinista: `offline` usa solo cache, `online` exige permiso live y
  `auto` usa live si Foundation autoriza o cache si la política bloquea.
- Clasificación estable `network_research_blocked` mediante
  `ErrNetworkResearchBlocked`, preservando también
  `privacy.ErrNetworkBlocked` como causa categorizable.
- Integración probada con la configuración Foundation deny-by-default y el log
  seguro de privacy, sin incluir queries, URLs, contenido ni workspace paths en
  la solicitud de autorización.
- Contrato y límites documentados en
  `docs/architecture/research-network-privacy.md` y sincronizados con la
  documentación de Foundation y application.

### Decisions

- `online` expresa intención de usar red, no permiso; una denegación de
  `privacy.allow_network` siempre gana y no consulta cache silenciosamente.
- `auto` cae a cache únicamente ante una denegación de política. No oculta
  errores de un provider ya autorizado con fallback automático; la política de
  source fallback pertenece a un paso posterior.
- `offline` no consulta el gate y nunca toca un provider live, incluso si
  `privacy.allow_network` está habilitado.
- Cache miss más imposibilidad de acceso live produce
  `network_research_blocked`; fallos reales de cache conservan
  `persistence_failure` y fallos live conservan `external_failure`.
- El gate protege solo operaciones externas. Evidencia, snapshots, registry,
  freshness y demás datos persistidos continúan disponibles sin red.
- No se implementaron HTTP client, DNS/SSRF, retries, cache encoding/writes,
  discovery provider, release discovery, CLI Research, Curriculum Compiler ni
  cambios de Student Core.

### Verification

- Tests de bloqueo y cero llamadas accidentales a providers para discovery,
  fetch y release lookup.
- Tests de autorización, modos inválidos, `offline` sin consulta al gate,
  fallback cacheado de `auto`, cache miss y fallos de cache diferenciados.
- Test de integración con configuración Foundation deny-by-default y logging de
  la operación estable `research.discovery`.
- `GOCACHE=/tmp/kelyro-i03-step7-fulltest1-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step7-fullvet1-gocache go vet ./...`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step7-quality-final-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go vet ./...`,
  `go test -race ./...`, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 8 es el siguiente paso pendiente y requiere autorización explícita.
- El Research HTTP Client deberá vivir detrás de `SourceFetcher` y seguirá
  dependiendo del gate aplicado en este paso; no deberá autorizarse a sí mismo.
- No implementar discovery, parsing, cache policy ni pasos posteriores durante
  el Paso 8.

## Step 08 — Research HTTP Client seguro y configurable

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Paquete de infraestructura reutilizable `internal/infra/researchhttp`, sin
  `http.Get` disperso ni dependencias externas nuevas.
- Configuración validada y acotada para timeout total por intento, dial, TLS,
  response headers, idle connections, pools, redirects, decoded body, attempts
  y exponential backoff.
- `User-Agent` obligatorio y acotado con identidad `Kelyro/...`, transporte
  reutilizable, HTTP/2, TLS 1.2 mínimo, certificate validation estándar y
  `http.ProxyFromEnvironment`.
- Lectura `max+1`, precheck de `Content-Length`, límite de headers, allowlist de
  media types y soporte seguro de gzip con límite aplicado después de la
  descompresión.
- Retry GET únicamente para errores transitorios y statuses 408/429/500/502/
  503/504, con `Retry-After` y exponential backoff siempre limitados;
  cancellation interrumpe request y backoff.
- SSRF deny-by-default para URL inicial y cada redirect, bloqueando user info,
  localhost, redes privadas/locales, direcciones no globales y endpoints de
  metadata; dial directo revalida y fija la IP resuelta.
- Hooks `RateLimiter` y `Observer` con metadata acotada; el evento observable no
  puede transportar URL, query, headers, body, credenciales ni error raw.
- Request headers sensibles/client-owned rechazados, headers sensibles
  eliminados defensivamente en redirects y response reducida a status,
  content-type, ETag, Last-Modified, locator final y body acotado.
- Errores tipados por invalid request, SSRF, redirect, timeout, HTTP status,
  oversize, content type, encoding, transport y rate-limit hook, con strings
  seguros para logging.
- Contrato, defaults, SSRF, retry, hooks, redacción y límites documentados en
  `docs/architecture/research-http-client.md`.

### Decisions

- El cliente HTTP es infraestructura y no implementa `SourceFetcher`; no crea
  `SourceSnapshot`, hashes, evidence ni cache records. Ese adapter pertenece al
  Paso 9.
- Privacy authorization permanece en los application services del Paso 7. El
  cliente no se autoautoriza ni puede reemplazar `privacy.allow_network`.
- `http.Client.Timeout` limita cada intento completo; `MaxAttempts` y
  `MaxBackoff` limitan la secuencia, y el context del caller puede imponer un
  deadline total menor.
- Solo 2xx y 304 son respuestas exitosas. Otros 4xx no se reintentan salvo 408
  y 429; invalid content, oversize, redirects inseguros y SSRF nunca se
  reintentan.
- Un hostname con respuestas DNS mixtas public/private se rechaza completo. El
  proxy estándar también queda sujeto a la address policy para que un proxy
  local no se convierta en bypass SSRF.
- Go maneja gzip automáticamente cuando el Transport lo solicita; el límite se
  verifica sobre el body ya decodificado y otros encodings residuales se
  rechazan.
- La documentación actual de Go standard library se consultó mediante
  Context7 para `http.Client`, `http.Transport`, redirects, proxy, compresión,
  `DialContext`, contexts y cierre de response bodies.
- No se implementaron Source Fetcher/Snapshot, cache writes, parsing,
  discovery, release ingestion, CLI Research, Curriculum Compiler ni cambios
  de Student Core.

### Verification

- Tests con `httptest.Server` para timeout, redirect limit, redirect SSRF,
  oversize, unexpected content type/encoding, 404, 429, 500 y cancellation.
- Tests adicionales de gzip, transient transport retry, bounded backoff,
  Retry-After, Kelyro User-Agent, safe response metadata, rate-limit/observer
  hooks, sensitive headers, safe error strings y config inválida.
- Tests de política SSRF para loopback, RFC1918, link-local, metadata IPv4/IPv6
  y user info, además de inspección de TLS/proxy/compression defaults.
- `GOCACHE=/tmp/kelyro-i03-step8-target-final-gocache go test ./internal/infra/researchhttp`.
- `GOCACHE=/tmp/kelyro-i03-step8-target-final-gocache go vet ./internal/infra/researchhttp`.
- `GOCACHE=/tmp/kelyro-i03-step8-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step8-full-gocache go vet ./...`.
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOCACHE=/tmp/kelyro-i03-step8-cross-gocache go test -exec=/bin/true ./internal/infra/researchhttp`.
- `GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 GOCACHE=/tmp/kelyro-i03-step8-cross-gocache go test -exec=/bin/true ./internal/infra/researchhttp`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step8-full-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go test -race ./...`, build y smokes
  de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 9 es el siguiente paso pendiente y requiere autorización explícita.
- `SourceFetcher` deberá adaptar `application.FetchRequest` a este cliente,
  calcular metadata/hash y crear el output transport-neutral sin duplicar
  timeouts, redirects, retries, SSRF ni content limits.
- No implementar normalización/parsing, discovery, evidence extraction ni
  pasos posteriores durante el Paso 9.

## Step 09 — Source Fetcher y Source Snapshot

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Adapter `internal/infra/researchfetch` para el puerto `SourceFetcher`, con
  versión explícita `source-fetch-v1`, timestamps UTC, locator final y metadata
  transport-neutral.
- Requests condicionales `If-None-Match`/`If-Modified-Since`, preservación
  acotada de ETag/Last-Modified y representación válida de `304 Not Modified`.
- Hash de contenido canónico versionado como SHA-256 lowercase sobre los bytes
  decodificados exactos, compartido por adapter y application service.
- Límite de response por request que solo puede estrechar el límite global del
  Research HTTP Client, manteniendo sus controles de SSRF, content type,
  compresión, timeout, redirect y retry.
- `SnapshotCaptureService` que resuelve fuente/último snapshot, envía
  validators por el `FetchService` protegido por privacy, verifica hash/length
  y añade una nueva observación inmutable.
- Origen explícito `live`/`cache`: redirects seguros conservan el locator final
  sin cambiar `SourceID`, mientras resultados offline nunca se registran como
  un fetch nuevo ni falsean `fetched_at`.
- Revalidación `304` que exige snapshot previo canónico y validator, registra
  un nuevo evento de fetch y referencia el snapshot revalidado sin duplicar ni
  inventar body.
- Políticas explícitas `metadata_only`, `normalized_excerpt` y
  `bounded_cached_body`; snapshots nunca persisten raw body, y las dos últimas
  solo producen copias defensivas acotadas para los pasos 10/32.
- Tests de body cambiado, ETag sin cambio con respuesta `200`, requests
  condicionales, `304`, content type inválido, límites global/per-request,
  historial append-only, body disposition y relaciones inválidas.
- Contrato y límites documentados en
  `docs/architecture/source-fetch-snapshots-v1.md` y sincronizados con la
  documentación de domain, application, HTTP y persistence.

### Decisions

- Un ETag es validator, no identidad de contenido. Toda respuesta body-bearing
  calcula SHA-256 aunque el provider conserve el mismo ETag.
- El hash cubre bytes después de la decodificación HTTP segura y antes de la
  normalización; Step 10 tendrá su propia representación derivada.
- Cada `2xx` y cada `304` válido añade un snapshot. Un `304` conserva su status,
  fetch time/version y copia content type/hash/length del snapshot previo; el
  resultado expone `RevalidatedSnapshotID`.
- No se añadió migration: el schema v23 ya contiene todos los campos requeridos
  y su repository es append-only. La referencia durable de un `304` se
  reconstruye por source, orden histórico y hash canónico.
- `metadata_only` descarta el body tras verificarlo.
  `normalized_excerpt` entrega input transitorio sin parsearlo ni persistirlo.
  `bounded_cached_body` entrega un candidato defensivo de máximo 1 MiB, pero
  cache encoding/write/expiry/eviction permanecen reservados al Paso 32.
- Privacy authorization permanece en `FetchService`; el adapter HTTP no se
  autoautoriza y el capture service no acepta un `SourceFetcher` live directo.
- No se implementaron normalization/parsing, discovery, evidence/claims,
  Research Cache, release ingestion, CLI Research, Curriculum Compiler ni
  cambios de Student Core.

### Verification

- Documentación actual de Go standard library consultada mediante Context7
  para requests/headers condicionales, respuestas `304` y SHA-256.
- Tests dirigidos de Research domain/application, Source Fetcher, HTTP client y
  persistence SQLite.
- `GOCACHE=/tmp/kelyro-i03-step8-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step8-full-gocache go vet ./...`.
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOCACHE=/tmp/kelyro-i03-step8-cross-gocache go test -exec=/bin/true ./internal/infra/researchfetch ./internal/research/application`.
- `GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 GOCACHE=/tmp/kelyro-i03-step8-cross-gocache go test -exec=/bin/true ./internal/infra/researchfetch ./internal/research/application`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step8-full-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go test -race ./...`, build y smokes
  de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 10 es el siguiente paso pendiente y requiere autorización explícita.
- Source Normalization podrá consumir `SnapshotCapture.NormalizationInput` y
  producir excerpts/estructura derivados sin alterar snapshots históricos.
- No implementar discovery, evidence extraction, cache persistence ni pasos
  posteriores durante el Paso 10.

## Step 10 — Source Normalization pipeline

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Contrato `NormalizedSource` enriquecido y validado para título, locator
  canónico, idioma, headings con ruta jerárquica, segmentos de texto, bloques
  de código, enlaces, fechas, version hints y versión de algoritmo.
- Adapter determinista `internal/infra/researchnormalize` con versión inmutable
  `source-normalization-v1` para HTML/XHTML, plain text, JSON y Markdown directo.
- Extracción HTML no renderizante que decodifica entidades, respeta atributos
  quoted, preserva estructura útil y descarta subárboles de script, style,
  template, SVG, navegación, formularios y otros elementos ruidosos mediante
  cierres de tag exactos.
- Normalización de texto y Markdown con whitespace estable, jerarquía de
  headings, fenced code y links; JSON con `json.Number`, claves ordenadas,
  arrays en orden de fuente y segmentos con ubicación tipo JSON Pointer.
- Resolución y canonicalización de enlaces HTTP(S) contra el locator final;
  esquemas inseguros, credenciales, locators malformados y fragmentos
  canónicos se rechazan o eliminan según corresponda.
- Extracción conservadora de language, fechas published/updated y version
  hints, sin inferir trust, release status ni significado semántico adicional.
- Límites explícitos para colecciones, segmentos y código; errores tipados para
  content type no soportado, documento inválido y output limit, preservando
  cancellation del context.
- `SnapshotCapture.NormalizationInput` ahora entrega una copia defensiva del
  `FetchedSource` validado para conservar body, locator final, media type y
  metadata de integridad necesarios por el normalizer.
- Golden fixtures para los cuatro formatos y tests adversariales de
  sanitización, jerarquía, links, UTF-8, JSON, respuestas bodyless,
  cancellation, límites y orden determinista.
- Contrato, seguridad y límites documentados en
  `docs/architecture/source-normalization-v1.md` y sincronizados con la
  documentación de snapshots, application y domain.

### Decisions

- La normalización es representación derivada: el snapshot inmutable del Paso
  9 y su hash SHA-256 sobre los bytes fetched continúan siendo la fuente
  histórica de verdad.
- No se añadió `golang.org/x/net/html`. La línea actual revisada requiere Go
  1.25 y Kelyro conserva compatibilidad con Go 1.24; el extractor lexical
  acotado usa únicamente standard library y no pretende ser browser, renderer
  HTML5 ni sanitizer reutilizable de HTML.
- El parser JSON ordena claves para producir output determinista y conserva el
  orden de arrays. El parser Markdown soporta deliberadamente un subconjunto
  directo y rechaza fences sin cierre en vez de adivinar estructura.
- Solo sobreviven locators HTTP(S) validados. El contenido externo siempre se
  trata como datos no confiables y nunca se ejecuta ni interpreta como
  instrucciones.
- No se añadió migration ni persistencia: los outputs normalizados no escriben
  raw web content ni alteran snapshots.
- No se implementaron PDF, discovery, evidence/claims, metadata persistence,
  Research Cache, release ingestion, Curriculum Compiler ni cambios de Student
  Core.

### Verification

- Context7 se consultó primero para el parser HTML; al no indexar el paquete,
  se revisó la documentación oficial de `golang.org/x/net/html` y su requisito
  de módulo antes de conservar una implementación sin dependencia nueva.
- Golden tests y tests adversariales de `internal/infra/researchnormalize`.
- Tests y vet dirigidos de normalizer, Research domain/application, Source
  Fetcher y persistence SQLite.
- `GOCACHE=/tmp/kelyro-i03-step8-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step8-full-gocache go vet ./...`.
- Cross-build tests Linux-hosted para Windows y Darwin de normalizer y
  application con `CGO_ENABLED=0`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step8-full-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, race, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 11 es el siguiente paso pendiente y requiere autorización explícita.
- Source Discovery deberá producir candidatos detrás de `SearchProvider`; sus
  resultados no serán evidencia y deberán respetar el privacy gate antes de
  cualquier operación live.
- No implementar discovery, evidence extraction, cache persistence, PDF ni
  pasos posteriores sin su autorización correspondiente.

## Step 11 — Source Discovery abstraction

Status: completed
Date: 2026-08-24
Release: unreleased

### Delivered

- Contratos vendor-neutral `SearchQuery`, `SearchOptions`, `SearchResult` y
  `SearchProvider`, con query/options separados como exige el plan y sin tipos
  de requests/responses de un proveedor concreto.
- Resultados candidatos con título, locator HTTP(S), snippet opcional,
  provider, rank y published hint temporal opcional; ningún resultado se
  convierte en source registrada, snapshot, evidence o claim.
- Normalización determinista de query, título, snippet y provider mediante
  whitespace estable, más canonicalización de locator a nivel documento sin
  fragmentos.
- Deduplicación por locator normalizado que conserva la primera aparición, el
  orden del provider y los ranks originales sin reordenar ni renumerar.
- Límite obligatorio positivo y máximo de 100 resultados, validación de todo
  el output antes de retornarlo y truncado solo después de deduplicar.
- Mismo pipeline para candidatos live y cacheados, manteniendo
  `external_failure`, `persistence_failure`, `unavailable` y
  `network_research_blocked` como categorías causales distintas.
- Provider estático determinista y sin red en
  `application/memory.StaticSearchProvider`, con copias defensivas para tests y
  desarrollo, reemplazable por futuros adapters reales.
- Tests de provider error, query/options normalizados, URLs duplicadas por deep
  link, published hints, bounds, cancellation, copias defensivas y preservación
  exacta de ranks no monotónicos.
- Contrato, lifecycle candidato, privacidad y trabajo diferido documentados en
  `docs/architecture/source-discovery.md` y sincronizados con la documentación
  de domain, application y network privacy.

### Decisions

- Search options se separan del texto de query para que el planner del Paso 12
  pueda producir intención estructurada sin acoplarse a ningún buscador.
- Los fragments no forman una identidad distinta de documento durante
  discovery; paths y query strings sí permanecen significativos porque Kelyro
  no adivina qué parámetros son tracking.
- Los duplicates conservan el primer candidato observado. Discovery no usa el
  rank para ordenar ni escoger silenciosamente otro resultado.
- El published hint continúa siendo metadata no verificada del provider; no
  alimenta freshness ni respalda claims antes de fetch y verificación.
- El límite global es un bound de seguridad del contrato, no la política de
  coste, triggers ni cache reservada para pasos posteriores.
- Privacy authorization continúa en `DiscoveryService`; el provider estático
  no introduce bypass y un futuro adapter live tampoco podrá autoautorizarse.
- No se añadieron API keys, dependencias externas, search adapter real, Query
  Planner, clasificación, evidence/claims, cache persistence, CLI Research,
  Curriculum Compiler ni cambios de Student Core.

### Verification

- Tests dirigidos de discovery, application, memory provider y privacy wiring.
- `GOCACHE=/tmp/kelyro-i03-step8-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step8-full-gocache go vet ./...`.
- Cross-build tests Linux-hosted para Windows y Darwin de application y app con
  `CGO_ENABLED=0`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step8-full-gocache go run ./tools/quality all`,
  incluyendo E2E Foundation/Student Core, `go test -race ./...`, vet, build y
  smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 12 es el siguiente paso pendiente y requiere autorización explícita.
- Query Planner v1 podrá producir `SearchQuery` y `SearchOptions` deterministas
  consumiendo topic, purpose, authority profile y target version.
- No implementar Evidence/Claims, live search, cache persistence ni pasos
  posteriores antes de su autorización independiente.

## Step 12 — Research Query Planner v1

Status: completed
Date: 2026-08-25
Release: unreleased

### Delivered

- Subpaquete puro `internal/research/queryplanner` con algoritmo stateless y
  determinista identificado de forma inmutable como `query-planner-v1`.
- Contratos validados `Input`, `ResearchQuery` y `ResearchQueryPlan` para topic,
  target version opcional, purpose, Authority Profile, query normalizada,
  desired source kind, required authority tier y prioridad secuencial.
- Matrices explícitas para los ocho `ResearchPurpose`, con variantes de
  official documentation, specification, release notes, deprecation, API
  reference, tutorial, source code y security según el propósito.
- Construcción estable de query que usa technology cuando existe o domain para
  tópicos genéricos, seguida de subject, target version opaca y calificador de
  intención, con whitespace Unicode colapsado.
- Orden authority-aware: los preferred kinds relevantes se promueven en orden
  declarado, la cobertura base del propósito nunca se pierde y otros preferred
  kinds se añaden mientras exista capacidad.
- Planes limitados a ocho queries, sin source kinds ni textos duplicados, con
  prioridades positivas `1..N` y el `MinimumTier` contextual del perfil.
- Tests de definition, release version-bound, security y tópico no tecnológico,
  además de cobertura de todos los purposes, bounds, determinismo, ownership de
  inputs y rechazo de estados inválidos.
- Contrato, ordering, mapping a discovery y límites documentados en
  `docs/architecture/query-planner-v1.md` y enlazados desde la arquitectura.

### Decisions

- El planner vive junto a las políticas puras de Research y depende solo del
  vocabulario `internal/research`; no importa application, providers, HTTP,
  SQLite, UI, Student Core ni dependencias externas.
- Authority Profile matching ocurre antes mediante `internal/research/authority`.
  El planner valida el perfil suministrado, pero no selecciona ni inventa un
  fallback silencioso.
- `RequiredAuthority` es un threshold para clasificación posterior; no convierte
  candidatos en trusted sources ni produce una `TrustDecision`.
- La cobertura canónica de cada purpose precede a preferred kinds no
  relacionados, evitando que un perfil grande desplace release notes,
  security standards u otras intenciones esenciales.
- El plan conserva intención provider-neutral. Un orchestrator futuro añadirá
  request ID y result limit al mapear query/kind/version hacia `SearchQuery` y
  `SearchOptions`; Step 12 no ejecuta discovery.
- No se añadieron IA, network access, persistencia, live search, clasificación,
  Evidence/Claims, cache, Curriculum Compiler ni cambios funcionales de Student
  Core.

### Verification

- Tests dirigidos y vet de `internal/research/queryplanner`.
- Suite completa de `internal/research/...`, incluida su ejecución bajo race.
- `GOCACHE=/tmp/kelyro-i03-step12-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step12-vet-gocache go vet ./...`.
- Cross-build tests Linux-hosted para Windows y Darwin de `queryplanner` con
  `CGO_ENABLED=0`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step12-quality-gocache go run ./tools/quality all`,
  incluyendo tests, E2E, vet, `go test -race ./...`, build y smokes de CLI.
- La puerta race expuso un test I-02 que aplicaba su timeout busy de 75 ms al
  `Open`; se separó setup de contención sin cambiar producción y se verificó en
  cinco repeticiones race antes del commit independiente `c41b05b`.
- `git diff --check`.

### Notes for next session

- El Paso 13 es el siguiente paso pendiente y requiere autorización explícita.
- Evidence/Claims podrá consumir sources fetched/normalized y conservar excerpts
  acotados con provenance, sin convertir discovery snippets en evidencia.
- No implementar Provenance, Citations, Freshness, live search, cache ni pasos
  posteriores antes de su autorización independiente.

## Step 13 — Evidence and Claim Model

Status: completed
Date: 2026-08-25
Release: unreleased

### Delivered

- Modelo de dominio `Evidence` ampliado con `context_before` y `context_after`
  opcionales y acotados, además del excerpt mínimo necesario, su hash canónico,
  ubicación, snapshot, timestamp y versión del extractor.
- Modelo `Claim` estructurado con scope y status scope explícitos, version scope
  opcional, tipo cerrado, confianza, timestamps y una o más referencias a
  evidence y source.
- Límites públicos y validados de 8 KiB para excerpts, 2 KiB por contexto y
  1 KiB para claim scope, siempre medidos sobre bytes UTF-8.
- Hash reproducible `sha256:<hex>` calculado sobre los bytes exactos del
  excerpt; evidence con hash ausente, mal formado o divergente es rechazado.
- Migración forward-only v26 para persistir contextos de evidence y scope/status
  scope de claims, con defaults compatibles para filas existentes y constraints
  equivalentes en SQLite.
- Round-trip del repositorio SQLite actualizado para conservar ambos contextos
  sin almacenar cuerpos web completos ni ampliar el alcance hacia extracción.
- Tests para claim sin evidence, bounds de excerpt/context, múltiples evidences,
  version scope, status scope, hash canónico y migración desde esquemas previos.
- Contrato y límites documentados en `docs/architecture/evidence-claims-v1.md`,
  con referencias sincronizadas en la arquitectura de dominio y persistencia.

### Decisions

- Discovery candidates y snippets continúan sin ser evidence. Una Evidence
  válida referencia un snapshot persistido y conserva únicamente el fragmento
  necesario y contexto acotado.
- `ClaimStatusScope` usa el vocabulario cerrado `all`, `stable`, `preview`,
  `experimental` y `legacy`; `all` permite expresar claims no ligados a un
  estado de release sin confundirlo con un valor vacío.
- `VersionScope` sigue siendo opcional y opaco para admitir SemVer, revisiones,
  ediciones o fechas sin imponer un esquema de versión específico.
- Los defaults de migración (`scope=general`, `status_scope=all`, contextos
  vacíos) preservan claims/evidence existentes; las nuevas escrituras pasan por
  validación estricta del dominio.
- Paso 13 define y persiste el modelo. No implementa extractor, ClaimRepository,
  Provenance Graph, citations, verification, live search, IA, cache, Curriculum
  Compiler ni cambios funcionales de Student Core.

### Verification

- Tests dirigidos y vet de `internal/research`,
  `internal/research/application` e `internal/storage/sqlite`.
- `GOCACHE=/tmp/kelyro-i03-step13-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step13-vet-gocache go vet ./...`.
- Cross-build tests Linux-hosted para Windows y Darwin de `internal/research` e
  `internal/storage/sqlite` con `CGO_ENABLED=0`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step13-quality-gocache go run ./tools/quality all`,
  incluyendo tests, E2E, vet, `go test -race ./...`, build y smokes de CLI.
- Migración verificada desde schemas anteriores hacia v26, incluidos defaults,
  round-trip y rechazo de valores fuera de bounds.
- `git diff --check`.

### Notes for next session

- El Paso 14 es el siguiente paso pendiente y requiere autorización explícita.
- Provenance Graph podrá enlazar los IDs y timestamps ya definidos sin cambiar
  la semántica de Evidence/Claim del Paso 13.
- No implementar Provenance, Citations, Freshness, live search, cache ni pasos
  posteriores antes de su autorización independiente.

## Step 14 — Provenance Graph v1

Status: completed
Date: 2026-08-25
Release: unreleased

### Delivered

- Grafo de dominio acotado `ProvenanceGraph` identificado de forma inmutable
  como `provenance-graph-v1`, con nodos tipados para request, run, query,
  discovered source, source, snapshot, evidence, claim y SourceBundle.
- Vocabulario cerrado de transiciones que admite ramas Query/Discovery y el
  camino explícito `run -> source` para fuentes registradas manualmente, sin
  permitir que candidates o snippets salten directamente a Evidence.
- Validación de IDs/nodos/edges únicos, endpoints existentes, estructura
  conectada, cobertura de la claim, timestamps, tool versions y ausencia de
  autociclos/ciclos inválidos.
- Soporte de múltiples fuentes/evidences que convergen en una claim y snapshots
  históricos exactos sin sustituirlos silenciosamente por el latest snapshot.
- `Explain` humano determinista y export/import JSON estricto, estable y
  acotado, sin excerpts ni raw source bodies.
- Límites explícitos de 512 nodos, 1.024 edges, labels de 1 KiB, tool versions
  de 256 bytes y export de 256 KiB.
- `ProvenanceRepository` y `ProvenanceService` con record, latest trace y export,
  más adapters deterministas de memoria y SQLite.
- Migración forward-only v27 para graphs append-only, con metadata/index latest
  por claim y constraints que vinculan ID/claim/algorithm con el JSON guardado.
- Wiring workspace-local y comando interno funcional
  `kelyro sources trace <claim-id>` para imprimir la explicación almacenada sin
  acceso de red.
- Contrato completo documentado en
  `docs/architecture/provenance-graph-v1.md` y referencias de dominio,
  aplicación y persistencia sincronizadas.

### Decisions

- Un grafo explica exactamente una claim, request y run, pero puede contener
  múltiples queries, candidates, sources, snapshots y evidences. Todo nodo
  anterior a la claim debe ser alcanzable desde request y conducir a esa claim.
- SourceBundle es terminal y opcional. El grafo se detiene allí: future
  Curriculum Concept pertenece a I-04 y no fue modelado ni persistido.
- Query/Discovery no son obligatorios para una fuente revisada manualmente; si
  existen, sus tool versions son obligatorias. Fetch y extraction versions son
  igualmente obligatorias en sus nodos.
- Cada append conserva audit history. `Trace` elige determinísticamente el graph
  más reciente por `recorded_at` e ID sin sobrescribir versiones anteriores.
- Los labels son metadata explicativa acotada, no contenido web. Export y read
  validan otra vez tamaño, campos conocidos, algoritmo y grafo completo.
- El scaffold lineal `Provenance` del Paso 1 se conserva compatible; el DAG v1
  completa las capacidades del Paso 14 sin modificar Citation/Deep Link.
- No se implementaron Citations, Deep Links, verification, freshness, conflict
  resolution, live search, IA, Curriculum Compiler ni cambios funcionales de
  Student Core.

### Verification

- Tests dirigidos y vet de Research domain/application/memory, SQLite,
  `researchdb`, app y CLI.
- Tests de full chain, múltiples fuentes, missing node, autociclo, desconexión,
  manual source, historical snapshot, bounds, JSON estable y defensive copies.
- Tests de migración/esquema v27, append/latest/duplicate SQLite y rechazo de
  metadata JSON divergente.
- `GOCACHE=/tmp/kelyro-i03-step14-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step14-vet-gocache go vet ./...`.
- Cross-build tests Linux-hosted para Windows y Darwin de todos los paquetes
  afectados con `CGO_ENABLED=0`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step14-quality-gocache go run ./tools/quality all`,
  incluyendo tests, E2E, vet, `go test -race ./...`, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 15 es el siguiente paso pendiente y requiere autorización explícita.
- Citations/Deep Links podrá consumir los paths validados sin alterar el graph
  ni convertir metadata de discovery en evidence.
- No implementar Citations, Deep Links, Freshness ni pasos posteriores antes de
  su autorización independiente.

## Step 15 — Citations and Deep Links v1

Status: completed
Date: 2026-08-25
Release: unreleased

### Delivered

- Generador puro y determinista `citation-v1` sobre una cadena validada
  source/snapshot/evidence, sin I/O, parsing ni inferencia de contenido.
- Citation ampliada con estrategia cerrada, section/heading/path hint UTF-8
  requerido y acotado a 2 KiB, snapshot date exacta, version scope opaco,
  `last_verified` y algorithm version inmutable.
- Selección por source kind para URL anchors, package symbols, specification o
  standard sections y release headings, usando únicamente fragments explícitos.
- Source-code permalinks validados por host canónico, commit hexadecimal
  inmutable, file path relativo limpio y rango exacto `#Lstart[-Lend]`.
- Fallback explícito `canonical URL + heading/path hint` cuando no existe un
  deep link estable, sin fabricar slugs o anchors.
- `CitationRepository` y `CitationService` para generate, append, get y list by
  evidence, con adapters deterministas de memoria y SQLite.
- Migración forward-only v28 para strategy, section, version scope, algorithm
  version e índice por evidence, preservando de forma conservadora citations
  legacy.
- Contrato, estrategia, cronología, persistencia y límites documentados en
  `docs/architecture/citations-deep-links-v1.md` y referencias arquitectónicas
  sincronizadas.

### Decisions

- `Source.Locator` es la URL canónica de la Citation; `SourceSnapshot.Locator`
  conserva por separado el locator exacto observado durante fetch.
- El generador no deriva anchors desde headings porque los algoritmos de slug
  varían por sitio. Sin fragment explícito se conserva el hint y se usa
  `canonical_fallback`.
- Package symbol, spec section y release heading comparten construcción segura
  canonical-plus-fragment, pero conservan estrategias distintas para auditoría
  y presentación futura.
- Source code no acepta anchors sobre ramas mutables. El adapter/reviewer aporta
  la URL específica del host y `citation-v1` verifica commit, archivo y líneas
  sin acoplar el dominio a GitHub u otro proveedor concreto.
- `last_verified` no puede preceder al snapshot ni a la extracción de Evidence;
  no se confunde con publication/update dates ni calcula Freshness.
- Citations son append-only y siguen disponibles offline; no requieren network
  access y Discovery metadata continúa sin ser Evidence.
- No se añadieron Freshness, Verification, Source Bundles, UI, live research,
  Curriculum Compiler ni cambios funcionales de Student Core.

### Verification

- Tests dirigidos y vet de Research domain/citation/application/memory y
  SQLite, incluidos anchor, no-anchor fallback, package/spec/release strategy,
  commit permalink, URLs inválidas, chronology, duplicate y round-trip.
- Migración v28 verificada desde schema v23 y schemas posteriores, con defaults
  legacy, constraints de section y schema final 28.
- `GOCACHE=/tmp/kelyro-i03-step15-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step15-vet-gocache go vet ./...`.
- Cross-build tests Linux-hosted para Windows y Darwin de Research y SQLite con
  `CGO_ENABLED=0`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step15-quality-gocache go run ./tools/quality all`,
  incluyendo tests, E2E, vet, `go test -race ./...`, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 16 es el siguiente paso pendiente y requiere autorización explícita.
- Freshness podrá consumir `last_verified`, source/snapshot dates y version
  scope sin modificar el contrato de Citation ni confundir esas fechas.
- No implementar Freshness, refresh scheduling ni pasos posteriores antes de su
  autorización independiente.

## Step 16 — Freshness Model v1

Status: completed
Date: 2026-08-25
Release: unreleased

### Delivered

- Modelo puro y versionado `freshness-v1`, con clock inyectable, estados
  `fresh`, `aging`, `stale` y `unknown`, score normalizado y reasons explícitos.
- Inputs diferenciados para `last_verified_at`, `source_updated_at`, cadence de
  releases, claim type, source kind y señal de nueva release conocida.
- Matrices TTL deterministas por claim/source kind, resolución conservadora por
  mínimo y precedencia de hints del Authority Profile: par exacto, claim,
  source y global.
- Triggers inmediatos a `stale` cuando la fuente fue actualizada después de la
  última verificación o existe una nueva release conocida; la ausencia de
  `last_verified_at` produce `unknown` sin inventar fechas.
- Bridge de aplicación hacia `FreshnessRecord` que conserva assessments
  conocidos y rechaza persistir `unknown`; no calcula `next_verify_at`.
- Authority Profiles YAML, memoria y SQLite ampliados con hints TTL validados,
  bounded y copiados defensivamente.
- Migración forward-only v29 para `freshness_ttl_hints_json`, con default legacy
  vacío y round-trip validado.
- Contrato completo documentado en `docs/architecture/freshness-v1.md` y
  referencias de dominio, aplicación, profiles y persistencia sincronizadas.

### Decisions

- El score decae linealmente como `max(0, 1 - age/(2 * TTL))`; `fresh` incluye
  la mitad del TTL, `aging` llega inclusivamente al TTL y luego es `stale`.
- La cadence solo acorta el TTL resuelto; nunca relaja la política del claim,
  source kind o Authority Profile.
- Publication/update, fetched/snapshot y last-verified permanecen como fechas
  distintas. Freshness usa únicamente los inputs explícitos del assessment.
- Los hints están limitados a 64 por profile y entre 1 y 3650 días; selectores
  duplicados son inválidos para evitar precedencias ambiguas.
- No se implementó refresh scheduling, cálculo de próxima verificación, release
  discovery, Resource Quality, UI, live research, Curriculum Compiler ni
  cambios funcionales de Student Core.

### Verification

- Tests dirigidos y vet de Research domain/freshness/application/memory,
  Authority YAML y SQLite.
- Tests de clock inyectable, límites exactos del TTL, score, defaults,
  precedencia de hints, cadence, release/update triggers, `unknown`, chronology,
  defensive copies, migración v29 y round-trip SQLite.
- `GOCACHE=/tmp/kelyro-i03-step16-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step16-vet-gocache go vet ./...`.
- Cross-build tests Linux-hosted para Windows y Darwin de los paquetes afectados
  con `CGO_ENABLED=0`.
- `GOMAXPROCS=2 GOCACHE=/tmp/kelyro-i03-step16-quality-gocache go run ./tools/quality all`,
  incluyendo tests, E2E, vet, `go test -race ./...`, build y smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 17 es el siguiente paso pendiente y requiere autorización explícita.
- Verification scheduling podrá consumir assessments conocidos y el
  `FreshnessRepository` existente sin alterar `freshness-v1` ni fabricar una
  fecha de verificación para evidencias `unknown`.
- No implementar scheduling ni pasos posteriores antes de su autorización
  independiente.

## Step 17 — Last Verified and Refresh Scheduling v1

Status: completed
Date: 2026-08-26
Release: unreleased

### Delivered

- Política pura y versionada `refresh-scheduling-v1` que consume un
  `freshness-v1` conocido y calcula `last_verified_at`, `next_verify_at`, razón
  de verificación y prioridad sin I/O ni background daemon.
- Deadline TTL exacto desde la última verificación y triggers inmediatos para
  nueva release, source changed, conflicto no resuelto, contenido sensible de
  seguridad y solicitud manual.
- Precedencia determinista entre triggers y prioridades cerradas
  `normal`/`high`/`critical`, con manual y security en critical, eventos de
  cambio/conflicto en high y TTL en normal.
- `FreshnessRecordFromSchedule` que exige que Freshness y Schedule compartan la
  misma fecha real de última verificación, conservando separadas las versiones
  de ambos algoritmos.
- Memory y SQLite `ListDue` ordenados por prioridad, deadline e ID estable, sin
  incluir schedules futuros y disponibles completamente offline.
- Migración forward-only v30 con metadata JSON acotada y validada para reason,
  priority y `refresh-scheduling-v1`, compatible con filas Step 16 no agendadas
  y con schedules legacy conservadoramente clasificados como TTL normal.
- Wiring workspace-local y comando read-only `kelyro sources stale`, con clock
  inyectable y output de subject, prioridad, razón, deadline y last verified.
- Contrato, triggers, precedencia, persistencia, orden de cola, CLI y límites
  documentados en `docs/architecture/refresh-scheduling-v1.md` y referencias
  arquitectónicas sincronizadas.

### Decisions

- El scheduler usa el `EvaluatedAt` validado del assessment como instante de
  vencimiento para eventos; no consulta otro clock ni fabrica timestamps.
- Nueva release y source changed se derivan de reason codes ya validados por
  `freshness-v1`; conflicto, security y manual son señales explícitas del
  scheduler.
- `unknown` no es agendable porque carece de `last_verified_at`; un caller debe
  obtener una verificación real antes de persistir scheduling state.
- Cuando coinciden triggers se conserva una razón primaria por precedencia
  explícita; no se depende del orden de inputs ni se oculta la prioridad.
- La metadata v30 se guarda como un objeto JSON cerrado de máximo 256 bytes en
  una sola migración aditiva. Esto mantiene constraints fuertes y evita el
  coste repetido de múltiples alteraciones de esquema en cada workspace/test.
- `kelyro sources stale` solo inspecciona el estado local: no habilita red, no
  ejecuta refresh y no inicia daemon, timer, goroutine ni automatización.
- No se implementaron Resource Quality, release discovery, Conflict Resolver,
  live refresh, Curriculum Compiler ni cambios funcionales de Student Core.

### Verification

- Tests de scheduler para due, not due, TTL exacto, release trigger, source
  changed, conflicto, security, manual trigger, precedencia y `unknown`.
- Tests de mapping application, due filtering/prioridad en memoria y SQLite,
  migración v30/backfill, JSON inválido, round-trip, app wiring y CLI render.
- Tests dirigidos y vet de Research, SQLite, `researchdb`, app y CLI.
- `GOCACHE=/tmp/kelyro-i03-step17-full-gocache go test ./...` y
  `GOCACHE=/tmp/kelyro-i03-step17-full-gocache go vet ./...` fuera del sandbox
  para permitir listeners locales deterministas de `httptest`.
- Race dirigido de SQLite con `GOMAXPROCS=2`, completado en 578.652 s dentro
  del timeout estándar.
- Cross-build tests Linux-hosted para Windows y Darwin de los paquetes
  afectados con `CGO_ENABLED=0`.
- Quality gate completo con `GOMAXPROCS=2`, cache aislada y
  `go run ./tools/quality all`, incluyendo tests, E2E, vet,
  `go test -race ./...`, build y smokes de CLI; la pasada final de SQLite race
  completó en 467.932 s.
- `git diff --check`.

### Notes for next session

- El Paso 18 es el siguiente paso pendiente y requiere autorización explícita.
- Resource Quality podrá consumir sources/evidence existentes sin confundir
  utilidad pedagógica con autoridad, Freshness o scheduling priority.
- No implementar Resource Quality ni pasos posteriores antes de su autorización
  independiente.

## Step 18 — Resource Quality Model v1

Status: completed
Date: 2026-08-26
Release: unreleased

### Delivered

- Modelo puro y determinista `resource-quality-v1` sobre ocho dimensiones
  revisadas: accuracy confidence, clarity, specificity, depth,
  maintainability, examples, accessibility y noise.
- `QualityScore` finito y normalizado en `[0,1]`, con fórmula ponderada
  explícita que da mayor peso a accuracy confidence e invierte noise como
  señal de calidad.
- Recomendaciones cerradas `evidence`, `further_reading`, `example`,
  `supplementary` y `reject`, con gates y precedencia deterministas además del
  score agregado.
- Caso explícito para recursos técnicamente fuertes pero pedagógicamente densos:
  pueden recomendarse como evidence sin calificarse como Further Reading.
- Nueve reasons ordenadas y validadas por assessment: una por dimensión y una
  terminal para la recomendación, con bands y valores visibles.
- Validación relacional que recalcula score y recomendación, exige exactamente
  `resource-quality-v1` y rechaza reasons desconocidas, duplicadas o faltantes.
- Contrato, fórmula, thresholds, precedencia, explainability y límites
  documentados en `docs/architecture/resource-quality-v1.md`, con índice y
  vocabulario de dominio sincronizados.

### Decisions

- Accuracy confidence es un input de revisión de contenido, no AuthorityTier,
  TrustDecision ni ClaimConfidence. El modelo no recibe source kind, publisher,
  dominio, popularidad, discovery rank, freshness ni scheduling priority.
- La recomendación `evidence` describe únicamente la forma técnica del recurso;
  no crea Evidence ni autoriza una Claim. Snapshot, extracción acotada, trust,
  provenance y verification continúan siendo contratos independientes.
- `example` y `further_reading` preceden a `evidence` cuando se cumplen sus gates
  pedagógicos; esto permite escoger el mejor uso sin convertir autoridad en
  facilidad de aprendizaje.
- Rejection tiene precedencia por accuracy confidence menor que `0.50`, score
  menor que `0.40` o noise mayor que `0.85`; el resto de thresholds está
  versionado y documentado como parte inmutable de v1.
- Los inputs faltantes no se inventan ni se derivan de metadata. Un futuro
  evaluator deberá preservar provenance y permanecer detrás de un contrato
  separado.
- No se añadieron repository, migration, adapter, red, CLI/TUI, release
  intelligence, Curriculum Compiler ni cambios funcionales de Student Core.

### Verification

- Tests de fórmula ponderada, reasons, ownership de slices, valores no finitos
  y fuera de rango, divergencias internas y cada uso recomendado.
- Tests específicos para fuente técnica densa, Further Reading, Example,
  Supplementary, rejection por accuracy/noise y thresholds inclusivos.
- `GOCACHE=/tmp/kelyro-i03-step18-research-gocache go test ./internal/research/...`.
- `GOCACHE=/tmp/kelyro-i03-step18-research-vet-gocache go vet ./internal/research/...`.
- Cross-build tests Linux-hosted para Windows y Darwin de Research/quality con
  `CGO_ENABLED=0`.
- `GOCACHE=/tmp/kelyro-i03-step18-full3-gocache go test ./...` fuera del
  sandbox para permitir listeners locales deterministas de `httptest`.
- `GOCACHE=/tmp/kelyro-i03-step18-vet2-gocache go vet ./...`.
- SQLite race dirigida completa en 619.039 s con el mismo conjunto de tests y
  `-timeout=20m`; dos pasadas previas alcanzaron exclusivamente el timeout
  estándar de 600 s durante migrations distintas, sin fallo funcional.
- Quality gate final con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache aislada y
  `go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y
  smokes de CLI.
- `git diff --check`.

### Notes for next session

- El Paso 19 es el siguiente paso pendiente y requiere autorización explícita.
- Release Intelligence podrá reutilizar SourceVersion y los repository ports
  existentes sin mezclar lifecycle/version status con Resource Quality.
- No implementar release discovery ni pasos posteriores antes de su
  autorización independiente.

## Step 19 — Release Intelligence Model

Status: completed
Date: 2026-08-26
Release: unreleased

### Delivered

- Entidad explícita `TechnologyRelease` para technology ID, version,
  released-at opcional, channel, lifecycle status, source IDs y verified-at,
  manteniendo `ReleaseRecord` como alias compatible con los ports existentes.
- Abstracción `VersionIdentifier`, compatible con `SourceVersion`, que conserva
  el texto exacto y clasifica determinísticamente `semantic`, `date_based` u
  `opaque` sin imponer SemVer.
- Parser estricto SemVer 2.0.0 para major/minor/patch, prerelease y build
  metadata, incluidos límites `uint64` y rechazo de ceros iniciales donde la
  especificación no los permite.
- Versiones calendáricas validadas en formatos `YYYY-MM-DD`, `YYYY.MM.DD`,
  `YYYYMMDD`, `YYYY-MM` y `YYYY.MM`, con precisión month/day explícita y fechas
  gregorianas reales.
- Fallback opaco lossless para esquemas generales como `go1.25`, `R2026a`,
  nombres de distribución, ediciones y cualquier identidad no SemVer/date.
- Channels cerrados stable/preview/beta/rc/experimental/nightly/unknown y
  lifecycle statuses current/superseded/legacy/eol/unknown.
- Invariante temporal que rechaza un `released_at` posterior a `verified_at`
  sin inventar una fecha cuando la fuente no la establece.
- Contrato completo documentado en
  `docs/architecture/release-intelligence-model.md`, con domain, application,
  persistence e índice arquitectónico sincronizados.

### Decisions

- La clasificación sigue precedencia SemVer estricto, fecha soportada y opaque.
  Los formatos de fecha usan mes/día con cero inicial para evitar ambigüedad con
  un SemVer válido.
- `NewVersionIdentifier` acepta cualquier identidad textual válida y conserva
  esquemas desconocidos como opaque; los constructores estrictos semantic/date
  permiten exigir uno de esos esquemas cuando el caller sí lo conoce.
- Build metadata forma parte de la identidad SemVer conservada, aunque una
  futura comparación de precedencia deba ignorarla según SemVer.
- Channel y lifecycle status permanecen independientes: preview describe canal
  de distribución, mientras legacy/EOL describen ciclo de vida.
- No se añadió migration: `release_records.version` de v23 ya conserva el texto
  exacto y la clasificación se reconstruye determinísticamente al leer.
- No se implementaron comparación/precedencia, current-stable selection,
  duplicate release policy, provider adapters, discovery, release-notes
  ingestion, auto-upgrade, Curriculum Compiler ni cambios de Student Core.

### Verification

- Tests de SemVer estable y prerelease/build, máximos `uint64`, ceros iniciales,
  semantic inválido, fechas válidas/invalidas, month/day precision y opaque.
- Tests de TechnologyRelease semantic/date/non-semver, stable/preview, enums
  cerrados y cronología release/verification.
- Round-trips de clasificación semantic en SQLite y date-based en el fake de
  memoria/application, sin cambio de schema.
- Tests y vet dirigidos de Research domain/application, SQLite y `researchdb`.
- Cross-build tests Linux-hosted de Research para Windows y Darwin con
  `CGO_ENABLED=0`.
- `GOCACHE=/tmp/kelyro-i03-step19-quality-gocache go test ./...` fuera del
  sandbox para permitir listeners locales deterministas de `httptest`.
- `GOCACHE=/tmp/kelyro-i03-step19-quality-gocache go vet ./...`.
- Quality gate final con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache aislada y
  `go run ./tools/quality all`, incluyendo tests, E2E, vet, race, build y
  smokes de CLI; SQLite race completó en 443.142 s.
- `git diff --check`.

### Notes for next session

- El Paso 20 es el siguiente paso pendiente y requiere autorización explícita.
- Release Discovery deberá permanecer detrás de provider adapters, respetar
  `privacy.allow_network`, producir snapshots/evidence y no depender solo de
  GitHub.
- No implementar discovery, release-notes ingestion ni pasos posteriores antes
  de su autorización independiente.

## Step 20 — Release Discovery y Release Notes ingestion

Status: completed
Date: 2026-08-26
Release: unreleased

### Delivered

- Servicio de aplicación `release-discovery-v1` que descubre releases para una
  tecnología únicamente desde sources aceptadas por Trust Registry y
  autorizadas y ordenadas por su Authority Profile.
- Contrato `ReleaseNotesProvider` libre de red y adapters vendor-neutral para
  feeds JSON y Atom; soportan páginas/changelogs, repositorios/tags y package
  registries oficiales sin acoplar el core ni el workflow a GitHub.
- Captura de cada feed mediante el pipeline existente de snapshots, fetch y
  `privacy.allow_network`, con hash de contenido, tamaño máximo de 1 MiB y
  límites explícitos para sources, releases y cambios.
- Normalización y deduplicación deterministas por identidad exacta
  `version + channel`, unión de sources/notas repetidas y rechazo de fechas
  conocidas incompatibles.
- Selección determinista de current stable y de la familia preview por
  precedencia SemVer/date-based o fecha publicada; previews permanecen
  separados y releases legacy/EOL nunca se reclasifican como current.
- Evidence acotada vinculada al snapshot y Claims `version_change` con
  `VersionScope`, status scope y confidence explícitos, usando IDs estables para
  que una nueva observación de la misma release sea idempotente.
- Persistencia transaccional de Evidence, Claims, releases y cambios de status
  mediante un port de ingestión atómica, implementado en memoria y SQLite; el
  repository de Claims valida todas sus relaciones source/evidence.
- Contrato, algoritmos, precedencia, límites y responsabilidades documentados en
  `docs/architecture/release-discovery-v1.md`, con los documentos de dominio,
  aplicación, persistencia, privacidad e índice sincronizados.

### Decisions

- Los providers interpretan bytes ya capturados y nunca hacen llamadas de red;
  discovery reutiliza `SnapshotCaptureService`, de modo que el único camino live
  conserva SSRF hardening, límites, redirects y el gate de privacidad existente.
- Search results no se convierten en releases ni evidence. Cada source debe
  existir en el registry, tener el kind permitido por el profile y una última
  decisión de trust accepted con tier suficiente.
- Stable y preview se clasifican como familias independientes. Legacy/EOL se
  preservan; identidades opacas sin fechas comparables y empates ambiguos se
  rechazan en vez de inventar precedencia.
- Una feed malformada conserva el snapshot para auditoría pero no persiste una
  ingestión parcial. El batch completo se valida y confirma atómicamente.
- No se añadió migration porque las tablas de releases, evidence y claims ya
  existían. Tampoco se implementó auto-upgrade, Curriculum Compiler,
  Deprecation Intelligence ni cambios de Student Core.

### Verification

- Tests de adapters JSON/Atom para formas repository/registry, stable, RC,
  feed malformada e integridad/tamaño de contenido.
- Tests de aplicación para new stable, separación preview, no releases, feed
  malformada con snapshot retenido, duplicados, prioridad del Authority Profile,
  corroboración multi-source y exclusión de legacy/EOL como current.
- Tests de memoria y SQLite para round-trip de Claims, status updates y rollback
  total de un batch inválido.
- `go test ./internal/research/application/... ./internal/infra/researchrelease ./internal/storage/sqlite`.
- `go test ./...` y `go vet ./...`.
- Cross-build tests Linux-hosted para Windows y Darwin de application,
  `researchrelease` y SQLite con `CGO_ENABLED=0`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 245.525 s.
- La primera ejecución del gate agotó el espacio de `/tmp` durante compilación
  race; se eliminaron únicamente caches aisladas de este paso y la pasada final
  terminó completa sin fallos.
- `git diff --check`.

### Notes for next session

- El Paso 21 es el siguiente paso pendiente y requiere autorización explícita.
- Deprecation & Legacy Intelligence podrá consumir releases, snapshots,
  evidence y Claims existentes sin cambiar la política de current stable.
- No implementar deprecation, historical sources ni pasos posteriores antes de
  su autorización independiente.

## Step 21 — Deprecation & Legacy Intelligence

Status: completed
Date: 2026-08-26
Release: unreleased

### Delivered

- Política determinista `deprecation-intelligence-v1` para conclusiones
  `deprecated`, `removed`, `legacy`, `historical_only` y `superseded` sobre
  prácticas, APIs o versiones generales.
- Señales estructuradas vinculadas a Claim, Evidence y source con dos caminos
  cerrados: statement explícito o strong inference; la ausencia en docs no es
  un tipo de señal válido.
- Validación completa de la cadena persistida `Claim → Evidence → Source`, del
  tipo `deprecation`, de la cronología frente al clock inyectado y del acuerdo
  exacto de status, versiones conocidas y replacement entre señales.
- Admisión de inferencia fuerte únicamente con al menos dos sources distintos y
  confidence `>= 0.8` en cada Claim, marcada durablemente como
  `multi_source_strong_inference` y separada de evidencia explícita.
- `DeprecationRecord` versionado con determination explícita, versions
  introduced/deprecated/removed opcionales, replacement opcional, Evidence,
  sources y `verified_at`.
- Repository port append-only, servicio de assessment/get/history y adapters
  deterministas de memoria y SQLite con copias defensivas y orden estable por
  subject, verificación e ID.
- Migration SQLite forward-only v31 para determination, algorithm version e
  índice de historial. Filas previas se conservan como
  `legacy_unclassified`/`deprecation-unversioned-legacy` sin inventar su base
  evidentiary.
- Contrato, admission policy, persistencia, compatibilidad y límites
  documentados en `docs/architecture/deprecation-intelligence-v1.md`, con
  domain, application, persistence e índice arquitectónico sincronizados.

### Decisions

- El servicio consume señales estructuradas; no parsea keywords ni interpreta
  prosa. El productor de la señal debe asociar su conclusión a Evidence
  literal acotada y tratar el contenido externo como datos no confiables.
- Todos los signals de un assessment deben ser homogéneos y concordar. Una
  discrepancia se rechaza como invalid state; el Paso 21 no anticipa ni oculta
  el Conflict Resolver reservado al Paso 23.
- El threshold y la corroboración por source son una admission policy local de
  deprecation-v1, no producen VerificationResult ni implementan el algoritmo
  general Multi-source Verification del Paso 24.
- Los registros son inmutables: una transición posterior de deprecated a
  removed/legacy se agrega al historial y nunca sobrescribe la guidance que
  aplicaba a versiones anteriores.
- V1 no exige ni inventa versiones o replacement ausentes. La identidad de
  versión continúa siendo opaca y general; tampoco se intenta ordenar esquemas
  que no son comparables.
- Deprecation Intelligence no modifica TechnologyRelease, no auto-upgradea ni
  recompila curriculum y no toca Student Core.
- No se implementó source temporal scope ni clasificación current/historical/
  version-bound/archived, reservada al Paso 22.

### Verification

- Tests de dominio para determinations, algorithm version, compatibilidad
  legacy y mínimo multi-source/evidence.
- Tests de aplicación para statement explícito, replacement opcional,
  inferencia fuerte corroborada, threshold de confidence, single-source,
  ausencia en docs, señales mixtas/discrepantes, claims no-deprecation,
  ownership de versiones e historial deprecated → removed.
- Tests SQLite de migration v31, defaults legacy no inventados, constraints de
  marker/version y multi-source, round-trip y subject history.
- `GOCACHE=/tmp/kelyro-i03-step21-target2-gocache go test ./internal/research/... ./internal/storage/sqlite`.
- `GOCACHE=/tmp/kelyro-i03-step21-full-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step21-full-gocache go vet ./...`.
- Cross-compile de todos los test binaries de Research y SQLite para Windows y
  Darwin con `CGO_ENABLED=0` y `go test -c`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 198.661 s.
- `git diff --check`.

### Notes for next session

- El Paso 22 es el siguiente paso pendiente y requiere autorización explícita.
- Historical Source handling podrá reutilizar el historial inmutable de
  deprecation, pero debe añadir scopes temporales de sources/citations/bundles
  sin reclasificar evidencia actual silenciosamente.
- No implementar historical source handling, Conflict Resolver ni pasos
  posteriores antes de su autorización independiente.

## Step 22 — Historical Source handling

Status: completed
Date: 2026-08-26
Release: unreleased

### Delivered

- Alcance temporal explícito en cada `Source`: `current`, `historical`,
  `version_bound` y `archived`; `version_bound` requiere una versión opaca
  concreta.
- Política pura y determinista `source-temporal-policy-v1`, aislada en
  `internal/research/temporal`, que clasifica cada uso como current guidance,
  exact-version authority, historical context o not applicable.
- Warnings deterministas para todo scope no-current. Documentación archivada y
  material histórico nunca se presentan silenciosamente como guía actual, pero
  old release notes pueden ser autoridad para el comportamiento de su versión
  exacta.
- `citation-v1` conserva ahora scope, warning y algoritmo temporal sin mezclar
  esa decisión con su estrategia de deep link. Las relaciones validan source,
  versión y anotación temporal exactas.
- `SourceBundleSource` captura ID, scope, versión y warning. Los miembros
  version-bound deben coincidir con el target del bundle y un bundle de
  `current_usage` con material no-current no puede quedar `ready` sin caveats.
- Clasificación explícita a través de `SourceService`/`SourceRepository`, con
  adapters de memoria y SQLite. Reclasificar una source no reescribe snapshots,
  Evidence ni citations previas.
- Migration SQLite forward-only v32 para scopes de sources/citations,
  annotations de citation y scopes de source bundle items. Filas previas se
  conservan como current; citations legacy reciben
  `source-temporal-legacy-current` y no pueden insertarse como registros nuevos.
- Contrato, matriz de aplicabilidad, persistencia, compatibilidad y límites
  documentados en `docs/architecture/historical-sources-v1.md`, con domain,
  application, persistence, citations e índice arquitectónico sincronizados.

### Decisions

- Temporal scope es independiente de kind, registry status, authority, trust y
  freshness. Un warning limita aplicabilidad; no degrada por sí mismo la
  autoridad histórica de una fuente oficial.
- V1 compara versiones solo por igualdad exacta. No inventa orden, rangos de
  compatibilidad ni equivalencias entre esquemas opacos.
- Un source no-current solo recibe `version_authority` para purpose
  `version_behavior` con target exacto. En current usage queda como historical
  context con warning o not applicable si existe mismatch conocido.
- Citations y miembros de bundles capturan la clasificación al crearse para que
  una reclasificación posterior de Source no cambie silenciosamente outputs
  históricos.
- El estado `conflicted` del bundle solo preserva una discrepancia visible; no
  decide qué Claim gana. No se implementó el Conflict Resolver del Paso 23.
- No se añadió networking, parsing, verificación multi-source, drift,
  Curriculum Compiler ni ninguna mutación de Student Core.

### Verification

- Tests de dominio para los cuatro scopes, requisito de versión y warnings.
- Tests de `source-temporal-policy-v1` para archived docs, old release notes con
  target exacto/mismatched y separación current/historical.
- Tests de citations y bundles para annotations durables, caveats obligatorios
  y conflicto current vs historical.
- Tests de aplicación y memoria para clasificación explícita y persistencia de
  una citation archivada.
- Tests SQLite de migration v32, defaults legacy, constraints/triggers,
  reclassification y round-trip de citation archivada sin reescribir la previa.
- `GOCACHE=/tmp/kelyro-i03-step22-target-gocache go test ./internal/research/... ./internal/storage/sqlite`.
- `GOCACHE=/tmp/kelyro-i03-step22-verify-gocache go test ./...`.
- `GOCACHE=/tmp/kelyro-i03-step22-verify-gocache go vet ./...`.
- Cross-compile de todos los test binaries de Research y SQLite para Windows y
  Darwin con `CGO_ENABLED=0` y `go test -c`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 227.672 s.
- La primera ejecución paralela de tests/vet globales agotó la cuota de `/tmp`;
  se limpiaron únicamente caches aisladas de este paso y las pasadas
  secuenciales posteriores, incluido el quality gate, terminaron sin fallos.
- `git diff --check`.

### Notes for next session

- El Paso 23 es el siguiente paso pendiente y requiere autorización explícita.
- Conflict Detection & Resolver v1 podrá consumir scopes, warnings y bundles
  temporalmente tipados, pero debe mantener la decisión versionada y explicable.
- No implementar Conflict Resolver, Multi-source Verification ni pasos
  posteriores antes de su autorización independiente.

## Step 23 — Conflict Detection & Resolver v1

Status: completed
Date: 2026-08-26
Release: unreleased

### Delivered

- Política pura, determinista y pairwise `conflict-resolver-v1` en
  `internal/research/conflict`, sin I/O, parsing de prosa, networking ni
  dependencia de providers.
- Señal semántica cerrada de contradicción o desacuerdo de recomendación: el
  caller identifica el par incompatible y v1 lo clasifica sin inventar
  conflictos mediante keywords o inferencia sobre contenido externo.
- Precedencia explícita para `temporal_mismatch`, `version_mismatch`,
  `scope_mismatch`, `recommendation_disagreement`, `authority_mismatch` y
  `direct_contradiction`.
- `Conflict` completado con confidence, reason, winner Claim/source/scope
  opcional, unresolved flag y algorithm version, incluyendo compatibilidad
  conservadora para registros legacy no versionados.
- Reglas contextuales explicables: current guidance frente a material
  no-current, separación de versiones/scopes, preferencia normativa limitada y
  diferencia mínima de dos authority tiers; empates oficiales comparables se
  conservan unresolved y se escalan.
- `ConflictResolutionService` con referencias Claim/source, validación de
  pertenencia, topic compartido, clock y última TrustDecision accepted; el
  orden del input se canonicaliza por Claim ID antes de producir la identidad
  estable.
- `ConflictRepository` append-only con Get e historial por Claim, implementado
  por los adapters deterministas de memoria y SQLite con copias defensivas,
  relaciones validadas y orden cronológico estable.
- Migration SQLite forward-only v33 para confidence, reason, winner metadata,
  algorithm version e índice temporal. Filas previas se leen como
  `conflict-unversioned-legacy` sin inventar winner.
- Contrato, precedencia, reglas, persistencia, compatibilidad y límites
  documentados en `docs/architecture/conflict-resolver-v1.md`, con domain,
  application, persistence e índice arquitectónico sincronizados.

### Decisions

- El resolver consume un par ya identificado como incompatible. Detectar
  contradicción semántica en prosa arbitraria pertenece a un productor
  estructurado o revisión humana futura; source content continúa siendo datos
  no confiables, nunca instrucciones.
- V1 es pairwise para que cada decisión tenga exactamente dos Claims
  auditables. Un conjunto mayor se expresa como pares append-only; no se crea
  un ganador global opaco.
- Temporal, version y applicability scope se evalúan antes de authority. Una
  fuente globalmente más fuerte no puede borrar silenciosamente guidance
  válida para otra versión o contexto.
- Specification/standard solo gana una Claim normativa si su authority tier
  revisado es al menos tan fuerte. Fuera de esa regla, v1 exige una diferencia
  mínima de dos tiers y nunca desempata por ID, orden, freshness o score alto.
- Un conflicto de versiones o scopes puede quedar resuelto sin winner: la
  resolución consiste en conservar ambos Claims dentro de sus límites.
- Conflict confidence describe la confianza en clasificación/resolución; no es
  truth probability, TrustDecision, freshness ni VerificationResult del Paso
  24.
- Reassessment agrega historia y no sobreescribe decisiones previas. No se
  modifican Evidence, Source Bundles, curriculum, Student Core ni mastery.

### Verification

- Tests de dominio/política para autoridad normativa clara, prevención de
  preferencia automática por label oficial, current vs historical, scopes de
  versión y contradicción unresolved entre documentos oficiales equivalentes.
- Tests de aplicación/memoria para resolución persistida, canonicalización,
  copias defensivas, historial por Claim, TrustDecision rechazada y source no
  perteneciente al Claim.
- Tests SQLite de migration v33, default legacy no inventado, constraints de
  winner, round-trip completo y lookup por Claim.
- `GOCACHE=/tmp/kelyro-i03-step23-target-final-gocache go test ./internal/research/... ./internal/storage/sqlite`.
- `GOCACHE=/tmp/kelyro-i03-step23-full-gocache go test ./...` fuera del sandbox
  para permitir los listeners loopback deterministas de `httptest`; la primera
  pasada sandboxed solo falló por `socket: operation not permitted`.
- `GOCACHE=/tmp/kelyro-i03-step23-full-gocache go vet ./...` y vet focalizado
  final con el cache `step23-target-final`.
- Cross-compile de todos los test binaries de Research y SQLite para Windows y
  Darwin con `CGO_ENABLED=0` y `go test -c`.
- Quality gate final completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 251.510 s.
- `git diff --check`.

### Notes for next session

- El Paso 24 es el siguiente paso pendiente y requiere autorización explícita.
- Multi-Source Verification podrá consumir conflictos v1 visibles y sus
  Claims/sources, pero debe implementar su propia policy de corroboración e
  independencia organizacional sin alterar el historial del resolver.
- No implementar Multi-Source Verification, Source Bundle ni pasos posteriores
  antes de su autorización independiente.

## Step 24 — Multi-Source Verification

Status: completed
Date: 2026-08-27
Release: unreleased

### Delivered

- Política pura y versionada `multi-source-verification-v1` para evaluar el set
  completo de fuentes declarado por cada Claim, con requisitos cerrados para
  definiciones/requisitos normativos, recomendaciones de producción, seguridad,
  técnicas comunitarias y soporte general.
- `VerificationResult` completado con requirement, reason codes, confidence,
  algorithm version y las cuatro métricas exigidas: source count, independent
  organization count, authority distribution y scope consistency.
- `VerificationService.Verify` reemplaza el recording arbitrario: carga Claim,
  Sources, últimas Trust Decisions, ownership/status revisado del Source
  Registry y conflictos append-only; luego ejecuta la policy y persiste un
  resultado inmutable. `Get` y `Latest` conservan lectura offline.
- Independencia organizacional derivada exclusivamente del Registry: domains,
  mirrors o páginas con la misma organización normalizada cuentan una vez y un
  ownership desconocido no se inventa como organización independiente.
- Integración temporal/versionada y de conflictos: observaciones fuera de scope
  quedan visibles pero no soportan la Claim; el último conflicto por par puede
  producir `conflicted` o `rejected` sin reejecutar ni alterar el resolver v1.
- Adapters memory/SQLite endurecidos para exigir que el source set del resultado
  coincida exactamente con `Claim.SourceIDs`, con copias defensivas y orden
  determinista.
- Migration SQLite forward-only v34 para requirement, métricas, reason codes y
  algorithm version. Filas previas permanecen
  `verification-unversioned-legacy` con clasificación y métricas conservadoras,
  sin inventar corroboración.
- Contrato, reglas, métricas, confianza, persistencia, compatibilidad y límites
  documentados en `docs/architecture/multi-source-verification-v1.md`, con
  domain, application, persistence e índice arquitectónico sincronizados.

### Decisions

- `definition` y `requirement` admiten una única primary source solo cuando su
  última Trust Decision es accepted tier A/B; recomendaciones alcanzan
  `verified` con dos fuentes fuertes de organizaciones independientes y una
  fuente fuerte queda como `verified_with_caveat`.
- Claims de seguridad exigen authority tier A en specification, standard,
  official documentation o source code. Cantidad de fuentes comunitarias no
  sustituye esa autoridad.
- Claims `example` modelan la técnica comunitaria del Paso 24 y requieren dos
  fuentes accepted de organizaciones independientes. El resto usa soporte
  general conservador.
- Authority distribution incluye todas las fuentes y conserva `unknown` cuando
  no hay Trust Decision. Solo fuentes accepted y scope-consistent participan en
  corroboración y conteo organizacional.
- `blocked`/`deprecated` Registry status o TrustDecision rejected no soportan la
  Claim. No se infiere una tier desde el kind, locator, título o contenido.
- Version-bound requiere match exacto; historical/archived soporta una Claim
  histórica o la versión exacta. Scope inconsistency nunca se oculta.
- El historial Conflict se reduce al último resultado por par canónico: un
  unresolved visible prevalece como `conflicted`; perder una resolución visible
  produce `rejected`.
- Confidence es Claim confidence limitada por status (`0.95`, `0.75`, `0.40`,
  `0.30`, `0.10`); no es truth probability ni reemplaza Trust, freshness o
  conflict confidence.
- No se añadieron network calls, Source Bundles, Curriculum Compiler, cambios de
  Student Core/mastery ni comportamiento de pasos posteriores.

### Verification

- Tests de política para primary source normativa, recomendaciones fuertes
  independientes y same-organization, autoridad de seguridad, corroboración
  comunitaria, trust/organization desconocidos, scope inconsistente y conflictos
  unresolved/resolved.
- Tests de aplicación/memoria para orquestación persistida, ownership de dos
  organizaciones, mirrors del mismo publisher, copias defensivas, Get/Latest y
  consumo posterior de conflicto visible.
- Tests SQLite para migration v34, defaults legacy conservadores, constraints,
  source-set integrity y round-trip completo de requirement/métricas/reasons.
- `GOCACHE=/tmp/kelyro-i03-step24-target1-gocache go test ./internal/research/... ./internal/storage/sqlite` y vet focalizado.
- `GOCACHE=/tmp/kelyro-i03-step24-full-gocache go test ./...` fuera del sandbox
  para permitir listeners loopback deterministas de `httptest`.
- `GOCACHE=/tmp/kelyro-i03-step24-full-gocache go vet ./...`.
- Cross-compile de todos los test binaries de Research y SQLite para Windows y
  Darwin con `CGO_ENABLED=0` y `go test -c`.
- Quality gate final completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 265.443 s.
- `git diff --check`.

### Notes for next session

- El Paso 25 es el siguiente paso pendiente y requiere autorización explícita.
- Source Bundle podrá consumir Claims, Evidence, provenance y resultados de
  verification ya persistidos, pero debe definir su propia lifecycle policy sin
  reinterpretar ni sobrescribir `multi-source-verification-v1`.
- No implementar Source Bundle ni pasos posteriores antes de su autorización
  independiente.

## Step 25 — Source Bundle

Status: completed
Date: 2026-08-27
Release: unreleased

### Delivered

- Contrato inmutable y acotado `source-bundle-v1` con topic, purpose, target
  version, Research Run, Claims, fuentes primary/supporting/historical,
  conflictos visibles, freshness agregada, issues cerrados, estado, summary,
  algoritmo y content hash.
- JSON canónico machine-readable de hasta 256 KiB y summary human-readable de
  hasta 8 KiB, sin cuerpos web, documentos completos ni excerpts de Evidence;
  parse estricto, orden determinista y hash SHA-256 reproducible que detecta
  cualquier alteración del payload.
- Ensamblador puro `internal/research/bundle` con clasificación de fuentes a
  partir de scope temporal y Trust Decision revisada, reducción del último
  conflicto por par de Claims y agregación conservadora
  `source-bundle-freshness-v1`.
- Precedencia explícita de lifecycle: evidencia/verificación/freshness faltante
  produce `incomplete`; conflicto unresolved produce `conflicted`; material
  histórico, conflicto resuelto, verification caveat o freshness aging/stale
  produce `ready_with_caveats`; solo un set sin issues produce `ready`.
- `SourceBundleService` offline sobre registros persistidos: exige un Research
  Run completado, carga Claims/Evidence/Sources/Trust/Verification/Conflicts/
  Freshness, ensambla, persiste y expone Get, Export y history por Run.
- Puerto append-only y fake de memoria con validación relacional, orden estable
  y copias defensivas.
- Migration SQLite forward-only v35 y adapter atómico para canonical JSON/hash,
  metadata indexada y filas ordenadas Claim/source con role, temporal scope,
  version y warning congelados.
- Compatibilidad conservadora: bundles previos siguen legibles como
  `source-bundle-unversioned-legacy`, con freshness unknown, fuentes
  unclassified y sin inventar JSON/hash v1.
- Contrato documentado en `docs/architecture/source-bundles-v1.md`, con domain,
  application, persistence e índice arquitectónico sincronizados.

### Decisions

- El bundle contiene identidades y annotations suficientes para recuperar
  Claims/Evidence locales; no duplica Evidence excerpts ni contenido externo.
- Primary requiere Trust Decision `accepted`, tier A/B y kind normativo/
  oficial de referencia. `accepted_as_supplement` y los demás kinds permanecen
  supporting; cantidad o popularidad no eleva autoridad.
- Archived/historical y version-bound que no coincide con el target quedan en
  historical. Un version-bound exacto puede participar como primary/supporting.
- Freshness agrega Claims: usa el score mínimo, el `last_verified_at` más
  antiguo y el peor state conocido; cualquier Claim sin record vuelve el
  agregado `unknown` e incompleto.
- `incomplete` precede a `conflicted` porque un conflicto no convierte un set
  sin evidencia suficiente en input compilable.
- El hash cubre toda la representación canónica salvo su propio campo,
  incluyendo IDs, timestamps, summary, state y versiones de algoritmo.
- Cada reensamblado crea un registro nuevo; no se reescriben bundles previos ni
  Trust/Verification/Conflict/Freshness históricos.
- No se añadieron network calls, Further Reading, Curriculum Compiler, cambios
  de Student Core/mastery ni comportamiento de pasos posteriores.

### Verification

- Fixtures añadidas para serialización/hash deterministas, tamper detection,
  missing required Evidence, ensamblado/application/memory y migration v35 con
  round-trip/legacy compatibility SQLite.
- `GOCACHE=/tmp/kelyro-i03-step25-target-gocache go test
  ./internal/research/... ./internal/storage/sqlite`.
- `GOCACHE=/tmp/kelyro-i03-step25-vet-gocache go vet ./...`.
- `GOCACHE=/tmp/kelyro-i03-step25-full-gocache go test ./...` fuera del sandbox
  para permitir listeners loopback deterministas de `httptest`.
- Cross-compile de todos los test binaries de Research y SQLite para Windows
  amd64 y Darwin amd64/arm64 con `CGO_ENABLED=0` y `go test -c`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 292.159 s.
- `gofmt` aplicado a los archivos Go modificados y `git diff --check` sin
  errores.

### Notes for next session

- El Paso 26 es el siguiente paso pendiente y requiere autorización explícita.
- No implementar Further Reading ni pasos posteriores antes de su autorización
  independiente.

## Step 26 — Further Reading Selection

Status: completed
Date: 2026-08-27
Release: unreleased

### Delivered

- Política pura, offline y versionada `further-reading-selection-v1` para
  escoger recursos útiles para estudiantes sin convertirlos en Evidence ni
  alterar Source Bundles.
- Siete categorías cerradas: official deep dive, tutorial, interactive
  resource, reference, community explanation, video supplement y source code;
  reading levels introductory/intermediate/advanced y acceso
  open/registration/paywalled/unknown explícitos.
- Admisión conservadora sobre Trust Decision accepted/supplement y assessments
  `resource-quality-v1` recomendados como further reading/example, con
  applicability de `source-temporal-policy-v1` y exclusiones explicables.
- Ranking determinista que pondera quality, reading-level fit, freshness,
  authority, access y temporal scope sin permitir que autoridad sustituya
  utilidad pedagógica.
- Deduplicación mediante clave revisada explícita y selección greedy con bonuses
  acotados por categoría y organización; organizaciones desconocidas no reciben
  diversidad inventada.
- Límite obligatorio de hasta siete seleccionados y 128 candidatos, con reason
  cerrado para cada candidato válido no elegido y desempate final por Source ID.
- Labels y warnings student-visible para community, registration/paywall,
  acceso desconocido, tutorial/resource stale, freshness unknown, material
  histórico, nivel superior al target y organización desconocida.
- Contrato, fórmula, orden, disclosures, límites y separación de
  responsabilidades documentados en
  `docs/architecture/further-reading-selection-v1.md`, con índice, dominio y
  Resource Quality sincronizados.

### Decisions

- Reading selection consume evaluaciones revisadas; no infiere reading level,
  paywall, ownership, duplicación semántica ni community status leyendo
  contenido externo.
- Quality conserva el mayor peso. Una primary source tier A evaluada únicamente
  como `evidence` no se promueve automáticamente a lectura; `example` sí puede
  entrar como material pedagógico.
- Trust `requires_verification`/`rejected` y versiones no aplicables se excluyen;
  historical y stale pueden conservar valor contextual, pero nunca pierden sus
  warnings.
- Paywall no implica rechazo automático: el resultado conserva simultáneamente
  access, label y warning para que una presentación no lo oculte.
- Duplicación usa una clave aportada por revisión; el algoritmo no compara prosa
  arbitraria ni hashes de contenido para inventar equivalencia.
- `interactive_resource` es una categoría de lectura revisada y no añade el
  source kind ni discovery especializado Playground reservado al Paso 27.
- No se añadieron repositories, migration SQLite, network calls, CLI/TUI,
  Curriculum Compiler ni cambios de Student Core/mastery.

### Verification

- Tests de selección para labels community/paywall, warning dedicado de stale
  tutorial, reading level superior, primary evidence no promovida, trust
  rejected, versión incompatible e historical warning.
- Tests de determinismo bajo reordenamiento, diversidad de categoría/
  organización, deduplicación revisada, reading-level fit, límites y community
  label obligatorio.
- `GOCACHE=/tmp/kelyro-i03-step26-research-gocache go test ./internal/research/...`.
- `GOCACHE=/tmp/kelyro-i03-step26-research-vet-gocache go vet ./internal/research/...`.
- `GOCACHE=/tmp/kelyro-i03-step26-full-gocache go test ./...` y
  `go vet ./...`.
- Cross-compile del test binary de `internal/research/furtherreading` para
  Windows amd64 y Darwin amd64/arm64 con `CGO_ENABLED=0` y `go test -c`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 302.530 s.
- `gofmt` aplicado a los archivos Go nuevos y `git diff --check` sin errores.

### Notes for next session

- El Paso 27 es el siguiente paso pendiente y requiere autorización explícita.
- No implementar Playground, Package Reference/Standards especializados ni
  pasos posteriores antes de su autorización independiente.

## Step 27 — Specialized Playground, Package Reference and Standards sources

Status: completed
Date: 2026-08-27
Release: unreleased

### Delivered

- Contrato de dominio cerrado y versionado `specialized-source-metadata-v1`
  para representar Playground, Package Reference y Standards sin supuestos de
  lenguaje o ecosistema.
- Playground registra interactividad, language/runtime, versión opcional,
  afiliación official/community y locator compartible; el nuevo source kind
  `playground` exige siempre metadata especializada válida.
- Package Reference registra package/module, symbol y versión opcionales,
  canonical docs y source code link opcional; Standards registra standards
  body, standard ID, revision opcional, estado cerrado y official locator.
- Validación relacional entre `Source` y su especialización, clon defensivo en
  application memory y codec JSON canónico, estricto y acotado a 8192 bytes.
- Trust Policy distingue Playground official (tier B) de community (tier D),
  Freshness aplica un TTL por defecto de 30 días y Further Reading exige
  Playground para `interactive_resource` y coherencia del label community.
- Migración forward-only SQLite v36 con columnas y constraints especializados;
  Playground usa una proyección física compatible `other` más
  `specialized_kind=playground`, revertida y validada por el adapter.
- Round-trip persistente y compatibilidad con filas legacy sin inventar
  metadata especializada; payloads malformados producen error de persistencia.
- Contrato y decisiones documentados en
  `docs/architecture/specialized-technical-sources-v1.md`, con índices y
  documentos de dominio, aplicación, persistencia, trust, freshness y further
  reading sincronizados.

### Decisions

- Package Reference y Standards admiten metadata ausente sólo para fuentes
  legacy; cuando existe, debe ser completa, canónica y del mismo kind.
- Canonical docs de Package Reference y official locator de Standards deben
  coincidir con `Source.Locator`; no se duplican identidades contradictorias.
- Versiones, revisiones, symbols y source code links son opcionales para no
  inventar datos que una fuente no declara; `unknown` es un estado explícito de
  Standards.
- La migración no modifica el CHECK histórico de `sources.kind`: la proyección
  de Playground preserva migraciones publicadas y sigue siendo atómica en la
  misma fila.
- No se añadió discovery de red, Community Resource Policy, query planner
  especializado, CLI/TUI, Curriculum Compiler ni cambios de Student Core o
  mastery.

### Verification

- Tests de validación y JSON canónico con ejemplos domain-general de Python,
  TypeScript e IETF, incluyendo vocabularios cerrados, relaciones de locators y
  mismatch de kinds/versiones.
- Tests de clon defensivo, trust official/community, freshness, Further Reading,
  constraints de migración, compatibilidad legacy y round-trip SQLite de los
  tres tipos especializados.
- `GOCACHE=/tmp/kelyro-i03-step27-target2-gocache go test
  ./internal/research/... ./internal/storage/sqlite` y `go vet` dirigido.
- `GOCACHE=/tmp/kelyro-i03-step27-full-gocache go test ./...` y
  `go vet ./...`.
- Cross-compile con `CGO_ENABLED=0` de los test binaries relevantes para
  Windows amd64 y Darwin amd64/arm64.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI.
- Auditoría sin hardcode de Go en el contrato especializado, `gofmt` aplicado y
  `git diff --check` sin errores.

### Notes for next session

- El Paso 28 es el siguiente paso pendiente y requiere autorización explícita.
- No implementar Community Resource Policy ni pasos posteriores antes de su
  autorización independiente.

## Step 28 — Community Resource Policy

Status: completed
Date: 2026-08-27
Release: unreleased

### Delivered

- Política pura y versionada `community-resource-policy-v1` para blog, forum,
  Q&A, conference talk, community tutorial y repository example sobre source
  kinds existentes y sin dependencias de red o almacenamiento.
- Rol `supplementary` y tier D por defecto; elevación máxima a
  `recognized_supplementary` tier C sólo cuando un Authority Profile aplicable
  reconoce explícitamente kind y organization/publisher o dominio.
- Diferenciación entre recurso y comment: comentarios permanecen
  `context_only`, tier D y requieren verificación incluso bajo un profile
  reconocido.
- Freshness revisada como input obligatorio; estados stale/unknown requieren
  verificación y nunca se infieren desde publication date o engagement.
- Votes/views aceptados sólo como señales informativas y excluidos por diseño
  de truth, authority, role y verification, con reason estable
  `popularity.ignored`.
- Attribution explícita con title, author/organization opcionales, publisher y
  locator canónico, sin inventar identidades ausentes.
- Integración conservadora con `trust-policy-v1`: valida identidad/freshness,
  evita que repository examples comunitarios hereden tier B de source code y
  mantiene todo resultado community como suplemento o verificación requerida.
- Contrato y límites documentados en
  `docs/architecture/community-resource-policy-v1.md`, con índices, dominio y
  Trust Policy sincronizados.

### Decisions

- Authority Profile no convierte una fuente comunitaria en normativa: la
  elevación está acotada a tier C y requiere coincidencia conjunta de kind e
  identidad organizacional/dominio.
- `AllowedSupplementaryKinds` permite uso contextual pero no eleva autoridad;
  la preferencia debe ser explícita en `PreferredKinds`.
- Los counts de engagement no se copian a razones ni fórmulas; inputs con cero
  o millones de interacciones producen la misma decisión.
- Conference talk utiliza el source kind host-neutral `video`, mientras sus
  metadatos específicos se reservan al Paso 29.
- No se añadió migración, repository, network adapter, CLI/TUI, transcript,
  Source Bundle, Curriculum Compiler ni cambio de Student Core/mastery.

### Verification

- Tests de los seis resource types, mapping de source kinds, supplement por
  defecto, elevación y no-elevación por Authority Profile, comment context-only,
  freshness, attribution y neutralidad ante popularidad.
- Tests de integración Trust para repository example reconocido y comment que
  requiere verificación.
- `GOCACHE=/tmp/kelyro-i03-step28-target-gocache go test
  ./internal/research/community ./internal/research/trust` y `go vet` dirigido.
- `go test -race ./internal/research/community ./internal/research/trust`.
- Suite completa `go test ./...` y `go vet ./...` con cache compartida aislada.
- Cross-compile del test binary community para Windows amd64 y Darwin
  amd64/arm64 con `CGO_ENABLED=0`.
- `gofmt` aplicado y `git diff --check` sin errores. Un primer intento de suite
  completa agotó una cuota temporal; tras limpiar exclusivamente caches Go
  regenerables de Kelyro, la repetición terminó correctamente.

### Notes for next session

- El Paso 29 está autorizado a continuación por el usuario.
- No implementar Source Diversity ni pasos posteriores antes de su autorización
  independiente.

## Step 29 — Video Supplement metadata

Status: completed
Date: 2026-08-27
Release: unreleased

### Delivered

- Contrato host-neutral y versionado `video-supplement-metadata-v1` para video
  supplements, integrado como metadata opcional de `SourceVideo`.
- Normalización sin duplicación: video URL, title, publisher y published_at
  permanecen en Source; channel, duration, description acotada,
  official/community, transcript availability y deep links temporales viven en
  el record específico.
- Transcript availability cerrada como available/partial/unavailable/unknown;
  el dominio, JSON y SQLite no contienen ningún campo de transcript text.
- Hasta 32 deep links ordenados, únicos y dentro de duration. Los locators son
  aportados por adapters y `DeepLinkAt` sólo devuelve coincidencias explícitas,
  sin construir sintaxis específica de un host.
- JSON canónico estricto de hasta 16 KiB, rechazo de unknown fields/trailing
  data y clon defensivo de la colección de deep links.
- Trust Policy asigna tier B a video official y D a community/legacy, pero
  mantiene todo video como supplement; emite reasons
  `authority.video_official/community`.
- Further Reading exige que el label community coincida con la afiliación
  revisada; Community Resource Policy rechaza un official video presentado
  como community conference talk. Freshness conserva TTL de 60 días.
- Migración forward-only SQLite v37 añade `video_metadata_json` acotado sólo
  para el kind físico video; filas legacy permanecen con metadata vacía y sin
  estados inventados.
- Contrato, persistencia e integraciones documentados en
  `docs/architecture/video-learning-resources-v1.md`, con índices, dominio,
  application, trust, freshness, Further Reading y Community Policy
  sincronizados.

### Decisions

- Title/publisher/published_at no se duplican dentro del JSON especializado;
  una fuente v1 exige publisher, published_at y locator coincidente.
- Metadata video sigue opcional para compatibilidad legacy. Vacío significa
  desconocido/no clasificado, no transcript unavailable ni community.
- Official mejora autoridad contextual hasta tier B, pero nunca cambia el rol
  suplementario ni vuelve el transcript evidencia primaria.
- Deep-link syntax queda fuera del domain; cada adapter declara el locator
  exacto asociado al timestamp conocido.
- Description, duration, deep links y JSON tienen límites explícitos; no se
  almacena media ni contenido web crudo.
- No se añadió provider de video, network access, transcript fetch, playback
  CLI/TUI, popularity ranking, Source Diversity, Curriculum Compiler ni cambio
  de Student Core/mastery.

### Verification

- Tests de codec canónico host-neutral, vocabulario transcript, límites,
  relaciones Source/video, deep links explícitos y clon defensivo.
- Tests de Trust official/community, Further Reading label, Community Policy,
  memory repository, migración legacy, constraints y round-trip SQLite.
- `GOCACHE=/tmp/kelyro-i03-step29-target-gocache go test
  ./internal/research/... ./internal/storage/sqlite` y `go vet` dirigido.
- Suite completa `go test ./...` y `go vet ./...`.
- Cross-compile de test binaries de domain, application, trust y SQLite para
  Windows amd64 y Darwin amd64/arm64 con `CGO_ENABLED=0`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 892.329 s.
- Auditoría sin provider específico en el domain, `gofmt` aplicado y
  `git diff --check` sin errores.

### Notes for next session

- El Paso 30 es el siguiente paso pendiente y requiere autorización explícita.
- No implementar Source Diversity ni pasos posteriores antes de su autorización
  independiente.

## Step 30 — Source Diversity policy

Status: completed
Date: 2026-08-27
Release: unreleased

### Delivered

- Política pura, determinista y versionada `source-diversity-v1` para evaluar
  la corroboración real de las fuentes asociadas a un claim persistido.
- Conteo conservador de fuentes independientes mediante componentes conexos:
  fuentes de la misma organización o con una dependencia revisada común se
  agrupan y no inflan la corroboración.
- Estados explícitos `sufficient`, `concentrated`, `unknown` y
  `normative_source_sufficient`, junto con conteos de fuentes totales,
  elegibles, independientes, organizaciones, kinds y perspectivas.
- Excepción acotada para una única fuente normativa aceptada: una
  Specification o Standard tier A puede bastar para claims de definición o
  requisito sin generar diversidad artificial.
- Evaluación separada de organization, source kind, perspective y rol técnico
  reference/implementation; geography y language quedan declaradas como
  dimensiones diferidas, no inferidas.
- Warnings estructurados y estables para falta de soporte aceptado,
  concentración organizacional, dependencia compartida, metadata desconocida
  y concentración o ausencia en las dimensiones descriptivas.
- `SourceDiversityService` compone Claim, Source, última TrustDecision y
  Trusted Registry persistidos con annotations revisadas del caller; exige
  cobertura exacta de todos los source IDs del claim y no persiste una segunda
  verdad derivada.
- Contrato y composición documentados en
  `docs/architecture/source-diversity-v1.md`, con índices y contratos vecinos
  de dominio, aplicación, verificación multi-source y bundles sincronizados.

### Decisions

- La independencia es un límite inferior auditable, no una inferencia
  optimista: compartir organización o dependency group une fuentes; metadata
  desconocida genera advertencias y nunca se inventa.
- Sólo decisiones Trust `accepted` o `accepted_supplement` participan como
  soporte elegible. Las demás fuentes permanecen visibles en el total pero no
  cuentan como corroboración.
- Las dimensiones descriptivas producen diagnósticos sin rebajar una
  independencia genuina entre organizaciones y dependencias distintas.
- Perspective y technical role usan vocabularios cerrados y el caller debe
  proporcionar clasificación revisada; la aplicación no intenta deducirla del
  contenido externo.
- Organization sólo se toma del Trusted Registry cuando la entrada coincide
  con el locator y aplica al topic/kind; dependency group nunca se deriva
  implícitamente de organización o dominio.
- No se añadió migración ni repository: el assessment es reproducible desde
  records canónicos existentes y annotations revisadas, preservando los
  contratos inmutables `multi-source-verification-v1` y `source-bundle-v1`.
- No se añadió Real Source Code Evidence, acceso de red, CLI/TUI, Curriculum
  Compiler ni cambios de Student Core o mastery.

### Verification

- Tests de fuente normativa única, concentración por organización, dependencia
  compartida entre organizaciones, independencia genuina y warnings separados
  de kind, perspective y reference/implementation.
- Tests de metadata desconocida, ausencia de Trust aceptado, cobertura exacta,
  vocabularios inválidos, determinismo ante reordenamiento y clon defensivo de
  warnings.
- Tests de integración de aplicación con Claim, Source, TrustDecision y
  Trusted Registry persistidos, además de annotations incompletas y dependency
  groups revisados.
- `GOCACHE=/tmp/kelyro-i03-step30-target-gocache go test
  ./internal/research/diversity ./internal/research/application`, `go vet`
  dirigido y `go test -race` de ambos paquetes.
- `GOCACHE=/tmp/kelyro-i03-step27-full-gocache go test ./...` y
  `go vet ./...`.
- Cross-compile con `CGO_ENABLED=0` de los test binaries de diversity y
  application para Windows amd64 y Darwin amd64/arm64.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 888.339 s.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 31 es el siguiente paso pendiente y requiere autorización explícita.
- No implementar Real Source Code Evidence ni pasos posteriores antes de su
  autorización independiente.

## Step 31 — Real Source Code Evidence

Status: completed
Date: 2026-08-29
Release: unreleased

### Delivered

- Contrato de dominio host-neutral y versionado `source-code-evidence-v1`
  integrado en Evidence mediante `SourceCodeLocator`: repositorio, permalink,
  commit, path, rango de líneas, symbol, version scope y licencia opcional.
- Permalinks obligatoriamente fijados a revisions hexadecimales de 7–64
  caracteres, mismo host que el repositorio y diferentes de la URL canónica;
  branches simbólicos como `main` no pueden presentarse como evidencia
  reproducible.
- Paths relativos limpios y portables, rangos positivos acotados a 200 líneas,
  metadata textual acotada, version scope requerido y licencia revisada
  opcional sin inferirla cuando falta.
- Relación validada entre Source/Evidence: nueva evidencia de kind
  `source_code` requiere locator v1; otros kinds lo rechazan y la versión debe
  coincidir cuando Source ya declara una.
- Citation v1 consume el permalink persistido en Evidence y elimina el input
  efímero duplicado, conservando `source_permalink` y presentación offline.
- Trust Policy probado para mantener Specification normativa en tier A y
  Source Code en tier B en language-specification, sin permitir que
  implementación reemplace silenciosamente una spec.
- Migración SQLite forward-only v38 con JSON canónico de hasta 8 KiB, guards
  por source kind, compatibilidad legacy sin metadata inventada y round-trip
  completo; memory/SQLite usan copias defensivas.
- Contrato, persistencia, citation integration, límites y fronteras
  documentados en `docs/architecture/real-source-code-evidence-v1.md` y
  documentos Research relacionados.

### Decisions

- SourceCodeLocator pertenece a la Evidence concreta, no a Source: path, rango
  y symbol describen el excerpt observado y pueden variar dentro de un mismo
  repositorio estable.
- El adapter de forge entrega el permalink exacto; domain valida componentes
  reproducibles sin construir URLs específicas de GitHub/GitLab.
- Version scope es siempre explícito y opaco; puede representar release,
  edición o alcance revisado por commit. Si Source tiene version, ambos scopes
  deben coincidir.
- License metadata es contexto de attribution, no una decisión automática de
  compatibilidad; ausencia permanece desconocida.
- Filas Evidence legacy sin locator siguen legibles, pero sólo nuevos records
  validados pueden llamarse Real Source Code Evidence v1.
- No se añadió provider GitHub, clone/Git execution, fetch de red, whole-file
  storage, license inference, Research Cache, CLI/TUI, Curriculum Compiler ni
  cambios de Student Core/mastery.

### Verification

- Tests de codec JSON canónico, clones defensivos, branch mutable, permalink
  sin commit, cross-host, traversal, line limit, version requerida y relaciones
  Source/Evidence.
- Tests de citation derivada desde metadata persistida, precedencia normativa
  Trust, migración legacy v37 → v38, constraints y round-trip SQLite.
- `GOCACHE=/tmp/kelyro-i03-step31-full-gocache go test ./...` fuera del sandbox
  para permitir listeners locales de `httptest`.
- `GOCACHE=/tmp/kelyro-i03-step31-vet-gocache go vet ./...`.
- Cross-compile del test binary Research para Windows amd64 y Darwin
  amd64/arm64 con `CGO_ENABLED=0`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 358.328 s.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 32 está autorizado a continuación por el usuario.
- No implementar Research Cost Control ni pasos posteriores antes de su
  autorización independiente.

## Step 32 — Research Cache and Offline Research Mode

Status: completed
Date: 2026-08-28
Release: unreleased

### Delivered

- Servicio versionado `research-cache-v1` con cinco layers explícitos:
  discovery, fetch metadata, bounded source, normalized source y source bundle.
- Política TTL/tamaño por layer: 24 h/256 KiB discovery, 24 h/32 KiB fetch
  metadata, 7 días/1 MiB bounded y normalized source, y 30 días/256 KiB source
  bundle.
- Límites globales de 512 records y 64 MiB con eviction determinista: expirados
  primero y luego oldest stored_at con desempate estable por layer/key.
- Outcomes `CacheLookup` con hit explícito, stale explícito y warning cerrado
  `stale_cache_used_offline`; fresh-only trata stale como miss y offline puede
  consumirlo conservando la advertencia.
- Adapter filesystem cross-platform en `.kelyro/cache/research`, derivado de
  `WorkspaceResearchCacheDir`, con nombres SHA-256, permisos restrictivos,
  staging y replace atómico.
- Envelopes JSON estrictos/acotados con schema y algorithm version, content
  hash canónico, timestamps/TTL, identidad y detección de unknown fields,
  trailing data, oversize, filename mismatch y corrupción de hash.
- Adapter offline concreto para SearchCache, SourceFetchCache y
  ReleaseLookupCache; fetched source se divide entre metadata/body y los codecs
  revalidan todos los records reconstruidos.
- Layers expuestas para normalized source y canonical Source Bundle JSON; bundle
  cacheado debe conservar la identidad pedida.
- SearchResult y FetchedSource cacheados exponen hit/stale/warning; live outputs
  limpian defensivamente metadata de cache aportada por providers externos.
- CLI completa `kelyro research cache status|clear`, con métricas por layer,
  stale/corrupt totals y confirmación visible de que snapshots/evidence no se
  modificaron.
- Wiring manual en bootstrap y aplicación, más contrato arquitectónico en
  `docs/architecture/research-cache-offline-v1.md` y documentos Foundation/
  Research sincronizados.

### Decisions

- Research cache vive en Foundation workspace cache y no en SQLite para que
  clear/eviction tengan una frontera física separada de la evidencia durable.
- La tabla genérica `research_cache_entries` v23 queda compatibility-only; no
  se modifica una migration publicada ni se la promueve a historical truth.
- Cache keys se hashean antes de usarlas como filenames; queries, URLs y caller
  keys no controlan paths ni aparecen en nombres de archivo.
- Cache stale no se presenta como fresh: sólo offline_allow_stale devuelve hit
  con warning estructurado. Fresh-only devuelve miss explícito.
- Clear elimina únicamente `.json` y staging files de los cinco directorios
  conocidos, no hace recursive deletion de `.kelyro`, workspace, SQLite ni
  caches de otros componentes.
- Un fallo/corrupción de cache es persistence_failure y nunca inventa evidencia;
  status cuenta corrupt files sin decodificarlos como records válidos.
- No se añadió Cost Control, paid-provider budget, Trigger Policy, nuevo network
  provider, Curriculum Compiler ni cambios de Student Core/mastery.

### Verification

- Tests de las cinco policies TTL/size, hit/miss, fresh-only, stale offline,
  warning, clones defensivos, per-layer limit, 512-entry eviction y hash
  corruption.
- Tests filesystem de path Foundation, permisos, atomic records, cinco layers,
  status, malformed JSON, tampered hash y scoped clear.
- Integración offline después de ocho días para discovery/fetch/release/
  normalized data con warnings y cero llamadas a live providers/gate.
- Test con SQLite real que persiste Source → SourceSnapshot → Evidence, ejecuta
  cache clear y vuelve a leer Snapshot/Evidence; cache de otro componente
  también permanece intacta.
- Tests application/CLI de routing, output y argumentos inválidos; smoke real
  `init → research cache status → research cache clear` en workspace temporal.
- `GOCACHE=/tmp/kelyro-i03-step32-full-gocache go test ./...` fuera del sandbox
  para listeners locales de `httptest` y `go vet ./...` con cache aislada.
- `go test -race` dirigido de application, researchcachefs, app y CLI.
- Cross-compile del test binary researchcachefs para Windows amd64 y Darwin
  amd64/arm64 con `CGO_ENABLED=0`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache
  aislada y `go run ./tools/quality all`: tests, E2E, vet, race, build y smokes
  de CLI; SQLite race completó en 357.881 s.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 33 es el siguiente paso pendiente y requiere autorización explícita.
- No implementar Research Cost Control ni pasos posteriores antes de esa
  autorización.

## Step 33 — Research Cost Control

Status: completed
Date: 2026-08-28
Release: unreleased

### Delivered

- Política provider-neutral `research-cost-control-v1` sobre cinco dimensiones:
  search requests, fetch requests, bytes, provider API calls y future model
  calls, sin asociar dinero ni precios de vendors.
- Budget v1 explícito por run y tópico, con límite UTC diario opcional; el
  default limita cada run a 6 búsquedas, 12 fetches, 8 MiB y 18 provider calls,
  y prohíbe model calls no autorizadas.
- `ResearchRun.Cost` con budget, uso acumulado, ahorro por cache válido y stop
  de budget con scope/reason visibles; runs legacy sin metadata siguen siendo
  válidos y legibles.
- Servicio de aplicación que aplica cache-before-network, detiene trabajo si la
  verificación ya está satisfecha o bastan las fuentes primarias, y reserva
  unidades antes de permitir una llamada externa.
- Migration forward-only v39 con controles por run y ledger append-only; los
  triggers SQLite impiden atómicamente exceder límites de run, tópico o día.
- Adapter memory equivalente, stats agregadas all-time/today/cache savings y
  comando humano `kelyro research stats`.
- Contrato, orden de decisión, concurrencia, compatibilidad y límites
  documentados en `docs/architecture/research-cost-control-v1.md`.

### Decisions

- Cost Control autoriza o rechaza trabajo, pero no hace network I/O; discovery,
  fetch, releases y futuros providers siguen siendo adapters externos.
- Las unidades son genéricas para evitar acoplar el dominio a vendors o precios;
  un model call tiene budget cero por defecto hasta que exista autorización e
  integración explícita.
- Un tópico lógico comparte budget por subject/domain/technology aunque use
  ResearchRequest IDs distintos; el budget diario opcional usa fecha UTC.
- El ledger se inserta con triggers en el mismo statement que valida el budget,
  por lo que una reserva rechazada no deja uso parcial ni depende de un check
  application-level susceptible a carreras.
- Una cache válida registra unidades ahorradas; stops por evidencia suficiente
  no se presentan falsamente como ahorro de cache.
- No se añadió Trigger Policy, scheduler, nuevo provider, investigación CLI de
  tópico, Curriculum Compiler ni cambios de Student Core/mastery.

### Verification

- Tests de validación del budget, metadata y stop reason, además de decisiones
  cache-first, verification satisfied, primary sufficient y run budget.
- Test SQLite de dos runs concurrentemente contables contra un topic budget:
  la segunda reserva se rechaza sin sumar unidades y conserva razón visible.
- Tests application/CLI de stats, today usage, cache savings, routing y output.
- `GOCACHE=/tmp/kelyro-i03-step33-test-gocache go test ./...` y `go vet ./...`
  con cache aislada completados antes del quality gate.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache y
  `GOTMPDIR` bajo `/home/mishaaac`: tests, E2E, vet, race, build y smokes CLI;
  SQLite race completó en 455.797 s.
- El primer intento de quality agotó la cuota tmpfs; se limpiaron únicamente
  los caches Go creados para el paso y el gate se repitió completo en disco.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 34 está autorizado a continuación por el usuario.
- Trigger Policy debe producir decisión y queue metadata únicamente; no añadir
  scheduler externo ni iniciar todavía el workflow CLI del Paso 35.

## Step 34 — Research Trigger Policies

Status: completed
Date: 2026-08-28
Release: unreleased

### Delivered

- Policy determinista `research-trigger-v1` con ocho causas cerradas: manual,
  missing evidence, freshness expired, new technology release, deprecation,
  unresolved conflict, future curriculum compile request y security refresh.
- Combinación estable de todas las causas aplicables, sin ocultar señales
  secundarias, con prioridades critical/high/normal explícitas.
- Expiración determinista cuando freshness es stale o `as_of >= next_verify_at`;
  ausencia de evidencia se decide únicamente desde count cero y ninguna
  ausencia de señal se convierte en un hecho inventado.
- `ResearchQueueItem` durable con request completo, triggers, priority, dedupe
  key lógico, timestamps, estado y algorithm version.
- Deduplicación de un único item `queued` por tópico/purpose/target version;
  una evaluación repetida retorna la metadata original y una transición
  terminal permite una nueva cola futura.
- Transiciones explícitas `queued → dispatched|cancelled`, sin mutar identidad,
  request, causas, priority ni algoritmo.
- Migration forward-only v40, adapter SQLite y fake memory con orden critical,
  high, normal, timestamp e ID estable.
- Contrato y frontera no-scheduler documentados en
  `docs/architecture/research-trigger-policy-v1.md`.

### Decisions

- Trigger Policy decide y persiste metadata; no crea goroutines, timers,
  workers, jobs externos ni network calls.
- Manual y security refresh son critical; deprecation, unresolved conflict y
  new release son high; freshness, missing evidence y future compile son normal
  salvo que coexistan con una señal más fuerte.
- El dedupe key excluye IDs y timestamps y conserva subject/domain/technology,
  purpose y target version, para converger requests equivalentes sin impedir
  reinvestigación después de dispatch/cancel.
- Una evaluación duplicada conserva los triggers/prioridad del item ya queued;
  no reescribe silenciosamente por qué se tomó la decisión original.
- No se añadió scheduler, provider, CLI de tópico, Curriculum Compiler ni
  cambios de Student Core/mastery.

### Verification

- Tests de los ocho triggers combinados en orden estable, no-trigger para
  evidencia suficiente/fresh y boundary exacto de expiración.
- Tests de dedupe independiente de IDs/timestamps, prioridad, clones defensivos,
  dispatch, cancel, requeue y rechazo de transiciones inválidas.
- Test SQLite de persistencia, dedupe activo, round-trip terminal y orden de
  cola critical antes de normal.
- `go test ./...`, `go vet ./...` y `git diff --check` con caches reutilizables
  fuera de tmpfs.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache y
  `GOTMPDIR` bajo `/home/mishaaac`: tests, E2E, vet, race, build y smokes CLI;
  SQLite race completó en 466.037 s.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 35 está autorizado a continuación por el usuario.
- El CLI debe reutilizar los servicios/policies existentes, respetar privacy y
  Cost Control, y no implementar todavía las vistas TUI del Paso 36.

## Step 35 — Research and Sources CLI

Status: completed
Date: 2026-08-28
Release: unreleased

### Delivered

- Superficie humana completa para `kelyro sources`, `sources list`, `sources
  show`, `sources conflicts`, `sources trace`, `sources stale` y `sources
  registry list`, conservando también la inspección individual del registry.
- `kelyro research topic <topic>` crea request/run durable en estado planned,
  adjunta el budget default de Cost Control y construye un query plan acotado
  `query-planner-v1` sin convertir candidatos en evidencia.
- La solicitud manual pasa por `research-trigger-v1`, conserva dedupe y queue
  metadata durable, y queda queued porque no se despachó un worker/provider.
- `kelyro research status <run-id>` carga request/run y, cuando existe, resume
  el Source Bundle más reciente con estado, fuentes primary/supporting,
  conflictos y última verificación.
- El workflow CLI `research-cli-workflow-v1` informa el estado de
  `privacy.allow_network`; ante la ausencia actual de un search adapter de
  producción deja discovery pending en vez de inventar resultados o claims.
- Lecturas application/SQLite/memory para conflictos no resueltos con orden
  determinista, además del cableado de Source, Bundle y Conflict services en el
  store workspace-local.
- Contrato, privacidad y límites documentados en
  `docs/architecture/research-cli-v1.md`.

### Decisions

- Un query plan es intención de discovery, no evidencia. Sin provider live no
  se marca el run completed ni se fabrica un bundle vacío; el run queda planned
  e inspeccionable por ID.
- El perfil genérico del CLI solo ordena clases deseadas y umbral de autoridad;
  exige corroboración y no otorga trust por sí mismo.
- `sources` es alias de `sources list`; los bodies externos nunca aparecen en
  los listados o detalles, solo metadata acotada e identidad/hash del snapshot.
- `sources conflicts` muestra exclusivamente conflictos unresolved; decisiones
  históricas resueltas siguen persistidas pero no se presentan como pendientes.
- No se añadió provider, scheduler, fetch automático, TUI, Curriculum Compiler
  ni mutación de Student Core/mastery.

### Verification

- Tests application de planificación/persistencia/status offline y de
  list/detail de sources.
- Tests CLI de routing, aliases, topics multi-palabra, status IDs, output humano,
  conflictos vacíos y errores de uso/IDs.
- Tests de application, CLI, researchdb, memory y SQLite, incluyendo el nuevo
  listado de conflictos no resueltos.
- Smoke real en workspace temporal: init, creación offline de topic, status por
  run ID durable, listado vacío de sources y conflictos sin acceso de red.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache y
  `GOTMPDIR` bajo `/home/mishaaac`: tests, E2E, vet, race, build y smokes CLI;
  SQLite race completó en 469.206 s.
- `gofmt` y `git diff --check` sin errores.

### Notes for next session

- El Paso 36 requiere autorización explícita antes de implementar Source
  Transparency views en TUI.
- Un provider live futuro debe entrar por `SearchProvider`, consultar el network
  gate y Cost Control antes de cada operación, y mantener cache/evidence offline.

## Step 36 — Source Transparency TUI

Status: completed
Date: 2026-08-28
Release: unreleased

### Delivered

- Seis vistas terminales para Research, Sources, Source detail, Claim detail,
  Conflicts y Freshness, accesibles desde Home mediante `R` y navegables sin
  extender el payload publicado de sesión de Student Core.
- Proyección application de transparencia que combina Source, último snapshot,
  decisión de trust/authority persistida y freshness persistida detrás de
  servicios estrechos; ausencia de evaluación se muestra como `unknown` y no se
  infiere autoridad a partir de `official` ni del source kind.
- Source detail muestra kind, authority, freshness, last verified, warning
  histórico/archived/version-bound, version scope y locator compacto sin query
  parameters; el locator HTTP(S) completo solo se usa al abrirlo.
- Claim detail reutiliza el provenance graph durable seleccionado desde un
  conflicto; Conflicts permite elegir claim y Freshness presenta el schedule
  persistido actualmente due sin recalcular algoritmos en TUI.
- Toda lectura se ejecuta como `tea.Cmd` y actualiza el modelo mediante mensajes
  tipados; `View()` no hace I/O, no reabre SQLite y no dispara discovery/fetch
  por render. Las listas largas mantienen visible la selección.
- Adapter `platformos` cross-platform para `platform.Platform`, con apertura
  HTTP(S) sin shell mediante `xdg-open`, `open` o `rundll32.exe`, validación de
  scheme/host/credentials y errores retryable visibles en Source detail.
- Contrato y límites documentados en
  `docs/architecture/research-source-transparency-tui-v1.md` y enlazados desde
  el índice de arquitectura.

### Decisions

- La autoridad visible siempre procede de la última `TrustDecision` persistida;
  la TUI no ejecuta Trust Policy ni concede autoridad implícita a fuentes
  oficiales. Freshness mantiene la misma semántica conservadora.
- El nuevo `TrustDecisionService` es un read boundary propiedad de application;
  la TUI no accede al repository ni a SQLite y `researchdb` conserva el wiring
  explícito.
- Research views son estado de inspección efímero: `Esc` vuelve a su padre y
  `h` a Home, sin migrar ni modificar session state de Foundation/Student Core.
- Las URLs visibles omiten query y fragment para evitar locators enormes o
  tracking noise; la apertura conserva el locator validado completo detrás de
  Platform.
- No se añadió provider, fetch automático, migration, Update Scan, Drift,
  Curriculum Compiler ni mutación de Student Core/mastery.

### Verification

- Tests TUI de navegación y carga asíncrona de las seis vistas, autoridad y
  freshness conocidas/desconocidas, warnings históricos, version scope,
  provenance de claims, schedule due, apertura/fallo del browser, render sin
  I/O, ancho de terminal y selección visible en listas largas.
- Test application de proyección Source + TrustDecision + Freshness persistidos
  y cierre del store; tests de `platformos` para Linux/macOS/Windows, rechazo de
  esquemas inseguros/credentials y fallo de opener.
- `GOCACHE=/home/mishaaac/.cache/kelyro-step36-gocache go test ./...`.
- `GOCACHE=/home/mishaaac/.cache/kelyro-step36-gocache go vet ./...`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache y
  `GOTMPDIR` neutral bajo `/home/mishaaac`: tests, E2E, vet, race, build y
  smokes CLI; SQLite race completó en 488.348 s.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 37 es el siguiente paso pendiente y requiere autorización explícita.
- Update Scan deberá comparar evidencia almacenada con señales actuales detrás
  de adapters y respetar `privacy.allow_network`; no fue iniciado en Step 36.

## Step 37 — Research Update Scan

Status: completed
Date: 2026-08-28
Release: unreleased

### Delivered

- Modelo `update-scan-v1` con inventario de tecnologías/releases/sources y
  freshness due, cinco señales cerradas, origins almacenado/live, orden y
  deduplicación deterministas, y razones explícitas de scan incompleto.
- Servicio application que analiza metadata local: releases current con
  historial previo, cambios entre hashes comparables o 404/410, freshness due,
  última deprecation por subject y conflictos unresolved.
- Puerto opcional `UpdateSignalProvider` que recibe únicamente metadata
  acotada y solo puede ejecutarse después del gate `research.update_scan`.
- `kelyro research update-scan` con inventario, señales, estado
  complete/incomplete y recordatorio explícito de que el reporte no modifica
  curriculum ni estado de estudiante.
- Lecturas globales deterministas de releases/deprecations en memory y SQLite,
  y wiring workspace-local sin añadir un provider live ficticio.
- Contrato, privacidad, semántica offline y límites documentados en
  `docs/architecture/update-scan-v1.md`.

### Decisions

- El scan es un reporte efímero y reproducible sobre el estado observado; no se
  añadió una tabla de scans ni se usa su repetición como prueba de drift.
- Una release local se señala como nueva solo cuando existe historial previo y
  el registro verificado más reciente es current; una única release conocida
  no se interpreta como cambio.
- Snapshots sin hash, incluido un 304, no prueban cambio. Un 404/410 almacenado
  sí señala que el source desapareció, sin convertirlo todavía en drift
  semántico.
- Network disabled, provider ausente y fallo del provider son estados
  incompletos distintos; todos conservan los resultados locales disponibles.
- No se añadió fetch/discovery, scheduler, Drift Detection, Impact Analysis,
  Curriculum Compiler ni mutación de Student Core/mastery.

### Verification

- Tests application de las cinco señales locales, inventario, modo offline,
  autorización previa al adapter, combinación live y fallo parcial explícito.
- Tests app/CLI de coordinación, parsing y salida humana de `update-scan`.
- Tests focalizados y vet de research, SQLite, researchdb, app y CLI.
- `go test ./...` y `go vet ./...` completos con cache/temporales en disco; el
  primer intento en `/tmp` agotó su cuota y el retry autorizado permitió los
  sockets loopback deterministas de `httptest`.
- Quality gate completo con `GOFLAGS=-timeout=20m`, `GOMAXPROCS=2`, cache y
  `GOTMPDIR` bajo el workspace: tests, E2E, vet, race, build y smokes CLI;
  SQLite race completó en 524.553 s.
- Smoke real sobre workspace temporal: `init` seguido de `research
  update-scan`, con estado `incomplete (network_disabled)` y cero señales sin
  inventar evidencia.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 38 está autorizado a continuación por el usuario.
- Drift Detection deberá tratar estas señales solo como candidatos y comparar
  bundles/evidencia estructurada antes de emitir un `DriftReport` v1.

## Step 38 — Evidence Drift Detection v1

Status: completed
Date: 2026-08-28
Release: unreleased

### Delivered

- Policy pura `drift-v1` que compara bundle/Claims anteriores con bundle,
  Claims, snapshots y releases estructuradas actuales mediante stable Claim
  IDs, sin network, SQLite, UI ni dependencias de curriculum.
- Clasificación de los seis drift types requeridos con severity/confidence
  explícitas y deterministas; findings del mismo tipo agrupan claims/evidence
  con orden estable, máxima severidad y confianza conservadora mínima.
- Normalización léxica v1 que ignora exclusivamente cambios de case,
  puntuación y whitespace; cambios restantes quedan visibles en vez de asumir
  equivalencia semántica no demostrada.
- Semántica unresolved que no convierte ausencia de Claim/snapshot actual en
  invalidación; un 404/410 confirmado sí produce `source_changed` con evidencia
  nueva vacía cuando corresponda.
- `DriftService.Detect` puro con IDs reproducibles y persistencia separada vía
  `Record`, evitando writes parciales durante una comparación multi-reporte.
- Migration forward-only v41 con confidence, algorithm version e índice de
  historial por bundle/type; filas anteriores quedan legibles como
  `drift-unversioned-legacy` sin confianza inventada.
- Contrato, tabla de severidad/confianza, limitaciones semánticas, unresolved,
  compatibilidad y límites documentados en `docs/architecture/drift-v1.md`.

### Decisions

- `ClaimID` es la identidad de revisión: wording nuevo con otro ID no se
  empareja heurísticamente y queda unresolved, para no inventar equivalencias.
- Un content hash distinto señala drift de source con confidence 0.50, pero no
  invalida automáticamente la Claim; source gone explícito eleva severity y
  conserva la falta de nueva evidencia.
- Una release solo supersede Claims que el caller mapea explícitamente y cuya
  versión anterior difiere; no se adivina relación por nombres de tecnología.
- La comparación de texto no usa LLM ni declara sinónimos equivalentes. Esta
  limitación conservadora es parte versionada de v1.
- No se implementó Impact Analysis, referencias a lesson/concept, Curriculum
  Compiler, migración automática ni mutación de Student Core/mastery.

### Verification

- Tests policy de wording sin drift semántico, cambio de versión, deprecación,
  recommendation, scope, source gone, release supersession y unresolved.
- Tests application de detección sin write, IDs deterministas, persistencia
  explícita/round-trip y rechazo de nuevos records legacy.
- Test de migration v41 que conserva un DriftReport preexistente con marker
  legacy y confidence desconocida, más round-trip SQLite de reportes v1.
- Tests focalizados y vet de research, SQLite, researchdb, app y CLI.
- `go test ./...` y `go vet ./...` completos; SQLite completó en 110.512 s.
- Quality gate completo: tests (SQLite 105.889 s), E2E (46.979 s), vet,
  race, build y smokes CLI. La repetición explícita de SQLite con `-race`
  completó en 452.796 s después de perderse el código de salida de la sesión
  original por truncación del canal.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 39 es el siguiente paso pendiente y requiere autorización explícita.
- Impact Analysis deberá consumir DriftReports ya persistidos y no ampliar
  Step 38 con referencias curriculares o acciones de migración.

## Step 39 — Research Impact Analysis v1

Status: completed
Date: 2026-08-29
Release: unreleased

### Delivered

- Policy pura `impact-analysis-v1` que traduce un `DriftReport` v1 persistido
  a evidencia, bundles y Claims afectados, severidad conservada y una acción
  recomendada cerrada, sin consultar curriculum ni Student Core.
- Contratos explícitos `ClaimImpactReference` y
  `TechnologyVersionReference` para concept refs, lesson refs y pares exactos
  tecnología/versión futuros; relaciones ajenas al drift y mappings duplicados
  se rechazan en vez de inferirse por nombres o wording.
- Precedencia determinista de acciones: informational → `no_action`, critical
  → `manual_review`, source change → `reverify`, version superseded →
  `recompile_future`, y cambios semánticos no críticos → `review_curriculum`.
- `ImpactService.Assess` carga drift durable y genera IDs reproducibles sin
  escribir; `Record` permanece como append explícito posterior a revisión.
- Migration forward-only v42 con Evidence/future refs y algorithm version;
  reportes anteriores siguen legibles como `impact-unversioned-legacy` sin
  inventar las nuevas relaciones.
- Contrato y límites documentados en
  `docs/architecture/impact-analysis-v1.md` y enlazados desde el índice.

### Decisions

- Impact conserva exactamente la severity del drift y nunca la reduce.
- Old/new Evidence se une y deduplica; old/new bundle se conserva salvo
  identidad repetida; todos los IDs se ordenan de forma estable.
- Las referencias curriculares son opacas y caller-supplied: I-03 no importa
  learning repositories ni valida existencia de concepts/lessons.
- Solo Drift v1 puede producir Impact v1; datos legacy siguen auditables pero
  no se reinterpretan con semántica moderna.
- Una acción recomendada es metadata para un owner futuro, no autorización de
  compilación, migración ni modificación de learner progress.

### Verification

- Tests de policy para deduplicación/proyección, referencias explícitas,
  precedencia de las cinco acciones, drift legacy y relaciones inválidas.
- Tests application para carga obligatoria de drift persistido, evaluación sin
  write, ID determinista y persistencia explícita/round-trip.
- Test de migration v42 que conserva un ImpactReport preexistente con marker
  legacy, además de round-trip SQLite del payload v1 completo.
- Tests focalizados de research/application/impact y SQLite.
- `GOCACHE=/tmp/kelyro-i03-step39-full-gocache go test ./...` y `go vet ./...`
  completos; el primer intento sandboxed solo falló al abrir loopback para
  `httptest` y el retry autorizado pasó.
- Quality gate completo con tests, E2E, vet, race, build y smokes CLI;
  SQLite race completó en 383.805 s.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 40 está autorizado a continuación por el usuario.
- Debe definir el hand-off selectivo y student-safe para I-04 sin implementar
  migración curricular real ni tocar Student Core.

## Step 40 — Selective Migration and Student-safe Update contracts

Status: completed
Date: 2026-08-29
Release: unreleased

### Delivered

- Contrato versionado `research-to-curriculum-update/v1` para el envelope que
  I-03 entregará a I-04 con Source Bundles old/new, DriftReports,
  ImpactReports, ChangeClassification, refs afectadas, rationale y unresolved.
- Identidad versionada de Source Bundle definida como el tuple inmutable
  `(bundle_id, content_hash, algorithm_version, verified_at)`, sin convertir
  topic/run ni un registro mutable en version identity.
- Vocabulario conservador de knowledge change: `no_knowledge_change`,
  `non_breaking`, `breaking` y `unknown`, con evidence gates y prohibición de
  clasificar por search candidate, hash aislado, ausencia o popularidad.
- Siete suggested migration classes, desde `no_migration` y `reverify_only`
  hasta `new_curriculum_version` y `manual_decision_required`, siempre como
  recomendaciones revisables por I-04 y nunca como comandos.
- Contrato `ConceptContinuity` para unchanged/revised/add/deprecate/replace/
  split/merge/remove, con directives futuras `preserve`,
  `preserve_and_reverify`, `do_not_transfer` y `not_applicable`.
- Diez invariantes student-safe que conservan definitions/instances previos,
  hechos educativos append-only y scope por instance, impiden rebind o mastery
  duplication silenciosos, y exigen preview, aceptación, idempotencia y audit.
- Decision gates, audit payload y límites explícitos para que unknown,
  inconsistencias, bundle no listo o mapping ambiguo detengan automatización.
- Contrato documentado en
  `docs/architecture/research-to-curriculum-update-contract.md` y enlazado desde
  el índice, Curriculum consumption y Learner Curriculum Instances.

### Decisions

- Stable concept ID solo expresa continuidad del mismo conocimiento y
  assessment meaning; títulos, orden y similitud textual no prueban identidad.
- Cualquier definition fingerprint distinto exige nueva curriculum version;
  selective recompilation no autoriza reusar `(curriculum_id, version)`.
- Breaking/unknown, replace, split, merge y removal no transfieren mastery
  automáticamente. El historial permanece visible en la instancia anterior.
- `ImpactReport.RecommendedAction` y suggested migration class son inputs para
  I-04; I-03 no decide compile/migration ni emite writes a Student Core.
- Step 40 define shapes y reglas, no añade Go types, persistence, classifier,
  compiler, CLI ni migration executor antes de sus specifications futuras.

### Verification

- Revisión cruzada con `curriculum-consumption/v1`, fingerprints inmutables,
  Curriculum Instance version isolation y sparse instance concept state de
  I-02.
- Auditoría documental de los requisitos del Paso 40: stable concept IDs,
  Source Bundle version, breaking/non-breaking, suggested migration class y
  Student Core untouched por I-03.
- `GOCACHE=/tmp/kelyro-i03-step39-full-gocache go test ./...` y `go vet ./...`
  completos (cache compartida con la verificación inmediatamente anterior).
- `git diff --check` y comprobación explícita de términos/gates obligatorios
  del contrato sin errores.

### Notes for next session

- El Paso 41 es el siguiente paso pendiente y requiere autorización explícita.
- Debe definir el Source-driven Compiler request/response para I-04 sin
  implementar todavía el Curriculum Compiler ni Learning Packs de producción.

## Step 41 — Source-driven Compiler contract for I-04

Status: completed
Date: 2026-08-29
Release: unreleased

### Delivered

- Contrato transport-neutral `source-driven-compiler/v1` para que I-04 consulte
  bundles, Claims, primary sources, conflictos, freshness y requisitos de
  verificación mediante un contexto topic/version/purpose/as-of común.
- View inmutable y consistente que conserva la identidad completa
  `(bundle_id, content_hash, algorithm_version, verified_at)` y prohíbe mezclar
  lecturas `latest` que nunca coexistieron.
- Eligibility cerrada `ready_for_compile`, `ready_with_caveats` y `not_ready`,
  con mapping conservador desde los cuatro estados reales de Source Bundle.
- Reason codes obligatorios para missing primary source, stale, conflicted,
  insufficient corroboration y version unknown, con IDs afectados y
  explicaciones/versiones de algoritmo.
- Gate fail-closed para contenido crítico: un resultado `not_ready` nunca se
  compila silenciosamente y caveats requieren aceptación revisada explícita.
- Semántica de `RequireVerification` definida como trabajo pendiente, no como
  prueba de verificación, permiso de red ni autorización para compilar.
- Contrato documentado en
  `docs/architecture/source-driven-compiler-contract.md` y enlazado desde el
  índice, Source Bundles y Curriculum consumption.

### Decisions

- I-03 decide hechos de evidencia y eligibility; I-04 declara criticality y es
  dueño de policy de compilación, versiones curriculares y provenance final.
- `ready_with_caveats` no equivale a aprobación silenciosa; I-04 conserva y
  reconoce warnings, mientras cualquier estado incomplete/conflicted/legacy o
  scope de versión incompatible queda `not_ready`.
- Missing freshness permanece unknown y no se renombra como stale; version
  unknown, falta de primary y corroboration insuficiente bloquean en vez de
  inferirse desde popularidad, provider rank o semejanza de dominio.
- Las operaciones son API lógica futura. Step 41 no añade Go types,
  repositories, persistence, red, Compiler, Learning Packs ni Student Core.

### Verification

- Revisión cruzada con `source-bundle-v1`, su state/issue precedence, roles
  primary inmutables, freshness aggregate y version identity del Step 40.
- Auditoría documental de las seis operaciones requeridas, tres estados de
  eligibility, cinco reasons mínimos y gate crítico del Paso 41.
- `GOCACHE=/home/mishaaac/.cache/kelyro-step41-gocache GOFLAGS=-timeout=20m
  go test ./...` completo.
- `GOCACHE=/home/mishaaac/.cache/kelyro-step41-gocache GOFLAGS=-timeout=20m
  go vet ./...` completo.
- `git diff --check` sin errores.

### Notes for next session

- El Paso 42 está autorizado a continuación por el usuario.
- Debe persistir metadata reproducible de Research Runs y exponer `research
  show <run-id>` sin prometer reproducción futura exacta de Internet.

## Step 42 — Research Audit Trail and reproducibility metadata

Status: completed
Date: 2026-08-29
Release: unreleased

### Delivered

- Modelo `research-audit-v1` como checkpoints append-only de Research Run, con
  lifecycle, cuatro versiones de policy/algorithm obligatorias, providers
  usados, modo/gate de red, cache hits, source count, bytes fetched y outcome.
- Metadata reproducible acotada para queries ordenadas, target technology/
  version, algoritmos adicionales y tuples exactos
  `(source_id, locator, snapshot_id, snapshot_hash)`.
- Canonicalización determinista, JSON estricto de máximo 256 KiB y content hash
  SHA-256 del checkpoint completo para detectar corrupción o modificación.
- `ResearchService.RecordAudit` y `AuditTrail`, con validación contra lifecycle
  y target durable; memory/SQLite verifican cada locator/hash contra el
  Snapshot real y devuelven copias/orden deterministas.
- Migration forward-only v43 con FK a Research Run, timestamp único por run,
  índice histórico, payload/hash/version acotados y triggers que impiden update
  o delete de checkpoints.
- El workflow manual guarda un checkpoint `planned` con sus queries y policies
  seleccionadas, gate de privacidad y cero actividad live inventada.
- `kelyro research show <run-id>` con historial detallado, algoritmos,
  providers, red, contadores, queries, snapshots/hashes y disclaimer explícito
  sobre la imposibilidad de garantizar el mismo Internet futuro.
- Contrato documentado en `docs/architecture/research-audit-v1.md` y enlazado
  desde el índice y la CLI de Research.

### Decisions

- Cada checkpoint es inmutable y un cambio de lifecycle requiere append; no se
  reescribe el pasado ni se deducen providers/sources al actualizar un run.
- Las versiones first-class indican policies seleccionadas, no prueban por sí
  solas que una etapa produjo output; outcome, refs y resultados durables
  conservan esa distinción.
- Providers usados y actividad quedan vacíos/cero cuando no ocurrieron. Legacy
  runs no reciben audit records sintéticos y se muestran como `not recorded`.
- Solo snapshots con hash canónico pueden entrar al audit v1; candidates no son
  sources observadas y metadata que no coincide con storage se rechaza.
- Audit reproduce inputs y decisiones almacenadas, no ejecuta red, no guarda
  bodies/excerpts y no promete replay byte-identical de servicios externos.
- No se añadió Compiler, curriculum migration ni mutación de Student Core.

### Verification

- Tests de dominio para canonicalización, round-trip JSON, hash estable,
  tampering, bounds y combinaciones lifecycle/red/target inválidas.
- Tests application/memory para checkpoints running/completed, ordering,
  defensive copies, duplicate conflict y lifecycle mismatch.
- Tests de migration v43 sin inventar metadata legacy, round-trip SQLite con
  locator/snapshot/hash durable y enforcement de immutability.
- Tests app/CLI del checkpoint inicial, coordinación de `research show`,
  parsing, render completo y disclaimer de reproducibility.
- `GOCACHE=/home/mishaaac/.cache/kelyro-step42-gocache GOFLAGS=-timeout=20m
  go test ./...` y `go vet ./...` completos.
- Quality gate completo con tests, E2E, vet, race, build y smokes CLI; SQLite
  race completó en 387.079 s. Un intento previo con `GOTMPDIR` forzado bajo
  `~/.cache` hizo que el fixture de backup incluyera su propio temporal; el
  retry con el temporal estándar pasó sin cambios de código.
- Smoke real sobre workspace temporal: `init`, `research topic "Go interfaces"`
  y `research show <run-id>`, con checkpoint planned, privacy gate disabled,
  cero actividad inventada, queries/versiones y disclaimer visibles.
- `gofmt` aplicado y `git diff --check` sin errores.

### Notes for next session

- El Paso 43 es el siguiente paso pendiente y requiere autorización explícita.
- Debe endurecer copyright, licensing y retention sin ampliar este audit con
  bodies externos ni modificar migrations publicadas.
