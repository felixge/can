package main

import (
	"fmt"
	"log"
	"time"

	"github.com/felixge/gkv"
)

func main() {
	log.SetFlags(0)
	rp := gkv.NewRepository(gkv.NewFileBackend("gkv"))
	blobs := make([]*gkv.Blob, 10)
	entries := map[string]gkv.ID{}
	for i := 0; i < len(blobs); i++ {
		blob := gkv.NewBlob([]byte(fmt.Sprintf("val-%d", i)))
		if err := rp.Save(blob); err != nil {
			log.Fatal(err)
		}
		blobs[i] = blob
		entries[fmt.Sprintf("key-%d", i)] = blob.ID()
	}
	index := gkv.NewIndex(entries)
	if err := rp.Save(index); err != nil {
		log.Fatal(err)
	}
	commit := gkv.NewCommit(time.Now(), index.ID())
	if err := rp.Save(commit); err != nil {
		log.Fatal(err)
	}
	if err := rp.SetHead(commit.ID()); err != nil {
		log.Fatal(err)
	}
	{
		head, err := rp.Head()
		if err != nil {
			log.Fatal(err)
		}
		commit, err := rp.Commit(head)
		if err != nil {
			log.Fatal(err)
		}
		index, err := rp.Index(commit.Index())
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%s", index.Raw())
	}
}
