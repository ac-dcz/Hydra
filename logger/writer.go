package logger

import (
	"io"
	"os"
)

func NewFileWriter(path string) io.Writer {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		Error.Println(err)
		panic(err)
	}
	return file
}
