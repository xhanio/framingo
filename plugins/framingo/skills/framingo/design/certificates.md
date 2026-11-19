# Certificates: who holds the key, which CA signs

## A requester keeps its private key

**Decision.** An agent sends a CSR to `POST /certificates/csr`; the manager
signs it and answers with the certificate, never a key, and stores it
without one. The request — `common_name`, `subject_alt_name`, `usage` —
decides who the certificate is for: a CSR naming anything else is refused
(400), and its signature proves the sender holds the key. A signed
certificate lasts 90 days unless the request says otherwise.

**Why.** A key the manager generates and sends is one more copy to leak — in
the response, the database, a log. And the CA signs what the request grants,
not what the CSR asks, so a CSR cannot widen its own identity.

## One CA per purpose

**Decision.** Two independent self-signed CAs, `preset.ServerCAName` and
`preset.AgentCAName`. The server CA signs the manager's own certificates —
`Issue` without a `ca_id`; agents verify the manager against it. The agent
CA signs every CSR, and the manager verifies agents against it alone
(`authnagent`).

**Why.** With one CA, whoever may sign a CSR — `usage: server`, the
manager's hostname — holds a certificate every agent takes for the manager.
Split, a CSR's certificate chains only to the agent CA, which no agent
verifies the manager against. Each can also be rotated, or lost, without the
other.

## CA keys live in the database alone

**Decision.** `Init` loads both CAs, creating them on first start; they are
certificate records that `List` leaves out and `Delete` refuses. Nothing is
written to disk. What verifies against them takes their certificates:
`GET /certificates/ca` (`certificate_read`) returns both, and the CSR
response carries `server_ca` for the agent to trust the manager by.

**Why.** A key file beside the config is a copy outside the database's
access control, one `git add` from a commit under `env/`. Whoever verifies —
nginx's `ssl_client_certificate`, an agent's trust store — needs the
certificate, not the key. The cost: the keys are as safe as the database; a
production CA keeps its root offline and its keys in a KMS or HSM.

## Not yet

- Trust is one certificate per CA, so a CA cannot be rotated with an
  overlap; a pool of trusted CAs comes first.
- No hierarchy: an offline root signing the two as intermediates.
- No enrollment token, renewal, or revocation, and a certificate is not
  bound to an agent record. `authnagent` is not registered: it checks only
  the signature — not expiry nor key usage — and its header path, for a
  proxy in front, carries no proof of possession.
