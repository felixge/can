package can

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Format defines a serialization format. Encode/Decode pairs are guaranteed to
// produce symmetrical output as determined by reflect.DeepEqual.
type Format interface {
	// EncodeBlob encodes a blob to the given Writer.
	EncodeBlob(io.Writer, io.Reader) error
	// DecodeBlob decodes a blob from the given Reader, and returns its data.
	DecodeBlob(io.Reader) (io.Reader, error)
	// EncodeTree encodes a tree to the given Writer.
	EncodeTree(io.Writer, Tree) error
	// DecodeTree decodes a tree from the given Reader, and returns it.
	DecodeTree(io.Reader) (Tree, error)
	// EncodeCommit encodes a commit to the given Writer.
	EncodeCommit(io.Writer, Commit) error
	// DecodeCommit decodes a commit from the given Reader, and returns it.
	DecodeCommit(io.Reader) (Commit, error)
}

// NewDefaultFormat returns the default format.
func NewDefaultFormat() Format {
	return &defaultFormat{}
}

const (
	blobPrefix   = "blob\n"
	treePrefix   = "tree\n"
	commitPrefix = "commit\n"
)

// defaultFormat implements the Format interface.
type defaultFormat struct{}

// EncodeBlob is part of the Format interface.
func (f *defaultFormat) EncodeBlob(w io.Writer, r io.Reader) error {
	b := bufio.NewWriter(w)
	if _, err := io.WriteString(b, blobPrefix); err != nil {
		return err
	} else if _, err := io.Copy(b, r); err != nil {
		return err
	}
	return b.Flush()
}

// DecodeBlob is part of the Format interface.
func (f *defaultFormat) DecodeBlob(r io.Reader) (io.Reader, error) {
	b := bufio.NewReader(r)
	if prefix, err := ioutil.ReadAll(io.LimitReader(b, int64(len(blobPrefix)))); err != nil {
		return nil, err
	} else if sp := string(prefix); sp != blobPrefix {
		return nil, fmt.Errorf("bad blob prefix: %q", sp)
	}
	return b, nil
}

// EncodeTree is part of the Format interface.
func (f *defaultFormat) EncodeTree(w io.Writer, t Tree) error {
	b := bufio.NewWriter(w)
	if _, err := io.WriteString(b, treePrefix); err != nil {
		return err
	}
	sort.Sort(t)
	for _, entry := range t {
		if _, err := fmt.Fprintf(b, "%s %s %d %s\n", entry.Kind, entry.ID, len(entry.Name), entry.Name); err != nil {
			return err
		}
	}
	return b.Flush()
}

// DecodeTree is part of the Format interface.
func (f *defaultFormat) DecodeTree(r io.Reader) (Tree, error) {
	b := bufio.NewReader(r)
	if prefix, err := ioutil.ReadAll(io.LimitReader(b, int64(len(treePrefix)))); err != nil {
	} else if sp := string(prefix); sp != treePrefix {
		return nil, fmt.Errorf("bad tree prefix: %q", sp)
	}
	var tree Tree
	for {
		if kind, err := b.ReadString(' '); err == io.EOF && len(kind) == 0 {
			return tree, nil
		} else if err != nil {
			return nil, err
		} else if id, err := b.ReadString(' '); err != nil {
			return nil, err
		} else if id, err := ParseID(id[:len(id)-1]); err != nil {
			return nil, err
		} else if nameLen, err := b.ReadString(' '); err != nil {
			return nil, err
		} else if nameLen, err := strconv.ParseInt(nameLen[:len(nameLen)-1], 10, 64); err != nil {
			return nil, err
		} else if name, err := ioutil.ReadAll(io.LimitReader(b, nameLen+1)); err != nil {
			return nil, err
		} else {
			tree = append(tree, &Entry{
				Kind: Kind(kind[:len(kind)-1]),
				ID:   id,
				Name: string(name[:len(name)-1]),
			})
		}
	}
}

// EncodeCommit is part of the Format interface.
func (f *defaultFormat) EncodeCommit(w io.Writer, c Commit) error {
	b := bufio.NewWriter(w)
	ut := c.Time.Unix()
	_, zo := c.Time.Zone()
	if _, err := io.WriteString(b, commitPrefix); err != nil {
		return err
	} else if _, err := fmt.Fprintf(b, "tree %s\n", c.Tree); err != nil {
		return err
	}
	for _, parent := range c.Parents {
		if _, err := fmt.Fprintf(b, "parent %s\n", parent); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(b, "time %d %+d\n", ut, zo); err != nil {
		return err
	} else if _, err := fmt.Fprintf(b, "\n%s", c.Message); err != nil {
		return err
	}
	return b.Flush()
}

// DecodeCommit is part of the Format interface.
func (f *defaultFormat) DecodeCommit(r io.Reader) (Commit, error) {
	b := bufio.NewReader(r)
	if prefix, err := ioutil.ReadAll(io.LimitReader(b, int64(len(commitPrefix)))); err != nil {
	} else if sp := string(prefix); sp != commitPrefix {
		return Commit{}, fmt.Errorf("bad commit prefix: %q", sp)
	}
	var commit Commit
fields:
	for {
		if field, err := b.ReadString(' '); err != nil {
			return commit, err
		} else if val, err := b.ReadString('\n'); err != nil {
			return commit, err
		} else {
			val = val[:len(val)-1]
			field = field[:len(field)-1]
			switch field {
			case "tree":
				if id, err := ParseID(val); err != nil {
					return commit, err
				} else {
					commit.Tree = id
				}
			case "parent":
				if id, err := ParseID(val); err != nil {
					return commit, err
				} else {
					commit.Parents = append(commit.Parents, id)
				}
			case "time":
				for i, s := range strings.Split(val, " ") {
					val, err := strconv.ParseInt(s, 10, 64)
					if err != nil {
						return commit, fmt.Errorf("bad time: %s: %s", s, err)
					}
					switch i {
					case 0:
						commit.Time = time.Unix(val, 0)
					case 1:
						commit.Time = commit.Time.In(time.FixedZone("", int(val)))
					}
				}
				// Empty time should produce zero time to allow symmetry of
				// encoding/decoding zero Commit value:
				if commit.Time.IsZero() {
					commit.Time = time.Time{}
				}
				break fields
			default:
				return commit, fmt.Errorf("unknown field: %s", field)
			}
		}
	}
	if c, err := b.ReadByte(); err != nil {
		return commit, err
	} else if want := byte('\n'); c != want {
		return commit, fmt.Errorf("bad end of fields: got=%q want=%q", c, want)
	} else if msg, err := ioutil.ReadAll(b); err != nil {
		return commit, err
	} else {
		// Empty Message should produce nil to allow symmetry of encoding/decoding
		// zero Commit value:
		if len(msg) > 0 {
			commit.Message = msg
		}
		return commit, nil
	}
}
