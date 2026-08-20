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

package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	Organization  = "UruFlow"
	CACommonName  = "uruflow-ca"
	caLifetime    = 10 * 365 * 24 * time.Hour
	leafLifetime  = 5 * 365 * 24 * time.Hour
	serialBitSize = 128
)

type Authority struct {
	certificate *x509.Certificate
	key         *ecdsa.PrivateKey
	certPath    string
	keyPath     string
}

type Material struct {
	CertPath string
	KeyPath  string
	Names    []string
}

func LoadOrCreateCA(certPath, keyPath string) (*Authority, error) {
	if fileExists(certPath) && fileExists(keyPath) {
		return loadCA(certPath, keyPath)
	}
	return createCA(certPath, keyPath)
}

func (a *Authority) CertificatePEM() (string, error) {
	data, err := os.ReadFile(a.certPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *Authority) EnsureLeaf(material Material) error {
	if fileExists(material.CertPath) && fileExists(material.KeyPath) {
		if valid, err := leafCovers(material); err == nil && valid {
			return nil
		}
	}
	return a.issueLeaf(material)
}

func (a *Authority) issueLeaf(material Material) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serial, err := newSerial()
	if err != nil {
		return err
	}

	hosts, addresses := splitNames(material.Names)
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{Organization}, CommonName: primaryName(material.Names)},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(leafLifetime),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              hosts,
		IPAddresses:           addresses,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, a.certificate, &key.PublicKey, a.key)
	if err != nil {
		return err
	}

	if err := writeCertificate(material.CertPath, der); err != nil {
		return err
	}
	return writeKey(material.KeyPath, key)
}

func createCA(certPath, keyPath string) (*Authority, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serial, err := newSerial()
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{Organization}, CommonName: CACommonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(caLifetime),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	if err := writeCertificate(certPath, der); err != nil {
		return nil, err
	}
	if err := writeKey(keyPath, key); err != nil {
		return nil, err
	}

	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}

	return &Authority{certificate: certificate, key: key, certPath: certPath, keyPath: keyPath}, nil
}

func loadCA(certPath, keyPath string) (*Authority, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	certBlock, _ := pem.Decode(certPEM)
	keyBlock, _ := pem.Decode(keyPEM)
	if certBlock == nil || keyBlock == nil {
		return nil, fmt.Errorf("pki: malformed CA material in %s", filepath.Dir(certPath))
	}

	certificate, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	return &Authority{certificate: certificate, key: key, certPath: certPath, keyPath: keyPath}, nil
}

func leafCovers(material Material) (bool, error) {
	certPEM, err := os.ReadFile(material.CertPath)
	if err != nil {
		return false, err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false, fmt.Errorf("pki: malformed certificate %s", material.CertPath)
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, err
	}
	if time.Now().After(certificate.NotAfter) {
		return false, nil
	}

	for _, name := range material.Names {
		if certificate.VerifyHostname(name) != nil {
			return false, nil
		}
	}
	return true, nil
}

func splitNames(names []string) ([]string, []net.IP) {
	hosts := make([]string, 0, len(names))
	addresses := make([]net.IP, 0, len(names))

	for _, name := range names {
		if name == "" {
			continue
		}
		if address := net.ParseIP(name); address != nil {
			addresses = append(addresses, address)
			continue
		}
		hosts = append(hosts, name)
	}
	return hosts, addresses
}

func primaryName(names []string) string {
	for _, name := range names {
		if name != "" {
			return name
		}
	}
	return Organization
}

func newSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), serialBitSize))
}

func writeCertificate(path string, der []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return os.WriteFile(path, encoded, 0o644)
}

func writeKey(path string, key *ecdsa.PrivateKey) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	return os.WriteFile(path, encoded, 0o600)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
