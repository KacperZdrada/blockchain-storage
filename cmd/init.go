package cmd

import (
	"blockchain-storage/core"
	"encoding/json"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialises the application for first time use",
	Long: `This command is used to initialise the application. It takes one mandatory argument, which is the password
			that will be used to encrypt all the files used for encryption. It also creates key files and folders that
			are necessary for the application to work`,
	Args: cobra.ExactArgs(1), // There is exactly one mandatory argument which is the password
	RunE: func(cmd *cobra.Command, args []string) error {
		// Make the directory where all application data will be held
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		// Create a filepath to the app directory (prefix with a '.' to make it hidden on Linux)
		appDir := filepath.Join(homeDir, ".blockchain-storage")
		err = os.MkdirAll(appDir, 0755)
		if err != nil {
			return err
		}

		// Create an empty map between strings (filenames) and keys (keys for encryption)
		filenameKeyMap := make(map[string]core.Key)
		bytes, err := json.Marshal(filenameKeyMap)
		if err != nil {
			return err
		}

		// Encrypt this empty map
		encryptedFile, err := core.EncryptFile(bytes, args[0])
		if err != nil {
			return err
		}

		// Convert the struct holding all encryption details into bytes
		fileBytes, err := json.Marshal(encryptedFile)
		if err != nil {
			return err
		}

		// Write the encrypted file struct
		err = os.WriteFile("keys.enc", fileBytes, 0600)
		if err != nil {
			return err
		}

		// Make the storage directory where all chunks will be held
		storageDir := filepath.Join(appDir, "chunk-storage")
		err = os.Mkdir(storageDir, 0755)
		if err != nil {
			return err
		}

		// Exit successfully
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}
