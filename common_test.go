package can

import "io/ioutil"

func tmpRepo() Repo {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
	rp := NewDirRepo(dir)
	if err := rp.Init(); err != nil {
		panic(err)
	}
	return rp
}
