package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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

var Endianness = binary.LittleEndian

type Efes struct {
	files [MAX_FILECOUNT]efesFileEntry
	data  efesDataSector
}

const SizeOf_efesFileEntry = 130

type efesFileEntry struct {
	name         [FILENAME_LENGTH_LIMIT]byte // limit size, maybe bytes?
	firstBlockIx uint16                      // index of the block in data sector
}

const SizeOf_efesDataBlock = 4098

type efesDataSector struct {
	totalBlockCount uint16
	blocks          []efesDataBlock
}

type efesDataBlock struct {
	nextDataBlockIx uint16
	data            [BLOCK_SIZE]byte
}

type efesFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	mode    fs.FileMode
}

func NewEfesFileSystem(filepath string) (*Efes, error) {
	fd, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	fsys := Efes{}
	metadataBytes := make([]byte, MAX_FILECOUNT*SizeOf_efesFileEntry)
	if rb, err := fd.Read(metadataBytes); err != nil || rb != MAX_FILECOUNT*SizeOf_efesFileEntry {
		return nil, err
	}
	for i := range MAX_FILECOUNT - 1 {
		aFileEntry := metadataBytes[i*SizeOf_efesFileEntry : (i+1)*SizeOf_efesFileEntry]
		fsys.files[i].name = [FILENAME_LENGTH_LIMIT]byte(aFileEntry[0:FILENAME_LENGTH_LIMIT])
		fsys.files[i].firstBlockIx = Endianness.Uint16(aFileEntry[FILENAME_LENGTH_LIMIT : FILENAME_LENGTH_LIMIT+2])
	}

	totalBlockCountBytes := make([]byte, 2)
	if rb, err := fd.Read(totalBlockCountBytes); err != nil || rb != 2 {
		return nil, err
	}
	totalBlockCount := Endianness.Uint16(totalBlockCountBytes)
	fsys.data.totalBlockCount = totalBlockCount

	dataRawBytes := make([]byte, totalBlockCount*SizeOf_efesDataBlock)
	if rb, err := fd.Read(dataRawBytes); err != nil || rb != int(totalBlockCount)*SizeOf_efesDataBlock {
		return nil, err
	}

	for i := range totalBlockCount {
		aDataEntry := dataRawBytes[i*SizeOf_efesDataBlock : (i+1)*SizeOf_efesDataBlock]
		nextDataBlockIx := Endianness.Uint16(aDataEntry[0:2])
		data := [BLOCK_SIZE]byte(aDataEntry[2:])
		fsys.data.blocks = append(fsys.data.blocks, efesDataBlock{
			nextDataBlockIx: nextDataBlockIx,
			data:            data,
		})
	}

	return &fsys, nil
}

func (fsys *Efes) SizeInBytes() int {
	totalSize := SizeOf_efesFileEntry * MAX_FILECOUNT
	totalSize += (int(fsys.data.totalBlockCount) * SizeOf_efesDataBlock)
	return totalSize
}

func (fsys *Efes) Serialize(path string) error {
	fd, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer fd.Close()

	if err := fd.Truncate(int64(fsys.SizeInBytes())); err != nil {
		return err
	}

	for _, f := range fsys.files {
		fd.Write(f.GetBytes())
	}
	fd.Write(fsys.data.GetBytes())
	return nil
}

func (fe efesFileEntry) GetBytes() []byte {
	feBytes := make([]byte, SizeOf_efesFileEntry)
	copy(feBytes[:FILENAME_LENGTH_LIMIT], fe.name[:])
	Endianness.PutUint16(feBytes[FILENAME_LENGTH_LIMIT:], fe.firstBlockIx)
	return feBytes
}

func (feds efesDataSector) GetBytes() []byte {
	totalSize := (feds.totalBlockCount * SizeOf_efesDataBlock) + 2 // 2 for totalBlockCount uint16
	fedsBytes := make([]byte, totalSize)
	Endianness.PutUint16(fedsBytes[:2], feds.totalBlockCount)

	dataSectorBytes := fedsBytes[2:]
	for i := range feds.totalBlockCount {
		st := int(SizeOf_efesDataBlock * i)
		end := st + SizeOf_efesDataBlock
		block := feds.blocks[i]
		Endianness.PutUint16(dataSectorBytes[st:st+2], block.nextDataBlockIx)
		copy(dataSectorBytes[st+2:end], block.data[:])
	}

	return fedsBytes
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
	blockIx          uint16
}

func newFile(filename string, fsys *Efes, fsysIx int) *efesFile {
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

	return &efesFile{
		filename:         filename,
		fileInfo:         newEfesFileInfo(filename, int64(size), fs.FileMode(0)),
		returnedDirIndex: -1,
		fsys:             fsys,
		fsysIx:           fsysIx,
		offset:           0,
		blockIx:          me.firstBlockIx,
	}
}

func newDirectory(directoryName string, fsys *Efes) *efesFile {
	return &efesFile{
		filename:         directoryName,
		fileInfo:         newEfesFileInfo(directoryName, int64(0), fs.ModeDir),
		returnedDirIndex: -1,
		fsys:             fsys,
		fsysIx:           0,
		offset:           0,
		blockIx:          0,
		isDir:            false,
	}
}

func (f *efesFile) Stat() (fs.FileInfo, error) {
	return f.fileInfo, nil
}

func (f *efesFile) Read(p []byte) (int, error) {
	block := f.fsys.data.blocks[f.blockIx]

	requestedSize := len(p)
	availableInBlock := len(block.data) - f.offset

	copySize := min(availableInBlock, requestedSize)
	copy(p, block.data[f.offset:f.offset+copySize])

	f.offset += copySize
	isEof := block.nextDataBlockIx == 0 && f.offset == BLOCK_SIZE
	diff := requestedSize - copySize

	if diff == 0 {
		if isEof {
			return copySize, io.EOF
		}
		return copySize, nil
	} else {
		if isEof {
			return copySize, io.EOF
		}
		f.blockIx = block.nextDataBlockIx
		f.offset = 0
		subsequentCopySize, err := f.Read(p[copySize:])
		return subsequentCopySize + copySize, err
	}
}

func (f *efesFile) Close() error { return nil }

func (f *efesFile) ReadDir(n int) ([]fs.DirEntry, error) {
	fmt.Printf("FileReadDir: %d\n", n)

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
			return f, nil
		}
	}
	return nil, os.ErrNotExist
}

func (fsys Efes) ReadDir(name string) ([]fs.DirEntry, error) {
	fmt.Printf("ReadDir: %s\n", name)
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

func (fsys Efes) getDirectory(name string) *efesFile {
	var parentFolder string

	if name == "." {
		parentFolder = ""
	} else {
		parentFolder = name + "/"
	}

	curDir := newDirectory(parentFolder, &fsys)
	seenDir := map[string]bool{}

	for fsysIx, file := range fsys.files {
		if file.firstBlockIx == 0 {
			continue
		}
		childName := filenameFromFixedBytes(file.name)
		if strings.HasPrefix(childName, parentFolder) {
			childName = childName[len(parentFolder):]
			var child *efesFile

			if ix := strings.Index(childName, "/"); ix != -1 {
				dirName := childName[:ix]
				if ok := seenDir[dirName]; ok {
					continue
				}
				seenDir[dirName] = true
				child = newDirectory(dirName, &fsys)
			} else {
				child = newFile(childName, &fsys, fsysIx)
			}

			curDir.children = append(curDir.children, *child)
		}
	}

	if len(curDir.children) > 0 {
		return curDir
	}

	return nil
}

func main() {
	efes, err := NewEfesFileSystem("./virtual.img")
	if err != nil {
		panic(err.Error())
	}

	const useFileServer = true
	if useFileServer {
		Serve(efes)
	} else {
		CliInteraction(efes)
	}
}

func Serve(fsys fs.FS) {
	http.Handle("/", http.FileServer(http.FS(fsys)))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func CliInteraction(efes *Efes) {
	var currentlyOpenFile fs.File = nil

	for {
		fmt.Print("> ")
		var line string
		fmt.Scanln(&line)

		if line == "files" {
			for i, file := range efes.files {
				if file.firstBlockIx == 0 {
					continue
				}
				fmt.Printf("%3d. %s\n", i, filenameFromFixedBytes(file.name))
			}

		} else if strings.HasPrefix(line, "file:") {
			filename := strings.TrimSpace(line[5:])
			for ix, file := range efes.files {
				if filenameFromFixedBytes(file.name) == filename {
					file := newFile(filename, efes, ix)
					fmt.Printf("Name: %s\n", file.filename)
					fmt.Printf("Size: %d\n", file.fileInfo.size)
					fmt.Printf(" Dir: %t\n", file.fileInfo.IsDir())
				}
			}
		} else if strings.HasPrefix(line, "open:") {
			// open:hi.txt
			filename := strings.TrimSpace(line[5:])
			fd, err := efes.Open(filename)

			if err != nil {
				fmt.Printf("could not open the file: %s\n", err.Error())
				continue
			}
			currentlyOpenFile = fd
			fmt.Printf("Now open: %s\n", filename)
		} else if strings.HasPrefix(line, "print:") {
			// print:150
			bytesAmount, _ := strconv.Atoi(strings.TrimSpace(line[6:]))

			requestedBytes := make([]byte, bytesAmount)
			bytesAmount, err := currentlyOpenFile.Read(requestedBytes)

			if err != nil {
				fmt.Printf("could not read the file[%d]: %s\n", bytesAmount, err.Error())
				continue
			}

			fmt.Printf("%s\n", string(requestedBytes))
		} else if strings.HasPrefix(line, "quit") {
			fmt.Println("quitting")
			break
		} else {
			fmt.Printf("unrecognized command: %s\n", line)
		}
	}
}
