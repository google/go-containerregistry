package arguments

import "os"

type CurrentWorkingDir func() string
type SymlinkEvaler func(string) (string, error)
type FileStatReader func(string) (os.FileInfo, error)
