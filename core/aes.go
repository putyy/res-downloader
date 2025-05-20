package core

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

type AESCipher struct {
	key []byte
}

func NewAESCipher(key string) *AESCipher {
	return &AESCipher{key: []byte(key)}
}

func (a *AESCipher) Encrypt(plainText string) (string, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", err
	}

	padding := block.BlockSize() - len(plainText)%block.BlockSize()
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	plainText = plainText + string(padText)

	cipherText := make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText[aes.BlockSize:], []byte(plainText))

	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func (a *AESCipher) Decrypt(cipherText string) (string, error) {
	cipherTextBytes, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", err
	}

	if len(cipherTextBytes) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	iv := cipherTextBytes[:aes.BlockSize]
	cipherTextBytes = cipherTextBytes[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(cipherTextBytes, cipherTextBytes)
	
	padding := int(cipherTextBytes[len(cipherTextBytes)-1])
	if padding > len(cipherTextBytes) || padding > aes.BlockSize {
		return "", errors.New("padding size error")
	}
	plainText := cipherTextBytes[:len(cipherTextBytes)-padding]

	return string(plainText), nil
}
