package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"time"
)

func filenameFromFixedBytes(arr [128]byte) string {
	raw := arr[:]
	end := bytes.IndexByte(raw, 0)
	if end == -1 {
		end = len(raw) // no null found, use full slice
	}
	return string(raw[:end])
}

const FILENAME_LENGTH_LIMIT = 128
const MAX_FILECOUNT = 200
const BLOCK_SIZE = 4 * 1024 // 4 kb

type efesFileEntry struct {
	name         [FILENAME_LENGTH_LIMIT]byte // limit size, maybe bytes?
	firstBlockIx uint16                      // index of the block in data sector
}

type efesDataBlock struct {
	nextDataBlockIx uint16
	data            [BLOCK_SIZE]byte
}

type efesDataSector struct {
	totalBlockCount uint16
	blocks          []efesDataBlock
}

type Efes struct {
	files [MAX_FILECOUNT]efesFileEntry
	data  efesDataSector
}

func (fsys Efes) getDirectory(name string) *efesFile {
	if name == "." || name == "./" {
		curDir := newFile(".", &fsys, 13) // rand number
		curDir.isDir = true
		curDir.returnedDirIndex = -1
		curDir.fileInfo.mode = fs.ModeDir

		for fsysIx, file := range fsys.files {
			if file.firstBlockIx == 0 {
				continue
			}
			curDir.children = append(curDir.children, newFile(filenameFromFixedBytes(file.name), &fsys, fsysIx))
		}

		return &curDir
	}

	return nil
}

type efesFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	mode    fs.FileMode
}

func newEfesFileInfo(name string, size int64, mode fs.FileMode) efesFileInfo {
	return efesFileInfo{
		name:    name,
		size:    size,
		modTime: time.Date(2025, time.August, 3, 14, 30, 0, 0, time.UTC),
		mode:    mode,
	}
}

func (fi efesFileInfo) Name() string               { return fi.name }
func (fi efesFileInfo) Size() int64                { return fi.size }
func (fi efesFileInfo) Mode() fs.FileMode          { return fi.mode }
func (fi efesFileInfo) Type() fs.FileMode          { return fi.mode }
func (fi efesFileInfo) ModTime() time.Time         { return fi.modTime }
func (fi efesFileInfo) IsDir() bool                { return fi.mode.IsDir() }
func (fi efesFileInfo) Sys() any                   { return nil }
func (fi efesFileInfo) Info() (fs.FileInfo, error) { return fi, nil }

type efesFile struct {
	filename         string
	children         []efesFile // filled if directory
	isDir            bool
	returnedDirIndex int
	fileInfo         efesFileInfo
	fsys             *Efes
	fsysIx           int
	offset           int
}

func newFile(filename string, fsys *Efes, fsysIx int) efesFile {
	me := fsys.files[fsysIx]
	size := 0
	nextBlockIx := me.firstBlockIx
	for {
		block := fsys.data.blocks[nextBlockIx]
		size += BLOCK_SIZE
		if block.nextDataBlockIx == 0 {
			break
		}
		nextBlockIx = block.nextDataBlockIx
	}

	return efesFile{
		filename:         filename,
		fileInfo:         newEfesFileInfo(filename, int64(size), fs.FileMode(0)),
		returnedDirIndex: -1,
		fsys:             fsys,
		fsysIx:           fsysIx,
		offset:           0,
	}
}

func (f *efesFile) Stat() (fs.FileInfo, error) {
	return f.fileInfo, nil
}

func (f *efesFile) Read(p []byte) (int, error) {
	me := f.fsys.files[f.fsysIx]
	block := f.fsys.data.blocks[me.firstBlockIx]

	copySize := min(len(block.data)-f.offset, len(p))
	if f.offset == len(block.data) {
		return 0, io.EOF
	}
	copy(p, block.data[f.offset:f.offset+copySize])
	f.offset += copySize
	return copySize, nil
}

func (f *efesFile) Close() error { return nil }
func (f *efesFile) ReadDir(n int) ([]fs.DirEntry, error) {
	fileInfos := []fs.DirEntry{}

	if n <= 0 {
		for ix, child := range f.children {
			if ix <= f.returnedDirIndex {
				continue
			}
			fileInfos = append(fileInfos, child.fileInfo)
			f.returnedDirIndex = ix
		}
		return fileInfos, nil
	} else {
		if f.returnedDirIndex == len(f.children)-1 {
			return fileInfos, io.EOF
		}
		for ix, child := range f.children {
			if ix <= f.returnedDirIndex || n <= 0 {
				continue
			}
			fileInfos = append(fileInfos, child.fileInfo)
			f.returnedDirIndex = ix
			n -= 1
		}
		return fileInfos, nil
	}
}

func (fsys Efes) Open(name string) (fs.File, error) {
	if dir := fsys.getDirectory(name); dir != nil {
		return dir, nil
	}
	for fsysIx, fileEntry := range fsys.files {
		theName := filenameFromFixedBytes(fileEntry.name)
		if theName == name {
			f := newFile(theName, &fsys, fsysIx)
			return &f, nil
		}
	}
	return nil, os.ErrNotExist
}

func (fsys Efes) ReadDir(name string) ([]fs.DirEntry, error) {
	if dir := fsys.getDirectory(name); dir != nil {
		fileInfos := []fs.DirEntry{}

		for _, child := range dir.children {
			fileInfos = append(fileInfos, newEfesFileInfo(
				child.filename,
				1,
				fs.FileMode(0),
			))
		}

		return fileInfos, nil
	}
	return nil, nil
}

func main() {
	// fileref, err := os.OpenFile("./virtual_file.img", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)

	// if err != nil {
	// 	panic("file not opened")
	// }

	// fileref.Truncate(10 * 1024 * 1024) // 10MB

	// defer fileref.Close()
}
