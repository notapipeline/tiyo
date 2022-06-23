// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package nginx

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

// KEYSIZE : Sets the keysize of the RSA algorithm
const KEYSIZE int = 2048

// CreateSSLCertificates : Create self signed certificates for the NGINX server
//
// If real certificates are required, you can safely replace the self signed ones
// with real certificates sharing the same name.
//
// It is unwise to change the configuration file directly as tiyo will have no
// knowledge of your changes and will replace the configuration if the pipeline is
// rebuilt for any reason.
func CreateSSLCertificates(hostname string) error {
	key, err := rsa.GenerateKey(rand.Reader, KEYSIZE)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"hostname"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 180),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	publicKey := pubkey(key)
	certificate, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey, key)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	out := &bytes.Buffer{}
	pem.Encode(out, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificate,
	})

	if err := write(out.String(), "/etc/ssl/"+hostname+"/certificate.crt"); err != nil {
		log.Error("Failed to create certificate ", err)
	}

	out.Reset()

	pem.Encode(out, pemblock(key))
	if write(out.String(), "/etc/ssl/"+hostname+"/certificate.key"); err != nil {
		log.Error("Failed to create key ", err)
	}

	return nil
}

func pubkey(key interface{}) interface{} {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	}
	return nil
}

func pemblock(key interface{}) *pem.Block {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	}
	return nil
}

func write(what, where string) error {
	if _, err := os.Stat(where); err != nil {
		if _, err := os.Stat(filepath.Dir(where)); os.IsNotExist(err) {
			os.Mkdir(filepath.Dir(where), 0755)
		}

		file, err := os.Create(where)
		if err != nil {
			return fmt.Errorf("failed to create %s. %s", where, err)
		}
		defer file.Close()
		if _, err := file.WriteString(what); err != nil {
			return fmt.Errorf("failed to write ssh key contents for %s. Error was: %s", where, err)
		}
	}
	return nil
}
