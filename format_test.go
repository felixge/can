package can

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/kylelemons/godebug/pretty"
)

func TestDefaultFormat_Blob(t *testing.T) {
	tests := []struct {
		Data []byte
		Want []byte
	}{
		{
			Data: []byte(""),
			Want: []byte("blob\n"),
		},
		{
			Data: []byte("Hello World"),
			Want: []byte("blob\nHello World"),
		},
		{
			Data: []byte("\nFoo loves\r\nbar\n"),
			Want: []byte("blob\n\nFoo loves\r\nbar\n"),
		},
	}
	format := NewDefaultFormat()
	for _, test := range tests {
		buf := bytes.NewBuffer(nil)
		if err := format.EncodeBlob(buf, bytes.NewReader(test.Data)); err != nil {
			t.Fatal(err)
		} else if got := buf.Bytes(); bytes.Compare(got, test.Want) != 0 {
			t.Fatalf("got=%q want=%q", got, test.Want)
		} else if r, err := format.DecodeBlob(buf); err != nil {
			t.Fatal(err)
		} else if gotData, err := ioutil.ReadAll(r); err != nil {
			t.Fatal(err)
		} else if bytes.Compare(gotData, test.Data) != 0 {
			t.Fatalf("got=%q want=%q", gotData, test.Data)
		}
	}
}

func TestDefaultFormat_Tree(t *testing.T) {
	tests := []struct {
		Tree Tree
		Want []byte
	}{
		{
			Tree: nil,
			Want: []byte("tree\n"),
		},
		{
			Tree: Tree{{Kind: KindBlob, Name: "foo", ID: MustID("0123456789")}},
			Want: []byte("tree\nblob 0123456789 3 foo\n"),
		},
		{
			Tree: Tree{{Kind: KindTree, Name: "foo", ID: MustID("0123456789")}},
			Want: []byte("tree\ntree 0123456789 3 foo\n"),
		},
		{
			Tree: Tree{
				{Kind: KindBlob, Name: "hi", ID: MustID("1234")},
				{Kind: KindBlob, Name: "how are you?", ID: MustID("8765")},
			},
			Want: []byte("tree\nblob 1234 2 hi\nblob 8765 12 how are you?\n"),
		},
		{
			Tree: Tree{
				{Kind: KindBlob, Name: "how are you?", ID: MustID("8765")},
				{Kind: KindBlob, Name: "hi", ID: MustID("1234")},
			},
			Want: []byte("tree\nblob 1234 2 hi\nblob 8765 12 how are you?\n"),
		},
	}
	format := NewDefaultFormat()
	for _, test := range tests {
		buf := bytes.NewBuffer(nil)
		if err := format.EncodeTree(buf, test.Tree); err != nil {
			t.Fatal(err)
		} else if got := buf.Bytes(); bytes.Compare(got, test.Want) != 0 {
			t.Fatalf("got=%q want=%q", got, test.Want)
		} else if gotTree, err := format.DecodeTree(buf); err != nil {
			t.Fatal(err)
		} else if diff := pretty.Compare(gotTree, test.Tree); diff != "" {
			t.Fatalf("%s", diff)
		}
	}
}

func TestDefaultFormat_Commit(t *testing.T) {
	tm := time.Date(2015, 2, 20, 13, 14, 33, 0, time.FixedZone("", 3600))
	tests := []struct {
		Commit Commit
		Want   []byte
	}{
		{
			Commit: Commit{},
			Want:   []byte("commit\ntree \ntime -62135596800 +0\n\n"),
		},
		{
			Commit: Commit{
				Tree:    MustID("0123456789"),
				Parents: []ID{MustID("0123"), MustID("45"), MustID("6789")},
				Time:    tm,
				Message: []byte("hi,\n\nhow are you?"),
			},
			Want: []byte("commit\ntree 0123456789\nparent 0123\nparent 45\nparent 6789\ntime 1424434473 +3600\n\nhi,\n\nhow are you?"),
		},
		{
			Commit: Commit{
				Tree:    MustID("0123456789"),
				Parents: []ID{MustID("6789"), MustID("45")},
				Time:    tm.In(time.FixedZone("", -1234)),
				Message: []byte("hi,\n\nhow are you?"),
			},
			Want: []byte("commit\ntree 0123456789\nparent 6789\nparent 45\ntime 1424434473 -1234\n\nhi,\n\nhow are you?"),
		},
	}
	format := NewDefaultFormat()
	for _, test := range tests {
		buf := bytes.NewBuffer(nil)
		if err := format.EncodeCommit(buf, test.Commit); err != nil {
			t.Fatal(err)
		} else if got := buf.Bytes(); bytes.Compare(got, test.Want) != 0 {
			t.Fatalf("got=%q want=%q", got, test.Want)
		} else if gotCommit, err := format.DecodeCommit(buf); err != nil {
			t.Fatal(err)
		} else if diff := pretty.Compare(gotCommit, test.Commit); diff != "" {
			t.Fatalf("%s", diff)
		}
	}
}
