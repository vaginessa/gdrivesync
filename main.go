package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func main() {
	// Load API key from environment variable
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY environment variable not set")
	}

	// Obtain a Drive service client using the API key
	client, err := drive.NewService(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Unable to create Drive service client: %v", err)
	}

	// Sync the specified local directory to Google Drive
	if err := syncToDrive(client, os.Getenv("SYNC_PATH")); err != nil {
		log.Fatalf("Error syncing to Google Drive: %v", err)
	}

	fmt.Println("Sync completed successfully.")
}

// Syncs the specified local directory to Google Drive
func syncToDrive(client *drive.Service, localDirectory string) error {
	// Retrieve existing files in Google Drive.
	driveFiles, err := listDriveFiles(client)
	if err != nil {
		return fmt.Errorf("Error listing Drive files: %v", err)
	}

	// Walk through the local directory and upload or update files in Google Drive.
	err = filepath.Walk(localDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fileName := filepath.Base(path)
			driveFile, exists := findDriveFile(driveFiles, fileName)

			if exists {
				// File exists in Google Drive, update it if changes were made.
				err := updateDriveFile(client, path, driveFile)
				if err != nil {
					log.Printf("Error updating file %s: %v", path, err)
				}
			} else {
				// File doesn't exist in Google Drive, upload it.
				err := uploadFile(client, path)
				if err != nil {
					log.Printf("Error uploading file %s: %v", path, err)
				}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("Error walking local directory: %v", err)
	}

	return nil
}

// Lists files in the user's Google Drive
func listDriveFiles(srv *drive.Service) ([]*drive.File, error) {
	files, err := srv.Files.List().Do()
	if err != nil {
		return nil, err
	}
	return files.Files, nil
}

// Helper function to find a file in the list of Google Drive files
func findDriveFile(driveFiles []*drive.File, fileName string) (*drive.File, bool) {
	for _, driveFile := range driveFiles {
		if driveFile.Name == fileName {
			return driveFile, true
		}
	}
	return nil, false
}

// Helper function to update a file in Google Drive
func updateDriveFile(client *drive.Service, localFilePath string, driveFile *drive.File) error {
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("Error opening file %s: %v", localFilePath, err)
	}
	defer file.Close()

	// Update the content of the existing file in Google Drive
	_, err = client.Files.Update(driveFile.Id, &drive.File{}).Media(file).Do()
	if err != nil {
		return fmt.Errorf("Error updating file %s in Google Drive: %v", localFilePath, err)
	}

	fmt.Printf("Updated file: %s\n", localFilePath)
	return nil
}

// Helper function to upload a file to Google Drive
func uploadFile(client *drive.Service, localFilePath string) error {
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("Error opening file %s: %v", localFilePath, err)
	}
	defer file.Close()

	// Upload the file to Google Drive
	_, err = client.Files.Create(&drive.File{Name: filepath.Base(localFilePath)}).Media(file).Do()
	if err != nil {
		return fmt.Errorf("Error uploading file %s to Google Drive: %v", localFilePath, err)
	}

	fmt.Printf("Uploaded file: %s\n", localFilePath)
	return nil
}
