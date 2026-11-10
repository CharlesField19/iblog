package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const maxFaviconBytes = 128 << 10

func validFavicon(data []byte) bool {
	if len(data) < 22 || !bytes.Equal(data[:4], []byte{0, 0, 1, 0}) {
		return false
	}
	count := int(data[4]) | int(data[5])<<8
	directoryEnd := 6 + count*16
	if count == 0 || len(data) < directoryEnd {
		return false
	}
	for i := 0; i < count; i++ {
		entry := 6 + i*16
		imageSize := int(uint32(data[entry+8]) | uint32(data[entry+9])<<8 | uint32(data[entry+10])<<16 | uint32(data[entry+11])<<24)
		imageOffset := int(uint32(data[entry+12]) | uint32(data[entry+13])<<8 | uint32(data[entry+14])<<16 | uint32(data[entry+15])<<24)
		if imageSize == 0 || imageOffset < directoryEnd || imageOffset > len(data) || imageSize > len(data)-imageOffset {
			return false
		}
		image := data[imageOffset : imageOffset+imageSize]
		if len(image) >= 8 && bytes.Equal(image[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
			if _, err := png.DecodeConfig(bytes.NewReader(image)); err != nil {
				return false
			}
		} else {
			if len(image) < 4 {
				return false
			}
			headerSize := binary.LittleEndian.Uint32(image[:4])
			if headerSize != 12 && headerSize != 40 && headerSize != 52 && headerSize != 56 && headerSize != 108 && headerSize != 124 {
				return false
			}
			if int(headerSize) > len(image) {
				return false
			}
		}
	}
	return true
}

func normalizeFavicon(data []byte) ([]byte, bool) {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		config, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 256 || config.Height > 256 {
			return nil, false
		}
		if len(data)+22 > maxFaviconBytes {
			return nil, false
		}
		width, height := byte(config.Width), byte(config.Height)
		if config.Width == 256 {
			width = 0
		}
		if config.Height == 256 {
			height = 0
		}
		header := []byte{0, 0, 1, 0, 1, 0, width, height, 0, 0, 1, 0, 32, 0}
		header = append(header, byte(len(data)), byte(len(data)>>8), byte(len(data)>>16), byte(len(data)>>24), 22, 0, 0, 0)
		return append(header, data...), true
	}
	return data, validFavicon(data)
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".favicon-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(perm); err == nil {
		_, err = tmp.Write(data)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func copyFavicon(source, target string, perm os.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if !validFavicon(data) {
		return fmt.Errorf("invalid favicon file: %s", source)
	}
	return atomicWriteFile(target, data, perm)
}

func (s *server) activeFaviconPath() string {
	if s.faviconActivePath != "" {
		return s.faviconActivePath
	}
	return filepath.Join(s.webDist, "favicon.ico")
}

func (s *server) ensureFaviconFiles() error {
	if s.faviconPath == "" || s.faviconDefaultPath == "" {
		return nil
	}
	if _, err := os.Stat(s.faviconPath); errors.Is(err, os.ErrNotExist) {
		if err = copyFavicon(s.faviconDefaultPath, s.faviconPath, 0600); err != nil {
			return fmt.Errorf("initialize favicon: %w", err)
		}
	} else if err != nil {
		return err
	}
	return copyFavicon(s.faviconPath, s.activeFaviconPath(), 0644)
}

func (s *server) saveFavicon(w http.ResponseWriter, r *http.Request) {
	const maxMultipartBytes = maxFaviconBytes + (32 << 10)
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBytes)
	if err := r.ParseMultipartForm(maxMultipartBytes); err != nil {
		fail(w, 400, "bad_request", "favicon upload is invalid")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "bad_request", "favicon file is required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFaviconBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxFaviconBytes {
		fail(w, 400, "bad_request", "favicon must be a valid ICO or PNG file up to 128 KB")
		return
	}
	data, ok := normalizeFavicon(data)
	if !ok {
		fail(w, 400, "bad_request", "favicon must be a valid ICO or PNG file up to 128 KB")
		return
	}
	if err = atomicWriteFile(s.faviconPath, data, 0600); err == nil {
		err = atomicWriteFile(s.activeFaviconPath(), data, 0644)
	}
	if err != nil {
		fail(w, 500, "internal", "favicon could not be saved")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *server) restoreFavicon(w http.ResponseWriter, r *http.Request) {
	err := copyFavicon(s.faviconDefaultPath, s.faviconPath, 0600)
	if err == nil {
		err = copyFavicon(s.faviconDefaultPath, s.activeFaviconPath(), 0644)
	}
	if err != nil {
		fail(w, 500, "internal", "default favicon could not be restored")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
