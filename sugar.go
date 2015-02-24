package can

import (
	"errors"
	"fmt"
	"io"
)

func NewSugar(rp Repo) *Sugar {
	return &Sugar{Repo: rp}
}

type Sugar struct {
	Repo
}

// Get returns a read closer for the Blob with the given key.
func (s *Sugar) Get(key []string) (io.ReadCloser, error) {
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
func (s *Sugar) Set(key []string, blob io.Reader, commit *Commit) (ID, error) {
	if len(key) == 0 {
		return nil, errors.New("empty key")
	}
	// First we try to fetch the current head and all existing trees that we have
	// need to merge with.
	var trees []Tree
	head, err := s.Head()
	if err == nil {
		parent, err := s.Commit(head)
		if err != nil {
			return nil, err
		}
		treeID := parent.Tree
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
		commit.Parents = append(commit.Parents, head)
	} else if err != nil && !IsNotFound(err) {
		return nil, err
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
				// If this is the root tree, set it on the given commit.
			} else if i == 0 {
				commit.Tree = prevTreeID
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
	// Finally we create the commit with the new Tree and update our head.
	if commitID, err := s.WriteCommit(*commit); err != nil {
		return nil, err
	} else {
		return commitID, s.WriteHead(commitID)
	}
}
