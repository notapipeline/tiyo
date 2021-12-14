// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
)

// Encrypt : Encrypt a message and send back the encrypted value as a base64 encoded string
//
// POST /encrypt
//
// Request parameters
// - value : the value to encrypt
//
// Response codes
// 200 OK message will contain encrypted value
// 400 Bad request if value is empty
// 500 Internal server error if message cannot be encrypted
func (api *API) Encrypt(c *gin.Context) {
	request := make(map[string]string)
	var err error
	if err = c.ShouldBind(&request); err != nil {
		request["value"] = c.PostForm("value")
	}

	if val, ok := request["value"]; !ok || val == "" {
		result := Result{
			Code:    400,
			Result:  "Error",
			Message: fmt.Sprintf("Bad request whilst encrypting passphrase"),
		}
		c.JSON(result.Code, result)
		return
	}

	var passphrase []byte
	if passphrase, err = EncryptData([]byte(request["value"]), api.Config.GetPassphrase("assemble")); err != nil {
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		c.JSON(result.Code, result)
		return
	}

	result := Result{
		Code:    200,
		Result:  "OK",
		Message: base64.StdEncoding.EncodeToString(passphrase),
	}
	c.JSON(result.Code, result)
}

// Decrypt Decrypts a given string and sends back the plaintext value
//
// INTERNAL Execution only
//
// POST /decrypt
//
// Request parameters:
// - value - the value to decrypt
// - token - the validation token to ensure decryption is allowed to take place
//
// Response codes:
// - 200 OK Message will be the decrypted value
// - 400 Bad request if value is empty, token is invalid or token matches value
// - 500 internal server error if value cannot be decoded - Message field may offer further clarification
//
// This function requires both a value and a 'token' to be passed in
// via context.
//
// token is the encrypted version of the passphrase used to encrypt passwords
// and should only be available to the flow server.
//
// This is to offer an additional level of security at the browser level to prevent
// attackers from decrypting a user stored password by accessing the decrypt api
// endpoint to have the server decrypt the password for them.
func (api *API) Decrypt(c *gin.Context) {
	request := make(map[string]string)
	var (
		err            error
		value          string
		token          string
		ok             bool
		decoded        []byte
		assemblePhrase string = api.Config.GetPassphrase("assemble")
	)

	if err = c.ShouldBind(&request); err != nil {
		request["value"] = c.PostForm("value")
		request["token"] = c.PostForm("token")
	}

	if value, ok = request["value"]; !ok || value == "" {
		result := Result{
			Code:    400,
			Result:  "Error",
			Message: "Bad request whilst decrypting data - missing value",
		}
		c.JSON(result.Code, result)
		return
	}

	if token, ok = request["token"]; !ok || token == "" || token == value {
		result := Result{
			Code:    400,
			Result:  "Error",
			Message: "Missing token for decryption or token matches password",
		}
		c.JSON(result.Code, result)
		return
	}

	decoded, _ = base64.StdEncoding.DecodeString(token)
	if decoded, err = DecryptData(decoded, assemblePhrase); err != nil || string(decoded) != assemblePhrase {
		result := Result{
			Code:    400,
			Result:  "Error",
			Message: "Failed to decode token or token is invalid",
		}
		c.JSON(result.Code, result)
		return
	}
	var passphrase []byte
	passphrase, _ = base64.StdEncoding.DecodeString(value)
	if passphrase, err = DecryptData(passphrase, assemblePhrase); err != nil {
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		c.JSON(result.Code, result)
		return
	}

	result := Result{
		Code:    200,
		Result:  "OK",
		Message: string(passphrase),
	}
	c.JSON(result.Code, result)
}

func createHash(key string) []byte {
	data := sha256.Sum256([]byte(key))
	return data[:]
}

func EncryptData(data []byte, passphrase string) ([]byte, error) {
	block, err := aes.NewCipher(createHash(passphrase))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func DecryptData(data []byte, passphrase string) ([]byte, error) {
	key := []byte(createHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
