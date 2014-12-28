package gkv

import (
	"bytes"
	"fmt"
	"time"
)
import "io"
import "testing"

func TestDecoder(t *testing.T) {
	tests := []struct {
		Data    []byte
		Want    Object
		WantErr error
	}{
		// happy cases (from README.md)
		{
			Data: []byte("blob 12\nHello World\n"),
			Want: NewBlob([]byte("Hello World")),
		},
		{
			Data: []byte("index 94\n3 bar 0a4d55a8d778e5022fab701977c5d840bbc486d0\n3 foo 13a6151685371cc7f1a1b7d2dca999092938e493\n"),
			Want: NewIndex(map[string]ID{
				"bar": mustNewID("0a4d55a8d778e5022fab701977c5d840bbc486d0"),
				"foo": mustNewID("13a6151685371cc7f1a1b7d2dca999092938e493"),
			}),
		},
		{
			Data: []byte("commit 165\ntime 1418327450 -3600\nindex c82a9efd857f436e0ececd7986cb8611b6b8f84e\nparent 119be3a4d2e8eef6fbf1e86d817fe58a452cf429\nparent b176e7d983ca7129334dde3779e6f155b3399351\n"),
			Want: NewCommit(
				time.Date(2014, 12, 11, 19, 50, 50, 0, time.UTC).In(time.FixedZone("UTC+1", -3600)),
				mustNewID("c82a9efd857f436e0ececd7986cb8611b6b8f84e"),
				mustNewID("119be3a4d2e8eef6fbf1e86d817fe58a452cf429"),
				mustNewID("b176e7d983ca7129334dde3779e6f155b3399351"),
			),
		},
		// error cases
		{
			Data:    []byte("bob 12\nHello World\n"),
			WantErr: newBadStringError("bo", []string{"blob ", "index ", "commit "}),
		},
		{
			Data:    []byte("blob 1l2\nHello World\n"),
			WantErr: newBadInt64Error("1l", '\n'),
		},
		{
			Data:    []byte("blob 12 Hello World\n"),
			WantErr: newBadInt64Error("12 ", '\n'),
		},
		{
			Data:    []byte("blob 12\nHello World\r"),
			WantErr: newBadStringError("\r", []string{"\n"}),
		},
		{
			Data:    []byte("blob 12\nHello World"),
			WantErr: io.ErrUnexpectedEOF,
		},
	}
	for i, test := range tests {
		decoder := NewDecoder(bytes.NewBuffer(test.Data))
		got, err := decoder.Decode()
		gotErr, wantErr := fmt.Sprintf("%s", err), fmt.Sprintf("%s", test.WantErr)
		if gotErr != wantErr {
			t.Errorf("test %d: gotErr=%s wantErr=%s", i, gotErr, wantErr)
			continue
		} else if err == nil && string(got.Canonical()) != string(test.Want.Canonical()) {
			t.Errorf("test %d: got=%q want=%q", i, got.Canonical(), test.Want.Canonical())
			continue
		}
	}
}

func mustNewID(s string) ID {
	if id, err := ParseId(s); err != nil {
		panic(err)
	} else {
		return id
	}
}
