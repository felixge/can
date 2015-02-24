package can

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestSugar_Get_Set(t *testing.T) {
	var (
		crp      = newCountingRepo(tmpRepo())
		s        = NewSugar(crp)
		checkSet = func(key []string, val string) func() {
			return func() {
				if _, err := s.Set(key, strings.NewReader(val), &Commit{}); err != nil {
					t.Errorf("checkSet: %s for key=%#v and val=%s", err, key, val)
				}
			}
		}
		checkGet = func(key []string, val string) func() {
			return func() {
				buf := &bytes.Buffer{}
				if rc, err := s.Get(key); err != nil {
					t.Errorf("checkGet: %s for key=%#v and val=%s", err, key, val)
				} else if _, err := io.Copy(buf, rc); err != nil {
					t.Errorf("checkGet: %s for key=%#v and val=%s", err, key, val)
				} else if buf.String() != val {
					t.Errorf("checkGet: got=%q want=%q key=%#v", buf.String(), val, key)
				}
			}
		}
		checkCount = func(want int) func() {
			return func() {
				if got := crp.WriteTreeCount; got != want {
					t.Errorf("checkCount: got=%d want=%d", got, want)
				}
			}
		}
		tests = []func(){
			checkSet([]string{"foo"}, "a"),
			checkCount(1),
			checkGet([]string{"foo"}, "a"),
			checkSet([]string{"foo", "bar"}, "b"),
			checkCount(3),
			checkGet([]string{"foo", "bar"}, "b"),
			checkSet([]string{"fubar"}, "c"),
			checkCount(4),
			checkGet([]string{"fubar"}, "c"),
			checkGet([]string{"foo", "bar"}, "b"),
			checkCount(4),
			checkSet([]string{"foo", "bar"}, "b"),
			checkCount(4),
			checkSet([]string{"foo", "bar"}, "d"),
			checkCount(6),
			checkGet([]string{"foo", "bar"}, "d"),
			checkGet([]string{"fubar"}, "c"),
		}
	)
	for _, test := range tests {
		test()
	}
}

func newCountingRepo(rp Repo) *countingRepo {
	return &countingRepo{Repo: rp}
}

type countingRepo struct {
	WriteTreeCount int
	Repo
}

func (c *countingRepo) WriteTree(tree Tree) (ID, error) {
	c.WriteTreeCount++
	return c.Repo.WriteTree(tree)
}
