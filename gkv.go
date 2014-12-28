package gkv

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"
)

type Object interface {
	ID() ID
	Canonical() []byte
}

var NilID = ID{}

func IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func NewID(o Object) ID {
	return sha1.Sum(o.Canonical())
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

func NewRepo(b Backend) *Repo {
	return &Repo{backend: b}
}

type Repo struct {
	backend Backend
}

func (r *Repo) Head() (ID, error) {
	head, err := r.backend.Load("HEAD")
	if err != nil {
		return ID{}, err
	}
	return ParseId(string(head))
}

func (r *Repo) SetHead(id ID) error {
	return r.backend.Save("HEAD", []byte(id.String()))
}

func (r *Repo) Commit(id ID) (*Commit, error) {
	obj, err := r.Load(id)
	if err != nil {
		return nil, err
	} else if commit, ok := obj.(*Commit); !ok {
		return nil, fmt.Errorf("unexpected type: %T", obj)
	} else {
		return commit, nil
	}
}

func (r *Repo) Index(id ID) (*Index, error) {
	obj, err := r.Load(id)
	if err != nil {
		return nil, err
	} else if index, ok := obj.(*Index); !ok {
		return nil, fmt.Errorf("unexpected type: %T", obj)
	} else {
		return index, nil
	}
}

func (r *Repo) Blob(id ID) (*Blob, error) {
	obj, err := r.Load(id)
	if err != nil {
		return nil, err
	} else if blob, ok := obj.(*Blob); !ok {
		return nil, fmt.Errorf("unexpected type: %T", obj)
	} else {
		return blob, nil
	}
}

func (r *Repo) Save(o Object) error {
	return r.backend.Save(r.objectPath(o.ID()), o.Canonical())
}

func (r *Repo) Load(id ID) (Object, error) {
	raw, err := r.backend.Load(r.objectPath(id))
	if err != nil {
		return nil, err
	}
	decoder := NewDecoder(bytes.NewBuffer(raw))
	if obj, err := decoder.Decode(); err != nil {
		return nil, err
	} else if gotID := obj.ID(); gotID != id {
		return nil, fmt.Errorf("corrupt object: got=%s want=%s", gotID, id)
	} else {
		return obj, nil
	}
}

func (r *Repo) objectPath(id ID) string {
	idS := id.String()
	return path.Join("objects", idS[0:2], idS[2:])
}

func NewIndex(entries IndexEntries) *Index {
	sort.Sort(entries)
	return &Index{entries: entries}
}

type IndexEntries []IndexEntry

func (kv IndexEntries) Less(i, j int) bool { return kv[i].Key < kv[j].Key }
func (kv IndexEntries) Swap(i, j int)      { kv[i], kv[j] = kv[j], kv[i] }
func (kv IndexEntries) Len() int           { return len(kv) }

type IndexEntry struct {
	Key string
	ID  ID
}

type Index struct {
	entries IndexEntries
}

func (idx *Index) ID() ID {
	return NewID(idx)
}

func (idx *Index) Entries() IndexEntries {
	cp := make(IndexEntries, len(idx.entries))
	copy(cp, idx.entries)
	return cp
}

func (idx *Index) Canonical() []byte {
	buf := bytes.NewBuffer(nil)
	for _, entry := range idx.entries {
		fmt.Fprintf(buf, "%d %s %s\n", len(entry.Key), entry.Key, entry.ID)
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

func (c *Commit) Time() time.Time {
	return c.time
}

func (c *Commit) Parents() []ID {
	cp := make([]ID, len(c.parents))
	copy(cp, c.parents)
	return cp
}

func (c *Commit) Parent() ID {
	return c.Parents()[0]
}

func (c *Commit) Canonical() []byte {
	buf := bytes.NewBuffer(nil)
	_, offset := c.time.Zone()
	fmt.Fprintf(buf, "time %d %+d\n", c.time.Unix(), offset)
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

func (b *Blob) Canonical() []byte {
	return []byte(fmt.Sprintf("blob %d\n%s\n", len(b.val)+1, b.val))
}

type Backend interface {
	Load(path string) ([]byte, error)
	Save(path string, data []byte) error
	List(path string) ([]string, error)
}

func NewFileBackend(dir string) Backend {
	return &FileBackend{dir: dir}
}

type FileBackend struct {
	dir string
}

func (f *FileBackend) Load(path string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(f.dir, path))
}

func (f *FileBackend) Save(path string, data []byte) error {
	return f.writeAtomic(filepath.Join(f.dir, path), data)
}

func (f *FileBackend) List(path string) ([]string, error) {
	return nil, nil
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
