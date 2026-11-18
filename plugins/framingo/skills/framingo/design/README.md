# Design Decisions

The reasoning behind framingo's and the example's shapes: each file is one
area the project met design questions in — what was chosen, why, and what
it costs. The `app/` and `pkgs/` references say *how* things are done;
these say *why*, so a change can tell a principle from an accident.

| File | In short |
|---|---|
| [who-decides.md](who-decides.md) | The route decides who may (path + permission, authz fails closed); the service keeps the rules about its own data and does not ask who calls; the handler orchestrates checks that span two services |
| [narrow-contracts.md](narrow-contracts.md) | Every dependency is the smallest interface: a service's own `Repository`, `common.RawMessageSender` over the bus, message handlers as `Named` |
| [data-access.md](data-access.md) | orm types never leave the repository; `types/repo` carries the contracts and repo-only inputs; the repository decides nothing |
| [service-wiring.md](service-wiring.md) | Platform first in every `New`; `Dependencies` lists nil as is; one instance per name; the api server starts last by ordering |
| [websockets.md](websockets.md) | `wsutil.Bridge` as a session utility; `wsutil.Close` by the handler's result; the server closes its connections on stop |
| [migrations.md](migrations.md) | Locally the daemon migrates on start; in prod a Job runs `exampleapp migrate` once per release; every key overridable from the environment |

When a design question is settled, add it to the file of its area — or a
new file when no area fits — and a row above.
