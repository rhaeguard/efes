# efes

an attempt to create a simple file system.

### todo

- [ ] create a file
- [ ] read a file
- [ ] delete a file
- [ ] copy a file
- [ ] move/rename a file


### structure

Current structure:

```json
{
    "files": [
        {
            "name": "hello.txt",
            "first_block_ix": 10
        }
    ],
    "data": {
        "total_block_count": 100,
        "blocks": [
            {
                "next_block_ix": 11,
                "data": 1110011
            },
            {
                "next_block_ix": 12,
                "data": 1110011
            },
            {
                "next_block_ix": 13,
                "data": 1110011
            }
        ]
    }
}
```

The idea is that we have separate file metadata and actual raw bytes sections. The data sector is divided into multiple evenly sized blocks. Each block contains data and a reference to another block that's part of a file - basically forming a linked list to represent the raw data for a file.

I've implemented the basic FileSystem interface in Golang, to the point that it's (just) usable.

<img width="2107" height="810" alt="image" src="https://github.com/user-attachments/assets/53753270-eb78-407a-bff9-e264c6dab016" />
