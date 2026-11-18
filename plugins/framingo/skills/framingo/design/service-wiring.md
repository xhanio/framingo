# Wiring services

## The platform comes first

**Decision.** Every example service's `New` takes the platform first —
`New(platform platform.Platform, ...)` — whether it uses it or not
(`_ platform.Platform` in `newManager` then). The platform bundles the
framework plumbing a service may reach: database, meter, pubsub, messagebus.

**Why.** One shape for every constructor; a service that later needs the
meter or the bus takes it from what it already holds, and no call site
changes.

## Dependencies lists what it holds, nil included

**Decision.** `Dependencies()` returns the services the manager holds, as
they are: `[]common.Service{m.repository, m.mb}`, the auth service's
optional LDAP and API-token sources too. An optional dependency the app did
not wire is nil, and the supervisor skips a nil entry everywhere it walks
`Dependencies` — registering, the init gate, the health monitor.

**Why.** A filter in every `Dependencies()` (type assertions, `if x != nil`)
was boilerplate that could be forgotten. Listing an optional source also
orders it: a wired LDAP source is initialized and started before auth,
which it was not while left out.

## A name stands for one instance

**Decision.** `Supervisor.Register` returns a `Conflict` for a second
instance under a name already taken; the same instance again is a no-op.
Registration is wiring, so the error fails startup.

**Why.** Stats, the dependency graph and by-name operations all key on the
name; a second instance under it would never run.

## The api server starts last by ordering, not by depending

**Decision.** The api server declares no dependencies; the wiring registers
it after `TopoSort`, so it starts after every service.

**Why.** A dependency gates `Init` and rolls health up: depending on the
services would keep the listener — the health listener with it — down
whenever one of them failed, while `/healthz` and `/readyz` should report
exactly that.
