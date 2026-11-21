# WebSockets: the bridge, closing, and stopping

## The bus bridge is a utility, not a messagebus method

**Decision.** `wsutil.Bridge(conn, messenger, opts...)` joins a WebSocket
connection to a bus `Messenger` both ways until the connection closes. It
lives in `pkg/utils/wsutil`, not on `model.MessageBus`. A `bridge` is one
connection's session — its two ends, its keepalive (`WithPing`), its
context — and the pumps hang on it.

**Why.** It needs a `Messenger` and nothing of the bus, so the bus no
longer imports `coder/websocket`, and the ping settings belong to the bridge
that pings, not to a bus that never does. The name says what it does: it
bridges two transports, both ways, blocking like `io.Copy`.

## A session that loses its subscription ends

**Decision.** When the messenger's channel closes under a running session —
the bus evicted a subscriber that stopped draining, or pubsub stopped — the
bridge ends the session with `StatusTryAgainLater`, "subscription dropped",
and returns `ErrSubscriptionDropped`. A write that fails ends it too.

**Why.** Before, the outbound pump returned alone: the connection stayed
open and heard nothing, and a client that sees an open connection has no
reason to reconnect.

## Close by what the handler returned

**Decision.** `wsutil.Close(ctx, conn, err)` ends a connection once its
handler returned: a normal closure for nil; nothing for the peer's own close;
released with nothing sent when the peer dropped it (EOF); going away once
`ctx` ended; the status and reason of a `wsutil.CloseWith(status, reason)` —
a refusal, a displacement — reporting no failure; otherwise an internal
error carrying `err`, the one case it returns, for the caller to log. A
reason is cut to the 123 bytes a close frame carries.

The connection is the server's: a handler returns, it does not close. The
one exception is the bridge ending a session itself — a dropped
subscription — which sends that close before it returns it.

**Why.** A handler that closes the connection itself bypasses the one
place closes are decided, and before `CloseWith` it had to, to send any
status but a normal or an internal one. The bridge closes first because it
must stop its own read: canceling the context a read waits on tears the
connection down with no close frame, and the peer would see a drop, not
the reason. coder/websocket drops a reason over the limit whole, its status
with it, so a long error used to reach the client as a bare internal
error.

## The server closes its connections when it stops

**Decision.** net/http's `Shutdown` neither closes nor waits for a hijacked
connection, so the api server does: each echo gets a context its WebSocket
handlers run under; `stop` ends it as it begins — `context.AfterFunc`
closes each connection going away and an upgrade from then on is refused —
then waits for the handlers until its own context ends. That context is also
what `wsutil.Close` is handed: the request's never ends for a hijacked
connection.

**Why.** Otherwise a stop left every handler running — a bridge's messenger
still subscribed to the bus — into the start after it. The wait group spans
starts, as the framework's daemons share one: a handler a stop gave up on
is still waited for by the next.
