package main

import "testing"

func TestQualifyLocalImport(t *testing.T) {
	for _, c := range []struct {
		importpath, gopathsrc, pwd, want string
		wantErr                          bool
	}{{
		importpath: "./cmd/foo",
		gopathsrc:  "/home/go/src",
		pwd:        "/home/go/src/github.com/my/repo",
		want:       "github.com/my/repo/cmd/foo",
	}, {
		importpath: "./foo",
		gopathsrc:  "/home/go/src",
		pwd:        "/home/go/src/github.com/my/repo/cmd",
		want:       "github.com/my/repo/cmd/foo",
	}, {
		// $PWD not on $GOPATH/src
		importpath: "./cmd/foo",
		gopathsrc:  "/home/go/src",
		pwd:        "/",
		wantErr:    true,
	}} {
		got, err := qualifyLocalImport(c.importpath, c.gopathsrc, c.pwd)
		if gotErr := err != nil; gotErr != c.wantErr {
			t.Fatalf("qualifyLocalImport returned %v, wanted err? %t", err, c.wantErr)
		}
		if got != c.want {
			t.Fatalf("qualifyLocalImport(%q, %q, %q): got %q, want %q", c.importpath, c.gopathsrc, c.pwd, got, c.want)
		}
	}

}
