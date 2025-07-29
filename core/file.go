package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/crypto/scrypt"
	"io"
	"os"
)

// Structure holding a file chunk
type Chunk struct {
	Data  []byte `json:"data"`
	Index int    `json:"index"`
}

// Secret key type used for encryption
type Key [32]byte

// Structure holding an encrypted chunk
type EncryptedChunk struct {
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
	Index      int    `json:"index"`
}

// Structure holding an encrypted file
type EncryptedFile struct {
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// Function that chunks a file given a filepath and a chunk size in MB
func ChunkFile(filepath string, chunkSizeMB int64) ([]*Chunk, error) {
	// Open the file and check for any errors. Defer the closing of the file for when the function returns
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Declare the chunks array and a buffer to hold the read chunks
	var chunks []*Chunk
	buffer := make([]byte, chunkSizeMB*1024*1024)
	counter := 0
	for {
		// Read the amount of bytes allowed in the buffer
		bytesRead, err := file.Read(buffer)

		// Check if the bytes read was greater than zero. This check is to prevent empty chunks if the file size is
		// a perfect multiple of the chunk size
		if bytesRead > 0 {
			// Create a copy of the bytes read and append to chunks (as buffer is only declared once outside loop)
			chunk := make([]byte, bytesRead)
			copy(chunk, buffer[:bytesRead])
			chunks = append(chunks, &Chunk{Data: chunk, Index: counter})
			counter++
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
func BuildFile(filepath string, chunks []*Chunk) error {
	// Creates a file given the filepath. If it already exists the file gets truncated
	// Check for any errors and defer the closing of the file until after the function returns
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Loop over every chunk and write it to the file (checking for an error at each)
	for _, chunk := range chunks {
		_, err := file.Write(chunk.Data)
		if err != nil {
			return err
		}
	}

	return nil
}

// Function used to generate a new secret cryptographic key for encryption
func NewKey() (*Key, error) {
	var key Key
	// Rand.reader generates cryptographically secure random bytes into the key variable
	// io.ReadFull is to ensure that the entire key length is produced
	_, err := io.ReadFull(rand.Reader, key[:])
	if err != nil {
		fmt.Printf("error generating a new secret cryptographic key: %s", err)
		return nil, err
	}
	return &key, nil
}

// Function used to encrypt a chunk
func EncryptChunk(chunk *Chunk, key *Key) (*EncryptedChunk, error) {
	// Create AES encryption engine
	block, err := aes.NewCipher(key[:])
	if err != nil {
		fmt.Printf("error creating AES cipher: %s", err)
		return nil, err
	}

	// Wrap the AES engine in the GCM mode of operation
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		fmt.Printf("error creating GCM: %s", err)
		return nil, err
	}

	// Create a nonce byte slice of the standard gcm size (12 bytes)
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		fmt.Printf("error creating nonce: %s", err)
		return nil, err
	}

	// Change the chunk index into a byte slice in a standard fashion with fixed length
	additionalData := make([]byte, 8)
	binary.BigEndian.PutUint64(additionalData, uint64(chunk.Index))

	// Create the ciphertext and also add GCM authentication
	// The chunk index is used as additional data that is authenticated to ensure the field has not been tampered with
	ciphertext := gcm.Seal(nil, nonce, chunk.Data, additionalData)

	return &EncryptedChunk{
		Nonce:      nonce,
		Ciphertext: ciphertext,
		Index:      chunk.Index,
	}, nil
}

// Function used to decrypt an encrypted chunk
func DecryptChunk(encryptedChunk *EncryptedChunk, key *Key) (*Chunk, error) {
	// Create AES encryption engine
	block, err := aes.NewCipher(key[:])
	if err != nil {
		fmt.Printf("error creating AES cipher: %s", err)
		return nil, err
	}

	// Wrap the AES encryption engine in the GCM mode of operation
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		fmt.Printf("error creating GCM: %s", err)
		return nil, err
	}

	// Check that the received nonce is the correct length
	if len(encryptedChunk.Nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}

	// Create a byte slice of the index received in a standard fashion
	additionalData := make([]byte, 8)
	binary.BigEndian.PutUint64(additionalData, uint64(encryptedChunk.Index))

	// Decrypt the ciphertext and check its GCM authenticity (including the received index)
	plaintext, err := gcm.Open(nil, encryptedChunk.Nonce, encryptedChunk.Ciphertext, additionalData)
	if err != nil {
		fmt.Printf("error decrypting encryptedChunk: %s", err)
		return nil, err
	}

	return &Chunk{Data: plaintext, Index: encryptedChunk.Index}, nil
}

// Function to encrypt a file given a user-provided password
func EncryptFile(plaintext []byte, password string) (*EncryptedFile, error) {
	// Generate a salt (random information that is used to generate secure key) to prevent two passwords from making
	// the same key
	salt := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		return nil, err
	}

	// Generate a 32 byte secure cryptographic key from the salt and user provided password
	key, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if err != nil {
		return nil, err
	}

	// Create AES encryption engine
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	// Wrap the AES engine in the gcm mode of operation
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Create a nonce byte slice of the standard gcm size (12 bytes)
	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	// Encrypt the file
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedFile{
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}

// Function used to decrypt a file given a user-provided password
func DecryptFile(encryptedFile *EncryptedFile, password string) ([]byte, error) {
	// Generate the same key used for encryption with the password and salt
	key, err := scrypt.Key([]byte(password), encryptedFile.Salt, 32768, 8, 1, 32)
	if err != nil {
		return nil, err
	}

	// Create a new AES encryption engine
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	// Wrap the engine in the AES mode of operation
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Check the provided nonce is the correct length
	if len(encryptedFile.Nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}

	// Decrypt the file
	plaintext, err := gcm.Open(nil, encryptedFile.Nonce, encryptedFile.Ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
