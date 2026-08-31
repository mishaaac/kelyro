# ADR-001 — Brave Web Search API como reference SearchProvider

- Status: accepted
- Date: 2026-08-30
- Scope: I-03C Step 5
- Decision owner: Kelyro Research infrastructure
- Last verified against provider documentation: 2026-08-30

## Context

I-03C necesita un único adapter inicial para discovery web público. El adapter
debe satisfacer el `SearchProvider` provider-neutral sin scraping HTML, browser,
LLM, crawler, trust decisions ni evidence extraction.

Los criterios de selección son:

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

## Decision

Seleccionar **Brave Web Search API** con provider ID estable `brave`.

El adapter de referencia usará exclusivamente:

```text
GET https://api.search.brave.com/res/v1/web/search
```

con `q`, `count` y paginación acotada. La autenticación será el header
`X-Subscription-Token`, cuyo valor se resolverá en un paso posterior mediante
Foundation Secrets bajo la referencia:

```text
research.search.brave.api_key
```

No usar Brave Answers, LLM Context, Summarizer, rich callbacks ni endpoints AI.
La decisión es Web Search estructurado solamente.

## Evidence matrix

| Criterio | Evidencia oficial vigente | Evaluación |
|---|---|---|
| documented API | La [referencia GET de Web Search](https://api-dashboard.search.brave.com/api-reference/web/search/get) publica endpoint, autorización, parámetros y schema; la [guía Web Search](https://api-dashboard.search.brave.com/app/documentation/web-search/get-started) incluye ejemplos y paginación. | Cumple. |
| structured JSON | La [quickstart oficial](https://api-dashboard.search.brave.com/documentation/quickstart) muestra `web.results[]` con `title`, `url`, `description` y `age`. | Mapeo directo al contrato Kelyro. |
| TLS | El endpoint publicado es `https://api.search.brave.com`. | Cumple; Step 7 congelará transport hardening. |
| authentication | La [guía de autenticación](https://api-dashboard.search.brave.com/documentation/guides/authentication) exige API key en `X-Subscription-Token` y advierte no exponerla en repositorios o clientes. | Compatible con Foundation Secrets. |
| rate limits | La [guía de rate limiting](https://api-dashboard.search.brave.com/documentation/guides/rate-limiting) documenta sliding window, `429` y headers `X-RateLimit-Limit`, `Policy`, `Remaining` y `Reset`. | Permite mapping y retry bounded. |
| cost visibility | La [página oficial de pricing](https://api-dashboard.search.brave.com/documentation/pricing) publica Search a USD 5 por 1,000 requests, USD 5 de créditos mensuales y capacidad de 50 requests/s. | Coste por request visible; verificar de nuevo antes de release. |
| URL/title/snippet | `web.results[].url`, `title` y `description` están documentados; `age` puede alimentar únicamente `PublishedHint`. | Cumple sin promover snippet/age a Evidence. |
| testability | REST/JSON simple sobre una operación GET, sin SDK obligatorio. | Adapter testeable con client inyectado y `httptest`, sin Internet. |

## Mapping al contrato Kelyro

| Brave Web Search | Kelyro |
|---|---|
| `q` | `SearchQuery.Text` |
| request provenance | `SearchQuery.RequestID` permanece local; no se envía al provider |
| `count` + `offset` | `SearchOptions.Limit`, con requests/páginas bounded |
| `web.results[].title` | `SearchResult.Title` |
| `web.results[].url` | `SearchResult.Locator` |
| `web.results[].description` | `SearchResult.Snippet` opcional |
| orden del array | `SearchResult.Rank`, posición absoluta zero-based |
| `web.results[].age` RFC3339 válido | `SearchResult.PublishedHint` opcional |
| constante local | `SearchResult.Provider = "brave"` |

La guía limita `count` a 20 por página y `offset` a 9. Un `Search` lógico puede
requerir varias API calls para un limit mayor que 20; cada call debe respetar
cost control, rate limits, cancellation y el hard cap de 100 candidates.
`query.more_results_available` impide solicitar páginas inexistentes.

El provider no expone un campo rank separado: su orden de resultados es el
ranking observado. Kelyro preservará la posición absoluta y nunca la tratará
como authority.

## Operational and policy constraints

### Persistence rights

La [FAQ oficial de Brave Search API](https://brave.com/search/api/) indica que
almacenar resultados total o parcialmente requiere un plan que conceda esos
derechos. Kelyro persiste provenance y sources seleccionadas, así que el uso
productivo queda condicionado a confirmar que el plan/terms activos permiten
esa persistencia mínima.

Sin confirmación, el provider debe permanecer `unavailable` y no se habilita un
live run. El adapter tampoco almacenará responses JSON completas ni snippets
extra; solo devolverá DTOs transient al application boundary.

### Query privacy

La [Privacy Notice de Brave Search API](https://api-dashboard.search.brave.com/privacy-policy)
vigente al decidir informa que las queries asociadas a una cuenta API pueden
retenerse hasta 90 días para billing y troubleshooting. Kelyro debe exponer esta
implicación en documentación/readiness antes de live use.

`privacy.allow_network=false` siempre impide la request, aunque `brave` esté
configurado y tenga credencial. Queries, URLs, workspace paths y student content
no se incluyen en logs de diagnóstico.

### Cost and quota accounting

- Cada HTTP request al provider incrementa `ProviderAPICalls`.
- Cada query lógica incrementa `SearchRequests` una vez, aunque pagine.
- Solo se solicita otra página si sigue siendo necesaria y
  `more_results_available=true`.
- Los headers de quota se interpretan como metadata bounded; nunca alteran
  trust ni evidence.
- Pricing y capacity son datos externos mutables y deben verificarse nuevamente
  antes del release que habilite el adapter.

## Alternatives considered

### Google Custom Search JSON API — rejected

La [documentación oficial de Google](https://developers.google.com/custom-search/v1/overview)
confirma JSON, API key y pricing visible, pero el servicio está cerrado a nuevos
clientes y los clientes existentes deben migrar antes del 1 de enero de 2027.
Además requiere un Programmable Search Engine configurado. No es un baseline
viable para una integración nueva.

### Microsoft Bing Search APIs — rejected

Microsoft documenta que [Bing Search APIs se retiraron el 11 de agosto de
2025](https://learn.microsoft.com/en-au/lifecycle/announcements/bing-search-api-retirement)
y ya no están disponibles para uso o signup. La alternativa sugerida es
Grounding with Bing Search dentro de Azure AI Agents, lo que introduciría una
dependencia AI expresamente fuera de alcance.

### HTML search-engine scraping — rejected

Viola el límite explícito de I-03C, es frágil ante layout/anti-bot changes y no
ofrece un contrato formal de auth, quota, cost o structured results.

## Consequences

### Positive

- Adapter pequeño sobre standard library y el HTTP boundary existente.
- JSON con mapeo directo al `SearchProvider` congelado.
- Índice independiente; Brave declara que la API no reempaqueta scraping de
  Google/Bing.
- Auth, quotas, pricing y errores documentados.
- No requiere dependencia Go externa ni IA.

### Negative

- Requiere cuenta, plan, payment details y API key.
- Queries salen hacia un servicio externo y pueden tener retención operativa.
- Persistencia necesita derechos contractuales explícitos.
- Paginación puede multiplicar coste por una sola query lógica.
- Kelyro asume una dependencia operativa vendor-specific en infra.

## Constraints for subsequent steps

Step 6 debe implementar `brave` solo en `internal/infra/researchsearch`, con
fixtures JSON deterministas y sin añadir SDK. Step 7 debe fijar endpoint/host,
timeouts, redirects, response limits, error/status mapping, cancellation,
rate-limit handling y redacción del token. Step 8 debe obtener la key solo de
Foundation Secrets. Domain y `SearchProvider` no importarán tipos Brave.

Esta ADR no implementa el adapter, no activa red y no contiene credenciales.

## Reconsideration triggers

- Brave retira o rompe el endpoint/schema seleccionado.
- Pricing, quota, privacy o storage terms dejan de ser compatibles con Kelyro.
- No puede contratarse un plan con los derechos de persistencia requeridos.
- Aparece una API formal más sostenible que preserve JSON, costes visibles,
  testability y ausencia de IA/scraping.
