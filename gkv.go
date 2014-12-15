package gkv

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

func NewID(o Object) ID {
	return sha1.Sum(o.Raw())
}

func ParseId(id string) (ID, error) {
	r := ID{}
	if len(id) != 40 {
		return r, fmt.Errorf("bad id: %s: not 40 chars", id)
	}
	d, err := hex.DecodeString(id)
	if err != nil {
		return r, fmt.Errorf("bad id: %s: %s", id, err)
	}
	copy(r[:], d)
	return r, nil
}

type ID [20]byte

func (id ID) String() string {
	return fmt.Sprintf("%x", id[:])
}

type Object interface {
	ID() ID
	Raw() []byte
}

func NewRepository(b Backend) *Repository {
	return &Repository{backend: b}
}

type Repository struct {
	backend Backend
	index   *Index
}

func (r *Repository) Head() (ID, error) {
	return r.backend.Head()
}

func (r *Repository) SetHead(id ID) error {
	return r.backend.SetHead(id)
}

func (r *Repository) Commit(id ID) (*Commit, error) {
	obj, err := r.Load(id)
	if err != nil {
		return nil, err
	} else if commit, ok := obj.(*Commit); !ok {
		return nil, fmt.Errorf("unexpected type: %T", obj)
	} else {
		return commit, nil
	}
}

func (r *Repository) Index(id ID) (*Index, error) {
	obj, err := r.Load(id)
	if err != nil {
		return nil, err
	} else if index, ok := obj.(*Index); !ok {
		return nil, fmt.Errorf("unexpected type: %T", obj)
	} else {
		return index, nil
	}
}

func (r *Repository) Save(o Object) error {
	return r.backend.SaveObject(o.ID(), o.Raw())
}

func (r *Repository) Load(id ID) (Object, error) {
	raw, err := r.backend.GetObject(id)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(raw)
	var (
		kind string
		size int64
	)
	if _, err := fmt.Fscanf(buf, "%s %d\n", &kind, &size); err != nil {
		return nil, err
	}
	switch kind {
	case "commit":
		var (
			sec    int64
			offset int
			index  string
		)
		// @TODO support negative offset
		if _, err := fmt.Fscanf(buf, "time %d %d\n", &sec, &offset); err != nil {
			return nil, err
		}
		t := time.Unix(sec, 0).In(time.FixedZone("", offset))
		if _, err := fmt.Fscanf(buf, "index %s\n", &index); err != nil {
			return nil, err
		}
		indexID, err := ParseId(index)
		if err != nil {
			return nil, fmt.Errorf("bad index: %s", err)
		}
		commit := &Commit{time: t, index: indexID}
		return commit, nil
	default:
		return nil, fmt.Errorf("unknown object kind: %s", kind)
	}
}

func NewIndex(entries map[string]ID) *Index {
	return &Index{entries: entries}
}

type Index struct {
	entries map[string]ID
}

func (idx *Index) ID() ID {
	return NewID(idx)
}

func (idx *Index) Raw() []byte {
	buf := bytes.NewBuffer(nil)
	for key, id := range idx.entries {
		fmt.Fprintf(buf, "%d %s %s\n", len(key), key, id)
	}
	header := []byte(fmt.Sprintf("index %d\n", buf.Len()))
	return append(header, buf.Bytes()...)
}

func NewCommit(time time.Time, index ID, parents ...ID) *Commit {
	return &Commit{time: time, index: index, parents: parents}
}

type Commit struct {
	time    time.Time
	index   ID
	parents []ID
}

func (c *Commit) ID() ID {
	return NewID(c)
}

func (c *Commit) Index() ID {
	return c.index
}

func (c *Commit) Raw() []byte {
	buf := bytes.NewBuffer(nil)
	_, offset := c.time.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
	}
	fmt.Fprintf(buf, "time %d %s%d\n", c.time.Unix(), sign, offset)
	fmt.Fprintf(buf, "index %s\n", c.index)
	for _, parent := range c.parents {
		fmt.Fprintf(buf, "parent %s\n", parent)
	}
	header := []byte(fmt.Sprintf("commit %d\n", buf.Len()))
	return append(header, buf.Bytes()...)
}

func NewBlob(val []byte) *Blob {
	return &Blob{val: val}
}

type Blob struct {
	val []byte
}

func (b *Blob) ID() ID {
	return NewID(b)
}

func (b *Blob) Val() []byte {
	return b.val
}

func (b *Blob) Raw() []byte {
	return []byte(fmt.Sprintf("blob %d\n%s", len(b.val), b.val))
}

type Backend interface {
	GetObject(id ID) ([]byte, error)
	SaveObject(id ID, raw []byte) error
	Head() (ID, error)
	SetHead(ID) error
}

func NewFileBackend(dir string) Backend {
	return &FileBackend{dir: dir}
}

type FileBackend struct {
	dir string
}

func (f *FileBackend) GetObject(id ID) ([]byte, error) {
	return ioutil.ReadFile(f.objectPath(id))
}

func (f *FileBackend) SaveObject(id ID, raw []byte) error {
	return f.writeAtomic(f.objectPath(id), raw)
}

func (f *FileBackend) Head() (ID, error) {
	id := ID{}
	data, err := ioutil.ReadFile(f.headPath())
	if err != nil {
		return id, err
	}
	copy(id[:], data)
	return id, nil
}

func (f *FileBackend) SetHead(id ID) error {
	return f.writeAtomic(f.headPath(), id[:])
}

func (f *FileBackend) writeAtomic(path string, data []byte) error {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	} else if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return err
	} else if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	return nil
}

func (f *FileBackend) headPath() string {
	return filepath.Join(f.dir, "HEAD")
}

func (f *FileBackend) objectPath(id ID) string {
	s := id.String()
	return filepath.Join(f.dir, "objects", s[0:2], s[2:])
}
