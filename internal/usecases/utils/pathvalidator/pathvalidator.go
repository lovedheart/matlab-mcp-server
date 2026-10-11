// Copyright 2025-2026 The MathWorks, Inc.

package pathvalidator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/matlab/matlab-mcp-server/internal/facades/osfacade"
)

var (
	ErrUnsupportedFileExtension = errors.New("file must be a MATLAB file")
	ErrPathNotFile              = errors.New("path is not a file")
	ErrPathNotFolder            = errors.New("path is not a folder")
	ErrResourceNotFound         = errors.New("resource not found")
	ErrInvalidAbsolutePath      = errors.New("not a valid absolute path")
)

type OSLayer interface {
	Stat(filePath string) (osfacade.FileInfo, error)
}

type PathValidator struct {
	osLayer OSLayer
}

func New(
	osLayer OSLayer,
) *PathValidator {
	return &PathValidator{
		osLayer: osLayer,
	}
}

func (v *PathValidator) ValidateMATLABSourceFile(filePath string) (string, error) {
	return v.validateMATLABFile(filePath, ".m")
}

func (v *PathValidator) ValidateMATLABCodeFile(filePath string) (string, error) {
	return v.validateMATLABFile(filePath, ".m", ".p")
}

func (v *PathValidator) validateMATLABFile(filePath string, allowedExts ...string) (string, error) {
	absPath, err := resolveAbsolutePath(filePath)
	if err != nil {
		return "", err
	}

	// Check the extension before doing any file system operations
	if !hasAllowedExtension(absPath, allowedExts) {
		return "", fmt.Errorf("%w (%s): %s", ErrUnsupportedFileExtension, strings.Join(allowedExts, ", "), absPath)
	}

	fileInfo, err := v.getResourceInfo(absPath)
	if err != nil {
		return "", err
	}

	if fileInfo.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrPathNotFile, absPath)
	}

	return absPath, nil
}

// hasAllowedExtension matches only the final extension (via filepath.Ext), so
// multi-part extensions such as ".tar.gz" are not supported.
func hasAllowedExtension(absPath string, allowedExts []string) bool {
	return slices.Contains(allowedExts, filepath.Ext(absPath))
}

func (v *PathValidator) ValidateFolderPath(filePath string) (string, error) {
	absPath, err := resolveAbsolutePath(filePath)
	if err != nil {
		return "", err
	}

	folderInfo, err := v.getResourceInfo(absPath)
	if err != nil {
		return "", err
	}

	if !folderInfo.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrPathNotFolder, absPath)
	}

	return absPath, nil
}

func (v *PathValidator) getResourceInfo(filePath string) (osfacade.FileInfo, error) {
	resourceInfo, err := v.osLayer.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrResourceNotFound, filePath)
		}
		return nil, fmt.Errorf("error accessing resource: %w", err)
	}

	return resourceInfo, nil
}

func resolveAbsolutePath(filePath string) (string, error) {
	cleanPath := filepath.Clean(filePath)

	if !filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("%s is %w", cleanPath, ErrInvalidAbsolutePath)
	}

	return cleanPath, nil
}
