# Configuration: read in Init, reloaded whole

## Config is read in Init, from the context

**Decision.** A service's constructor takes its dependencies and options;
the configuration it can reload it reads in `Init`, from the
`confutil.Config` the supervisor puts in `Init`'s context
(`confutil.FromContext(ctx)`), under the root its name gives
(`nameutil.ConfigRoot(m.name)`). What `Init` reads replaces what the options
set, whole: a key the config leaves unset falls back to the default, not to
the constructed value.

**Why.** `Init` runs at start and again on every restart, so one path serves
both: a restart is how a changed configuration takes effect, without
rebuilding the service. Options stay for what the wiring fixes and tests set.

**Cost.** The keys a service reads are not in its constructor's signature;
its doc names them (`db.connection.*` in `pkgs/db.md`).

## A reload is seen whole

**Decision.** What one `Init` reads it publishes whole, so a reader sees the
old configuration or the new, never a mix: under the one lock its readers
take — the example service's greeting, the tracing and metrics managers'
settings — or inside the immutable value a restart swaps in, as
`db.Manager`'s exec timeout travels with its pool.

**Why.** A service restarted alone — by the monitor, or by
`RestartService` — runs its `Init` while its dependents go on serving, so
requests are in flight; SIGHUP's restart stops everything first, but a stop
that passes its deadline leaves handlers behind. Fields rewritten one by one
under a reader are a data race, and a reader caught between two of them sees
a configuration no file ever said.
