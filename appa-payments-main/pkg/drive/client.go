package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Client defines the methods for interacting with Google Drive.
type Client interface {
	UploadFile(ctx context.Context, fileHeader *multipart.FileHeader) (string, error)
	DeleteFile(ctx context.Context, fileID string) error
}

// Client wraps the Google Drive service.
type client struct {
	service  *drive.Service
	folderID string
	logger   *zap.Logger
}

// NewClient creates a new Google Drive client using the provided context and credentials file.
func NewClient(ctx context.Context, credentials, folderID, tokenData string, logger *zap.Logger) (Client, error) {

	config, err := google.ConfigFromJSON([]byte(credentials), drive.DriveFileScope)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	token := &oauth2.Token{}
	if err := json.Unmarshal([]byte(tokenData), token); err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	httpClient := config.Client(ctx, token)

	srv, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	// query := fmt.Sprintf("'%s' in parents", folderID)
	// files, err := srv.Files.List().Q(query).Fields("files(id, name, webViewLink, webContentLink)").Context(ctx).Do()
	// if err != nil {
	// 	logger.Error(err.Error(), zap.Any("folderID", folderID))
	// 	return nil, err
	// }

	// logger.Info("Google Drive folder contents", zap.Any("files", files.Files))

	return &client{service: srv, folderID: folderID, logger: logger}, nil
}

func generateUUID() string {
	return uuid.New().String()
}

// UploadFile uploads a file to Google Drive.
func (c *client) UploadFile(
	ctx context.Context,
	fileHeader *multipart.FileHeader,
) (string, error) {
	// 1. Open the file from the multipart.FileHeader
	file, err := fileHeader.Open()
	if err != nil {
		c.logger.Error(err.Error(), zap.Any("fileHeader", fileHeader))
		return "", err
	}
	defer file.Close()

	// 2. get mime type
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// 3. Create the Google Drive file metadata
	driveFile := &drive.File{
		Name:     fmt.Sprintf("%s_%s", generateUUID(), fileHeader.Filename),
		MimeType: mimeType,
		Parents:  []string{c.folderID},
	}

	// 4. Upload the file to Google Drive
	res, err := c.service.Files.
		Create(driveFile).
		Context(ctx).
		Media(file).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		c.logger.Error(err.Error(), zap.Any("driveFile", driveFile))
		return "", err
	}

	webViewLink, err := c.getWebViewLinkByID(ctx, res.Id)
	if err != nil {
		return "", err
	}

	// 5. Public access
	permission := &drive.Permission{
		Type: "anyone",
		Role: "reader",
	}
	_, err = c.service.Permissions.Create(res.Id, permission).Context(ctx).Do()
	if err != nil {
		c.logger.Error(err.Error(), zap.Any("permission", permission))
		return "", err
	}

	return webViewLink, nil
}

// FindFileIDByWebViewLink finds a file in Google Drive by its webViewLink.
func (c *client) FindFileIDByWebViewLink(ctx context.Context, webViewLink string) (string, error) {
	query := fmt.Sprintf("webViewLink='%s'", webViewLink)
	files, err := c.service.Files.List().Q(query).Fields("files(id, name, webViewLink, webContentLink)").Context(ctx).Do()
	if err != nil {
		c.logger.Error(err.Error(), zap.Any("webViewLink", webViewLink))
		return "", err
	}

	if len(files.Files) == 0 {
		return "", fmt.Errorf("file not found")
	}

	return files.Files[0].Id, nil
}

// DeleteFileByWebViewLink deletes a file from Google Drive by its ID.
func (c *client) DeleteFile(ctx context.Context, webViewLink string) error {
	fileID := strings.TrimPrefix(webViewLink, "https://drive.google.com/file/d/")
	fileID = strings.TrimSuffix(fileID, "/view?usp=drivesdk")

	err := c.service.Files.Delete(fileID).Context(ctx).Do()
	if err != nil {
		c.logger.Error(err.Error(), zap.Any("fileID", fileID))
		return err
	}
	return nil
}

// getWebViewLinkByID retrieves a file's webViewLink from Google Drive by its ID.
func (c *client) getWebViewLinkByID(ctx context.Context, fileID string) (string, error) {
	file, err := c.service.Files.Get(fileID).Context(ctx).
		SupportsAllDrives(true).
		Fields("id, name, webViewLink, webContentLink").
		Do()
	if err != nil {
		c.logger.Error(err.Error(), zap.Any("fileID", fileID))
		return "", err
	}

	if file.WebViewLink == "" {
		c.logger.Error("file.WebViewLink is empty", zap.Any("fileID", fileID))
		return "", fmt.Errorf("file.WebViewLink is empty")
	}

	return file.WebViewLink, nil
}
