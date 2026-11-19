# Roadmap

What framingo and its example are to do next. Within an area the items are
in the order planned; the areas are independent of each other. An item says
what it is and where it stands, and links to the design note that holds the
reasoning. A finished item is deleted — git history keeps it — and its
design note brought up to date.

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
