// Copyright 2025-2026 The MathWorks, Inc.

package pathvalidator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matlab/matlab-mcp-server/internal/usecases/utils/pathvalidator"
	osfacademocks "github.com/matlab/matlab-mcp-server/mocks/facades/osfacade"
	mocks "github.com/matlab/matlab-mcp-server/mocks/usecases/utils/pathvalidator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_HappyPath(t *testing.T) {
	// Arrange
	mockOsLayer := &mocks.MockOSLayer{}

	// Act
	validator := pathvalidator.New(mockOsLayer)

	// Assert
	assert.NotNil(t, validator, "New() should return a non-nil Validator")
}

func TestValidator_ValidateMATLABSourceFile_HappyPath(t *testing.T) {
	// Arrange
	mockOsLayer := &mocks.MockOSLayer{}
	defer mockOsLayer.AssertExpectations(t)

	mockFileInfo := &osfacademocks.MockFileInfo{}
	defer mockFileInfo.AssertExpectations(t)

	validator := pathvalidator.New(mockOsLayer)

	testPath, absErr := filepath.Abs("test.m")
	require.NoError(t, absErr)

	mockOsLayer.EXPECT().
		Stat(testPath).
		Return(mockFileInfo, nil).
		Once()

	mockFileInfo.EXPECT().
		IsDir().
		Return(false).
		Once()

	// Act
	result, err := validator.ValidateMATLABSourceFile(testPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, testPath, result)
}

func TestValidator_ValidateMATLABSourceFile_RejectsPCode(t *testing.T) {
	// Arrange
	mockOsLayer := &mocks.MockOSLayer{}
	defer mockOsLayer.AssertExpectations(t)

	validator := pathvalidator.New(mockOsLayer)

	testPath, absErr := filepath.Abs("test.p")
	require.NoError(t, absErr)

	// Act
	_, err := validator.ValidateMATLABSourceFile(testPath)

	// Assert
	require.ErrorIs(t, err, pathvalidator.ErrUnsupportedFileExtension)
}

func TestValidator_ValidateMATLABCodeFile_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
	}{
		{
			name:     "MATLAB source file",
			fileName: "test.m",
		},
		{
			name:     "P-code file",
			fileName: "test.p",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockOsLayer := &mocks.MockOSLayer{}
			defer mockOsLayer.AssertExpectations(t)

			mockFileInfo := &osfacademocks.MockFileInfo{}
			defer mockFileInfo.AssertExpectations(t)

			validator := pathvalidator.New(mockOsLayer)

			testPath, absErr := filepath.Abs(tt.fileName)
			require.NoError(t, absErr)

			mockOsLayer.EXPECT().
				Stat(testPath).
				Return(mockFileInfo, nil).
				Once()

			mockFileInfo.EXPECT().
				IsDir().
				Return(false).
				Once()

			// Act
			result, err := validator.ValidateMATLABCodeFile(testPath)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, testPath, result)
		})
	}
}

func TestValidator_ValidateMATLABCodeFile_InvalidPath(t *testing.T) {
	folderPath, absErr := filepath.Abs("./")
	require.NoError(t, absErr)

	tests := []struct {
		name        string
		filePath    string
		expectedErr error
	}{
		{
			name:        "MATLAB file with relative path",
			filePath:    filepath.Join(".", "relative", "folder", "test.m"),
			expectedErr: pathvalidator.ErrInvalidAbsolutePath,
		},
		{
			name:        "Folder path",
			filePath:    folderPath,
			expectedErr: pathvalidator.ErrUnsupportedFileExtension,
		},
		{
			name:        "Empty path",
			filePath:    "",
			expectedErr: pathvalidator.ErrInvalidAbsolutePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockOsLayer := &mocks.MockOSLayer{}
			defer mockOsLayer.AssertExpectations(t)

			validator := pathvalidator.New(mockOsLayer)

			// Act
			_, err := validator.ValidateMATLABCodeFile(tt.filePath)

			// Assert
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestValidator_ValidateMATLABCodeFile_NotMATLABFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
	}{
		{
			name:     "Non-MATLAB file",
			fileName: "file.txt",
		},
		{
			name:     "File without extension",
			fileName: "noextension",
		},
		{
			name:     "Capital M",
			fileName: "script.M",
		},
		{
			name:     "Capital P",
			fileName: "script.P",
		},
		{
			name:     "Live Script file",
			fileName: "script.mlx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockOsLayer := &mocks.MockOSLayer{}
			defer mockOsLayer.AssertExpectations(t)

			validator := pathvalidator.New(mockOsLayer)

			filePath, absErr := filepath.Abs(tt.fileName)
			require.NoError(t, absErr)

			// Act
			_, err := validator.ValidateMATLABCodeFile(filePath)

			// Assert
			require.ErrorIs(t, err, pathvalidator.ErrUnsupportedFileExtension)
		})
	}
}

func TestValidator_ValidateMATLABCodeFile_PathIsAFolder(t *testing.T) {
	// Arrange
	mockOsLayer := &mocks.MockOSLayer{}
	defer mockOsLayer.AssertExpectations(t)

	mockFileInfo := &osfacademocks.MockFileInfo{}
	defer mockFileInfo.AssertExpectations(t)

	validator := pathvalidator.New(mockOsLayer)

	// path has .m extension to pass suffix check but is registered as a folder
	testPath, absErr := filepath.Abs("folder.m")
	require.NoError(t, absErr)

	mockOsLayer.EXPECT().
		Stat(testPath).
		Return(mockFileInfo, nil).
		Once()

	mockFileInfo.EXPECT().
		IsDir().
		Return(true).
		Once()

	// Act
	_, err := validator.ValidateMATLABCodeFile(testPath)

	// Assert
	require.ErrorIs(t, err, pathvalidator.ErrPathNotFile)
}

func TestValidator_ValidateMATLABCodeFile_StatFails(t *testing.T) {
	// Arrange
	mockOsLayer := &mocks.MockOSLayer{}
	defer mockOsLayer.AssertExpectations(t)

	testPath, absErr := filepath.Abs("test.m")
	require.NoError(t, absErr)

	mockOsLayer.EXPECT().
		Stat(testPath).
		Return(nil, os.ErrNotExist).
		Once()

	validator := pathvalidator.New(mockOsLayer)

	// Act
	_, err := validator.ValidateMATLABCodeFile(testPath)

	// Assert
	require.ErrorIs(t, err, pathvalidator.ErrResourceNotFound)
}

func TestValidator_ValidateMATLABCodeFile_PathCleaning(t *testing.T) {
	testPath, absErr := filepath.Abs("test.m")
	require.NoError(t, absErr)

	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "Path with double slashes",
			filePath: strings.ReplaceAll(testPath, string(filepath.Separator), string(filepath.Separator)+string(filepath.Separator)),
			expected: testPath,
		},
		{
			name:     "Path with dot segments",
			filePath: filepath.Join(filepath.Dir(testPath), ".", "test.m"),
			expected: testPath,
		},
		{
			name:     "Path with dot-dot segments",
			filePath: filepath.Join(filepath.Dir(testPath), "subdir", "..", "test.m"),
			expected: testPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockOsLayer := &mocks.MockOSLayer{}
			defer mockOsLayer.AssertExpectations(t)

			mockFileInfo := &osfacademocks.MockFileInfo{}
			defer mockFileInfo.AssertExpectations(t)

			validator := pathvalidator.New(mockOsLayer)

			mockOsLayer.EXPECT().
				Stat(tt.expected).
				Return(mockFileInfo, nil).
				Once()

			mockFileInfo.EXPECT().
				IsDir().
				Return(false).
				Once()

			// Act
			result, err := validator.ValidateMATLABCodeFile(tt.filePath)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidator_ValidateFolderPath_HappyPath(t *testing.T) {
	// Arrange
	mockOsLayer := &mocks.MockOSLayer{}
	defer mockOsLayer.AssertExpectations(t)

	mockFileInfo := &osfacademocks.MockFileInfo{}
	defer mockFileInfo.AssertExpectations(t)

	validator := pathvalidator.New(mockOsLayer)

	testPath, absErr := filepath.Abs("./")
	require.NoError(t, absErr)

	mockOsLayer.EXPECT().
		Stat(testPath).
		Return(mockFileInfo, nil).
		Once()

	mockFileInfo.EXPECT().
		IsDir().
		Return(true).
		Once()

	// Act
	result, err := validator.ValidateFolderPath(testPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, testPath, result)
}

func TestValidator_ValidateFolderPath_FailsForRelativePath(t *testing.T) {
	// Arrange
	mockOsLayer := &mocks.MockOSLayer{}
	defer mockOsLayer.AssertExpectations(t)

	validator := pathvalidator.New(mockOsLayer)

	testPath := filepath.Join(".", "relative", "folder")

	// Act
	_, err := validator.ValidateFolderPath(testPath)

	// Assert
	require.ErrorIs(t, err, pathvalidator.ErrInvalidAbsolutePath)
}

func TestValidator_ValidateFolderPath_FailsForFilePath(t *testing.T) {
	// Arrange
	mockOsLayer := &mocks.MockOSLayer{}
	defer mockOsLayer.AssertExpectations(t)

	mockFileInfo := &osfacademocks.MockFileInfo{}
	defer mockFileInfo.AssertExpectations(t)

	validator := pathvalidator.New(mockOsLayer)

	testPath, absErr := filepath.Abs("test.m")
	require.NoError(t, absErr)

	mockOsLayer.EXPECT().
		Stat(testPath).
		Return(mockFileInfo, nil).
		Once()

	mockFileInfo.EXPECT().
		IsDir().
		Return(false).
		Once()

	// Act
	_, err := validator.ValidateFolderPath(testPath)

	// Assert
	require.ErrorIs(t, err, pathvalidator.ErrPathNotFolder)
}

func TestValidator_ValidateFolderPath_StatFails(t *testing.T) {
	// Arrange
	mockOsLayer := &mocks.MockOSLayer{}
	defer mockOsLayer.AssertExpectations(t)

	testPath, absErr := filepath.Abs("./")
	require.NoError(t, absErr)

	mockOsLayer.EXPECT().
		Stat(testPath).
		Return(nil, os.ErrNotExist).
		Once()

	validator := pathvalidator.New(mockOsLayer)

	// Act
	_, err := validator.ValidateFolderPath(testPath)

	// Assert
	require.ErrorIs(t, err, pathvalidator.ErrResourceNotFound)
}
