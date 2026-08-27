# I-03 Research & Source Intelligence — Progress Log

## Estado general

Current step: 24
Last completed step: 23
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
