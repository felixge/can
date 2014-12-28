package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/felixge/gkv"
)

func main() {
	if err := realMain(); err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}
}

func realMain() error {
	var (
		dir = flag.String("dir", "./gkv", "Directory for file backend.")
	)
	flag.Parse()
	backend := gkv.NewFileBackend(*dir)
	repo := gkv.NewRepo(backend)
	switch cmd := flag.Arg(0); cmd {
	case "pull":
		remote := gkv.NewRepo(gkv.NewFileBackend(flag.Arg(1)))
		return cmdPull(repo, remote)
	case "clone":
		remote := gkv.NewRepo(gkv.NewFileBackend(flag.Arg(1)))
		return cmdClone(repo, remote)
	case "show":
		return cmdShow(repo, flag.Arg(1))
	case "ls":
		return cmdLs(repo)
	case "log":
		return cmdLog(repo)
	case "rm":
		return cmdRm(repo, flag.Arg(1))
	case "get":
		return cmdGet(repo, flag.Arg(1))
	case "set":
		return cmdSet(repo, flag.Arg(1), flag.Arg(2))
	default:
		return fmt.Errorf("unknown cmd: %s", cmd)
	}
}

func cmdPull(local, remote *gkv.Repo) error {
	//head, err := remote.Head()
	//if err != nil {
	//return err
	//}
	//for {
	//remoteCommit, err := remote.Commit(head)
	//if err != nil {
	//return err
	//}
	//localCommit, err := local.Commit(remoteCommit.ID())
	//if err == nil {
	//break
	//} else if !gkv.IsNotExist(err) {
	//return err
	//}
	//}
	return nil
}

func cmdClone(local, remote *gkv.Repo) error {
	if _, err := local.Head(); err == nil {
		return fmt.Errorf("refusing to clone into existing repo")
	} else if !gkv.IsNotExist(err) {
		return err
	}
	head, err := remote.Head()
	if err != nil {
		return err
	}
	remoteHead := head
	for {
		commit, err := remote.Commit(head)
		if err != nil {
			return err
		} else if err := local.Save(commit); err != nil {
			return err
		}
		index, err := remote.Index(commit.Index())
		if err != nil {
			return err
		} else if err := local.Save(index); err != nil {
			return err
		}
		for _, blobID := range index.Entries() {
			if blobID == gkv.NilID {
				continue
			} else if blob, err := remote.Blob(blobID); err != nil {
				return err
			} else if err := local.Save(blob); err != nil {
				return err
			}
		}
		head = commit.Parent()
		if head == gkv.NilID {
			return local.SetHead(remoteHead)
		}
	}
}

func cmdLs(repo *gkv.Repo) error {
	head, err := repo.Head()
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for {
		commit, err := repo.Commit(head)
		if err != nil {
			return err
		}
		index, err := repo.Index(commit.Index())
		if err != nil {
			return err
		}
		for key, blobID := range index.Entries() {
			if known[key] {
				continue
			}
			if blobID != gkv.NilID {
				fmt.Printf("%s %s\n", blobID, key)
			}
			known[key] = true
		}
		head = commit.Parent()
		if head == gkv.NilID {
			return nil
		}
	}
}

func cmdShow(repo *gkv.Repo, id string) error {
	pID, err := gkv.ParseId(id)
	if err != nil {
		return err
	}
	obj, err := repo.Load(pID)
	if err != nil {
		return err
	}
	fmt.Printf("%s", obj.Raw())
	return nil
}

func cmdLog(repo *gkv.Repo) error {
	head, err := repo.Head()
	if err != nil {
		return err
	}
	for {
		fmt.Printf("commit %s\n", head)
		commit, err := repo.Commit(head)
		if err != nil {
			return err
		}
		fmt.Printf("time %s\n", commit.Time())
		fmt.Printf("index %s\n", commit.Index())
		fmt.Printf("parent %s\n\n", commit.Parent())
		index, err := repo.Index(commit.Index())
		if err != nil {
			return err
		}
		for key, blobID := range index.Entries() {
			fmt.Printf("  %s %s\n", blobID, key)
		}
		heads := commit.Parents()
		if len(heads) == 0 {
			return nil
		}
		head = heads[0]
		fmt.Printf("\n")
	}
}

func cmdRm(repo *gkv.Repo, key string) error {
	index := gkv.NewIndex(map[string]gkv.ID{key: gkv.NilID})
	if err := repo.Save(index); err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil && !gkv.IsNotExist(err) {
		return err
	}
	commit := gkv.NewCommit(time.Now(), index.ID(), head)
	if err := repo.Save(commit); err != nil {
		return err
	}
	return repo.SetHead(commit.ID())
}

func cmdGet(repo *gkv.Repo, key string) error {
	head, err := repo.Head()
	if err != nil {
		return err
	}
	for {
		commit, err := repo.Commit(head)
		if err != nil {
			return err
		}
		index, err := repo.Index(commit.Index())
		if err != nil {
			return err
		}
		for indexKey, blobID := range index.Entries() {
			if indexKey != key {
				continue
			}
			blob, err := repo.Blob(blobID)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", blob.Val())
			return nil
		}
		head = commit.Parent()
		if head == gkv.NilID {
			return fmt.Errorf("key not found: %s", key)
		}
	}
}

func cmdSet(repo *gkv.Repo, key, val string) error {
	blob := gkv.NewBlob([]byte(val))
	if err := repo.Save(blob); err != nil {
		return err
	}
	index := gkv.NewIndex(map[string]gkv.ID{key: blob.ID()})
	if err := repo.Save(index); err != nil {
		return err
	}
	var heads []gkv.ID
	head, err := repo.Head()
	if err == nil {
		heads = []gkv.ID{head}
	} else if !gkv.IsNotExist(err) {
		return err
	}
	commit := gkv.NewCommit(time.Now(), index.ID(), heads...)
	if err := repo.Save(commit); err != nil {
		return err
	}
	if err := repo.SetHead(commit.ID()); err != nil {
		return err
	}
	fmt.Printf("%s\n", commit.ID())
	return nil
}
