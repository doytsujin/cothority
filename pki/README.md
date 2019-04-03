Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) :: PKI

# Public Key Infrastructure

This service has the sole purpose of providing an API to get proofs that public
keys hold by the conodes have a known pair secret key. In the context of the
BLS CoSi scheme, a rogue public-key could be use to forge a correct signature
using a single malicious node registered with a very specific public key.

Even if the usage is very specific, it has been implemented as a service open
to external requests so that it is possible for a client to ask for the Proofs
of Possession of a given conode. It's not harmful because the API will only
sign the public key appended to a nonce, thus avoiding to open an oracle.

Note that the main key pair is authentified during the TLS handshake and is then
not part of the proofs but only the services registered with a given suite
because those generate a key pair. A request will then contain multiple proofs
for a single conode.

## Storage

Because the only purpose of a Proof of Possession is to show that the private
is known, it can be stored and reused indefinitely. The service then stores the
requests so that it can provide future ones more efficiently.

The database is used for that goal. One bucket contains the key/value pairs
where the key is the public key marshalled into bytes and the value is the proof
itself (Public key, Nonce, Signature).

## Suites

Each suite has its own sign and verify algorithms. Two suites are supported:
- Ed25519
- BN256

If it happens that a service needs a different suite, it should be added in that
service so that the new kind of key pair will be verified.

## Attacks on Proof of Possession

This section describes some known attacks that need to be known so that we don't
open an API that could allow an attacker to forge a Proof of Possession.

### BLS

[This paper](https://link.springer.com/content/pdf/10.1007%2F978-3-540-72540-4.pdf)
(page 251; 4.3) contains the description of an attack on the Proof of Possession.
Basically if an attacker happens to be forge a signature of its public key
(with the nonce) using the private key of the targeted honest peer, it could
make a Proof of Possession of its public key. Fortunately we don't have any
oracle open.