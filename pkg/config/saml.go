// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
package config

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"fmt"
	"net/url"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	log "github.com/sirupsen/logrus"
)

type SAML struct {
	IDPMetadata string             `json:"idp_metadata"`
	PrivateKey  []byte             `json:"private_key"`
	Certificate []byte             `json:"certificate"`
	SamlSP      *samlsp.Middleware `json:"-"`
}

func (config *Config) ConfigureSAML() error {
	log.Infof("Setting up SAML configuration")

	if len(config.SAML.IDPMetadata) == 0 {
		return fmt.Errorf("no idp metadata")
	}
	entity := &saml.EntityDescriptor{}
	err := xml.Unmarshal([]byte(config.SAML.IDPMetadata), entity)

	if err != nil && err.Error() == "Expected element type <EntityDescriptor> but have <EntitiesDescriptor>" {
		entities := &saml.EntitiesDescriptor{}
		if err := xml.Unmarshal([]byte(config.SAML.IDPMetadata), entities); err != nil {
			return err
		}

		err = fmt.Errorf("no entity found with IDPSSODescriptor")
		for i, e := range entities.EntityDescriptors {
			if len(e.IDPSSODescriptors) > 0 {
				entity = &entities.EntityDescriptors[i]
				err = nil
			}
		}
	}
	if err != nil {
		return err
	}

	keyPair, err := tls.X509KeyPair(config.SAML.Certificate, config.SAML.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to load SAML keypair: %s", err)
	}

	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse SAML certificate: %s", err)
	}

	rootURL := url.URL{
		Scheme: "https",
		Host:   config.Flow.Hostname,
		Path:   "/",
	}

	newsp, err := samlsp.New(samlsp.Options{
		URL:               rootURL,
		Key:               keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate:       keyPair.Leaf,
		IDPMetadata:       entity,
		AllowIDPInitiated: true,
	})

	if err != nil {
		log.Warnf("failed to configure SAML: %s", err)
		config.SAML.SamlSP = nil
		return fmt.Errorf("failed to configure SAML: %s", err)
	}

	newsp.ServiceProvider.AuthnNameIDFormat = saml.EmailAddressNameIDFormat

	config.SAML.SamlSP = newsp
	log.Infof("successfully configured SAML")
	return nil
}
