# I-03C Live Research Closure — Progress Log

## Estado general

Current step: 25
Last completed step: 24
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

## Step 04 — Search Provider configuration v1

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- Claves `research.search.provider`, `max_results_per_query` y
  `max_queries_per_run` añadidas al schema de configuración con defaults
  `""`, `8` y `4`.
- Configuración tipada `ResearchSearchConfig` con resolución layered, bounds de
  1–100 resultados y 1–8 queries, y provider ID genérico validado.
- Estados `configured`, `missing_credentials`, `disabled` y `unavailable`
  implementados mediante capability booleans sin transportar secretos.
- Parser/encoder/updater TOML estricto extendido para la tabla conocida
  `[research.search]`, preservando rechazo de tablas/keys desconocidas.
- Contrato, defaults, readiness, Secrets y privacy boundaries documentados en
  `research-search-configuration-v1.md` y enlazados desde arquitectura.
- Tests de defaults, overrides, todos los estados, valores inseguros/bounds,
  roundtrip TOML anidado y actualización con preservación de comentarios.

### Decisions

- Provider vacío significa disabled y conserva comportamiento offline seguro.
- El provider ID es un slug genérico; el Paso 4 no contiene allowlist ni nombre
  de vendor.
- Los límites de configuración nunca superan los hard caps existentes de
  discovery y `query-planner-v1`.
- Credenciales no forman parte de `Settings` ni TOML. El nombre
  `research.search.<provider>.api_key` queda solo como contrato para Foundation
  Secrets del Paso 8.
- Configurar provider no habilita red: `privacy.allow_network` sigue siendo un
  gate independiente y obligatorio.
- `SchemaVersion` permanece en 1 porque las claves son aditivas y los archivos
  v1 existentes siguen siendo válidos.

### Verification

- `go test ./internal/config ./internal/infra/configfs -count=1`.
- `go vet ./internal/config ./internal/infra/configfs`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 5 debe seleccionar un único reference production provider mediante
  ADR y documentación primaria vigente.
- No implementar adapter, transport hardening ni Secrets wiring reservados a
  los Pasos 6–8.

## Step 05 — Reference production SearchProvider seleccionado

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- ADR-001 acepta Brave Web Search API con provider ID `brave` y endpoint Web
  Search HTTPS/JSON como único adapter inicial.
- Matriz de documented API, JSON, TLS, authentication, rate limits, cost
  visibility, URL/title/snippet y testability respaldada por documentación
  oficial vigente.
- Mapping provider → `SearchQuery`/`SearchOptions`/`SearchResult` congelado,
  incluyendo paginación bounded, rank posicional y `age` como hint opcional.
- Google Custom Search JSON API y Bing Search APIs rechazados con evidencia
  oficial de cierre a nuevos clientes/discontinuación y retiro, respectivamente.
- Restricciones de persistence rights, query retention, privacy, quota/cost y
  separación de endpoints AI registradas antes de implementación.

### Decisions

- Solo Brave Web Search: no Answers, LLM Context, Summarizer, rich callbacks ni
  otro endpoint AI.
- El provider vive únicamente en infra/config/Doctor; domain y application
  mantienen el contrato vendor-neutral.
- No se añadirá SDK: el REST/JSON será probado con client inyectado y
  `httptest`.
- Cada página HTTP cuenta como `ProviderAPICalls`; una query lógica cuenta una
  vez como `SearchRequests`.
- Live use exige confirmar un plan que permita la persistencia mínima de
  provenance/sources. Sin esa confirmación, readiness es `unavailable`.
- Pricing, quotas, privacy y storage terms son externos y deben revalidarse
  antes del release que habilite el adapter.

### Verification

- Context7 library `/websites/api-dashboard_search_brave_app_web-search_get-started`
  consultada para request/response, pagination y limits.
- Revisión de referencias oficiales Brave para API reference, quickstart,
  authentication, rate limiting, pricing, privacy y storage rights.
- Revisión oficial de lifecycle para Google Custom Search JSON API y Microsoft
  Bing Search APIs.
- ADR contrastada con `SearchProvider`, `ResearchSearchConfig`, privacy y cost
  control existentes.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 6 es el siguiente paso pendiente y requiere autorización explícita.
- Implementar el adapter Brave en infra con fixtures deterministas, sin tocar
  Secrets integration, production wiring ni transport hardening de pasos
  posteriores.

## Step 06 — Production SearchProvider adapter

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- Adapter `researchsearch.Brave` añadido como implementación de
  `application.SearchProvider` sobre el endpoint Web Search seleccionado.
- Mapping bounded de `q`, `count` y `offset` a `SearchResult`, con title,
  locator sin fragment, snippet opcional, provider ID, rank posicional y
  `PublishedHint` solo para timestamps RFC3339 válidos.
- Paginación limitada a 20 resultados por página, offsets 0–9 y hard cap de
  100 candidates; `more_results_available` evita requests innecesarias.
- Errores tipados para authentication, rate limit, invalid request,
  unavailable, malformed response y transport sin incluir query, body, URL ni
  credential en el texto.
- Metadata bounded de los cuatro headers `X-RateLimit-*` disponible en errores
  y observaciones de página no secretas para audit/cost posteriores.
- Fixtures JSON y contract tests deterministas para request/auth, mapping,
  normalización, paginación, límites, status mapping, metadata y cancellation.

### Decisions

- El adapter acepta un `HTTPClient` mínimo inyectado. El cliente de producción
  hardened se incorpora en el Paso 7 sin acoplar domain/application a HTTP.
- La API key entra al constructor en memoria, pero no se resuelve todavía:
  Foundation Secrets permanece reservado al Paso 8.
- `DesiredKind` y `TargetVersion` se ignoran como hints no soportados; no se
  convierten en parámetros vendor ni decisiones de trust/authority.
- La response JSON completa es transient y bounded a 1 MiB; no se persiste.
- Cada observación equivale a una página/API call. El conteo durable sigue
  reservado al wiring de cost/audit, sin duplicar ese servicio en el adapter.
- No se añadió SDK, dependencia externa, privacy wiring, fetch, evidence,
  claims, bundle u orchestration.

### Verification

- `go test ./internal/infra/researchsearch -count=1`.
- `go vet ./internal/infra/researchsearch`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 7 debe proveer y auditar el transporte HTTP de producción usado por
  este adapter, con endpoint/origin fijos y redacción de credenciales.
- No integrar Secrets, privacy gates ni production assembly todavía; están
  reservados a los Pasos 8–10.

## Step 07 — Search HTTP transport hardened

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- `SecureHTTPClient` de producción añadido con endpoint Brave exacto, request
  GET sin body, parámetros/header allowlisted y redirects completamente
  bloqueados.
- `TransportConfig` bounded con defaults para total request, dial, TLS
  handshake, response headers, idle pool y connection counts.
- Transporte directo sin proxy de entorno, TLS mínimo 1.2, verificación de
  certificados del sistema, HTTP/2, compression deshabilitada y User-Agent
  propiedad de Kelyro.
- Responses limitadas a 64 KiB de headers por default y 1 MiB de body JSON
  identity, con chequeo de Content-Type, Content-Length y bytes reales.
- Cancellation preservada; errores de transporte sanitizados y redirects
  impedidos antes de poder reenviar `X-Subscription-Token`.
- Status mapping y rate-limit metadata auditados, sin retry automático que
  esconda coste adicional.
- Documento `research-search-transport-v1.md` creado con la diferencia
  obligatoria entre el search endpoint fijo y los result URLs no confiables que
  solo puede descargar `SourceFetcher` mediante `researchhttp.Client`.
- Tests deterministas y con race detector para TLS/config, endpoint pinning,
  redirects, headers, timeout, cancellation, redacción, media type y body size.

### Decisions

- El transporte de search no se generaliza ni reemplaza `researchhttp.Client`:
  sus amenazas y destinos son distintos y las result URLs nunca entran en él.
- No se siguen redirects, ni siquiera same-origin, para hacer imposible que el
  token salga del path fijado y mantener una API call observable por request.
- No hay retry interno. Un 429 conserva metadata bounded y vuelve como
  `rate_limited`; cualquier retry posterior deberá pasar cost control.
- Errores y observaciones omiten token, query, request URL, response body y
  result URLs. El paquete no incorpora logging.
- Privacy authorization, Secrets resolution, provider selection y production
  assembly permanecen fuera de alcance hasta sus pasos explícitos.

### Verification

- `go test -race ./internal/infra/researchsearch -count=1`.
- `go vet ./internal/infra/researchsearch`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 8 es el siguiente paso pendiente y requiere autorización explícita.
- Resolver `research.search.brave.api_key` solo mediante Foundation Secrets;
  no añadir la key a config, SQLite, logs, doctor output o fixtures.

## Step 08 — Foundation Secrets integration

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- `NewBraveFromSecrets` añadido como factory acotada que solicita únicamente
  `research.search.brave.api_key` mediante el contrato Foundation Secrets.
- Compatibilidad directa con la precedencia existente del secret store:
  `KELYRO_SECRET_RESEARCH_SEARCH_BRAVE_API_KEY` primero y keychain nativo como
  fallback, sin duplicar lógica de credenciales en Research.
- API key validada y transferida solo al adapter en memoria; no existe DTO,
  getter, config field, persistence path ni status que devuelva el valor.
- Estados seguros `available`, `missing`, `unavailable` e `invalid`, con mapping
  al readiness provider-neutral de `ResearchSearchConfig`.
- Resumen permitido congelado como `Provider: configured` y
  `Credential: available`, sin provider key, referencia interna o valor.
- Errores de backend sanitizados y formatting normal/Go del provider redacted
  para impedir que una inspección diagnóstica accidental refleje el token.
- Tests deterministas para referencia exacta, construcción, uso del header,
  readiness, missing/unavailable/invalid, sanitización y ausencia de lectura
  cuando el transporte no está configurado.

### Decisions

- Se reutiliza `storage.SecretStore.Get` mediante un port local de solo lectura;
  el adapter no enumera `Status` ni depende de backends Linux/macOS/Windows.
- Un secret store nativo no disponible no bloquea el fallback por variable de
  entorno, porque esa precedencia sigue perteneciendo al adapter Foundation.
- Errores desconocidos del backend se convierten en
  `ErrCredentialUnavailable` sin copiar su texto potencialmente sensible.
- El estado `invalid` tampoco incluye longitud, prefijo ni ningún fragmento de
  la key.
- No se añadió production assembly, Doctor, privacy gate, búsqueda live, cost
  wiring ni cambios de persistencia; pertenecen a pasos posteriores.

### Verification

- `go test -race ./internal/infra/researchsearch -count=1`.
- `go vet ./internal/infra/researchsearch`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 9 es el siguiente paso pendiente y requiere autorización explícita.
- Antes de cualquier `Search`, el flujo productivo debe comprobar
  `privacy.allow_network`; una denegación debe producir `network_disabled` y
  cero llamadas al provider.

## Step 09 — Privacy/network gate before Search

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- La frontera `DiscoveryService` existente quedó confirmada como el único
  camino autorizado hacia el production `SearchProvider`: valida inputs y
  consulta Foundation `privacy.NetworkGate` antes de invocar el adapter.
- Razón de acceptance `network_disabled` añadida como sentinel detectable con
  `errors.Is`, preservando la clasificación compatible
  `network_research_blocked` y la causa Foundation `privacy.ErrNetworkBlocked`.
- Test de integración directo con el adapter Brave prueba que
  `privacy.allow_network=false` devuelve las tres identidades esperadas y
  produce exactamente cero requests HTTP.
- La documentación de privacidad aclara que el adapter no lee configuración ni
  puede autorizarse a sí mismo.

### Decisions

- Se reutilizó `NetworkResearchAccess`; no se creó un gate, policy o wrapper
  alternativo dentro de infraestructura.
- `network_disabled` se incorpora como razón específica sin renombrar el error
  público I-03 `network_research_blocked`, evitando romper callers existentes.
- El modo offline y el bloqueo por policy comparten la razón de acceptance; la
  causa Foundation permanece disponible solo cuando la policy produjo la
  denegación.
- No se añadió cost control, production wiring, Doctor ni ejecución live; esos
  cambios pertenecen a pasos posteriores.

### Verification

- `go test -race ./internal/research/application ./internal/infra/researchsearch -count=1`.
- `go vet ./internal/research/application ./internal/infra/researchsearch`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 10 debe aplicar el cost control I-03 antes de cada búsqueda real y
  mantener bounded queries, resultados y provider API calls.

## Step 10 — Cost Control on live searches

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- `NewCostControlledDiscoveryService` añadido como frontera live scoped a un
  `ResearchRun`, reutilizando `ResearchCostService` y su ledger durable.
- Cada búsqueda lógica reserva exactamente una unidad `SearchRequests` después
  de privacy y antes de entrar al provider.
- Contrato companion `CostControlledSearchProvider` añadido sin modificar
  `SearchProvider`; exige autorización inmediatamente antes de cada API call.
- Brave implementa el contrato y reserva una unidad `ProviderAPICalls` antes de
  cada page request, incluida paginación; una denegación evita la request y
  detiene trabajo adicional.
- `live-search-cost-policy-v1` aplica `max_results_per_query`; los límites
  durables del run aplican max searches y max provider calls de forma atómica.
- Clasificación estable `budget_exceeded` añadida para que orchestration pueda
  distinguir un stop presupuestario de fallos externos o de privacidad.
- Tests deterministas prueban reserva previa, límite de resultados, privacy
  antes de cost, límite por búsqueda, corte de paginación y conteo exacto de
  páginas Brave sin Internet público.

### Decisions

- Se cuentan API calls reales inmediatamente antes del intento HTTP en vez de
  reservar una estimación máxima de páginas que podría sobrecargar el ledger.
- Una búsqueda que alcanza el provider consume una unidad lógica aunque el
  presupuesto impida su primera página; la API call denegada consume cero.
- La factory cost-controlled exige el companion contract; un provider que no
  puede demostrar autorización por call queda unavailable en producción.
- Brave no ofrece un coste monetario confiable por response. Se conservan
  unidades provider-neutral y no se deriva moneda desde pricing externo
  mutable.
- El constructor legacy `NewDiscoveryService` permanece para adapters offline,
  fixtures y compatibilidad; production wiring debe elegir explícitamente la
  variante cost-controlled en el Paso 11.
- No se añadió composition root, Doctor, orchestrator, worker ni ejecución CLI.

### Verification

- `go test -race ./internal/research/application ./internal/infra/researchsearch -count=1`.
- `go vet ./internal/research/application ./internal/infra/researchsearch`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 11 debe ensamblar config, Secrets, privacy, Brave y la variante
  cost-controlled de discovery; nunca debe seleccionar el provider static de
  fixtures en producción.

## Step 11 — Production provider wiring

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- `researchsearch.Factory` añadido como assembly productivo por run sobre el
  transporte Brave hardened, Foundation Secrets, privacy, cost control y los
  límites de search resueltos.
- Contrato provider-neutral `LiveSearchProviderFactory` incorporado al
  application boundary, con settings sin credenciales y dependencias explícitas
  para run, clock, costs, gate y secret reader.
- Composition root real de `cmd/kelyro/main.go` construye el transporte fijo con
  User-Agent versionado, comparte el SecretStore Foundation e inyecta la factory
  mediante `WithResearchSearch`.
- `researchSearchForRun` resuelve configuración global/project/CLI, crea el gate
  desde la policy efectiva y entrega todos los límites/dependencias a la factory
  sin ejecutar búsqueda.
- `max_queries_per_run` limita el plan persistido y el budget durable
  `SearchRequests`; `max_results_per_query` alimenta el policy del Paso 10.
- Provider vacío y desconocido fallan explícitamente sin leer Secrets ni usar
  `StaticSearchProvider`; Brave es el único adapter production seleccionado.
- Tests prueban wiring application, selección real de Brave, privacy heredada,
  cero HTTP al bloquear red y ausencia de fallback/secret reads para provider
  disabled o unknown.

### Decisions

- La factory se conserva idle en el servicio y ensambla por run porque config,
  privacy y cost ledger son workspace/run-scoped; no se crea un singleton de
  `DiscoveryService` con identidad incorrecta.
- Construir el transporte y el adapter no hace red. Solo una futura llamada a
  `DiscoveryService.Search` puede llegar al endpoint tras privacy y budget.
- Se añadió `ResearchSearchFromResolved` para extraer la sección desde settings
  ya resueltos sin reinterpretar defaults vacíos como overrides explícitos.
- No se consume la queue ni se ejecuta discovery desde CLI; orchestration sigue
  reservada al Paso 13 y pasos posteriores.

### Verification

- `go test -race ./internal/config ./internal/research/application ./internal/infra/researchsearch ./internal/app ./cmd/kelyro -count=1`.
- `go vet ./internal/config ./internal/research/application ./internal/infra/researchsearch ./internal/app ./cmd/kelyro`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 12 debe reportar network policy, provider y credential readiness sin
  construir un run, reservar coste ni ejecutar una búsqueda.

## Step 12 — Doctor readiness

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- Sección opcional `Research Search` añadida a Doctor con checks independientes
  `Network policy enabled`, `Provider configured` y `Credential available`.
- `researchsearch.Factory.Probe` implementa readiness state-only para provider
  disabled/configured/unavailable y credential
  not_applicable/available/missing/unavailable/invalid.
- Application Doctor resuelve config y privacy efectivas del workspace y
  proyecta la sonda productiva al input presentation-neutral de Doctor.
- CLI y TUI reutilizan el rendering genérico por secciones; CLI queda cubierto
  explícitamente para las tres líneas healthy solicitadas.
- Offline/default, provider disabled y credential missing son estados claros no
  fatales; adapter/store unavailable y credential invalid se muestran como
  fallos dentro de una capacidad opcional sin romper Foundation Doctor.
- Tests prueban estados healthy/offline, mapping application, rendering CLI,
  lecturas de Secrets solo para Brave configurado y cero llamadas HTTP durante
  todas las sondas.

### Decisions

- Doctor no llama `Build`: no necesita run ID, cost service ni provider usable
  para reportar readiness y no debe reservar trabajo.
- La sonda puede leer y validar la referencia exacta de Secrets, pero reutiliza
  la sanitización del Paso 8 y solo devuelve enums sin valor ni error backend.
- Research Search permanece optional porque `privacy.allow_network=false` y
  provider vacío son defaults offline válidos; no deben convertir un entorno
  Foundation sano en exit failure.
- No se ejecutó búsqueda de prueba ni health request al vendor; Doctor nunca
  consume quota ni transmite una query.
- No se añadió orchestrator, lifecycle execution ni queue consumption.

### Verification

- `go test -race ./internal/research/application ./internal/infra/researchsearch ./internal/doctor ./internal/app ./internal/cli ./internal/tui -count=1`.
- `go vet ./internal/research/application ./internal/infra/researchsearch ./internal/doctor ./internal/app ./internal/cli ./internal/tui`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 13 es el siguiente paso pendiente y requiere autorización explícita;
  debe coordinar servicios existentes sin implementar HTTP, trust,
  normalization, verification o SQLite directamente.

## Step 13 — Research Orchestrator v1

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- `LiveResearchOrchestrator` añadido como coordinador application-level con
  identidad explícita de queue item, request, run y modo de red.
- Secuencia v1 fija `search → register_candidates → fetch → snapshot →
  normalize → extract → verify → bundle → finalize` sobre servicios de etapa
  inyectados, sin implementar sus políticas ni adapters dentro del
  coordinador.
- Artefactos tipados de hand-off para candidates, sources, fetched content,
  snapshots, normalized content, Evidence, Claims, Verification y bundle.
- Carga durable y reconciliación queue → request → run antes de ejecutar
  trabajo, con rechazo de items cancelados, runs terminales e identidades
  divergentes.
- Resultado parcial observable y corte en el primer error; las etapas
  posteriores no se invocan después de un fallo.
- Tests deterministas de orden total, short-circuit, identidad durable,
  cancelación y dependencias obligatorias.

### Decisions

- El orchestrator fija orden y ownership, pero cada etapa conserva su frontera:
  no contiene HTTP, trust, normalization, verification ni SQLite.
- `LiveResearchStageService` es un seam de composición pequeño para conectar
  los servicios existentes y los adapters acotados de pasos posteriores sin
  ampliar sus interfaces antes de necesitarlas.
- Search results permanecen en `SearchResults`; no se proyectan a Evidence ni
  Claims en el coordinador.
- Queue claiming/ack/retry no forma parte de este paso. El coordinador solo
  carga el item existente; el consumo corresponde al Paso 15.
- Las transiciones del run tampoco se inventan en este paso. El Paso 14 debe
  conectar la máquina de estados existente al inicio, fallo, cancelación y
  success del orchestrator.

### Verification

- `go test ./internal/research/application -run LiveResearchOrchestrator -count=1`.
- `go vet ./internal/research/application`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 14 debe reutilizar `ResearchRunStatus` y persistir transiciones
  válidas alrededor del orchestrator; no debe crear un segundo lifecycle.
- `completed` solo puede persistirse después de que la etapa bundle haya
  devuelto un bundle durable.

## Step 14 — ResearchRun lifecycle completado

Status: completed
Date: 2026-08-30
Release: unreleased

### Delivered

- Política pura `research-run-lifecycle-v1` añadida sobre el `ResearchRun`
  existente, con transiciones forward-only e idempotencia del estado actual.
- `ResearchService.TransitionRun` carga, valida y persiste la única state
  machine; `UpdateRun` también rechaza saltos inválidos y mutaciones de
  identidad/start time.
- El orchestrator transiciona `planned → running` antes del primer stage,
  persiste `failed` al primer error y `cancelled` al observar cancelación o
  deadline del contexto.
- Success exige un bundle con ID válido y `RunID` coincidente después de la
  etapa bundle; `running → completed` ocurre solo tras bundle y finalize.
- El assembler y los repositories memory/SQLite permiten append sobre un run
  `running`, haciendo posible el orden durable `bundle → completed`; el caso
  `completed` previo se conserva por compatibilidad I-03.
- Contrato documentado en `research-run-lifecycle-v1.md` y referencias de
  dominio, application y Source Bundle actualizadas.
- Tests de transición pura/application, terminalidad de success/failure/
  cancellation, bundle obligatorio y append pre-completion memory/SQLite.

### Decisions

- Se conservaron los estados persistidos `planned`, `running`, `completed`,
  `failed` y `cancelled`. Search/register/fetch/snapshot/normalize/extract/
  verify/bundle son etapas dentro de `running`, no una segunda state machine.
- No hizo falta una migración: el schema I-03 ya expresa todos los estados
  durables necesarios y evita una reconstrucción destructiva de tablas.
- Los estados conceptuales detallados del plan se observan mediante la
  secuencia tipada del orchestrator, mientras la terminalidad durable sigue
  siendo provider-neutral y compatible con datos existentes.
- Un retorno de la etapa bundle representa append durable por contrato; el
  orchestrator valida la identidad pero no inspecciona SQLite ni duplica la
  persistencia.
- Para registrar cancelación después de que el contexto se cierre se usa un
  contexto derivado sin cancelación únicamente para la transición terminal;
  no se inicia ningún stage ni trabajo de red nuevo.
- El Paso 14 no cambia estados de queue. Claim/ack/retry sigue reservado al
  Paso 15.

### Verification

- `go test ./internal/research ./internal/research/application ./internal/research/bundle ./internal/storage/sqlite -count=1`.
- `go vet ./internal/research ./internal/research/application ./internal/research/bundle ./internal/storage/sqlite`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 15 debe consumir la queue existente y delegar la ejecución al
  orchestrator; no debe recrear lifecycle, stage ordering ni otra queue.
- Ack/retry debe preservar la identidad lógica y no reabrir runs terminales.

## Step 15 — Existing queue consumer

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `ResearchQueueConsumer` añadido sobre `ResearchTriggerService`,
  `ResearchService` y `LiveResearchOrchestrator`, sin crear otra queue ni
  duplicar el lifecycle del run.
- Semántica durable `claim → execute → ack/retry/fail/cancel` implementada con
  metadata de ejecución versionada en la fila existente de
  `research_trigger_queue`.
- Claim atómico y observable: un solo consumer adquiere el trabajo; consumers
  repetidos reciben `in_flight` o el resultado terminal idempotente sin volver
  a invocar el orchestrator.
- Success persiste execution `completed` y conserva `dispatched` como ack de
  queue; failure permanente hace `failed/dispatched`; cancellation hace
  `cancelled/cancelled`; failure transitorio vuelve a `retry/queued`.
- Retry conserva queue/request y permite que una ejecución posterior use un
  nuevo `ResearchRun` de la misma request, incrementando el attempt count.
- Migración forward-only 44 incorpora únicamente columnas de execution state
  e índice claimable; no modifica migraciones publicadas ni reconstruye la
  tabla.
- Adapters memory y SQLite, tests del consumer y roundtrip SQLite cubren
  success, replay idempotente, claim concurrente, retry, fallo permanente y
  cancelación sin Internet.

### Decisions

- Los estados originales de trigger queue siguen siendo
  `queued/dispatched/cancelled`; la metadata `research-queue-worker-v1`
  distingue claim, retry, ack y failure sin reinterpretar ni reemplazar
  `research-trigger-v1`.
- `dispatched` continúa siendo el estado terminal de la queue existente y
  funciona como ack durable. El execution status diferencia un ack exitoso de
  un fallo permanente sin ampliar el enum histórico.
- Un claim permanece en la misma fila queued, pero queda fuera de
  `ListQueued`; el dedupe key sigue activo durante la ejecución y evita otra
  identidad lógica.
- `unavailable`, `persistence_failure`, `external_failure` y `conflict` son
  retryables en este boundary. El consumer no hace un loop automático: deja el
  item queued y cada reintento posterior vuelve a pasar por orchestration,
  privacy y cost control.
- Las causas persistidas se limitan al `ErrorKind` estable; no se guardan
  mensajes externos, headers, bodies ni secretos.
- El Paso 15 no ensambla stages productivos ni conecta todavía el comando CLI;
  esa ejecución síncrona y acotada corresponde al Paso 16.

### Verification

- `go test -race ./internal/research/application -count=1`.
- `go test -race ./internal/storage/sqlite -run 'ResearchTriggerQueue' -count=1`.
- `go test ./internal/research/application ./internal/storage/sqlite`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 16 debe invocar este consumer desde `research topic` con un deadline
  acotado y devolver el resultado en el mismo proceso, sin daemon ni goroutine
  detached.
- Los stages concretos query-to-bundle siguen reservados a los Pasos 17–31.

## Step 16 — Initial execution path without daemon

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `ResearchTopicExecutor` añadido como composition seam síncrono entre el
  comando `research topic` y el `ResearchQueueConsumer` del Paso 15.
- El request de ejecución entrega el store workspace-scoped, queue ID, run ID,
  modo `auto`, policy observable y una copia bounded del Query Plan cuyas
  queries ya quedaron auditadas.
- `research topic` conserva el orden durable `enqueue/start/audit → execute` y
  aplica un deadline local fijo de dos minutos a la ejecución inicial.
- El resultado del consumer vuelve al `ResearchCLIView`: queue settlement,
  run terminal, artifacts/bundle cuando existan y disposition retry/in-flight
  sin releer Internet ni convertir queries en Evidence.
- `WithResearchTopicExecutor` solo inyecta el executor; no inicia goroutines,
  timers persistentes, scheduler, daemon ni trabajo detached.
- Test de integración application-level construye el consumer real sobre la
  queue memory, ejecuta el command en el mismo call stack, verifica deadline,
  request/plan bounded, ack durable y run completed.

### Decisions

- El timeout de dos minutos es un bound de seguridad del path inicial, no una
  nueva configuración ni una política de retry. Un deadline observado por el
  orchestrator conserva la terminalidad/cancelación del Paso 14 y del worker.
- El executor recibe el store solo durante `Execute`; el command no cierra el
  workspace hasta que la ejecución síncrona termina.
- El seam permanece opcional mientras los stages productivos se incorporan en
  los Pasos 17–31. Si todavía no está ensamblado, el comportamiento offline
  existente se conserva: queue/run quedan pending en vez de fingir success o
  instalar no-op stages.
- No se añadió command alternativo, daemon, polling, background retry ni una
  segunda queue.
- Este paso conecta control flow solamente. No implementa candidate mapping,
  registration, fetch, snapshot, normalization, extraction, verification ni
  bundle assembly reservados a pasos posteriores.

### Verification

- `go test -race ./internal/app ./internal/cli ./internal/research/application -count=1`.
- `go test ./internal/app ./internal/cli ./internal/research/application`.
- `go vet ./internal/app ./internal/cli ./internal/research/application`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 17 debe implementar únicamente Search Result → Source Candidate y
  luego podrá conectarse como primer stage concreto al executor.
- Production composition seguirá sin activar el executor hasta que sus stages
  obligatorios existan; no usar placeholders que completen trabajo falso.

## Step 17 — Search Result to Source Candidate

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- DTO transitorio `SourceCandidate` añadido con locator normalizado y una lista
  bounded de observaciones `SourceCandidateDiscovery`.
- Mapper puro `source-candidate-mapper-v1` convierte cada `SearchResult` uno a
  uno y en orden estable, sin deduplicar ni persistir anticipadamente.
- Cada observación conserva request/query, title, snippet, provider, rank,
  `discovered_at`, published hint opcional y metadata cache defensiva.
- Query, title, snippet y provider se normalizan con la política existente;
  fragments se eliminan del locator mediante el mismo boundary de discovery.
- `LiveResearchArtifacts` transporta candidates defensivamente entre stages y
  clona slices/pointers de provenance.
- Tests cubren mapping completo, orden, duplicados todavía separados,
  normalización, bounds, cancellation y ownership defensivo de timestamps.

### Decisions

- Candidate es application data transitoria: no tiene `SourceID`, `SourceKind`,
  authority tier, trust decision, Evidence ni Claim.
- Title, snippet, published hint y rank siguen siendo observaciones no
  confiables del provider. Search rank nunca se convierte en authority.
- Un candidate contiene una lista de discoveries desde su creación para que el
  Paso 18 pueda fusionar URLs multi-query sin perder provenance.
- El mapper no consulta repositories, no registra sources, no hace red y no
  deduplica. Esas responsabilidades permanecen en los Pasos 18–20.
- Los cache flags se preservan porque describen el origen de discovery, pero no
  cambian el estatus candidate-only del resultado.

### Verification

- `go test -race ./internal/research/application -count=1`.
- `go test ./internal/research/application`.
- `go vet ./internal/research/application`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 18 debe normalizar/deduplicar candidates entre queries y contra
  Sources existentes, preservando todas las observaciones de discovery.
- El Paso 18 no debe registrar nuevas Sources ni asignar trust.

## Step 18 — Candidate deduplication

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- Servicio `source-candidate-deduplication-v1` añadido sobre el
  `SourceRepository` existente, sin nueva persistencia ni segunda registry.
- Canonicalización conservadora fusiona scheme/host equivalentes, elimina
  fragments, quita puertos HTTP(S) default y representa el root con `/`.
- Candidates multi-query se agrupan en orden de primera aparición y conservan
  cada observación distinta de query/provider/rank/time/title/snippet/cache.
- Observaciones exactamente repetidas se eliminan sin perder discoveries de
  otra query o provider.
- Sources ya persistidas se indexan por el mismo locator canónico; el resultado
  enlaza su `SourceID` sin modificarla ni crear otra Source.
- Resultado bounded incluye input/duplicate/existing/discovery counts y se
  transporta defensivamente en `LiveResearchArtifacts`.
- Tests cubren canonical URL, default port/root path, fragments, multi-query
  provenance, exact duplicates, existing Source, query-string distinction,
  bounds, cancellation, persistence failure y ownership defensivo.

### Decisions

- Path, trailing slash no vacía y query string permanecen significativos. No
  se eliminan tracker params, no se reordenan queries y no se adivinan URLs
  equivalentes más allá de reglas sintácticas seguras.
- La referencia a “existing registry” del plan se resuelve contra las Sources
  estables del `SourceRepository`, cuyo locator es único. El catálogo
  `SourceRegistryEntry` describe familias/organizaciones y no contiene URLs de
  recursos individuales.
- Una colisión canónica ambigua entre dos Sources existentes es invalid state;
  nunca se elige una identidad durable arbitrariamente.
- Existing Source match no implica trust, freshness, authority ni Evidence;
  solo evita crear una identidad duplicada en el Paso 19.
- El servicio es read-only, no hace red y no registra candidates. Ingestion
  permanece reservada al Paso 19.
- El total de inputs y discoveries se limita a 500 por run; empty input evita
  una lectura innecesaria del repository.

### Verification

- `go test -race ./internal/research/application -run 'SourceCandidate' -count=1`.
- `go test ./internal/research/application -run 'SourceCandidate' -count=1`.
- `go test ./internal/research/application -count=1`.
- `go vet ./internal/research/application`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 19 debe consumir `DeduplicatedSourceCandidate`, reutilizar
  `ExistingSourceID` cuando exista y registrar únicamente nuevas Sources.
- Registration debe persistir discovery metadata sin derivar trust automático.

## Step 19 — Source Registry ingestion

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `source-candidate-registration-v1` reutiliza el `SourceRepository` estable,
  conserva Sources existentes y registra únicamente locators canónicos nuevos.
- Nuevas Sources live se crean de forma conservadora como `other/current`, con
  título observado y sin convertir rank, snippet ni published hint en
  clasificación, authority o trust.
- `DiscoveredSource` completado con vínculo a Source y metadata bounded de
  request/query/title/snippet/provider/rank/time/publication/cache.
- Repositorio append-only de discoveries añadido a memory y SQLite, con
  migración forward-only 45, foreign keys, locator guard e inmutabilidad.
- IDs estables hacen idempotentes los retries; una carrera de inserción por
  locator se resuelve reutilizando la Source durable, no creando duplicados.
- Los artifacts del orchestrator transportan las discoveries durables para la
  provenance posterior sin convertirlas en Evidence.
- Tests cubren registro nuevo/existente, replay, metadata durable, roundtrip
  SQLite, bounds y ausencia explícita de trust automático.

### Decisions

- El Source Registry del paso es el repositorio de identidades `Source`; el
  catálogo `SourceRegistryEntry` conserva su función separada de metadata
  revisada por organización y no se altera con resultados web.
- `SourceOther` es la única clasificación segura antes de fetch/normalize y de
  políticas posteriores; el provider no decide `SourceKind`.
- Published hints permanecen únicamente en la observación de discovery. No se
  escriben en `Source.Metadata.PublishedAt` hasta que contenido posterior las
  sustente.
- Registration no depende de `TrustRegistryRepository` y no puede escribir una
  `TrustDecision`; authority/trust siguen reservados a pasos posteriores.
- Un fallo después de crear Source es recuperable: el siguiente retry reconoce
  la Source por locator e inserta las observations faltantes con IDs estables.

### Verification

- `go test -race ./internal/research/application -count=1`.
- `go test -race ./internal/storage/sqlite -run 'Test(SourceDiscoveryRoundTripSQLite|OpenCreatesAndMigratesNewDatabase)$' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 20 debe consumir `Artifacts.Sources` y usar exclusivamente el
  `FetchService`/adapter HTTP ya hardened.
- Los fallos por Source deben conservarse de forma bounded y permitir success
  parcial; snapshot/cache permanecen reservados al Paso 21.

## Step 20 — Existing HTTP Fetcher wiring

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `live-source-fetch-v1` añadido como stage entre `Artifacts.Sources` y el
  `FetchService` existente, sin duplicar transport ni abrir red directamente.
- Policy bounded reutiliza `research-processing-limits-v1`: máximo 200 Sources,
  8 fetches concurrentes por defecto (hard cap 32), 64 MiB por run y allocation
  de hasta 4 MiB por Source repartida dentro del presupuesto total.
- Success parcial conserva bodies exitosos y fallos individuales bounded por
  Source ID/locator/error kind; si todos fallan, el error clasificado de menor
  índice termina el stage.
- Cancellation sigue siendo terminal aunque otra Source ya haya respondido;
  outputs/failures mantienen orden estable y ownership defensivo.
- `FetchService` endurecido para rechazar bodies que excedan el
  `FetchRequest.MaximumBytes`, aun si un adapter viola su contrato.
- El orchestrator conserva artifacts parciales devueltos por un stage fallido,
  permitiendo que failure/audit posterior observe los fetch failures.
- Composición del binario conecta `researchhttp.Client` →
  `researchfetch.Fetcher` → `FetchService` → live fetch stage, resolviendo la
  privacy gate por workspace/comando antes de cualquier llamada.
- Tests cubren partial success, all-failed, privacy denied con cero adapter
  calls, allocation/bounds, body oversize y composición productiva.

### Decisions

- El stage depende del port `FetchService`; no conoce HTTP. Por ello SSRF,
  redirects, content types, retries y timeouts permanecen exclusivamente en el
  adapter hardened ya probado.
- La composición inicial no inyecta `SourceFetchCache`: Snapshot + Cache wiring
  está reservado al Paso 21 y no se simula con un fallback vacío.
- Failure artifacts no retienen mensajes externos, headers, URLs redirigidas ni
  bodies; la taxonomía estable es suficiente para `fetch_failed_partial`.
- El request-specific limit puede reducir pero nunca elevar el límite global
  del HTTP client. La división del presupuesto evita pedir un aggregate mayor
  a 64 MiB incluso con el máximo de Sources.
- No se crean snapshots, Evidence, Claims ni trust durante fetch.

### Verification

- `go test -race ./internal/research/application -run 'Test(LiveSourceFetch|FetchService)' -count=1`.
- `go test -race ./internal/app ./internal/research/application ./internal/infra/researchfetch ./internal/infra/researchhttp ./cmd/kelyro -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 21 debe consumir `Artifacts.FetchedSources`, reutilizar snapshot y
  `researchcachefs`, y mantener bodies fuera de SQLite.
- Partial failures deben acompañar los successes sin fabricar snapshot para la
  Source fallida.

## Step 21 — Snapshot + Cache wiring

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `live-source-snapshot-v1` consume los fetches exitosos del stage anterior y
  reutiliza `SnapshotCaptureService` sin repetir requests de red.
- `CaptureFetched` separa observaciones live de datos cacheados: live agrega
  historial inmutable; cache solo puede reutilizar el latest snapshot durable
  con locator final y hash canónico coincidentes.
- Bodies live bounded se escriben en el `researchcachefs` existente solo
  después del snapshot durable; `no-store` suprime la escritura y los fallos de
  cache quedan como failure data parcial.
- Revalidación `304` agrega exactamente un nuevo snapshot y recupera el body
  previo desde cache únicamente como input transitorio de normalización.
- La clave de cache usa el locator registrado de Source aunque el adapter HTTP
  termine en otro locator seguro tras redirects.
- La composición por workspace abre el mismo adapter para fallback de fetch y
  escritura post-snapshot, manteniendo cuerpos fuera de SQLite.
- Artifacts transportan snapshots, inputs defensivos de normalización y fallos
  bounded de snapshot/cache para conservar provenance y partial success.

### Decisions

- Un cache hit sin historia durable coincidente es `invalid_state`; cache es
  aceleración descartable y nunca inventa una observación `fetched_at`.
- El body no forma parte de `SourceSnapshot`. Solo metadata/hash quedan en
  SQLite; el normalizador posterior recibe una copia transitoria.
- Snapshot failure por Source permite continuar si otra Source produjo
  snapshot; si todas fallan, el stage termina con persistence failure.
- Cache failure no invalida un snapshot ya durable. El resultado lo conserva
  para audit posterior y puede continuar a normalización con el body live.
- El adapter verifica el tamaño JSON/base64 antes de escribir metadata para
  evitar una media entrada predecible cuando el encoding excede la policy.
- No se crean Evidence, Claims, trust ni comportamiento de I-04.

### Verification

- `go test -race ./internal/research/application ./internal/infra/researchcachefs ./internal/infra/researchdb ./internal/app -run 'Test(LiveSourceSnapshot|SnapshotCapture|OfflineAdapter|ServiceAssemblesSnapshot|ServiceAssemblesProductionResearchFetch|ServiceResearchFetchPrivacy)' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 22 debe consumir exclusivamente `Artifacts.NormalizationInputs`, que
  ya resuelve body live, cache hit y revalidación `304` contra historia durable.
- Debe reutilizar los normalizadores HTML/Markdown/JSON/text existentes sin
  persistir raw content ni comenzar Evidence extraction.

## Step 22 — Normalizer wiring

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `live-source-normalization-v1` conecta los inputs transitorios del Paso 21
  con el port `SourceNormalizer` existente.
- Cada input se verifica contra su SourceSnapshot durable por Source ID,
  locator final y hash canónico antes de cruzar el boundary del adapter.
- El binario inyecta `researchnormalize.New()` y la composición application
  construye el stage sin conocer parsers concretos.
- HTML/XHTML, Markdown, JSON y plain text reutilizan exactamente
  `source-normalization-v1`; no se añadió parser ni dependencia externa.
- Resultados normalizados y todas sus colecciones/pointers se copian
  defensivamente dentro de `LiveResearchArtifacts`.
- Fallos por documento quedan bounded por Source/locator/kind y permiten
  partial success; cancellation y el caso sin ninguna fuente normalizada son
  terminales.
- Tests de wiring ejecutan los cuatro formatos existentes, partial/all failure,
  rechazo de input sin snapshot y composición productiva.

### Decisions

- El stage consume `NormalizationInputs`, no `FetchedSources`: así una
  revalidación `304` normaliza el body durable recuperado de cache y nunca un
  response body inexistente.
- NormalizedSource es dato derivado transitorio; snapshot/hash continúan siendo
  la verdad histórica y raw bodies no se escriben en SQLite.
- Un adapter no puede cambiar Source ID ni locator. Una salida con identidad
  diferente se registra como failure, no se acepta en provenance.
- Unsupported media, documento inválido y output limit permanecen decisiones
  del normalizador existente; el stage no agrega heurísticas ni fallback parser.
- Este paso no crea Evidence/Claims, no asigna trust y no comienza I-04.

### Verification

- `go test -race ./internal/research/application ./internal/infra/researchnormalize ./internal/app ./cmd/kelyro -run 'Test(LiveSourceNormalization|ServiceAssemblesExistingResearchNormalizer|Normalizer)' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 23 debe diseñar el extractor determinista sobre
  `Artifacts.NormalizedSources`; no debe reabrir fetch/snapshot/normalization.
- Search snippets siguen siendo candidates y no pueden sustituir Evidence.

## Step 23 — Deterministic Evidence Extractor v1 design

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- Contrato transitorio `EvidenceCandidate` ligado a Source + snapshot, con
  kind cerrado, locator determinista, excerpt/hash, contexto bounded, score,
  señales cerradas y versión inmutable `evidence-extractor-v1`.
- Request del extractor exige topic/purpose, target version opcional y match
  exacto de Source ID + locator entre `NormalizedSource` y snapshot durable.
- Tres representaciones iniciales: heading, passage y structured metadata;
  links/code quedan fuera del extractor genérico por no probar hechos ni
  satisfacer el locator especializado de source-code Evidence.
- Política de score entero documentada con anchor temático obligatorio,
  threshold 50, orden estable, dedupe y límites de 24 candidates por Source y
  1.000 por run.
- Locators snapshot-local explícitos `heading[n]`, `text[n]` y
  `metadata/...`, sin inventar una relación heading→párrafo que el normalizador
  actual no conserva.
- Bounds conservadores de 2 KiB por excerpt y 512 bytes por contexto, por
  debajo de los ceilings del modelo Evidence ya publicado.

### Decisions

- Search snippet/rank no cruzan este boundary. Solo contenido ya fetched,
  snapshotted y normalized puede producir candidates.
- Relevancia y marcadores release/version/deprecation son señales de selección,
  no Claims ni trust. La authority de la Source permanece reservada al Paso 27.
- Un marcador aislado en contenido no relacionado no basta: cada candidate
  necesita anchor en subject/technology/domain/target version o en el título/
  headings del documento.
- El score vive únicamente en el candidate transitorio. `Evidence` persiste el
  excerpt literal y su provenance, no una falsa medida de verdad.
- Este paso define contratos y política; no implementa selector, persistence,
  Claims, verificación, bundle ni I-04.

### Verification

- `go test ./internal/research/application -run 'EvidenceCandidate|EvidenceExtractionRequest' -count=1`.
- `go vet ./internal/research/application`.
- `git diff --check`.

### Notes for next session

- El Paso 24 debe implementar exactamente esta matriz sobre
  `Artifacts.NormalizedSources`, persistir Evidence idempotente y fallar sin
  fallback cuando no exista ningún candidate relevante.
- El Paso 24 no debe derivar Claims ni aplicar trust/freshness/verification.

## Step 24 — Deterministic Evidence Extractor v1 implementation

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- Selector puro `evidence-extractor-v1` implementado sobre title, headings,
  text segments y version metadata ya normalizados, sin reabrir fetch ni body.
- Scoring entero estable del Paso 23 con anchors temáticos, señales exactas,
  lexicon explícito release/version/deprecation y soporte de target versions
  opacas sin imponer SemVer.
- Ventanas UTF-8 bounded de 2 KiB alrededor de la señal admitida y contextos
  adyacentes de hasta 512 bytes, conservando excerpt/hash literal.
- Orden total score/kind/location/hash, dedupe de excerpts y límites efectivos
  de 24 candidates por Source y 1.000 por run.
- `LiveEvidenceExtractionService` valida la cadena Source → snapshot →
  NormalizedSource, persiste Evidence antes del siguiente stage y rechaza el
  caso sin evidencia relevante.
- IDs estables e idempotencia de replay/concurrencia: Evidence byte-idéntica se
  reutiliza y una colisión semántica nunca se sobrescribe.
- `EvidenceCandidates` y Evidence persistida añadidos al hand-off defensivo del
  orchestrator; workspace composition reutiliza el EvidenceRepository SQLite
  existente sin migration ni dependencia nueva.
- Tests cubren docs oficiales, release notes, specification, community,
  irrelevant content, bounds/focus, determinismo, cancellation, persistence,
  replay idempotente y composition.

### Decisions

- Los facts siguen siendo excerpt literal, no Claim. Señales y score solo
  seleccionan candidatos y no se persisten como medida de verdad.
- Un marker release/deprecation necesita anchor temático local, por target o
  por title/heading del documento; no se admite keyword-only evidence.
- Metadata de versión explícita puede ser opaca (`2026`, edición o revisión);
  v1 no fuerza SemVer ni inventa release status.
- Sources ya clasificadas como `source_code` se omiten en el extractor
  genérico porque Evidence de código exige commit/path/lines/permalink
  revisados. No se fabrica ese locator desde prose normalizada.
- No se crean Claims, TrustDecision, Freshness, Verification, Citation ni
  SourceBundle; permanecen en pasos posteriores.

### Verification

- `go test -race ./internal/research/application ./internal/app ./internal/infra/researchdb -run 'EvidenceExtractor|LiveEvidenceExtraction|AssemblesDeterministicEvidence' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 25 debe diseñar únicamente `claim-extractor-v1` sobre Evidence ya
  persistida; no debe implementarlo ni inferir claims ambiguas.
- El Paso 26 será responsable de la implementación determinista y bounded de
  Claims; authority/trust continúa reservada al Paso 27.
