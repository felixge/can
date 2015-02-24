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
	"sort"
	"time"
)

// Repo provides access to a can repository.
type Repo interface {
	// Head returns the ID of the head commit.
	Head() (ID, error)
	// WriteHead sets the ID of the head commit.
	WriteHead(ID) error
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

// Tree holds a list of entries, sorted by name in ascending order.
type Tree []*Entry

func (t Tree) Len() int           { return len(t) }
func (t Tree) Less(i, j int) bool { return t[i].Name < t[j].Name }
func (t Tree) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

// Get returns the Entry with the given name, or nil if it does not exist. The
// tree must be sorted prior to calling Get.
func (t Tree) Get(name string) *Entry {
	if i := t.index(name); i >= 0 {
		return t[i]
	}
	return nil
}

// Add adds or updates the given entry and returns the resulting tree.
func (t Tree) Add(entry *Entry) Tree {
	if i := t.index(entry.Name); i >= 0 {
		t[i] = entry
	} else {
		t = append(t, entry)
	}
	return t
}

func (t Tree) index(name string) int {
	i := sort.Search(len(t), func(i int) bool {
		return t[i].Name >= name
	})
	if i < len(t) && t[i].Name == name {
		return i
	}
	return -1
}

// Entry defines a Tree entry.
type Entry struct {
	Kind Kind
	Name string
	ID   ID
}

// Equal returns if one entry is equal to the another.
func (e *Entry) Equal(other *Entry) bool {
	return e.Kind == other.Kind && e.Name == other.Name && e.ID.Equal(other.ID)
}

// Kind represents the kind of objects Kit deals with.
type Kind string

const (
	KindBlob   Kind = "blob"
	KindTree   Kind = "tree"
	KindCommit Kind = "commit"
)

// Commit defines a commit object.
type Commit struct {
	Tree    ID
	Parents []ID
	Time    time.Time
	Message []byte
}

func IsNotFound(err error) bool {
	if nf, ok := err.(NotFounder); ok {
		return nf.NotFound()
	}
	return os.IsNotExist(err)
}

type notFoundError string

func (n notFoundError) Error() string  { return string(n) }
func (n notFoundError) NotFound() bool { return true }

type NotFounder interface {
	NotFound() bool
}

func NewDirRepo(path string) *DirRepo {
	return &DirRepo{
		tmp:    filepath.Join(path, "tmp"),
		obj:    filepath.Join(path, "obj"),
		head:   filepath.Join(path, "head"),
		format: NewDefaultFormat(),
	}
}

// Check Repo interface compliance
var _ = Repo(&DirRepo{})

type DirRepo struct {
	tmp    string
	obj    string
	head   string
	format Format
}

func (d *DirRepo) Init() error {
	for _, dir := range []string{d.tmp, d.obj} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func (d *DirRepo) Head() (ID, error) {
	if head, err := ioutil.ReadFile(d.head); err != nil {
		return nil, err
	} else {
		return ParseID(string(head))
	}
}

func (d *DirRepo) WriteHead(id ID) error {
	return ioutil.WriteFile(d.head, []byte(id.String()), 0600)
}

func (d *DirRepo) Blob(id ID) (io.ReadCloser, error) {
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

func (d *DirRepo) WriteBlob(r io.Reader) (ID, error) {
	return d.write(r)
}

func (d *DirRepo) Tree(id ID) (Tree, error) {
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

func (d *DirRepo) WriteTree(t Tree) (ID, error) {
	return d.write(t)
}

func (d *DirRepo) Commit(id ID) (Commit, error) {
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

func (d *DirRepo) WriteCommit(c Commit) (ID, error) {
	return d.write(c)
}

func (d *DirRepo) write(o interface{}) (ID, error) {
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
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return nil, err
	}
	return id, nil
}

func (d *DirRepo) path(id ID) string {
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
