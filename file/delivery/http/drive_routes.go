package file

import (
	driveUsecase "larsa-files/file/usecase"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/gorilla/mux"
)

func SetupDriveRoutes(r *mux.Router, driveUsecase *driveUsecase.DriveUsecase) {
	driveHandler := NewDriveHandler(driveUsecase)
	driveUploadHandler := NewDriveUploadHandler(driveUsecase)

	// Drive API routes
	driveRouter := r.PathPrefix("/drive").Subrouter()

	// Health check
	driveRouter.HandleFunc("/health", common.MiddlewaresHandler(
		driveHandler.HealthCheck(),
	).ServeHTTP).Methods("GET")

	// Folder Management Routes
	driveRouter.HandleFunc("/folders", common.MiddlewaresHandler(
		driveHandler.CreateFolder(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("POST")

	driveRouter.HandleFunc("/folders/{id}", common.MiddlewaresHandler(
		driveHandler.GetFolder(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")

	driveRouter.HandleFunc("/folders/{id}", common.MiddlewaresHandler(
		driveHandler.UpdateFolder(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("PATCH")

	driveRouter.HandleFunc("/folders/{id}/protection", common.MiddlewaresHandler(
		driveHandler.UpdateFolderProtection(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("PATCH")

	driveRouter.HandleFunc("/folders/{id}", common.MiddlewaresHandler(
		driveHandler.DeleteFolder(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("DELETE")

	driveRouter.HandleFunc("/trash/empty", common.MiddlewaresHandler(
		driveHandler.EmptyTrash(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("DELETE")

	// Navigation and Browsing Routes
	driveRouter.HandleFunc("/folders/{id}/contents", common.MiddlewaresHandler(
		driveHandler.GetFolderContents(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")

	driveRouter.HandleFunc("/contents", common.MiddlewaresHandler(
		driveHandler.GetFolderContents(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")

	driveRouter.HandleFunc("/recent", common.MiddlewaresHandler(
		driveHandler.GetRecentItems(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")

	driveRouter.HandleFunc("/search", common.MiddlewaresHandler(
		driveHandler.SearchItems(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")

	// Deep search (files + folders recursively)
	driveRouter.HandleFunc("/deepsearch", common.MiddlewaresHandler(
		driveHandler.DeepSearchItems(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")

	// File Management Routes
	driveRouter.HandleFunc("/files", common.MiddlewaresHandler(
		driveUploadHandler.UploadFileToDrive(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("POST")

	driveRouter.HandleFunc("/files/bulk", common.MiddlewaresHandler(
		driveUploadHandler.UploadMultipleFilesToDrive(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("POST")

	driveRouter.HandleFunc("/files/{id}", common.MiddlewaresHandler(
		driveHandler.GetFile(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")

	driveRouter.HandleFunc("/files/{id}", common.MiddlewaresHandler(
		driveHandler.UpdateFile(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("PATCH")

	driveRouter.HandleFunc("/files/{id}", common.MiddlewaresHandler(
		driveHandler.DeleteFile(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("DELETE")

	// Move Operations
	driveRouter.HandleFunc("/items/{id}/move", common.MiddlewaresHandler(
		driveHandler.MoveItem(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("PATCH")

	// Copy Operations
	driveRouter.HandleFunc("/items/{id}/copy", common.MiddlewaresHandler(
		driveHandler.CopyItem(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("POST")

	// Bulk Operations
	driveRouter.HandleFunc("/bulk/move", common.MiddlewaresHandler(
		driveHandler.BulkMove(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("PATCH")

	driveRouter.HandleFunc("/bulk/copy", common.MiddlewaresHandler(
		driveHandler.BulkCopy(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("POST")
	// Bulk rename files
	driveRouter.HandleFunc("/bulk/rename", common.MiddlewaresHandler(
		driveHandler.BulkRename(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("PATCH")

	driveRouter.HandleFunc("/bulk/delete", common.MiddlewaresHandler(
		driveHandler.BulkDelete(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("DELETE")

	// Simple APIs for files and folders with pagination and filtering
	driveRouter.HandleFunc("/files", common.MiddlewaresHandler(
		driveHandler.GetFiles(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")
	driveRouter.HandleFunc("/files/filter", common.MiddlewaresHandler(
		driveHandler.FilterFiles(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("POST")

	driveRouter.HandleFunc("/folders", common.MiddlewaresHandler(
		driveHandler.GetFolders(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")

	// Folder Tree
	driveRouter.HandleFunc("/tree", common.MiddlewaresHandler(
		driveHandler.GetFolderTree(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")
}
