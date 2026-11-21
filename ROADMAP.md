# Roadmap

What framingo and its example are to do next. Within an area the items are
in the order planned; the areas are independent of each other. An item says
what it is and where it stands, and links to the design note that holds the
reasoning. A finished item is deleted — git history keeps it — and its
design note brought up to date.

## Checks

The lint config holds the import boundaries and the forbidden calls,
`server.CheckRouter` a router's config against its handlers, and
`framingo verify` gives one verdict on a change
([design/checks.md](plugins/framingo/skills/framingo/design/checks.md)).
Next, in order:

- **Analyzers, in `pkg/tools/`.** Run by `framingo verify` and `framingo
  vet`, and loadable as `go vet -vettool` and as a golangci-lint plugin, for
  apps as for the framework. First `startctx` (a
  `Start` keeping its context, or deriving from it without `WithoutCancel`)
  and `depslist` (a held `common.Service` missing from `Dependencies`); then
  `platformfirst`, `gormfield` (a `*gorm.DB` kept in a field outside db) and
  `confinit` (`confutil.FromContext` outside `Init`). A candidate after them:
  the keys a service reads in `Init` against the keys its config file sets,
  since a misspelled key lands silently on the default.
- **The docs and a release, verified.** A doc's snippet against the function
  it copies, every markdown link and anchor, the four version surfaces
  agreeing — what a doc review and a release check by hand today.
- **Candidates, not designed:** the architecture as a graph a tool can query
  — packages, routes, the supervisor's dependencies — and what a change to
  one contract reaches; a set of tasks to measure how well a coding agent
  works in the framework.

## Sessions in a shared store

The example keeps its sessions in one process's memory: a second replica
would not know them, and a user's sessions end only when an event says so.

- **A shared store.** Sessions in Redis or the database, so any replica
  serves any session.
- **Revocation that cannot be lost.** A user's sessions end on
  `LocalUsersDeleted`, `LocalUserPasswordReset` and
  `LocalUserIdentityChanged`, published after the commit. One lost — a full
  subscriber queue drops it by default — leaves the session alive, and every
  refresh extends it. A store alone does not fix that, since ending the
  sessions is still a second write after the commit: either the store is the
  database and the user's change ends its sessions in the same transaction,
  or a session is checked on use against a version the change bumps.

## Events: what the bus may lose

- **Delivery, said per driver.** `pubsub.md` says an in-process bus is no
  guarantee; it does not say, driver by driver, when a message is lost and
  what a consumer must do about it. Known so far, of the kafka driver: an
  instance reads from the latest offset once its own group holds the
  partition, so what is published while it starts — half a minute on a new
  cluster — never reaches it; the first write to a topic not yet created
  fails as it creates it, the caller getting the error; and each instance
  leaves its group on the broker until the broker's retention drops it. The
  driver creating its topic at `Start` would end the failed first write, not
  the gap while an instance starts: that is `LastOffset`, chosen so that a new
  instance replays no history, and the design weighs the two together.
- **Candidate, not designed:** a transactional outbox for events that must
  follow a commit, perhaps on the change capture the config cache needs.

## Routers

- **Candidate, not designed:** OpenAPI generated from `router.yaml` and the
  api types.

## Toward 1.0

- **What 1.0 keeps.** Until 1.0 a change is free to break whatever a better
  design needs; 1.0 names the surface that stays from then on.
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
