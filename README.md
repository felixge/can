**Work in progress:** Nothing to see here yet.

# GKV

GKV is a git inspired key value store.

## Motivation

The main goal is to provide a simple subset of git that is suitable as a key
value store for distributed applications with small databases (i.e. less than 1
million records). The prime example of this are multi-device applications with
offline support that use 1 database per user.

## Objects

GKV uses the following object types for storing data:

* blob: Stores raw values.
* index: Maps key strings to value hashes. Similar to trees in git.
* commit: References previous commits and indexes.

The basic object format is described using ABNF notation with the following
reocuring rules:

```
number = 1*DIGIT
binary = *%x00-ff
hash   = 1*(DIGIT / "a" / "b" / "c" / "d" / "e" / "f")
```

### Blob

ABNF:

```
blob   = "blob " size %x00 value
size   = number
value  = binary
```

Example:

```
"blob 11\0Hello world
```

### Index

ABNF:

```
index   = "index " size %x00 1*(keysize %x00 key valref "\n")
number  = 1*DIGIT
size    = number
keysize = number
key     = binary
valref  = hash
```

Example:

```
"index 9\03\0foobar\n"
```
