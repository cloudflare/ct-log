Architecture
------------

Normal Trillian-based Certificate Transparency (CT) logs require three services:
1. A CT personality frontend, that implements CT-specific APIs by querying a
   backend service called a Trillian server.
2. A Trillian server, that connects to a datastore to answer user queries and
   queue new leaves for addition to the tree.
3. A Trillian signer, that (in the background) reads queued leaves from the
   (possibly many) Trillian servers, orders them, and integrates them into the
   next Signed Tree Head for the log.

This log has all three of those components, but they are glued together into the
same logical process, and there can only be one of these processes running at a
time. The reason for this restriction is the choice of database:

Rather than storing everything in a SQL-based database, this log uses two
different datastores: a local (on-disk) KV store like LevelDB, and an object
storage provider like AWS S3. Metadata, indices, and unsequenced leaves are
stored in the local database, while all of the log's certificates are kept in
object storage.

The log service is not meant to be exposed to the internet directly, it is meant
to be accessed through Cloudflare's edge. This will heavily cache the responses
to common queries like *get-sth* and *get-roots*, reducing load on the server.
The server also does not answer *get-entries* requests -- this is done by a
Cloudflare Worker. The Worker fetches directly from the object storage provider
and re-formats the internal format of data into the format expected by the user.

Motivation for these decisions are provided in the Cost Analysis write-up.
