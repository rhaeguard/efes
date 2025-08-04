package main

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
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
	b0 := newEfesDataBlock(0)
	b1 := newEfesDataBlock(0)
	copy(b1.data[:], allBytes[:BLOCK_SIZE])

	fsys.data.blocks = append(fsys.data.blocks, b0, b1)
	if err := fstest.TestFS(fsys, "hi.txt"); err != nil {
		t.Error(err)
	}
}

func newEfesDataBlock(nextDataBlockId uint16) efesDataBlock {
	return efesDataBlock{
		nextDataBlockIx: nextDataBlockId,
	}
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

func getTestData(n int) []byte {
	data, err := os.ReadFile("test_data.txt")
	if err != nil {
		panic(err.Error())
	}
	arr := make([]byte, n)
	copy(arr, data[:n])
	return arr
}

func TestEfesReadFile(t *testing.T) {
	fsys := Efes{}
	fsys.files[0].firstBlockIx = 1
	fsys.files[0].name = filename("hi.txt")
	fsys.data.totalBlockCount = 10

	allBytes := getTestData(4 * BLOCK_SIZE)
	b0 := newEfesDataBlock(0)
	b1 := newEfesDataBlock(3)
	b2 := newEfesDataBlock(0) // empty
	b3 := newEfesDataBlock(4)
	b4 := newEfesDataBlock(6)
	b5 := newEfesDataBlock(0) // empty
	b6 := newEfesDataBlock(0)
	copy(b1.data[:], allBytes[BLOCK_SIZE*0:BLOCK_SIZE*1])
	copy(b3.data[:], allBytes[BLOCK_SIZE*1:BLOCK_SIZE*2])
	copy(b4.data[:], allBytes[BLOCK_SIZE*2:BLOCK_SIZE*3])
	copy(b6.data[:], allBytes[BLOCK_SIZE*3:BLOCK_SIZE*4])

	fsys.data.blocks = append(fsys.data.blocks, b0, b1, b2, b3, b4, b5, b6)

	contextBytes, err := fs.ReadFile(fsys, "hi.txt")
	if err != nil {
		t.Error(err)
	} else {
		if len(contextBytes) != 4*BLOCK_SIZE {
			t.Fail()
		}
		// os.WriteFile("output.txt", contextBytes, 0644)
	}

}

func TestEfesSerde(t *testing.T) {
	fsys := Efes{}
	fsys.files[0].firstBlockIx = 1
	fsys.files[0].name = filename("alice_part_1.txt")
	fsys.files[1].firstBlockIx = 2
	fsys.files[1].name = filename("alice_part_2.txt")
	fsys.data.totalBlockCount = 11

	alicePart1 := getTestData(4 * BLOCK_SIZE)
	alicePart2 := getTestData(5 * BLOCK_SIZE)
	b0 := newEfesDataBlock(0)
	b1 := newEfesDataBlock(3)
	b2 := newEfesDataBlock(5)
	b3 := newEfesDataBlock(4)
	b4 := newEfesDataBlock(6)
	b5 := newEfesDataBlock(7)
	b6 := newEfesDataBlock(0)
	b7 := newEfesDataBlock(9)
	b8 := newEfesDataBlock(0)
	b9 := newEfesDataBlock(10)
	b10 := newEfesDataBlock(0)
	// copying part 1
	copy(b1.data[:], alicePart1[BLOCK_SIZE*0:BLOCK_SIZE*1])
	copy(b3.data[:], alicePart1[BLOCK_SIZE*1:BLOCK_SIZE*2])
	copy(b4.data[:], alicePart1[BLOCK_SIZE*2:BLOCK_SIZE*3])
	copy(b6.data[:], alicePart1[BLOCK_SIZE*3:BLOCK_SIZE*4])
	// copying part 2
	copy(b2.data[:], alicePart2[BLOCK_SIZE*0:BLOCK_SIZE*1])
	copy(b5.data[:], alicePart2[BLOCK_SIZE*1:BLOCK_SIZE*2])
	copy(b7.data[:], alicePart2[BLOCK_SIZE*2:BLOCK_SIZE*3])
	copy(b9.data[:], alicePart2[BLOCK_SIZE*3:BLOCK_SIZE*4])
	copy(b10.data[:], alicePart2[BLOCK_SIZE*4:BLOCK_SIZE*5])

	fsys.data.blocks = append(fsys.data.blocks, b0, b1, b2, b3, b4, b5, b6, b7, b8, b9, b10)

	fmt.Printf("Fsys Size (bytes): %d\n", fsys.SizeInBytes())
	if err := fsys.Serialize("./virtual.img"); err != nil {
		t.Errorf("Error: %s\n", err)
	}

	if newFsys, err := NewEfesFileSystem("./virtual.img"); err != nil {
		t.Errorf("Error: %s\n", err)
	} else {
		if len(newFsys.files) != 200 {
			t.Errorf("Expected 200 files, Got: %d\n", len(newFsys.files))
		}

		if int(newFsys.data.totalBlockCount) != len(newFsys.data.blocks) {
			t.Errorf("Expected 11 blocks, Got: %d\n", len(newFsys.data.blocks))
		}

	}
}
