package gkv

import (
	"bytes"
	"crypto/sha1"
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
	return ID{}, nil
}

type ID [20]byte

func (id ID) String() string {
	return fmt.Sprintf("%x", id[:])
}

type Object interface {
	ID() ID
	Raw() []byte
}

func NewGKV(b Backend) *GKV {
	return &GKV{backend: b}
}

type GKV struct {
	backend Backend
	index   *Index
}

func (g *GKV) Save(o Object) error {
	return g.backend.SaveObject(o.ID(), o.Raw())
}

func (g *GKV) Load(ref string) (Object, error) {
	return nil, nil
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

func NewCommit(time time.Time, partial ID, parents ...ID) *Commit {
	return &Commit{time: time, partial: partial, parents: parents}
}

type Commit struct {
	time    time.Time
	partial ID
	parents []ID
}

func (c *Commit) ID() ID {
	return NewID(c)
}

func (c *Commit) Raw() []byte {
	buf := bytes.NewBuffer(nil)
	_, offset := c.time.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
	}
	fmt.Fprintf(buf, "time %d %s%d\n", c.time.Unix(), sign, offset)
	fmt.Fprintf(buf, "partial %s\n", c.partial)
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
}

func NewFileBackend(dir string) Backend {
	return &FileBackend{dir: dir}
}

type FileBackend struct {
	dir string
}

func (f *FileBackend) GetObject(id ID) ([]byte, error) {
	return ioutil.ReadFile(f.path(id))
}

func (f *FileBackend) SaveObject(id ID, raw []byte) error {
	file, err := ioutil.TempFile("", id.String())
	if err != nil {
		return err
	}
	defer file.Close()
	p := f.path(id)
	if _, err := file.Write(raw); err != nil {
		return err
	} else if err := os.MkdirAll(filepath.Dir(p), 0777); err != nil {
		return err
	} else if err := os.Rename(file.Name(), p); err != nil {
		return err
	}
	return ioutil.WriteFile(p, raw, 0666)
}

func (f *FileBackend) path(id ID) string {
	s := id.String()
	return filepath.Join(f.dir, s[0:2], s[2:])
}
