**Work in progress:** Nothing to see here yet. This is highly experimental.

# GKV

GKV is a Git inspired low-level key value store.

## Comparison to similar projects

The main goal of GKV is to create a simple low-level database suitable for
offline usage across many devices with good support for syncing and conflict
resolution supporting up to 1 million records.

### Git

Each git commit references a full tree object. This works fine for git, as most
repositories are laid out in a hierarchical manner, causing the top level tree
object to be usually small. A key value store however may have hundreds of
thousands of top level keys without any hierarchies. For this reason GKV
commits reference partial indexes, which include only the key value pairs that
have changed. This greatly reduces the costs for writing and syncing data, but
introduces a O(N) cost for naive key lookups. However, this can be turned into
a one time costs by keeping a cache of the current index.

Additionally git includes many advanced optimizations such as pack files which
makes it non-trivial to create native clients.

### CouchDB

To be written ...

## Objects

GKV uses the following object types for storing data:

* blob: Stores raw values.
* index: Maps key strings to value hashes. Similar to trees in git.
* commit: References previous commits and indexes.

The basic object format is given in ABNF with the following recurring rules:

```
number    = 1*DIGIT
binary    = *%x00-ff
id        = 20(DIGIT / "a" / "b" / "c" / "d" / "e" / "f")
time      = timestamp " " offset ; unix UTC timestamp in seconds, followed by zone offset in seconds
timestamp = number
offset    = ( "+" / "-" ) number
```

### Blob

ABNF:

```
blob   = "blob " size "\n" value "\n"
size   = number
value  = binary
```

Example:

```
"blob 12\nHello World\n"
```

### Index

ABNF:

```
index    = "index " size "\n" 1*(keysize " " key " " blob_id "\n")
size     = number
keysize  = number
key      = binary
blob_id  = id
```

Keys must be sorted in ascending byte order.

Example:

```
"index 94\n3 bar 0a4d55a8d778e5022fab701977c5d840bbc486d0\n3 foo 13a6151685371cc7f1a1b7d2dca999092938e493\n"
```

### Commit

ABNF:

```
commit      = "commit" size "\n"
              "time " commit_time "\n"
              "index " index_id "\n"
              1*("parent " commit_id "\n")
size        = number
commit_time = time
index_id    = id
commit_id   = id
```

Example:

```
"commit 165\ntime 1418327450 -3600\nindex c82a9efd857f436e0ececd7986cb8611b6b8f84e\nparent 119be3a4d2e8eef6fbf1e86d817fe58a452cf429\nparent b176e7d983ca7129334dde3779e6f155b3399351\n"
```
