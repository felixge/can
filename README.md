**Work in progress:** Nothing to see here yet. This is highly experimental.

# Can - The airtight database for your personal data!

Can is a a key value store suitable for small data (up to 1 million documents),
focused on portability, fast syncronization, reliability and encryption.

It is based on Git's data model, but does not implement complex optimizations
such as packfiles which would reduce portability and are of limited use for
encrypted repositories.

## Objects

Can implements the following data types from Git:

* blob: Stores raw values.
* tree: Maps key strings to blob or tree ids.
* commit: References previous commits and trees.

The canonical data format is given in ABNF with the following recurring rules:

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
blob   = "blob \n" value
value  = binary
```

Example:

```
blob
Hello World
```

### Tree

ABNF:

```
index    = "tree\n" 1*(kind " " id " " keysize " " key)
kind     = ( "tree" / "blob" )
keysize  = number
key      = binary
```

Keys must be sorted in ascending byte order.

Example:

```
tree
blob 0a4d55a8d778e5022fab701977c5d840bbc486d0 2 hi
tree 13a6151685371cc7f1a1b7d2dca999092938e493 12 how are you?
```

### Commit

ABNF:

```
commit     = "commit\n" "tree " tree_id "\n" 1*("parent " parent_id "\n") "time " time "\n" "\n" message
tree_id   = id
parent_id = id
message   = binary
```

Example:

```
commit
tree c82a9efd857f436e0ececd7986cb8611b6b8f84e
parent 119be3a4d2e8eef6fbf1e86d817fe58a452cf429
parent b176e7d983ca7129334dde3779e6f155b3399351
time 1424434473 +3600

hi,\n\nhow are you?
```
