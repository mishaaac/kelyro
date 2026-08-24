# I-03 Research & Source Intelligence — Progress Log

## Estado general

Current step: 9
Last completed step: 8
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
