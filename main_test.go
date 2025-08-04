package main

import (
	"fmt"
	"io/fs"
	"math/rand"
	"testing"
	"testing/fstest"
)

func filename(input string) [128]byte {
	filename := []byte(input)
	var arr [128]byte
	copy(arr[:], filename)
	return arr
}

func TestEfes(t *testing.T) {
	fsys := Efes{}
	fsys.files[0].firstBlockIx = 1
	fsys.files[0].name = filename("hi.txt")
	fsys.data.totalBlockCount = 10

	allBytes := randomAscii(4 * 1024)
	b0 := newEfesDataBlock()
	b1 := newEfesDataBlock()
	copy(b1.data[:], allBytes[:BLOCK_SIZE])

	fsys.data.blocks = append(fsys.data.blocks, b0, b1)
	if err := fstest.TestFS(fsys, "hi.txt"); err != nil {
		t.Error(err)
	}
}

func newEfesDataBlock() efesDataBlock {
	return efesDataBlock{}
}

func randomAscii(n int) []byte {
	const printableStart = 32
	const printableEnd = 126
	b := make([]byte, n)

	for i := range b {
		b[i] = byte(rand.Intn(printableEnd-printableStart+1) + printableStart)
	}
	return b
}

func TestEfesReadFile(t *testing.T) {
	fsys := Efes{}
	fsys.files[0].firstBlockIx = 1
	fsys.files[0].name = filename("hi.txt")
	fsys.data.totalBlockCount = 10

	allBytes := randomAscii(4 * 1024)
	b0 := newEfesDataBlock()
	b1 := newEfesDataBlock()
	copy(b1.data[:], allBytes[:BLOCK_SIZE])

	fsys.data.blocks = append(fsys.data.blocks, b0, b1)

	contextBytes, err := fs.ReadFile(fsys, "hi.txt")
	if err != nil {
		t.Error(err)
	} else {
		fmt.Printf("%d\n", len(contextBytes))
	}

}
