# I-03C Live Research Closure — Progress Log

## Estado general

Current step: 43
Last completed step: 42
Baseline commit: acbfc63
I-03 status before correction: PARTIAL

## Gaps

- live query-to-bundle smoke and closure reviews pending

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

## Step 25 — Conservative Claim Extractor v1 design

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- Contrato `ClaimCandidate` ligado exactamente a Source + snapshot + Evidence
  persistida, con statement/hash literal, scope, qualifiers, confidence,
  marker, sentence index y versión `claim-extractor-v1`.
- Seis familias cerradas mapeadas a los ClaimTypes publicados: definition,
  version/release, deprecation, availability/support, requirement y
  recommendation.
- Statements limitados a 2 KiB, markers a 256 bytes, máximo ocho candidates por
  Evidence y 1.000 por run.
- Ambiguity gate documentado: zero/multiple families, hedge, qualifiers
  incompatibles, sentence truncada o dependencia de otra Evidence producen
  cero candidate.
- Version scope conserva únicamente un valor opaco explícito; status scope
  requiere un único qualifier closed y nunca se infiere por source kind.
- Confidence fija por family expresa directness del extractor, no truth/trust.
- Dedupe semántico exacto y agregación futura multi-source definidos sin
  declarar equivalentes statements con wording diferente.

### Decisions

- El statement de Claim será una oración byte-identical dentro de
  `Evidence.Excerpt`; context no puede completar una afirmación ausente.
- Cada candidate nace de una Evidence. El Paso 26 podrá agrupar solo statements
  idénticos y conservar todos los SourceIDs/EvidenceIDs.
- Una oración que encaja en más de una family se descarta; no se aplica
  precedencia heurística ni paráfrasis.
- Scope, version/status qualifiers y confidence quedan explícitos antes de
  persistir, pero authority/corroboration no alteran la extracción.
- Este paso no implementa sentence splitting, lexicon matching, persistence,
  Claims durables, verification, bundle ni I-04.

### Verification

- `go test ./internal/research/application -run 'ClaimCandidate|ClaimFamily' -count=1`.
- `go vet ./internal/research/application`.
- `git diff --check`.

### Notes for next session

- El Paso 26 debe implementar fielmente este contrato y rechazar ambigüedad;
  no debe ampliar families ni introducir equivalencia semántica/LLM.
- Trust, freshness y multi-source verification permanecen en Pasos 27–29.

## Step 26 — Deterministic Claim Extractor v1 implementation

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- Selector puro `claim-extractor-v1` implementado con sentence splitting
  conservador y lexicon cerrado inglés/español para las seis families del
  Paso 25.
- Ambiguity gate efectivo para múltiples families/statuses, hedges, fragments
  sin puntuación terminal, URLs/código/quotes y statements sin anchor local.
- Version y status scope se copian únicamente desde qualifiers literales; las
  confidence constants permanecen explícitas por family y no expresan truth.
- `LiveClaimExtractionService` valida cada Evidence contra su registro durable
  y contra la cadena exacta Source → snapshot antes de derivar una Claim.
- Statements byte-idénticos se agregan con todos sus SourceIDs/EvidenceIDs;
  wording distinto nunca se declara equivalente.
- Claims usan IDs semánticos estables y persistence idempotente ante replay o
  carrera, sin sobrescribir una colisión con contenido diferente.
- Cada Evidence que respalda una Claim recibe una citation durable generada por
  `citation-v1`; una Claim sin citation correspondiente no puede salir del
  stage.
- El stage `extract` productivo compone Evidence y Claims en orden, reutiliza
  los repositorios SQLite existentes y transporta candidates, Claims y
  citations con ownership defensivo.

### Decisions

- V1 requiere oración completa con terminador explícito; un trailing fragment
  se omite en vez de adivinar su cierre gramatical.
- Release/version necesita `version`/`versión` más un valor opaco literal; no
  se interpreta SemVer ni se extraen números incidentales.
- Citation es una relación Source/snapshot/Evidence, por lo que se deduplica
  por Evidence incluso cuando una Evidence produce más de una Claim.
- `LastVerified` de la citation registra el momento en que el stage revalidó la
  cadena durable para derivar la Claim; freshness sigue siendo independiente y
  queda reservado al Paso 28.
- No se aplican authority, trust, freshness, corroboration, verification,
  conflictos ni bundle durante extracción.

### Verification

- `go test -race ./internal/research/application ./internal/app ./internal/infra/researchdb -run 'ClaimExtractor|LiveClaimExtraction|AssemblesDeterministicEvidence' -count=1`.
- Tests de las seis families, ambiguity/hedge gates, bounds, determinismo,
  cancellation, agregación multi-source, citations y replay idempotente.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 27 debe evaluar únicamente las Sources que realmente respaldan las
  Claims y persistir `TrustDecision` mediante `trust-policy-v1`.
- Search rank/provider metadata no puede cruzar como authority ni modificar
  una tier; freshness se integrará por separado en el Paso 28.

## Step 27 — Authority / Trust integration

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `live-trust-evaluation-v1` conecta las Sources que respaldan Claims reales
  con `trust-policy-v1` y persiste sus `TrustDecision` en el repository I-03.
- Authority se calcula exclusivamente por la policy existente desde Source
  kind + use case; provider, rank, snippet y orden de discovery no forman parte
  del request de evaluación.
- Trusted Source Registry existente se carga como catálogo y solo se entrega a
  la policy cuando locator, kind y contexto topic/domain aplican exactamente.
- Use cases de security, package API, historical behavior y language
  specification se seleccionan mediante purpose/source metadata durable.
- Relevance se deriva del statement literal ya evidence-backed; directness es
  primary porque la oración está contenida byte-identical en Evidence.
- Stability conserva el status scope explícito de la Claim y usa `unknown`
  cuando el extractor no observó un qualifier.
- Multi-source no se promueve a independent: permanece corroboration unknown
  hasta la diversity/verification del Paso 29.
- Trust decisions quedan en los artifacts defensivos del stage `extract` para
  consumo posterior, además de persistirse con razones/version completas.

### Decisions

- Freshness se entrega como `unknown` en este paso y obliga a verificación; el
  Paso 28 calculará el estado temporal antes de una reevaluación definitiva.
- Una nueva Source registrada como `other` conserva tier E/rejected. No se
  cambia su clasificación por hostname, título, provider ni search rank.
- Una Source ya clasificada usa la authority contextual normal de
  `trust-policy-v1`; registry puede volverla más conservadora o bloquearla,
  pero nunca elevar su baseline tier.
- Cuando una Source participa en varias Claims, relevance/stability se agregan
  por la observación más conservadora; TrustDecision continúa siendo por
  Source y contexto de research run.
- No se implementa corroboration/diversity, verification, conflicts, bundle,
  classification heurística ni I-04.

### Verification

- `go test -race ./internal/research/application ./internal/app ./internal/infra/researchdb -run 'LiveTrust|AssemblesDeterministicEvidence' -count=1`.
- Tests de tier B/unknown freshness, tier E rejected, registry blocked,
  multi-source sin independencia y rank 1 vs 99 con decisión idéntica.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 28 debe usar snapshot/Evidence verification time y metadata temporal
  durable sin confundir fetched, published, updated y last verified.
- Freshness conocida debe persistirse por Claim y alimentar una reevaluación
  de trust; release/deprecation signals deben seguir siendo explícitos.

## Step 28 — Freshness / Temporal Scope integration

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `live-temporal-evaluation-v1` conecta Claims/citations reales con
  `freshness-v1` y `source-temporal-policy-v1` sin modificar ambas policies.
- Metadata `published_at`, `updated_at` y version hints se captura de la salida
  normalizada bounded, con fallback a Source metadata durable y provenance por
  Source; nunca se infiere desde search snippets.
- Cada Source recibe una assessment temporal explícita current,
  version-authority, historical-context o not-applicable con warning/version.
- Freshness se calcula por par Claim/Source usando `citation.last_verified`;
  `snapshot.fetched_at` permanece separado y no se usa como sustituto.
- Assessments multi-source se agregan por peor state/score y verificación más
  antigua en un `FreshnessRecord` durable por Claim, compatible con el Source
  Bundle I-03 existente.
- Authority Profiles actuales aportan TTL hints cuando hay match exacto; sin
  profile se conservan los defaults `freshness-v1`.
- Release intelligence solo activa `known_new_release` cuando un release
  durable `current`, ligado a la Source, difiere de la versión baseline
  explícita.
- Deprecation intelligence puede avanzar `last_verified_at` únicamente cuando
  el record durable comparte la misma Source y Evidence de una Claim de
  deprecation.
- Trust se reevalúa después de freshness y persiste una decisión final con
  `fresh`, `aging` o `stale` real en sus razones.
- Artifacts transportan observaciones temporales, assessments por Claim/Source,
  records agregados y TrustDecisions con copias defensivas.

### Decisions

- Publication, source update, snapshot fetch, Evidence extraction, citation
  verification y freshness evaluation siguen siendo timestamps distintos.
- Version hints se preservan como metadata observada; no cambian
  `Source.Version` ni `TemporalScope` porque eso exigiría clasificación
  revisada, no una heurística live.
- Un release `current` diferente solo es trigger cuando existe baseline
  explícita y comparte Source; no se ordenan versiones opacas ni se asume
  SemVer.
- Historical/version-bound behavior depende exclusivamente del scope/version
  durable y de `source-temporal-policy-v1`; una fecha antigua no reclasifica la
  Source automáticamente.
- El inventario release/deprecation consumido por run tiene hard cap de 5.000
  records; Sources, Claims y citations conservan sus límites I-03C.
- Este paso no hace discovery de releases, no crea deprecation conclusions, no
  verifica diversidad y no implementa I-04.

### Verification

- `go test -race ./internal/research/application ./internal/app ./internal/infra/researchdb -run 'LiveTemporal|LiveTrust|AssemblesDeterministicEvidence' -count=1`.
- Tests de metadata temporal normalizada, version authority, freshness
  persistida, update trigger, current-release trigger, deprecation exacta y
  reevaluación trust con stale.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 29 debe consumir Claims, Sources, TrustDecisions y temporal scope ya
  persistidos para ejecutar verification/diversity existente.
- Multi-source wording idéntico aún conserva corroboration unknown; solo la
  policy de verification puede confirmar independencia y suficiencia.

## Step 29 — Multi-source Verification integration

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `live-multi-source-verification-v1` ejecuta el `VerificationService` I-03
  existente para cada Claim real del run y conserva orden determinista por ID.
- Cada resultado se persiste por el repository append-only existente y se
  devuelve en los artifacts con status, requirement, métricas, reasons,
  confidence y `multi-source-verification-v1` intactos.
- La misma etapa reutiliza `source-diversity-v1` por Claim. Organization se
  obtiene únicamente del Trusted Source Registry; perspective, technical role
  y dependency group permanecen explícitamente unknown/vacíos porque el flujo
  live no dispone de clasificación humana revisada.
- Los assessments de diversity quedan asociados a su Claim en artifacts con
  warnings y dimensiones diferidas defensivamente copiadas.
- El workspace SQLite ensambla los servicios canónicos de verification y
  diversity sobre Claims, Sources, TrustDecisions, Registry y conflicts ya
  persistidos; el composition boundary de app solo los conecta al stage.

### Decisions

- Los resultados solicitados se expresan con el vocabulario I-03 existente:
  `verified`, soporte con caveat como `verified_with_caveat`, `conflicted` e
  `insufficient_evidence`; no se creó un segundo status `supported`.
- La verificación no usa provider, rank, snippet ni cantidad bruta como
  autoridad. Sólo TrustDecision, Registry ownership/status, temporal scope y
  conflicts participan en la policy existente.
- Diversity diagnostica metadata desconocida de forma conservadora; no infiere
  perspective/dependency desde hostname, publisher o contenido externo.
- Verification y diversity pueden diferir en sus diagnósticos porque son
  policies independientes: el resultado canónico de corroboración sigue
  siendo `VerificationResult` y diversity aporta warnings auditables.
- No se ensambló Source Bundle, no se finalizaron run/queue, no se añadieron
  llamadas de red y no se implementó I-04.

### Verification

- Tests de integración memory para múltiples Claims desordenadas, resultados
  persistidos, policy version, diversity conservadora y evidence insuficiente.
- Test del composition boundary app sobre los servicios workspace-scoped.
- `go test -race ./internal/research/application ./internal/research/diversity ./internal/app ./internal/infra/researchdb -run 'LiveMultiSource|LiveVerification' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 30 debe ensamblar un `source-bundle-v1` desde el run real y sus
  Claims ya verificadas, sin duplicar bodies/excerpts ni reinterpretar policies.
- El assembler existente acepta un run `running` para construir el hand-off;
  el Paso 31 debe publicar `completed` sólo después de validar que ese bundle
  durable pertenece al mismo run.

## Step 30 — Source Bundle from a real run

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `live-source-bundle-v1` toma el Run y las Claims reales de los artifacts,
  normaliza sus IDs y delega el ensamblado a `SourceBundleService` existente.
- El stage exige que el bundle durable devuelto pertenezca exactamente al Run
  y contenga el mismo set de Claims antes de exponerlo al orchestrator.
- El workspace SQLite ahora ensambla `SourceBundleService` con los repositories
  reales de request/run, Claims, Sources, Evidence, TrustDecisions,
  VerificationResults, conflicts y freshness; lectura/export siguen usando el
  mismo servicio y repository append-only.
- `source-bundle-v1` conserva roles primary/supporting/historical, issues,
  freshness agregada, conflicts, estado, summary y hash canónico reproducible.
- El bundle y sus colecciones/punteros se copian defensivamente en todos los
  hand-offs del orchestrator.
- El composition boundary de app conecta el bundle workspace-scoped al stage
  sin acceso directo a SQLite ni reimplementación de policy.

### Decisions

- Request, Run y query plan quedan referenciados por `RunID`: el audit durable
  del Run conserva queries y policy versions. Claims referencian Evidence y
  cada Evidence referencia snapshot/Source; el bundle no duplica bodies,
  excerpts ni records ya persistidos.
- El assembler acepta únicamente runs `running` o `completed`; durante live
  research se construye sobre `running`, y el Paso 31 es responsable de
  publicar el estado terminal sólo después de validar el bundle.
- El estado puede ser `ready`, `ready_with_caveats`, `incomplete` o
  `conflicted` según los records canónicos. La integración no eleva ni corrige
  resultados insuficientes.
- No se modificó `source-bundle-v1`, no se añadió migration, no se finalizó la
  queue/run y no se implementó provenance ni I-04.

### Verification

- Test de integración memory con Run running, Claim/Evidence/Source/Trust/
  Verification/Freshness reales, persistencia append-only y bundle `ready`.
- Tests de Claim set vacío, match exacto run/Claims y ownership defensivo.
- Test del composition boundary app y tests existentes del assembler/export.
- `go test -race ./internal/research/application ./internal/research/bundle ./internal/app ./internal/infra/researchdb -run 'LiveSourceBundle|LiveBundle' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 31 debe convertir Run y queue execution en terminal success sólo
  cuando el bundle durable validado pertenece al mismo Run.
- Un fallo debe conservar una razón estructurada segura y no dejar
  `DiscoveryPending=true` para outcomes terminales.

## Step 31 — Transactional run and queue finalization

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `research-finalization-v1` define una única settlement request cerrada para
  success, retry, permanent failure y cancellation, ligada a queue item, Run,
  attempt, timestamp y bundle opcional.
- `ResearchQueueConsumer` ya no observa un Run completado por separado y luego
  hace ack: recibe del orchestrator un Run `running` + bundle durable y delega
  Run transition + queue execution settlement a una sola operación.
- Success actualiza atómicamente `run.status=completed`, `completed_at`, queue
  status `dispatched` (ack histórico), execution `completed` y
  `execution_bundle_id`.
- Retry actualiza atómicamente Run `failed` + execution `retry` y mantiene la
  queue `queued`; permanent failure deja Run `failed`, queue `dispatched` y
  execution `failed`; cancellation deja ambos lados cancelados.
- El resultado terminal vuelve al orchestration result y la CLI existente
  deriva `DiscoveryPending=false` para completed/failed/cancelled.
- El finalization stage reabre el bundle por ID y exige Run, hash y
  `source-bundle-v1` coincidentes antes de permitir settlement.
- Adapter memory con lock único y adapter SQLite con transacción única, compare
  and set sobre execution `claimed`, attempt y Run identity.
- Migración forward-only v46 añade `execution_bundle_id` con foreign key e
  índice parcial; los execution records previos permanecen legibles.
- El store workspace-scoped expone tanto el validation stage como el servicio
  de finalización transaccional para el futuro composition root del comando.

### Decisions

- El orchestrator conserva exclusivamente `planned → running` y prepara el
  resultado; terminalidad pertenece al consumer que posee el claim de queue.
  Así no existe una ventana durable `Run completed / queue claimed`.
- `dispatched` sigue siendo el ack terminal de la queue I-03 existente;
  `execution_status=completed` distingue success sin crear una segunda queue.
- La razón persistida de failure continúa siendo el `ErrorKind` cerrado
  (`network_research_blocked`, `budget_exceeded`, etc.). No se persisten
  mensajes de provider, URLs sensibles, headers, bodies, stack traces ni
  secretos.
- Un success exige que `bundle_id` exista y pertenezca al mismo Run. Un bundle
  ausente o cruzado aborta toda la transacción y deja Run running + queue
  claimed para reconciliación segura.
- El path legacy `SettleExecution` permanece para compatibilidad/tests de la
  queue, pero el consumer live usa exclusivamente `ResearchFinalizationService`.
- No se añadió provenance, audit terminal, cost final ni I-04.

### Verification

- Tests memory de commit conjunto Run/queue/bundle ID, failure kind seguro,
  terminalidad e intento repetido rechazado.
- Test SQLite de commit real y rollback completo ante bundle de otro Run,
  verificando que ninguna de las dos aggregates queda parcialmente mutada.
- Tests actualizados del orchestrator para preparación no terminal y del queue
  consumer para ack, retry, permanent failure, cancellation e idempotencia.
- Tests de finalization stage contra bundle durable y composition boundary app.
- `go test -race ./internal/research/application ./internal/app ./internal/infra/researchdb ./internal/storage/sqlite -run 'Finalization|QueueConsumer|LiveResearchOrchestrator' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 32 puede registrar provenance end-to-end sobre las identidades ya
  terminalizadas sin cambiar la transacción Run/queue.
- El Paso 33 sigue siendo dueño del audit terminal y cost final; no debe
  duplicar `execution_bundle_id` ni persistir mensajes externos como failure.

## Step 32 — End-to-end provenance

Status: completed
Date: 2026-08-31
Release: unreleased

### Delivered

- `live-research-provenance-v1` construye y persiste un
  `provenance-graph-v1` por cada Claim real después de ensamblar el Source
  Bundle y antes del validation stage de finalización.
- Cada grafo enlaza las identidades reales ResearchRequest → ResearchRun →
  query → DiscoveredSource → Source → SourceSnapshot → Evidence → Claim →
  SourceBundle.
- El stage exige un `VerificationResult` válido y único por Claim, con el mismo
  conjunto exacto de Sources, antes de publicar el enlace Claim → Bundle.
- Queries sin ID durable reciben IDs semánticos estables derivados del request
  y texto exacto; los graph IDs son estables por run, Claim y bundle.
- Replay idempotente acepta únicamente el grafo byte-equivalente ya persistido;
  una colisión divergente falla sin sobrescribir historia.
- Los artifacts transportan copias defensivas de los grafos para el consumer y
  la composition boundary de app reutiliza el `ProvenanceService` del
  workspace.

### Decisions

- Se preservó sin cambios el vocabulario estable de `provenance-graph-v1`.
  Verification sigue siendo un record durable separado: el live stage valida
  Claim → Verification → Bundle antes de grabar el edge histórico Claim →
  SourceBundle, en lugar de introducir un node kind incompatible.
- Para una Source observada por múltiples queries se selecciona de forma
  determinista una observación durable que pruebe el camino de discovery; las
  demás observaciones continúan disponibles en `source_discoveries`.
- Provider/rank se conservan exclusivamente como provenance de discovery y no
  participan en authority, trust ni verificación.
- Labels del grafo son metadata bounded y no copian snippets, excerpts, bodies
  ni statements externos. Snapshot y Evidence conservan sus tool versions
  durables.
- Este paso no registra audit terminal, no reconcilia coste y no cambia la
  transacción Run/queue del Paso 31.

### Verification

- Tests de chain completa, ausencia de discovery/verification, persistencia,
  replay idempotente, ownership defensivo y orden del orchestrator.
- `go test -race ./internal/research/application ./internal/app ./internal/infra/researchdb ./internal/storage/sqlite -run 'LiveResearchProvenance|LiveResearchOrchestrator|LiveProvenance|Provenance' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 33 debe producir el checkpoint audit terminal desde los artifacts ya
  validados y la conciliación durable de `ResearchRun.Cost`.
- Provider ID, adapter version, counts, bytes, outcome, bundle ID y policy
  versions pertenecen al audit/cost metadata, no deben duplicarse dentro de
  cada provenance graph.

## Step 33 — Audit + Cost accounting

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- `live-research-terminal-audit-v1` deriva un checkpoint terminal desde el
  audit planned, los artifacts reales y el ledger durable de coste.
- La extensión aditiva `live-research-execution-audit-v1` registra provider ID,
  adapter version, provider API calls, query/result/fetch counts, bytes
  observados, cache hits, outcome, failure kind cerrado, bundle ID, coste usado,
  ahorro por cache y budget-stop.
- Query planner, trust, freshness y conflict policy versions se heredan del
  checkpoint planned durable; los algoritmos live efectivamente observados se
  agregan como metadata versionada.
- `ResearchQueueConsumer` prepara audit + coste antes del settlement y los
  entrega a `ResearchFinalizationService` usando el mismo `finalized_at`.
- Los adapters memory y SQLite publican Run, queue execution, bundle reference
  y audit terminal como una sola operación; SQLite prueba rollback completo si
  el audit no coincide con request/run/snapshots.
- El resultado terminal y los replays idempotentes exponen el checkpoint
  durable, y el Run retornado contiene el `ResearchCostMetadata` final.
- Fetch cost control reserva `fetch_requests + maximum bounded bytes` antes de
  llamar al adapter live; un budget denial no alcanza la red. Cache offline
  registra unidades evitadas sin sumar uso.
- El factory del provider entrega `brave` + `brave-web-search-v1` como metadata
  no secreta para que el futuro composition root complete el SearchExecution
  accounting.

### Decisions

- La ejecución terminal es una extensión opcional de `research-audit-v1`; los
  checkpoints históricos sin `execution` conservan exactamente su JSON/hash y
  siguen siendo legibles sin migration.
- `providers_used` continúa significando providers realmente llamados. Un
  adapter configurado pero resuelto por cache no se inventa como usado.
- `bytes_fetched` registra bytes realmente observados; `cost_used.bytes`
  registra la capacidad bounded reservada antes del fetch. La diferencia es
  intencional y auditable.
- Failure persiste sólo el `ErrorKind` cerrado. No se guardan mensajes de
  provider, credentials, headers, endpoints, bodies, snippets, excerpts,
  stack traces ni workspace paths.
- El audit terminal forma parte de la transacción del Paso 31; no duplica
  `execution_bundle_id`, sino que valida y referencia la misma identidad.
- No se añadió moneda/precio vendor, migration, I-04 ni dependencia de IA.

### Verification

- Tests de provider/adapter y JSON roundtrip; conteos query/result/fetch/bytes;
  coste final; provider cost sin attribution rechazado; pre-reserva de fetch;
  cache savings; budget denial antes del adapter.
- Tests memory/SQLite de commit conjunto y rollback por audit inválido, además
  de consumer success/retry/failure/cancellation/idempotencia con checkpoint.
- `go test -race ./internal/research ./internal/research/application ./internal/app ./internal/infra/researchsearch ./internal/storage/sqlite -run 'TerminalAudit|ExecutionAudit|CostControlledFetch|ResearchFinalization|QueueConsumer|ResearchTopicExecutes|FactoryBuilds' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 34 debe componer en el comando real todos los stages ya cerrados y
  poblar `LiveSearchExecutionMetadata` con los descriptor fields del factory y
  los counts observados por cada ejecución.
- La UX del Paso 34 puede renderizar el audit/coste ya durable; no debe crear
  una segunda fuente de métricas ni reconstruir metadata desde texto CLI.

## Step 34 — Complete `research topic` UX

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- El composition root productivo de `research topic` activa por defecto el
  executor síncrono existente cuando search, fetch, cache, normalizer,
  configuración y Secrets están disponibles en el binario real.
- El executor ensambla el único `ResearchQueueConsumer` y el orchestrator v1
  con search, candidate deduplication/registration, fetch, snapshot,
  normalization, extraction, trust/temporal evaluation, verification, bundle,
  provenance, validation, terminal audit y finalization existentes.
- El search stage ejecuta el Query Plan bounded, aplica los límites resueltos,
  convierte resultados a candidates y conserva provider ID, adapter version,
  queries/resultados observados y provider API calls conciliadas desde el
  ledger durable.
- El store workspace-scoped expone los servicios existentes de deduplicación y
  registro sin filtrar repositories SQLite al composition root.
- La salida de `research topic` completado muestra queries planificadas,
  Sources descubiertas/fetched, Evidence, Claims, verificaciones y el Source
  Bundle durable; ya no presenta discovery pending después de terminal success.
- El validation stage rechaza bundles `incomplete` o `conflicted`; sólo
  `ready` y `ready_with_caveats` pueden preceder un Run completed.

### Decisions

- El assembly del provider ocurre dentro del search stage, después del claim de
  queue, para que configuración/credenciales ausentes se liquiden por el
  lifecycle existente y no dejen un falso success ni inicien trabajo detached.
- `LiveSearchExecutionMetadata` toma llamadas del cost ledger, no de texto CLI
  ni de una segunda métrica in-memory. Queries/results provienen de los
  artifacts exactos de la ejecución.
- El executor inyectable del Paso 16 permanece como seam de tests. Servicios
  incompletos que deliberadamente no configuran el runtime live conservan el
  comportamiento pending usado por tests/consumidores parciales; el binario
  productivo configura todas las dependencias y ejecuta el path completo.
- Search results continúan siendo candidates. La UX cuenta Evidence y Claims
  sólo desde artifacts producidos por sus stages respectivos.
- No se añadió daemon, segunda queue, migration, dependencia externa, IA ni
  comportamiento de I-04.

### Verification

- Tests del search stage con dos queries, mapping de candidates y metadata
  provider/cost observada.
- Test CLI del resumen terminal query-to-bundle y ausencia de
  `Discovery: pending` en success.
- Test de rejection de bundle incomplete antes de finalization.
- Tests dirigidos de app, CLI, application, researchdb y composition del
  binario.
- `go test ./...` fuera del sandbox para habilitar listeners loopback de los
  fixtures `httptest`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 35 debe enriquecer exclusivamente `research status/show` usando Run,
  queue/audit y bundle ya durables.
- Provider, queries, counts, warnings, bundle y failure reason deben salir de
  records tipados; no reconstruirlos desde logs o mensajes externos.

## Step 35 — Improve `research status/show`

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- Proyección `ResearchRunProgressCLIView` añadida para reconstruir progreso
  exclusivamente desde `ResearchRun`, audit trail y Source Bundle durables.
- `research status` y `research show` reabren el workspace y muestran status,
  phase, queries, provider/adapter/API calls, resultados, fetch attempts,
  snapshots, warnings, bundle ID/state y failure reason seguro.
- Phase distingue `queued`, `query_to_bundle` y los estados terminales
  completed/failed/cancelled sin introducir otra state machine persistida.
- Queries proceden del checkpoint planned; provider, conteos y failure reason
  proceden de la extensión terminal `ResearchAuditExecution`.
- Warnings combinan progreso parcial observable (`fetches > snapshots`) con
  los `SourceBundleIssue` cerrados y versionados; no incluyen mensajes de red.
- Status vuelve a mostrar el estado real del Run. El estado del bundle queda
  separado junto a su ID, evitando presentar `ready_with_caveats` como si fuera
  un lifecycle status.
- `research show` conserva todos los checkpoints, hashes, policy versions,
  snapshots y disclaimer existentes debajo del nuevo resumen durable.

### Decisions

- No se añadió una tabla/columna de progreso: los stages ya producen toda la
  información durable requerida en Run, audit y bundle.
- Si existe audit terminal y bundle, sus IDs deben coincidir; una divergencia
  falla la inspección en lugar de mostrar una combinación incoherente.
- `partial_source_processing` es una advertencia factual y conservadora cuando
  hubo más fetch attempts que snapshots; no adivina si la pérdida ocurrió en
  fetch, snapshot, cache o normalization.
- Provider ausente se muestra como `none`; failure reason ausente como `none`.
  Nunca se renderizan credentials, headers, endpoints, bodies ni mensajes
  externos.
- No se implementaron tests del adapter, conformance, E2E nuevos, smoke live,
  daemon, IA ni I-04; pertenecen a pasos posteriores.

### Verification

- Tests de proyección durable para failure con provider/counters/reason y
  completion con bundle caveat/warning.
- Tests application de roundtrip lógico status/show sobre el store reabierto.
- Tests CLI de status running y show con todos los campos nuevos.
- Expectativa del E2E existente actualizada para separar Run status y bundle
  state, sin ampliar todavía su alcance al Paso 38.
- Tests dirigidos de app, CLI, application, researchdb y composition.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 36 debe cubrir exhaustivamente el adapter Brave con `httptest` y sin
  Internet público; no modificar la UX salvo que revele una regresión real.
- Los Pasos 38–41 siguen siendo dueños de la matriz E2E query-to-bundle,
  privacy, partial failure e idempotency.

## Step 36 — Production SearchProvider unit tests

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- Matriz HTTP del adapter Brave ejecutada exclusivamente contra
  `httptest.Server`, sin acceso a Internet público.
- Casos success y empty verifican el request GET bounded, query params,
  credencial, media type, user agent, compresión identity y mapping de
  resultados normalizados.
- Respuestas malformed y oversized se rechazan como `response` sin consumir
  contenido sin límite.
- Estados 401 y 403 se clasifican como `authentication`, 429 como
  `rate_limited` con metadata bounded y 500 como `unavailable`.
- Timeout real del cliente se clasifica como `unavailable`; cancelación del
  caller durante una petición en vuelo preserva `context.Canceled` y detiene el
  handler local.

### Decisions

- Las pruebas atraviesan juntas las fronteras reales `Brave` y
  `SecureHTTPClient`; el endpoint se sustituye sólo dentro del paquete de test
  por el servidor loopback.
- Se mantuvieron los tests aislados existentes de mapping, pagination, cost
  authorization, input validation y transport hardening; la nueva matriz
  prueba outcomes end-to-end del adapter sin duplicar producción.
- No se cambió código productivo, UX, configuración, contracts ni lifecycle.

### Verification

- `go test -race ./internal/infra/researchsearch -run 'BraveSearchProviderHTTP|BraveSearch' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 37 debe extraer un suite de conformance reusable y ejecutarlo contra
  `StaticSearchProvider` y el adapter Brave.
- Los checks vendor-specific de HTTP/status/limits permanecen en este paquete;
  conformance debe probar únicamente el contrato neutral `SearchProvider`.

## Step 37 — SearchProvider conformance tests

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- Paquete reusable `searchprovidertest` añadido como test harness del port
  application-owned `SearchProvider`, listo para adapters futuros.
- El mismo suite se ejecuta contra `StaticSearchProvider` y contra el adapter
  real Brave, sin dos copias divergentes de las expectativas.
- Conformance cubre resultados válidos y bounded, hints opcionales, empty,
  cancelación, preservación de inputs y ownership defensivo de slices y
  timestamps retornados.
- El fixture estático ahora respeta `SearchOptions.Limit`; el suite detectó que
  antes devolvía todo su result set aunque el caller pidiera menos resultados.
- Brave cruza el suite mediante su implementación real y una respuesta nativa
  JSON determinista inyectada por `HTTPClient`, sin Internet público.

### Decisions

- El suite contiene sólo semántica provider-neutral. Status HTTP, headers,
  pagination, response bounds, auth, rate limiting y timeouts permanecen en los
  unit tests vendor-specific del Paso 36.
- La factory reusable recibe fixtures contractuales y debe devolver la
  implementación real; así un futuro adapter puede traducirlos a su wire format
  sin debilitar las mismas assertions.
- `StaticSearchProvider` continúa siendo network-free y defensivo. Honrar
  `Limit` es una corrección de conformance acotada, no una nueva policy.
- No se cambió la interface `SearchProvider`, el DiscoveryService, la UX, el
  lifecycle ni la configuración productiva.

### Verification

- `go test -race ./internal/research/application/... ./internal/infra/researchsearch -run 'SearchProviderConformance' -count=1`.
- `go test -race ./internal/research/application/... ./internal/infra/researchsearch -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 38 debe añadir el E2E query-to-bundle completo con Search API y
  content server locales; este paso no adelanta esa matriz.
- Adapters futuros deben invocar `searchprovidertest.Run` además de conservar
  sus propios tests de transporte y mapping vendor-specific.

## Step 38 — E2E query-to-bundle without Internet

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- E2E `TestResearchTopicQueryToBundleEndToEnd` añadido bajo el gate `e2e`, sin
  acceso a Internet público.
- Una Search API `httptest.Server` autenticada devuelve dos URLs bounded; un
  content server local separado sirve dos documentos HTML mediante hosts
  `.fixture.test` resueltos sólo a loopback.
- El escenario invoca el composition root síncrono real de `research topic`
  con workspace filesystem, SQLite, research cache, cost control, privacy
  gate, fetch endurecido y normalizador productivos.
- La cadena observable cubre query → URLs → Sources → fetch → snapshots →
  normalize → Evidence → Claims → verification → Source Bundle → provenance.
- Tras cerrar y reabrir el workspace se verifican Run completed,
  `DiscoveryPending=false`, bundle durable con hash y un grafo durable por
  Claim.
- Cada grafo exige la cadena completa ResearchRequest → ResearchRun → query →
  DiscoveredSource → Source → SourceSnapshot → Evidence → Claim → SourceBundle
  con las identidades reales del resultado.

### Decisions

- Las dos fuentes conocidas se precargan como documentación oficial trusted en
  el registry, igual que un workspace con catálogo revisado; los resultados de
  búsqueda siguen siendo candidates y no asignan trust por sí mismos.
- Las páginas contienen dos statements distintos y conservadores para que cada
  Claim tenga soporte primario explícito sin inventar independencia entre dos
  URLs ni debilitar Trust/Verification Policy.
- El único adapter reemplazado es la API de búsqueda externa. Su factory usa el
  mismo `LiveSearchBuildRequest`, privacy gate, durable cost ledger y
  `CostControlledDiscoveryService` que exige el composition root.
- El fetch de contenido usa `NewLoopbackFixtureClient`, disponible sólo bajo el
  build tag E2E; la política productiva de direcciones públicas no se relajó.
- No se cambió código productivo, UX, schema, lifecycle, políticas ni I-04.

### Verification

- `go test -race -tags=e2e ./tests/e2e -run '^TestResearchTopicQueryToBundleEndToEnd$' -count=1`.
- `go test -tags=e2e ./tests/e2e -count=1`.
- `go vet -tags=e2e ./tests/e2e`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 39 debe comprobar `privacy.allow_network=false` con cero llamadas a
  la Search API y cero fetches, sin reutilizar este success como atajo.
- Partial failure, idempotency y live smoke continúan reservados a los Pasos
  40–42.

## Step 39 — E2E privacy disabled

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- E2E `TestResearchTopicPrivacyDisabledEndToEnd` añadido bajo el gate `e2e`
  con `privacy.allow_network=false` y toda la infraestructura live configurada.
- El fixture cuenta por separado invocaciones al método del provider, requests
  a la Search API y requests a cada documento del content server.
- El escenario exige cero provider calls, cero Search API requests y cero
  fetches aun cuando provider, credencial, discovery, fetcher y normalizer
  están disponibles.
- El workspace reabierto confirma un Run terminal `failed`, audit durable con
  `network_research_blocked`, cero resultados/fetches/providers usados y cero
  bundles persistidos.

### Decisions

- La factory sí debe construirse una vez: esto demuestra que el bloqueo ocurre
  en la compuerta de privacidad del flujo real y no por configuración ausente.
- `QueryCount` conserva la consulta planificada aunque la red esté bloqueada;
  por ello la frontera se prueba con contadores directos del provider/HTTP y
  con `ResultCount`, `FetchCount` y providers usados iguales a cero.
- Se reutilizan los mismos servidores loopback y el mismo adapter fixture del
  E2E query-to-bundle, pero no se precargan Sources ni registry entries porque
  ninguna candidate debe atravesar discovery.
- No se cambió código productivo, UX, schema, lifecycle, políticas ni I-04.

### Verification

- `go test -race -tags=e2e ./tests/e2e -run '^TestResearchTopicPrivacyDisabledEndToEnd$' -count=1`.
- `go test -race -tags=e2e ./tests/e2e -run '^TestResearchTopic(QueryToBundle|PrivacyDisabled)EndToEnd$' -count=1`.
- `go test -tags=e2e ./tests/e2e -count=1`.
- `go vet -tags=e2e ./tests/e2e`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 40 debe cubrir fallos parciales bounded de fuentes sin cambiar la
  frontera offline demostrada aquí.
- Idempotency y live smoke continúan reservados a los Pasos 41–42.

## Step 40 — E2E partial source failure

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- E2E `TestResearchTopicPartialSourceFailureEndToEnd` añadido bajo el gate
  `e2e`, con una Search API local que devuelve exactamente cuatro Sources.
- El content server ejecuta la matriz A success, B timeout, C 404 y D success;
  los dos fallos quedan preservados como `unavailable` y `external_failure` sin
  impedir que las dos Sources restantes alcancen snapshots, Claims y bundle.
- El escenario confirma Run `completed`, cuatro intentos de fetch, dos
  snapshots, un único bundle durable y cero requests fuera de loopback.
- `research status` reabre el workspace y reconstruye el warning
  `partial_source_processing` desde audit y bundle durables.
- El máximo productivo por Source se redujo de 4 MiB a 2 MiB para reconciliarlo
  con el presupuesto v1 de 8 MiB por Run: así cuatro fetches bounded pueden ser
  autorizados sin ampliar el presupuesto ni omitir Sources por scheduling.

### Decisions

- Timeout y 404 son fallos individuales, no outcomes terminales mientras otras
  Sources satisfagan verification y permitan un bundle válido.
- El HTTP fixture usa un solo intento por Source para aislar partial failure de
  la política de retry de transporte, que ya tiene cobertura propia.
- El 404 atraviesa la frontera productiva actual como `external_failure`; el
  test no introduce una taxonomía HTTP nueva fuera del alcance del paso.
- El warning no se persiste como texto duplicado: se deriva del conteo durable
  `fetches > snapshots`, conforme a la proyección de progreso existente.
- No se cambió schema, provider, lifecycle, trust/verification policy, UX ni
  ninguna frontera de red o privacidad.

### Verification

- `go test -race -tags=e2e ./tests/e2e -run '^TestResearchTopicPartialSourceFailureEndToEnd$' -count=1`.
- `go test -race -tags=e2e ./tests/e2e -run '^TestResearchTopic(QueryToBundle|PrivacyDisabled|PartialSourceFailure)EndToEnd$' -count=1`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 41 debe interrumpir un intento después de producir datos durables y
  reintentar el mismo queue item mediante un nuevo Run conforme al modelo.
- El replay no debe duplicar lógicamente Source, snapshot, Claim ni bundle.
- El smoke live continúa reservado al Paso 42.

## Step 41 — E2E retry/idempotency

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- E2E `TestResearchTopicRetryIdempotencyEndToEnd` añadido bajo el gate `e2e`
  sobre el composition root síncrono real de `research topic`.
- El primer intento ejecuta search, fetch y snapshot para dos Sources y luego
  recibe una interrupción transitoria controlada en normalization.
- El primer Run queda durable como `failed`, la queue original como `retry`,
  `attempts=1`, dos snapshots persistidos, failure kind seguro `unavailable` y
  ningún bundle.
- La segunda invocación reutiliza el mismo ResearchRequest y queue item, crea
  el nuevo Run permitido por el modelo y completa con `attempts=2`.
- El retry se ejecuta con red deshabilitada: discovery usa un SearchCache
  determinista y fetch usa el cache filesystem productivo llenado por el primer
  intento, con cero llamadas adicionales a Search API o content server.
- Tras reabrir el workspace se verifican exactamente dos Sources, los mismos
  dos snapshot IDs, cada Claim del resultado durable, cero bundles para el Run
  fallido, un bundle para el Run completado y un único settlement de queue.

### Decisions

- La interrupción se inyecta en la frontera `SourceNormalizer`, inmediatamente
  después de que el stage anterior haya persistido snapshots; no se añadió un
  hook productivo ni una state machine alternativa.
- `unavailable` es transitorio según `research-queue-worker-v1`; el consumer no
  crea un loop automático y la segunda invocación explícita posee el retry.
- El retry conserva la identidad lógica de request/queue pero usa un Run nuevo,
  conforme al acceptance contract y a la arquitectura existente.
- Reutilizar cache con `privacy.allow_network=false` prueba simultáneamente que
  el replay durable no necesita refetch ni crea una segunda observación de
  snapshot para contenido ya capturado.
- Claims y bundle se construyen sólo en el intento que supera normalization;
  Source y snapshot se reutilizan por identidad durable.
- No se cambió código productivo, schema, lifecycle, política de retry,
  provider, UX, trust/verification ni I-04.

### Verification

- `go test -race -tags=e2e ./tests/e2e -run '^TestResearchTopicRetryIdempotencyEndToEnd$' -count=1`.
- `go test -race -tags=e2e ./tests/e2e -count=1`.
- `go vet -tags=e2e ./tests/e2e`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 42 es el único paso pendiente y requiere un smoke live explícitamente
  opt-in contra el provider externo real.
- El smoke no debe usar URL hardcodeada ni contaminar las suites offline.

## Step 42 — Live SearchProvider smoke

Status: completed
Date: 2026-09-01
Release: unreleased

### Delivered

- Smoke `TestLiveResearchSearchProvider` añadido a `tests/live` bajo el opt-in
  exacto `KELYRO_LIVE_RESEARCH_SEARCH_TESTS=1`.
- El test construye el transporte Brave endurecido y el adapter real mediante
  Foundation Secrets, pasando toda búsqueda por `privacy.NetworkGate` y
  `DiscoveryService`.
- La credencial se resuelve únicamente como
  `research.search.brave.api_key`; el test no lee, registra ni imprime su valor.
- Una query bounded solicita hasta tres resultados al endpoint externo y exige
  al menos una URL válida, provider `brave`, rank no negativo y ausencia de
  URLs duplicadas.
- Ninguna URL de resultado, título, snippet, posición exacta o conteo exacto
  está hardcodeado; las invariantes toleran cambios normales del índice.
- La guía de integración live documenta opt-in, referencia de Foundation
  Secrets, comando de ejecución y alcance de assertions.

### Decisions

- El gate se evalúa antes de construir transporte cuando el opt-in está
  ausente; por ello `go test ./...` continúa completamente offline.
- Habilitar explícitamente el smoke exige una credencial usable y convierte su
  ausencia en fallo accionable, no en un skip silencioso.
- Se usa `SearchOptions.Limit=3` para mantener una sola request bounded sin
  depender de paginación ni ranking del provider.
- Context7 confirmó el contrato vigente de Brave Web Search: endpoint GET,
  header `X-Subscription-Token`, parámetros `q`/`count` y URL en
  `web.results[].url`.
- El test no persiste resultados, no descarga las URLs descubiertas y no
  adelanta el query-to-bundle reservado al Paso 43.
- No se cambió código productivo, schema, provider contract, lifecycle, costes,
  UX ni I-04.

### Verification

- `go test ./tests/live -run '^TestLiveResearchSearchProvider$' -count=1 -v`
  (skip esperado sin opt-in, antes de construir red).
- `go vet ./tests/live`.
- `go test ./...`.
- `go vet ./...`.
- `git diff --check`.

### Notes for next session

- El Paso 43 debe reutilizar el provider y URL retornada realmente para cubrir
  fetch → normalize → evidence → claim → verify → bundle.
- El smoke completo debe afirmar invariantes, no ranking ni contenido exacto.
- Security review y pasos posteriores permanecen fuera de alcance.
