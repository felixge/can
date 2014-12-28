package gkv

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

type Decoder struct {
	r *bufio.Reader
}

func (d *Decoder) Decode() (Object, error) {
	if obj, err := d.decode(); err == io.EOF {
		return nil, io.ErrUnexpectedEOF
	} else {
		return obj, err
	}
}

func (d *Decoder) decode() (Object, error) {
	kind, err := readOneOf(d.r, "blob ", "index ", "commit ")
	if err != nil {
		return nil, err
	}
	size, err := readInt64(d.r, false, '\n')
	if err != nil {
		return nil, err
	}
	switch kind[0 : len(kind)-1] {
	case "blob":
		if data, err := ioutil.ReadAll(io.LimitReader(d.r, size-1)); err != nil {
			return nil, err
		} else if _, err := readOneOf(d.r, "\n"); err != nil {
			return nil, err
		} else {
			return NewBlob(data), nil
		}
	case "index":
		r := newByteCounter(d.r)
		var entries IndexEntries
		for r.Count() < size {
			if keySize, err := readInt64(r, false, ' '); err != nil {
				return nil, err
			} else if key, err := readString(r, keySize); err != nil {
				return nil, err
			} else if _, err := readOneOf(r, " "); err != nil {
				return nil, err
			} else if id, err := readId(r); err != nil {
				return nil, err
			} else if _, err := readOneOf(r, "\n"); err != nil {
				return nil, err
			} else {
				entries = append(entries, IndexEntry{Key: key, ID: id})
			}
		}
		return NewIndex(entries), nil
	case "commit":
		r := newByteCounter(d.r)
		if _, err := readOneOf(r, "time "); err != nil {
			return nil, err
		} else if ts, err := readInt64(r, false, ' '); err != nil {
			return nil, err
		} else if tz, err := readInt64(r, true, '\n'); err != nil {
			return nil, err
		} else if _, err := readOneOf(r, "index "); err != nil {
			return nil, err
		} else if indexID, err := readId(r); err != nil {
			return nil, err
		} else if _, err := readOneOf(r, "\n"); err != nil {
			return nil, err
		} else {
			parentIDs := []ID{}
			for r.Count() < size {
				if _, err := readOneOf(r, "parent "); err != nil {
					return nil, err
				} else if parentID, err := readId(r); err != nil {
					return nil, err
				} else if _, err := readOneOf(r, "\n"); err != nil {
					return nil, err
				} else {
					parentIDs = append(parentIDs, parentID)
				}
			}
			t := time.Unix(ts, 0).In(time.FixedZone("", int(tz)))
			return NewCommit(t, indexID, parentIDs...), nil
		}
	default:
		panic("unreachable")
	}
}

func newByteCounter(r *bufio.Reader) *byteCounter {
	return &byteCounter{r: r}
}

type byteCounter struct {
	r     *bufio.Reader
	count int64
}

func (b *byteCounter) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.count += int64(n)
	return n, err
}

func (b *byteCounter) ReadByte() (byte, error) {
	c, err := b.r.ReadByte()
	if err == nil {
		b.count++
	}
	return c, err
}

func (b *byteCounter) Count() int64 {
	return b.count
}

func readId(r io.Reader) (ID, error) {
	id, err := readString(r, 40)
	if err != nil {
		return ID{}, err
	}
	return ParseId(id)
}

func readString(r io.Reader, n int64) (string, error) {
	data, err := ioutil.ReadAll(io.LimitReader(r, n))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readInt64(r io.ByteReader, signed bool, end byte) (int64, error) {
	if end >= '0' && end <= '9' {
		panic("bad end")
	}
	const maxSize = 20
	var buf string
	for {
		c, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		buf = buf + string(c)
		if len(buf) > 0 && c == end {
			val, err := strconv.ParseInt(buf[0:len(buf)-1], 10, 64)
			if err != nil {
				return 0, newBadInt64Error(buf, end)
			}
			return val, nil
			return 0, newBadInt64Error(buf, end)
			// (╯°□°）╯︵ ┻━┻
		} else if len(buf) > maxSize ||
			(!signed && !isDigit(c)) ||
			(signed && len(buf) > 1 && !isDigit(c)) ||
			(signed && len(buf) == 1 && !isDigit(c) && !isSign(c)) {
			return 0, newBadInt64Error(buf, end)
		}
	}
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isSign(c byte) bool {
	return c == '-' || c == '+'
}

func readOneOf(r io.ByteReader, options ...string) (string, error) {
	var s string
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		s = s + string(c)
		var valid bool
		for _, option := range options {
			if option == s {
				return s, nil
			} else if strings.HasPrefix(option, s) {
				valid = true
				break
			}
		}
		if !valid {
			return s, newBadStringError(s, options)
		}
	}
}

func newBadInt64Error(got string, end byte) error {
	return fmt.Errorf("got=%q expected int64 + %q", got, string(end))
}

func newBadStringError(got string, expected []string) error {
	return fmt.Errorf("got=%q expected=%q", got, strings.Join(expected, "|"))
}
