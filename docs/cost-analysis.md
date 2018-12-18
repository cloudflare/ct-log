Cost Analysis
-------------

#### Traffic Model

This section establishes a basic model of a CT log's utilization.

All the Nimbus shards combined receive traffic on the order of:
- **add-chain** - 2 req/s
- **add-pre-chain** - 10 req/s
- **get-entries** - 10 req/s

All other endpoints can be assumed to receive 0 req/s.

A general Trillian-based CT log needs the following datastores:
- **Root Store:** Stores the most recent STH. Small and constant-size.
- **Subtree Cache:** Stores reduced data about the Merkle tree that helps
  quickly compute proofs. 64 bytes per log entry; requires versioned random
  access ("get key X with version <= y").
- **Leaf Index:** Maps a leaf's Merkle hash and Identity hash to the index of
  the leaf in the log. 80 bytes per log entry; randomly accessed.
- **Leaf Store:** Stores raw, ordered leaf data. 6 kilobytes per log entry;
  random access and streaming in-order access; data is immutable once stored.
- **Leaf Queue:** Stores unsequenced leaves from add-(pre-)chain requests. May
  be several megabytes but is functionally constant-size; this is the only
  datastore which is written to by multiple processes at once.

One request to each endpoint has the following impact on the underlying datastores:
- **get-entries**
  - 1 read from the *Root Store* to verify request bounds.
  - 1 large sequential read from the *Leaf Store* to get the data the user
    requested.
- **add-chain** / **add-pre-chain**
  - 1 read from the *Leaf Index* to detect if the certificate is a duplicate.
  - 1 write to the *Leaf Queue* to persist the user's certificate.

And sequencing, which is assumed to happen every 300s, has the following effect
on the underlying datastores:
- 1 read, 1 write to the *Root Store* to update the STH.
- 30 writes to the *Subtree Cache* to update the Merkle tree.
- 7200 random-access writes to the *Leaf Index* to index by Merkle and Identity
  hash.
- 1 large sequential write to the *Leaf Store* to sequence the leaves.
- 3600 random-access reads, 3600 random-access deletes from the *Leaf Queue* to
  read and remove unsequenced leaves.

(Where the numbers come from: There are 2 *add-chain* req/s plus 10
*add-pre-chain* req/s, so 12 certs/s total, 3600 certs = 12 certs/s &times;
300s, 7200 hashes = 3600 certs &times; 2 hashes/cert)

Datastore utilization can then be estimated as follows, assuming an existing
corpus of 250,000,000 certificates:
- **Root Store**
  - 1 kb
  - 10 read/s ; 10 kb/s
  - 0 write/s
- **Subtree Cache**
  - 16 gb
  - 0 read/s ; 0 kb/s
  - 0 write/s
- **Leaf Index**
  - 20 gb
  - 12 read/s ; 0 kb/s
  - 24 write/s
- **Leaf Store**
  - 1.5 tb
  - 10 read/s ; 6 mb/s
  - 0 write/s
- **Leaf Queue**
  - 5 mb
  - 12 read/s ; 72 kb/s
  - 24 write/s

The egress bandwidth that was predicted for the *Leaf Store* was significantly
off, so the actual number was used instead. It would've predicted 61 mb/s = 1024
entries/req &times; 6 kb/entry &times; 10 req/s. It was likely off because of
monitors that ask for less than 1024 entries at once.


#### Cloud Services

This section discusses the different cloud services that can be used, and
describes their pricing.

There are three ways that I'm aware of to store data in the cloud:
1. **A managed database** like MySQL or Spanner. These seem quite expensive and
   unnecessary, so will not be considered further.
2. **Block storage** like AWS EBS. This is high-speed, disk-like storage. You
   pay a premium for the amount of storage *allocated* and do not pay for
   ingress/egress. Ideal for small amounts of data accessed frequently.
3. **Object storage** like AWS S3. This is low-speed, KV-like storage. You pay
   a low value for the amount of storage *consumed* and typically pay a high
   amount for egress. Ideal for large amounts of data with simple access
   patterns.

DigitalOcean offers block storage in addition to VMs:

| Service                 | Billing                         |
|-------------------------|---------------------------------|
| Minimal virtual machine | $5/month                        |
| Download bandwidth      | $0.01/gb/month; first 1 tb free |
| Block storage allocated | $0.10/gb/month                  |

Backblaze has an object storage service called B2:

| Service            | Billing                                               |
|--------------------|-------------------------------------------------------|
| Storage consumed   | $0.005/gb/month; first 10 gb free                     |
| Download bandwidth | $0.01/gb/month; first 1 gb per day free               |
| Download API call  | $0.004 per 10k req per month; first 2.5k per day free |

Additionally, B2 waives the cost of download bandwidth if served through
Cloudflare's edge but not the cost of the API call, if I understand correctly.

Cloudflare's Worker service can help take advantage of the fact that B2 will
waive the cost of egress:

| Service         | Billing                                          |
|-----------------|--------------------------------------------------|
| Activation cost | $5/month                                         |
| Requests        | $0.50 per 1M reqs per month; first 10M reqs free |


#### Architecture Decisions

This section attempts to justify the log's architecture, in terms of price and
performance.

**Which datastores should use block storage, and which should use object
storage?**

- **Root Store** Block storage. This is an arbitrary decision.
- **Subtree Cache** Block storage. This is an arbitrary decision.
- **Leaf Index** Block storage.
  - Block storage: We pay for 20 gb of block storage, so $2/month.
  - Object storage: We pay for 20 gb of object storage, 12 API calls per second, so $12/month.
- **Leaf Store** Object storage.
  - Block storage: We pay for 1.5 tb of block storage, so $150/month.
  - Object storage: We pay for 1.5 tb of object storage, 10 API calls per second, so $18/month.
- **Leaf Queue** Block storage. Writing to object storage can take several
  seconds, which means that every *add-(pre-)chain* request would take several
  seconds, which is unacceptable.

**Should get-entries be implemented as a Cloudflare Worker?**

Yes.

Without the Worker, our DigitalOcean VM has to receive a get-entries request,
download it from B2, and then upload it to Cloudflare. Paying DigitalOcean for
the bandwidth alone would cost $146/month.

With the Worker, get-entries requests never go to our VM and we pay $13/month to
Cloudflare.


#### Total Cost of Operation

DigitalOcean:

| Service                 | Billing |
|-------------------------|--------:|
| Minimal virtual machine |      $5 |
| Download bandwidth      |      $0 |
| Block storage allocated |      $5 |

Backblaze:

| Service            | Billing |
|--------------------|--------:|
| Storage consumed   |      $7 |
| Download bandwidth |      $0 |
| Download API call  |     $12 |

Cloudflare:

| Service         | Billing |
|-----------------|--------:|
| Activation cost |      $5 |
| Requests        |      $8 |

Total cost: $42/month
