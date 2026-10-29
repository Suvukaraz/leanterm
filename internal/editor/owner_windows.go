package editor

import "io/fs"

func keepOwner(string, fs.FileInfo) bool { return true }
