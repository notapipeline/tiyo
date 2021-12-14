package config

import (
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

/**
 * Config methods for time based one time password
 */

func (c *Config) ResetTotp(email string) (*otp.Key, error) {
	return c.GenerateTOTP(email)
}

func (c *Config) GenerateTOTP(email string) (*otp.Key, error) {
	key, err := totp.Generate(
		totp.GenerateOpts{
			Issuer:      c.Assemble.Host,
			AccountName: email,
		},
	)
	if err != nil {
		return nil, err
	}

	return key, nil
}
