# I-03C — Acceptance contract query-to-bundle

## Propósito

Este contrato define cuándo el flujo live de I-03C está completo desde el
binario real. Es independiente del provider concreto y no prescribe todavía
configuración, tipos Go, wiring ni estrategia de ejecución.

La entrada mínima es un topic válido enviado mediante:

```text
kelyro research topic "<tema>"
```

La salida durable debe ser un `ResearchRun` terminal y auditable. Un resultado
de search es solo un candidate; nunca cuenta como Evidence antes de registration,
fetch, snapshot, normalization y extracción.

## Pipeline obligatorio

```text
topic
→ query plan
→ search
→ URLs
→ source registration
→ fetch
→ snapshot
→ normalize
→ evidence
→ claims
→ trust / verify
→ bundle
→ completed run
```

Ninguna etapa puede omitirse en un success live. Una representación recuperada
del cache offline puede sustituir la operación de red correspondiente, pero no
las validaciones ni las relaciones de provenance posteriores.

## Invariantes por etapa

| Etapa | Condición de aceptación |
|---|---|
| topic | El `ResearchRequest` conserva topic, purpose, target version opcional y timestamp validados. |
| query plan | Usa `query-planner-v1`, contiene al menos una query y respeta sus límites. |
| search | Cada query ejecutada pasa por `DiscoveryService`, privacy y cost control; live y cache permanecen distinguibles. |
| URLs | Cada candidate tiene locator HTTP(S) válido. Duplicados canónicos no cuentan como sources distintas. |
| source registration | Cada candidate seleccionado obtiene `SourceID` estable; ranking/snippet no se convierten en evidence. |
| fetch | Cada request pasa por `FetchService`, privacy y límites de bytes. Fallos individuales quedan registrados. |
| snapshot | Todo contenido usado después está ligado a un `SourceSnapshot` durable con hash y metadata de fetch. |
| normalize | Solo contenido validado se convierte en `NormalizedSource`; contenido externo sigue siendo datos no confiables. |
| evidence | Cada Evidence es bounded, literal y referencia exactamente source + snapshot + location + excerpt hash + extractor version. |
| claims | Cada Claim es conservador, está respaldado por Evidence persistida y conserva scope/version/qualifiers cuando aplican. |
| trust / verify | Authority, trust, freshness, conflicts y multi-source verification usan algoritmos versionados; ausencia o conflicto no se ocultan. |
| bundle | El Source Bundle referencia el run, claims, sources, verification, freshness/conflicts y provenance persistidos. |
| completed run | El bundle durable existe antes de persistir `status=completed`; audit y cost metadata reflejan el outcome final. |

## Success mínimo

Un run solo cumple el acceptance contract cuando todas estas condiciones son
verdaderas al releer el workspace:

```text
status = completed
discovery_pending = false
searches_performed > 0
sources_discovered > 0
sources_fetched >= 1
bundle_id != empty
```

Además:

- el bundle existe y su estado es `ready` o `ready_with_caveats`;
- al menos una Claim del bundle traza a Evidence, snapshot y source durables;
- cada source usada por una Claim tiene una decisión de trust explícita;
- cada Claim incluida conserva un resultado de verification explícito;
- el audit final conserva queries, versiones de algoritmos, network mode,
  provider, métricas y outcome;
- el cost metadata conserva uso real y cualquier ahorro por cache;
- `research status <run-id>` encuentra el mismo run y bundle después de reabrir
  el workspace.

`searches_performed` cuenta invocaciones válidas de discovery, sean live o
servidas desde cache. La auditoría debe distinguir el origen. Un run offline
puede completar si el cache contiene todo lo necesario; no puede fingir que
hubo red.

## Failure classes

Las clases siguientes son nombres estables del acceptance contract. La
taxonomía de errores Go y su mapping detallado pertenecen a pasos posteriores.

| Clase | Condición | Resultado mínimo |
|---|---|---|
| `network_disabled` | Privacy o modo offline impiden una operación necesaria y no existe fallback cache suficiente. | No se invoca red; run terminal `failed`; causa segura y auditable. |
| `provider_unconfigured` | Se necesita discovery live pero no existe provider usable/configurado. | Cero intentos de search externo; run terminal `failed`; no se expone secreto. |
| `provider_auth_failed` | El provider rechaza credenciales. | No se reintenta indefinidamente; run terminal `failed`; credenciales ausentes de logs/audit. |
| `provider_rate_limited` | El provider agota la política acotada de retry/backoff o indica que no debe reintentarse. | Run terminal `failed`, salvo que un fallback cache suficiente permita completar; retry/cost auditados. |
| `search_failed` | Discovery falla por una causa externa no clasificada arriba. | Run terminal `failed`, salvo fallback cache suficiente; causa preservada. |
| `no_results` | Las queries válidas terminan sin candidates utilizables. | `sources_discovered=0`, ningún fetch, run terminal `failed`. |
| `fetch_failed_partial` | Al menos un candidate falla al descargar, pero otro puede continuar. | Fallo individual auditado; el run puede completar solo si aún satisface verification y bundle. |
| `verification_insufficient` | Las Claims extraídas no alcanzan el mínimo de corroboración/trust aplicable. | No se publica bundle ready; run terminal `failed` con la insuficiencia visible. |
| `bundle_not_buildable` | Persisted evidence/claims/verification no permiten ensamblar un bundle válido. | No se marca completed; run terminal `failed`; no se inventan claims ni referencias. |
| `cancelled` | El contexto o una acción explícita cancela el trabajo. | Se detiene trabajo nuevo, se preserva lo ya durable y el run termina `cancelled`. |

`fetch_failed_partial` es la única clase de esta lista que puede coexistir con
`status=completed`: debe quedar como warning/audit y nunca ocultarse. Si el
trabajo restante no alcanza el success mínimo, la causa terminal adicional
será `verification_insufficient` o `bundle_not_buildable`.

## Reglas de terminalidad y persistencia

- Un run iniciado no puede permanecer `planned` o `running` cuando la ejecución
  síncrona devuelve control por success, failure o cancellation.
- `completed` exige bundle durable; nunca se marca anticipadamente.
- `failed` y `cancelled` no exigen bundle y conservan audit/cost acumulados.
- Un fallo no convierte search results en Evidence ni deja relaciones parciales
  inválidas.
- Reintentar el mismo queue item no crea una segunda identidad lógica de request
  ni una segunda queue; puede crear un nuevo run conforme al modelo existente.
- Ningún failure message, audit record o cost record incluye API keys, headers
  de autenticación ni bodies web sin límites.

## Escenarios mínimos de aceptación futura

1. Success determinista con provider fixture, dos URLs, al menos un fetch,
   Evidence/Claim verificable, bundle durable y run completed.
2. Privacy disabled sin cache: cero llamadas al provider/fetcher y
   `network_disabled`.
3. Provider ausente: `provider_unconfigured` y run failed.
4. Provider sin resultados: `no_results`, cero fetches y run failed.
5. Un fetch falla y otro alcanza bundle: completed con
   `fetch_failed_partial` visible.
6. Evidence insuficiente: `verification_insufficient`, sin bundle ready y run
   failed.
7. Cancelación: run cancelled y ninguna operación nueva después de observar el
   contexto cancelado.
8. Roundtrip: cerrar/reabrir el workspace y recuperar run, audit, metrics,
   provenance y bundle idénticos.

Todos los escenarios ordinarios usan fixtures deterministas o `httptest`. Las
pruebas con Internet público permanecen opt-in y no bloquean la suite normal.
