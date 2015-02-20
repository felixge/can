package can

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// Repo provides access to a can repository.
type Repo interface {
	//Head() (ID, error)
	//WriteHead(ID) error
	// Blob returns the Blob for the given id.
	Blob(id ID) (io.ReadCloser, error)
	// WriteBlob store the given Blob and returns its id.
	WriteBlob(io.Reader) (ID, error)
	// Tree returns the Tree for the given id.
	Tree(id ID) (Tree, error)
	// WriteTree store the given Tree and returns its id.
	WriteTree(Tree) (ID, error)
	// Commit returns the Commit for the given id.
	Commit(id ID) (Commit, error)
	// WriteCommit store the given Commit and returns its id.
	WriteCommit(Commit) (ID, error)
}

// ParseID parses the given hex id string into an ID, or returns an error.
func ParseID(id string) (ID, error) {
	if id == "" {
		return nil, nil
	} else if d, err := hex.DecodeString(id); err != nil {
		return nil, fmt.Errorf("bad id: %s: %s", id, err)
	} else {
		return d, nil
	}
}

// MustID returns the ID for the given hex id, or panics on error.
func MustID(id string) ID {
	r, err := ParseID(id)
	if err != nil {
		panic(err)
	}
	return r
}

// ID holds an object id. The size depends on the inner workings of the Repo
// implementation, but will typically be 20 bytes for sha1.
type ID []byte

// String implements the Stringer interface. Returns the hex value of the id.
func (id ID) String() string {
	return fmt.Sprintf("%x", []byte(id))
}

// Equal returns true if the id is equal to other.
func (id ID) Equal(other ID) bool {
	return bytes.Compare(id, other) == 0
}

// Tree holds a list of entries.
type Tree []Entry

func (t Tree) Len() int           { return len(t) }
func (t Tree) Less(i, j int) bool { return t[i].Name < t[j].Name }
func (t Tree) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

// Entry defines a Tree entry.
type Entry struct {
	Kind Kind
	Name string
	ID   ID
}

// Kind represents the kind of objects Kit deals with.
type Kind string

const (
	KindBlob   = "blob"
	KindTree   = "tree"
	KindCommit = "commit"
)

// Commit defines a commit object.
type Commit struct {
	Tree    ID
	Parents []ID
	Time    time.Time
	Message []byte
}

func NewDirRepo(path string) (Repo, error) {
	rp := &dirRepo{
		tmp:    filepath.Join(path, "tmp"),
		obj:    filepath.Join(path, "obj"),
		format: NewDefaultFormat(),
	}
	for _, dir := range []string{rp.tmp, rp.obj} {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return nil, err
		}
	}
	return rp, nil
}

type dirRepo struct {
	tmp    string
	obj    string
	format Format
}

func (d *dirRepo) Blob(id ID) (io.ReadCloser, error) {
	file, err := os.Open(d.path(id))
	if err != nil {
		return nil, err
	}
	iv := NewIDVerifier(file, id)
	r, err := d.format.DecodeBlob(iv)
	if err != nil {
		file.Close()
		return nil, err
	}
	return NewReadCloser(r, file), nil
}

func (d *dirRepo) WriteBlob(r io.Reader) (ID, error) {
	return d.write(r)
}

func (d *dirRepo) Tree(id ID) (Tree, error) {
	file, err := os.Open(d.path(id))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	iv := NewIDVerifier(file, id)
	tree, err := d.format.DecodeTree(iv)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

func (d *dirRepo) WriteTree(t Tree) (ID, error) {
	return d.write(t)
}

func (d *dirRepo) Commit(id ID) (Commit, error) {
	file, err := os.Open(d.path(id))
	if err != nil {
		return Commit{}, err
	}
	defer file.Close()
	iv := NewIDVerifier(file, id)
	commit, err := d.format.DecodeCommit(iv)
	if err != nil {
		return Commit{}, err
	}
	return commit, nil
}

func (d *dirRepo) WriteCommit(c Commit) (ID, error) {
	return d.write(c)
}

func (d *dirRepo) write(o interface{}) (ID, error) {
	tmpFile, err := ioutil.TempFile(d.tmp, "")
	if err != nil {
		return nil, err
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	iw := NewIDWriter(tmpFile)
	switch t := o.(type) {
	case Tree:
		if err := d.format.EncodeTree(iw, t); err != nil {
			return nil, err
		}
	case Commit:
		if err := d.format.EncodeCommit(iw, t); err != nil {
			return nil, err
		}
	case io.Reader:
		if err := d.format.EncodeBlob(iw, t); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("bad type: %#v", t)
	}
	id := iw.ID()
	path := d.path(id)
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return nil, err
	}
	return id, nil
}

func (d *dirRepo) path(id ID) string {
	s := id.String()
	return filepath.Join(d.obj, s[0:2], s[2:])
}

type IDWriter interface {
	io.Writer
	ID() ID
}

func NewIDWriter(w io.Writer) IDWriter {
	return &idWriter{w: w, h: sha1.New()}
}

type idWriter struct {
	w io.Writer
	h hash.Hash
}

func (w *idWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if _, err := w.h.Write(p); err != nil {
		return n, err
	}
	return n, err
}

func (w *idWriter) ID() ID {
	return w.h.Sum(nil)
}

func NewIDVerifier(r io.Reader, id ID) io.Reader {
	return &idVerifier{r: r, want: id, h: sha1.New()}
}

type idVerifier struct {
	r    io.Reader
	h    hash.Hash
	want ID
}

func (v *idVerifier) Read(p []byte) (int, error) {
	n, err := v.r.Read(p)
	if _, err := v.h.Write(p[0:n]); err != nil {
		return n, err
	}
	if err == io.EOF {
		if got := ID(v.h.Sum(nil)); !got.Equal(v.want) {
			return n, fmt.Errorf("bad id: got=%s want=%s", got, v.want)
		}
	}
	return n, err
}

func NewReadCloser(r io.Reader, c io.Closer) io.ReadCloser {
	return &readCloser{r, c}
}

type readCloser struct {
	io.Reader
	io.Closer
}
