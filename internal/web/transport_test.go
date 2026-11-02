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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/observe"
)

type transportOperator struct{ fixtureOperator }

func (*transportOperator) Snapshot(context.Context) (observe.Snapshot, error) {
	return observe.Snapshot{At: time.Now()}, nil
}

func TestHTTP2MultiplexesLiveTabs(t *testing.T) {
	h, _ := newConsole(t, &transportOperator{}, true)
	cookie, _ := signIn(t, h, "admin")
	certificateServer := httptest.NewTLSServer(http.NotFoundHandler())
	pair := certificateServer.TLS.Certificates[0]
	certificateServer.Close()
	key, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Web = config.WebConfig{Host: "127.0.0.1", Port: 0, TLS: true, Cert: certPath, Key: keyPath}
	if err = h.Listen(cfg); err != nil {
		t.Fatal(err)
	}
	var connections atomic.Int32
	transport := &http.Transport{ForceAttemptHTTP2: true, MaxConnsPerHost: 1, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		connections.Add(1)
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < 10; i++ {
		r, _ := http.NewRequestWithContext(ctx, "GET", "https://"+h.http.Addr+"/api/stream", nil)
		r.AddCookie(cookie)
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.ProtoMajor != 2 || response.StatusCode != 200 {
			t.Fatal(response.Proto, response.StatusCode)
		}
	}
	request, _ := http.NewRequestWithContext(ctx, "GET", "https://"+h.http.Addr+"/api/session", nil)
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || connections.Load() != 1 {
		t.Fatal("live tabs blocked requests or used multiple connections", response.StatusCode, connections.Load())
	}
}
