package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
)

// Example usage of the Drive Module API
func main() {
	baseURL := "http://localhost:3200/drive"

	// Example 1: Create a folder
	createFolderExample(baseURL)

	// Example 2: Upload a file to a folder
	uploadFileExample(baseURL)

	// Example 3: Search for files
	searchFilesExample(baseURL)

	// Example 4: Move files between folders
	moveFilesExample(baseURL)

	// Example 5: Bulk operations
	bulkOperationsExample(baseURL)
}

func createFolderExample(baseURL string) {
	fmt.Println("=== Creating Folder Example ===")

	// Create root folder
	folderData := map[string]interface{}{
		"name": "My Documents",
	}

	jsonData, _ := json.Marshal(folderData)
	resp, err := http.Post(baseURL+"/folders", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error creating folder: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Created folder: %+v\n", result)
}

func uploadFileExample(baseURL string) {
	fmt.Println("=== Upload File Example ===")

	// Create a test file
	fileContent := []byte("This is a test file content")

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file
	fileWriter, _ := writer.CreateFormFile("file", "test-document.txt")
	fileWriter.Write(fileContent)

	// Add folder ID (optional)
	writer.WriteField("folderId", "your-folder-id-here")

	writer.Close()

	// Make request
	req, _ := http.NewRequest("POST", baseURL+"/files", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error uploading file: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Uploaded file: %+v\n", result)
}

func searchFilesExample(baseURL string) {
	fmt.Println("=== Search Files Example ===")

	// Search for files
	resp, err := http.Get(baseURL + "/search?q=document&type=file")
	if err != nil {
		fmt.Printf("Error searching files: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Search results: %+v\n", result)
}

func moveFilesExample(baseURL string) {
	fmt.Println("=== Move Files Example ===")

	// Move a file to a different folder
	moveData := map[string]interface{}{
		"targetFolderId": "destination-folder-id",
	}

	jsonData, _ := json.Marshal(moveData)
	req, _ := http.NewRequest("PATCH", baseURL+"/items/file-id/move", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error moving file: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Moved file: %+v\n", result)
}

func bulkOperationsExample(baseURL string) {
	fmt.Println("=== Bulk Operations Example ===")

	// Bulk move files
	bulkMoveData := map[string]interface{}{
		"itemIds":  []string{"file1-id", "file2-id", "file3-id"},
		"action":   "move",
		"targetId": "destination-folder-id",
	}

	jsonData, _ := json.Marshal(bulkMoveData)
	req, _ := http.NewRequest("PATCH", baseURL+"/bulk/move", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error bulk moving files: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Bulk move result: %+v\n", result)
}

// Frontend Integration Example
func frontendIntegrationExample() {
	fmt.Println("=== Frontend Integration Example ===")

	// This shows how the API responses would be used in a frontend
	// to build a drive-like interface similar to your Figma designs

	// Example API response structure for folder contents
	folderContentsResponse := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"_id":       "folder-id-1",
				"name":      "Documents",
				"type":      "folder",
				"size":      0,
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-01T00:00:00Z",
			},
			{
				"_id":       "file-id-1",
				"name":      "report.pdf",
				"type":      "file",
				"size":      1024000,
				"mimeType":  "application/pdf",
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-01T00:00:00Z",
			},
		},
		"pagination": map[string]interface{}{
			"totalPages": 5,
			"perPage":    10,
			"totalCount": 50,
		},
	}

	fmt.Printf("Frontend would receive: %+v\n", folderContentsResponse)

	// Example of how to handle different file types for icons
	fileTypeIcons := map[string]string{
		"application/pdf":        "pdf-icon",
		"image/jpeg":             "image-icon",
		"video/mp4":              "video-icon",
		"application/zip":        "archive-icon",
		"text/plain":             "text-icon",
		"application/json":       "code-icon",
		"application/javascript": "js-icon",
	}

	fmt.Printf("File type icons mapping: %+v\n", fileTypeIcons)
}
