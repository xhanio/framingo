# Roadmap

What framingo and its example are to do next. Within an area the items are
in the order planned; the areas are independent of each other. An item says
what it is and where it stands, and links to the design note that holds the
reasoning. A finished item is deleted — git history keeps it — and its
design note brought up to date.

## Restarts under load

The restart policy holds already: only a liveness failure restarts a
service, a dependency's outage fails readiness instead, and restarts spent
escalate to the platform through `/healthz`
([supervisor.md](plugins/framingo/skills/framingo/pkgs/supervisor.md#health-probes--escalation)).
What a restart does to the requests in flight is not yet safe.

- **A handle swaps what it holds safely.** A restart reruns `Init`, and
  `db.Manager`'s reconnect replaces the pool `ORM()` and `FromContext` hand
  out with no synchronization — a data race with every query in flight.
  Every `Init` a restart reruns that replaces what other goroutines read gets
  the same check, and a test restarts a dependency while its dependents
  serve, under `-race`.
- **The rule, written down.** A dependent keeps its dependency's handle,
  never what the handle hands out, so a dependency's restart needs no restart
  of its dependents: `design/service-wiring.md` says so, and why the
  supervisor never restarts them.

## Events: what the bus may lose

- **Session revocation off the best-effort path.** The example ends a
  user's sessions on `LocalUsersDeleted`, `LocalUserPasswordReset` and
  `LocalUserIdentityChanged`, published after the commit. One lost — a full
  subscriber queue drops it by default — leaves the session alive, and every
  refresh extends it. A session checked on use, against what it was issued
  for, needs no message to arrive.
- **Delivery, said per driver.** `pubsub.md` says an in-process bus is no
  guarantee; it does not say, driver by driver, when a message is lost and
  what a consumer must do about it.
- **Candidate, not designed:** a transactional outbox for events that must
  follow a commit, perhaps on the change capture the config cache needs.

## Routers

- **The router check in the framework.** Registration fails on a handler
  `router.yaml` names that `Handlers()` lacks, but a handler whose signature
  discovery skips is caught only by `TestHandlersCoverConfig`, which each of
  the example's seven routers repeats; a framework helper makes it one line.
- **Candidate, not designed:** OpenAPI generated from `router.yaml` and the
  api types.

## Toward 1.0

- **A compatibility policy.** What is stable and what may still break:
  v0.9.0 alone carried twenty breaking changes.
- **The end-to-end checks as a suite.** What was checked by hand this
  release — a paused database, restarts, the TLS handshakes, the certificate
  flows — kept and runnable instead of rewritten each time.
- **Candidate, not designed:** splitting the module, once the coupling
  between its packages is measured.

## Certificates: rotation and a hierarchy

Done so far: a requester keeps its key, and the example has a server CA and
an agent CA whose keys live in the database alone
([design/certificates.md](plugins/framingo/skills/framingo/design/certificates.md)).

- **A pool of trusted CAs.** Trust is one certificate per CA; a pool lets a
  CA be replaced while the old one's certificates still verify.
- **A root and intermediates.** An offline root signs the server and agent
  CAs as intermediates, and a server sends its chain.

## Agent bootstrap and authentication

An agent's way in, end to end: how it gets its first certificate, and how
the manager knows it by that certificate afterwards. Parked until it is
designed as a whole; the items below are what that design has to settle.

- **Enrollment.** A one-time token authorizes an agent's first CSR; today
  `POST /certificates/csr` takes a user session with `certificate_write`.
- **Binding.** A certificate is bound to an agent record, so the manager
  knows which agent presents it.
- **authnagent.** Takes the agent from the handshake's verified chain
  instead of checking a signature and parsing the common name itself; it
  stays unregistered until then.
- **The proxy path.** A certificate a proxy forwards in a header proves no
  possession of its key: accepted from a trusted proxy only, or dropped.
- **mTLS in the example.** The api server is given the agent CA as
  `ClientCAs`. The CA is loaded by Init, after the server's options are
  built, so the server needs a source it reads at handshake time.
- **Renewal.** An agent renews before its 90 days end, authenticated by the
  certificate it holds.
- **Revocation.** A denylist checked on every agent request.

## Metrics

- **db pool gauges.** Designed, not built: connection counts by state and
  the pool's limits, registered by the db manager on each connect
  ([design](docs/superpowers/specs/2026-09-06-framework-metrics-design.md#deferred-db-pool-gauges-design-kept-for-the-next-version)).
- **Candidates, not designed:** Prometheus pull, runtime metrics, histogram
  views and buckets, delta temporality
  ([follow-ups](docs/superpowers/specs/2026-08-30-otel-metrics-design.md#out-of-scope-explicit-follow-ups)).

## Config cache by CDC

A config cache fed by change data capture. `pkg/types/orm` — `Record`,
`Referenced`, `Reference` — is kept for it, unused until then. Not designed
yet.
