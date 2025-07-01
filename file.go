package blockchain_storage

import (
	"io"
	"os"
)

// Function that chunks a file given a filepath and a chunk size in MB
func chunkFile(filepath string, chunkSizeMB int64) ([][]byte, error) {
	// Open the file and check for any errors. Defer the closing of the file for when the function returns
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Declare the chunks array and a buffer to hold the read chunks
	var chunks [][]byte
	buffer := make([]byte, chunkSizeMB*1024*1024)
	for {
		// Read the amount of bytes allowed in the buffer
		bytesRead, err := file.Read(buffer)

		// Check if the bytes read was greater than zero. This check is to prevent empty chunks if the file size is
		// a perfect multiple of the chunk size
		if bytesRead > 0 {
			// Create a copy of the bytes read and append to chunks (as buffer is only declared once outside loop)
			chunk := make([]byte, bytesRead)
			copy(chunk, buffer[:bytesRead])
			chunks = append(chunks, chunk)
		}
		if err != nil {
			// If the error is an end of file, must break out of the loop as no more bytes to read
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return chunks, nil
}

// Function that builds a file from its chunks
func buildFile(filepath string, chunks [][]byte) error {
	// Creates a file given the filepath. If it already exists the file gets truncated
	// Check for any errors and defer the closing of the file until after the function returns
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Loop over every chunk and write it to the file (checking for an error at each)
	for _, chunk := range chunks {
		_, err := file.Write(chunk)
		if err != nil {
			return err
		}
	}

	return nil
}
