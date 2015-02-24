package can

import (
	"bytes"
	"io/ioutil"
	"sort"

	"github.com/kylelemons/godebug/pretty"
)
import (
	"testing"
	"time"
)

func Test_DirRepo(t *testing.T) {
	rp := tmpRepo()
	blobs := map[string][]byte{
		"0cd5a7d8dc5a48bb59c0205146e4aac675dfe74a": []byte("Hello"),
		"054f22c17948d775ac4b327c7987c7acff4b8d64": []byte("World"),
	}
	for idS, data := range blobs {
		testBlob(t, rp, data, MustID(idS))
	}
	trees := map[string]Tree{
		"29ee187f331966f235b3f67404b71e812f893825": Tree{
			{
				Kind: KindBlob,
				ID:   MustID("0cd5a7d8dc5a48bb59c0205146e4aac675dfe74a"),
				Name: "blob 1",
			},
			{
				Kind: KindBlob,
				ID:   MustID("054f22c17948d775ac4b327c7987c7acff4b8d64"),
				Name: "blob 2",
			},
		},
	}
	for idS, tree := range trees {
		testTree(t, rp, tree, MustID(idS))
		sort.Sort(sort.Reverse(tree))
		testTree(t, rp, tree, MustID(idS))
	}
	commits := map[string]Commit{
		"04f81807bae3f1091ef8c7feb475430432cfd7e3": Commit{
			Tree:    MustID("0123456789"),
			Parents: []ID{MustID("0123"), MustID("45"), MustID("6789")},
			Time:    time.Date(2015, 2, 20, 13, 14, 33, 0, time.FixedZone("", 3600)),
			Message: []byte("hi,\n\nhow are you?"),
		},
		"54623d8bce90c016793a6c759484a4aa4044d6a0": Commit{
			Tree:    MustID("23456789"),
			Parents: []ID{MustID("0123"), MustID("45"), MustID("6789")},
			Time:    time.Date(2015, 2, 20, 13, 14, 33, 0, time.FixedZone("", 3600)),
			Message: []byte("hi,\n\nhow are you?"),
		},
	}
	for idS, commit := range commits {
		testCommit(t, rp, commit, MustID(idS))
	}
}

func testBlob(t *testing.T, k Repo, data []byte, wantID ID) {
	in := bytes.NewReader(data)
	id, err := k.WriteBlob(in)
	if err != nil {
		t.Fatal(err)
	} else if !id.Equal(wantID) {
		t.Fatalf("bad id: got=%s want=%s", id, wantID)
	} else if out, err := k.Blob(id); err != nil {
		t.Fatal(err)
	} else {
		defer out.Close()
		if dataOut, err := ioutil.ReadAll(out); err != nil {
			t.Fatal(err)
		} else if bytes.Compare(dataOut, data) != 0 {
			t.Fatalf("bad blob data: got=%s want=%s", dataOut, data)
		}
	}
}

func testTree(t *testing.T, k Repo, in Tree, wantID ID) {
	id, err := k.WriteTree(in)
	if err != nil {
		t.Fatal(err)
	} else if !id.Equal(wantID) {
		t.Fatalf("bad id: got=%s want=%s", id, wantID)
	} else if out, err := k.Tree(id); err != nil {
		t.Fatal(err)
	} else if diff := pretty.Compare(out, in); diff != "" {
		t.Fatalf("%s", diff)
	}
}

func testCommit(t *testing.T, k Repo, in Commit, wantID ID) {
	id, err := k.WriteCommit(in)
	if err != nil {
		t.Fatal(err)
	} else if !id.Equal(wantID) {
		t.Fatalf("bad id: got=%s want=%s", id, wantID)
	} else if out, err := k.Commit(id); err != nil {
		t.Fatal(err)
	} else if diff := pretty.Compare(out, in); diff != "" {
		t.Fatalf("%s", diff)
	}
}
