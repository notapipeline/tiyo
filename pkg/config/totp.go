package config

import (
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

/**
 * Config methods for time based one time password
 */

func (c *Config) ResetTotp() (*otp.Key, error) {
	c.Admin.TotpKey = ""

	/*if err := c.save(); err != nil {
		return nil, err
	}*/

	return c.GenerateTOTP()
}

func (c *Config) GenerateTOTP() (*otp.Key, error) {
	key, err := totp.Generate(
		totp.GenerateOpts{
			Issuer:      c.Assemble.Host,
			AccountName: c.Admin.Email,
		},
	)
	if err != nil {
		return nil, err
	}

	return key, nil
}
