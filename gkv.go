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

var NilID = ID{}

func IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

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
	return r.backend.Save(r.objectPath(o.ID()), o.Raw())
}

func (r *Repo) Load(id ID) (Object, error) {
	raw, err := r.backend.Load(r.objectPath(id))
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
	case "blob":
		val := buf.Bytes()
		val = val[0 : len(val)-1]
		return &Blob{val: val}, nil
	case "commit":
		var (
			sec    int64
			offset int
		)
		// @TODO support negative offset
		if _, err := fmt.Fscanf(buf, "time %d %d\n", &sec, &offset); err != nil {
			return nil, err
		}
		t := time.Unix(sec, 0).In(time.FixedZone("", offset))
		var index string
		if _, err := fmt.Fscanf(buf, "index %s\n", &index); err != nil {
			return nil, err
		}
		indexID, err := ParseId(index)
		if err != nil {
			return nil, fmt.Errorf("bad index: %s", err)
		}
		var parent string
		if _, err := fmt.Fscanf(buf, "parent %s\n", &parent); err != nil {
			return nil, err
		}
		parentID, err := ParseId(parent)
		if err != nil {
			return nil, fmt.Errorf("bad parent: %s", err)
		}
		return &Commit{time: t, index: indexID, parents: []ID{parentID}}, nil
	case "index":
		entries := map[string]ID{}
		for buf.Len() > 0 {
			var keySize int
			if _, err := fmt.Fscanf(buf, "%d ", &keySize); err != nil {
				return nil, err
			}
			key := make([]byte, keySize)
			if n, err := buf.Read(key); err != nil {
				return nil, err
			} else if n != keySize {
				return nil, fmt.Errorf("short read")
			}
			var blobIDStr string
			if _, err := fmt.Fscanf(buf, " %s\n", &blobIDStr); err != nil {
				return nil, err
			}
			blobID, err := ParseId(blobIDStr)
			if err != nil {
				return nil, err
			}
			entries[string(key)] = blobID
		}
		return &Index{entries: entries}, nil
	default:
		return nil, fmt.Errorf("unknown object kind: %s", kind)
	}
}

func (r *Repo) objectPath(id ID) string {
	idS := id.String()
	return path.Join("objects", idS[0:2], idS[2:])
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

func (idx *Index) Entries() map[string]ID {
	cp := make(map[string]ID, len(idx.entries))
	for key, val := range idx.entries {
		cp[key] = val
	}
	return cp
}

func (idx *Index) Raw() []byte {
	var keys = make(sort.StringSlice, 0, len(idx.entries))
	for key, _ := range idx.entries {
		keys = append(keys, key)
	}
	sort.Sort(keys)
	buf := bytes.NewBuffer(nil)
	for _, key := range keys {
		fmt.Fprintf(buf, "%d %s %s\n", len(key), key, idx.entries[key])
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

func (c *Commit) Raw() []byte {
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

func (b *Blob) Raw() []byte {
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
