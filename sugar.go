package can

import (
	"errors"
	"fmt"
	"io"
)

func NewSugar(rp Repo) Sugar {
	return &sugar{Repo: rp}
}

type Sugar interface {
	Repo
	HeadCommit() (Commit, error)
	Keys(treeID ID, prefix []string) (KeyIterator, error)
	Get(key []string) (io.ReadCloser, error)
	Set(treeID ID, key []string, blob io.Reader) (ID, error)
}

type sugar struct {
	Repo
}

// HeadCommit returns the head commit, or an error.
func (s *sugar) HeadCommit() (Commit, error) {
	if id, err := s.Head(); err != nil {
		return Commit{}, err
	} else {
		return s.Commit(id)
	}
}

func (s *sugar) Keys(treeID ID, prefix []string) (KeyIterator, error) {
	var (
		tree Tree
		err  error
	)
	for _, name := range prefix {
		if tree, err = s.Tree(treeID); err != nil {
			return nil, err
		} else if entry := tree.Get(name); entry == nil {
			return nil, notFoundError(fmt.Sprintf("entry %q not found for prefix: %#v", name, prefix))
		} else if entry.Kind != KindTree {
			return nil, notFoundError(fmt.Sprintf("entry %q is %s for prefix: %#v", name, entry.Kind, prefix))
		} else {
			treeID = entry.ID
		}
	}
	return &keyIterator{key: prefix, rp: s.Repo, stack: []Tree{tree}}, nil
}

type KeyIterator interface {
	Next() ([]string, ID, error)
}

type keyIterator struct {
	key   []string
	rp    Repo
	stack []Tree
}

func (k *keyIterator) Next() ([]string, ID, error) {
	for {
		if len(k.stack) == 0 {
			return nil, nil, io.EOF
		} else if tree := k.stack[len(k.stack)-1]; len(tree) == 0 {
			k.stack = k.stack[:len(k.stack)-1]
			if len(k.stack) == 0 {
				continue
			}
			k.stack[len(k.stack)-1] = k.stack[len(k.stack)-1][1:]
			k.key = k.key[:len(k.key)-1]
		} else if entry := tree[0]; entry.Kind == KindTree {
			if tree, err := k.rp.Tree(entry.ID); err != nil {
				return nil, nil, err
			} else {
				k.stack = append(k.stack, tree)
				k.key = append(k.key, entry.Name)
			}
		} else if entry.Kind == KindBlob {
			k.stack[len(k.stack)-1] = tree[1:]
			return append(k.key, entry.Name), entry.ID, nil
		} else {
			return nil, nil, fmt.Errorf("corrupt tree: %s", entry.ID)
		}
	}
}

// Get returns a read closer for the Blob with the given key.
func (s *sugar) Get(key []string) (io.ReadCloser, error) {
	head, err := s.Head()
	if err != nil {
		return nil, err
	}
	commit, err := s.Commit(head)
	if err != nil {
		return nil, err
	}
	treeID := commit.Tree
	for i, k := range key {
		tree, err := s.Tree(treeID)
		if err != nil {
			return nil, err
		}
		if entry := tree.Get(k); entry == nil {
			return nil, notFoundError(fmt.Sprintf("entry for %q not found for key %#v", k, key))
		} else if i == len(key)-1 {
			return s.Blob(entry.ID)
		} else {
			treeID = entry.ID
		}
	}
	panic("unreachable")
}

// Set commits the given key and blob value using the given commit details and
// returns the ID of the new head. It's ok for the underlaying repo to not have
// a head prior to calling Set. Set may return neither ID nor error, which
// means that no commit was created because the repo already had the desired
// key value pair.
func (s *sugar) Set(treeID ID, key []string, blob io.Reader) (ID, error) {
	if len(key) == 0 {
		return nil, errors.New("empty key")
	}
	// First we try to fetch the current head and all existing trees that we have
	// need to merge with.
	var trees []Tree
	if treeID != nil {
		for _, k := range key {
			tree, err := s.Tree(treeID)
			if err != nil {
				return nil, err
			}
			trees = append(trees, tree)
			if entry := tree.Get(k); entry == nil || entry.Kind == KindBlob {
				break
			} else {
				treeID = entry.ID
			}
		}
	}
	// Then we create the blob
	blobID, err := s.WriteBlob(blob)
	if err != nil {
		return nil, err
	}
	// And finally we iterate over all keys backwards to create or update the
	// trees.
	var prevTreeID ID
	for i := len(key) - 1; i >= 0; i-- {
		var entry *Entry
		// The first entry is the one pointing to our blob.
		if prevTreeID == nil {
			entry = &Entry{Name: key[i], Kind: KindBlob, ID: blobID}
			// All others are trees pointing to the prevTreeID tree.
		} else {
			entry = &Entry{Name: key[i], Kind: KindTree, ID: prevTreeID}
		}
		// The tree is nil unless we have an existing tree for the current path.
		var tree Tree
		if i < len(trees) {
			tree = trees[i]
		}
		// Check if the current tree needs updating, and if so update our entry and
		// write out the updated tree.
		if existing := tree.Get(entry.Name); existing == nil || !existing.Equal(entry) {
			// Add the entry to the tree and write it out
			if prevTreeID, err = s.WriteTree(tree.Add(entry)); err != nil {
				return nil, err
				// If this is the root tree, we are done
			} else if i == 0 {
				break
			}
			// If this is the first tree node (the leaf node) and there was no need
			// for an update, we don't need to commit anything as the tree remains
			// unchanged.
		} else if prevTreeID == nil {
			return nil, nil
			// If the first tree node changed, all nodes up to the root should change
			// too, otherwise the tree must have been corrupt.
		} else {
			return nil, fmt.Errorf("corrupt tree: key=%#v tree=%#v", key, tree)
		}
	}
	return prevTreeID, nil
}
