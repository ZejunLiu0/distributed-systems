package store

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

var (
	Debug   = true
	NeedSig = false
)

func PrintAssert(cond bool, s string, args ...interface{}) {
	if !cond {
		PrintExit(s, args...)
	}
}

func PrintDebug(s string, args ...interface{}) {
	if !Debug {
		return
	}
	PrintAlways(s, args...)
}

func PrintExit(s string, args ...interface{}) {
	PrintAlways(s, args...)
	os.Exit(1)
}

func PrintAlways(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}

/*
	Arg: public key file name
	Return: *rsa.PublicKey
*/
func readPublicKey(pubKeyF string) *rsa.PublicKey {
	keyBytes, err := ioutil.ReadFile(pubKeyF)

	block, _ := pem.Decode(keyBytes)
	if block == nil || block.Type != "RSA PUBLIC KEY" {
		errCheck(errors.New("failed to decode PEM block containing public key"))
	}
	pubKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	errCheck(err)
	return pubKey
}

/*
	Arg: private key file name
	Return: *rsa.PrivateKey
*/
func readPrivateKey(privateKeyF string) *rsa.PrivateKey {
	keyBytes, err := ioutil.ReadFile(privateKeyF)

	block, _ := pem.Decode(keyBytes)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		errCheck(errors.New("failed to decode PEM block containing private key"))
	}
	private, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	errCheck(err)
	return private
}

// func setPubKeyF(pubKF string) {
// 	publicKeyFile = pubKF
// }
