package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": apiError{Code: code}})
}
func decode(r *http.Request, target any) error {
	return decodeLimit(r, target, 2<<20)
}

func decodeLimit(r *http.Request, target any, limit int64) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > limit {
		return errors.New("request body too large")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("unexpected content after JSON body")
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }
