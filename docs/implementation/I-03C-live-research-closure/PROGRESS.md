# I-03C Live Research Closure — Progress Log

## Estado general

Current step: 14
Last completed step: 13
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
