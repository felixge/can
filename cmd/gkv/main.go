package main

import (
	"fmt"
	"log"
	"time"

	"github.com/felixge/gkv"
)

func main() {
	log.SetFlags(0)
	g := gkv.NewGKV(gkv.NewFileBackend("gkv"))
	blobs := make([]*gkv.Blob, 10)
	entries := map[string]gkv.ID{}
	for i := 0; i < len(blobs); i++ {
		blob := gkv.NewBlob([]byte(fmt.Sprintf("val-%d", i)))
		if err := g.Save(blob); err != nil {
			log.Fatal(err)
		}
		blobs[i] = blob
		entries[fmt.Sprintf("key-%d", i)] = blob.ID()
	}
	index := gkv.NewIndex(entries)
	if err := g.Save(index); err != nil {
		log.Fatal(err)
	}
	commit := gkv.NewCommit(time.Now(), index.ID())
	if err := g.Save(commit); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s %s", commit.ID(), commit.Raw())
}
