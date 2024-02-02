package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	tokenFile       = "token.json"
	localFolderPath = "C:\\testsync\\"
	gDriveFolderID  = "folder_identifier"
)

// File represents a local file
type File struct {
	Name string
	Path string
}

// uploadToGoogleDrive uploads a local file to Google Drive.
func uploadToGoogleDrive(service *drive.Service, localFilePath, parentFolderID string) error {
	file, err := os.Open(localFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileName := filepath.Base(localFilePath)

	// Check if the file already exists on Google Drive
	if existingFileID := getDriveFileID(service, fileName, parentFolderID); existingFileID != "" {
		fmt.Printf("Updating %s on Google Drive...\n", fileName)

		_, err := service.Files.Update(existingFileID, nil).Media(file).Do()
		return err
	}

	// File doesn't exist, create a new file
	fmt.Printf("Uploading %s to Google Drive...\n", fileName)
	driveFile := &drive.File{
		Name:     fileName,
		Parents:  []string{parentFolderID},
		MimeType: "application/octet-stream",
	}

	_, err = service.Files.Create(driveFile).Media(file).Do()
	return err
}

// getDriveFileID retrieves the ID of an existing file on Google Drive.
func getDriveFileID(service *drive.Service, fileName, parentFolderID string) string {
	query := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false", fileName, parentFolderID)
	files, err := service.Files.List().Q(query).Do()
	if err != nil {
		log.Printf("Error checking if file exists: %v\n", err)
		return ""
	}

	if len(files.Files) > 0 {
		return files.Files[0].Id
	}

	return ""
}

// getTokenFromWeb uses Config to request a Token. It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	// Start a local server to receive the authorization code
	authCodeCh := make(chan string)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		authCodeCh <- code
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Authorization code received. You can now close this window."))
	})

	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err)
		}
	}()

	// Open the user's default web browser
	openBrowser(authURL)

	// Wait for the authorization code
	code := <-authCodeCh

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		log.Fatalf("Unable to exchange code for token: %v", err)
	}

	return tok, nil
}

// getClient uses a Context and Config to retrieve a Token then generate a Client. It returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tok, err := tokenFromFile()
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			log.Fatalf("Unable to retrieve token from web: %v", err)
		}
		saveToken(tok)
	}
	return config.Client(context.Background(), tok)
}

// tokenFromFile retrieves a Token from a local file.
func tokenFromFile() (*oauth2.Token, error) {
	file, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.Unmarshal(file, tok)
	return tok, err
}

// saveToken saves a token to a local file.
func saveToken(token *oauth2.Token) {
	data, err := json.Marshal(token)
	if err != nil {
		log.Fatalf("Unable to marshal token: %v", err)
	}
	err = os.WriteFile(tokenFile, data, 0644)
	if err != nil {
		log.Fatalf("Unable to write token file: %v", err)
	}
}

// openBrowser opens the default web browser to the specified URL.
func openBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

// listLocalFiles returns a list of files in the specified local folder.
func listLocalFiles(folderPath string) ([]File, error) {
	var files []File
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(folderPath, path)
			if err != nil {
				return err
			}
			files = append(files, File{Name: relPath, Path: path})
		}
		return nil
	})
	return files, err
}

// syncFolder uploads new or modified local files to Google Drive.
func syncFolder(service *drive.Service, localFolderPath, parentFolderID string) error {
	localFiles, err := listLocalFiles(localFolderPath)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, localFile := range localFiles {
		wg.Add(1)
		go func(file File) {
			defer wg.Done()

			// Upload the file to Google Drive (with overwrite)
			err := uploadToGoogleDrive(service, file.Path, parentFolderID)
			if err != nil {
				log.Printf("Error syncing %s: %v\n", file.Name, err)
			}
		}(localFile)
	}

	wg.Wait()

	return nil
}

// fileExistsOnDrive checks if a file with the given name exists in the specified Google Drive folder.
func fileExistsOnDrive(service *drive.Service, fileName, parentFolderID string) bool {
	query := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false", fileName, parentFolderID)
	files, err := service.Files.List().Q(query).Do()
	if err != nil {
		log.Printf("Error checking if file exists: %v\n", err)
		return false
	}
	return len(files.Files) > 0
}

func main() {
	// Retrieve OAuth configuration from environment variables
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("Missing CLIENT_ID or CLIENT_SECRET environment variables")
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  "http://localhost:8080", // Use local server for redirect URI
		Scopes: []string{
			"https://www.googleapis.com/auth/drive.file", // Adjust scope as needed
		},
		Endpoint: google.Endpoint,
	}

	client := getClient(config)

	// Use the client to interact with the Google Drive API
	service, err := drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to create Drive service: %v", err)
	}

	// Example: Sync local folder with Google Drive
	err = syncFolder(service, localFolderPath, gDriveFolderID)
	if err != nil {
		log.Fatalf("Error syncing folder: %v", err)
	}

	fmt.Println("Sync complete.")
}
