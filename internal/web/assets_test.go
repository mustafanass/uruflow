/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 *
 * uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * MIT License for more details.
 *
 * You should have received a copy of the MIT License
 * along with uruflow. If not, see the LICENSE file in the project root.
 */

package web

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticAssetsNegotiateCompressionWithoutChangingTheirContent(t *testing.T) {
	source := []byte(strings.Repeat("export const version = '3.0.0';\n", 100))
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write(source)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{"assets/app.js": &fstest.MapFile{Data: source}, "assets/app.js.gz": &fstest.MapFile{Data: compressed.Bytes()}}
	for _, encoding := range []string{"gzip", "br, gzip;q=0.8", "*;q=1", "gzip;q=0, *;q=1", "br", "", "gzip;q=invalid"} {
		for _, method := range []string{"GET", "HEAD"} {
			request := httptest.NewRequest(method, "http://workspace.test/assets/app.js", nil)
			request.Header.Set("Accept-Encoding", encoding)
			response := httptest.NewRecorder()
			serveAsset(response, request, files, "assets/app.js")
			gzipExpected := encoding == "gzip" || encoding == "br, gzip;q=0.8" || encoding == "*;q=1"
			if (response.Header().Get("Content-Encoding") == "gzip") != gzipExpected {
				t.Fatal(encoding, response.Header())
			}
			if response.Header().Get("Vary") != "Accept-Encoding" || response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
				t.Fatal(response.Header())
			}
			size := len(source)
			if gzipExpected {
				size = compressed.Len()
			}
			if response.Header().Get("Content-Length") != strconv.Itoa(size) {
				t.Fatal("incorrect representation length", response.Header())
			}
			if method == "HEAD" {
				if response.Body.Len() != 0 {
					t.Fatal("HEAD returned a body")
				}
				continue
			}
			data := response.Body.Bytes()
			if gzipExpected {
				reader, err := gzip.NewReader(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				data, err = io.ReadAll(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(data, source) {
				t.Fatal("asset content changed")
			}
		}
	}
}
