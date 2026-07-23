# Go zip library

[![CI](https://github.com/STARRY-S/zip/actions/workflows/ci.yaml/badge.svg)](https://github.com/STARRY-S/zip/actions/workflows/ci.yaml)

This project is based on the [archive/zip](https://github.com/golang/go/tree/master/src/archive/zip) Go standard library. It adds a new [Updater](updater.go) that allows appending new files to the existing zip archive without having to decompress the entire-file and allows overwriting of existing files stored in the zip archive.

## Usage

```go
import "github.com/STARRY-S/zip"

// Open an existing test.zip archive with read/write only mode for Updater.
f, err := os.OpenFile("test.zip", os.O_RDWR, 0)
handleErr(err)
zu, err := zip.NewUpdater(f)
handleErr(err)
defer zu.Close()

// Updater supports modify the zip comment.
err = zu.SetComment("Test update zip archive")
handleErr(err)

// Append a new file into existing archive.
// Use [zip.APPEND_MODE_OVERWRITE] to overwrite if the file already exists.
// The Append method will create a new io.Writer.
w, err := zu.Append("example.txt", zip.APPEND_MODE_OVERWRITE)
handleErr(err)
// Write data into writer.
_, err = w.Write([]byte("hello world"))
handleErr(err)
```

For more example usage, please refer to [updater_example_test.go](./updater_example_test.go).

## License

[BSD 3-Clause](LICENSE)

This zip library is based on the [Go standard library](https://github.com/golang/go).
