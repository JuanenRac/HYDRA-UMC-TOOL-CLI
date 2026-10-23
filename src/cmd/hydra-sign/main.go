// HYDRA-UMC-TOOL-CLI - src/cmd/hydra-sign/main.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
//
// hydra-sign signs and verifies the release binaries of this project. For each file F it writes F.sha256 (the SHA-256 digest in the
// `sha256sum` format) and F.sig (the Ed25519 signature of the contents of F, in base64). The private key is read from a PEM (PKCS#8)
// file given by --key and is never printed, logged or copied; verification needs only the public key.
//
//	hydra-sign sign   --key PRIVATE.pem FILE...
//	hydra-sign verify --pub PUBLIC.pem  FILE...
//	hydra-sign pubkey --key PRIVATE.pem            (prints the PUBLIC key as PEM)
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// loadPrivateKey reads an Ed25519 private key from a PKCS#8 PEM file.
func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the private key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("the private key file is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("the private key is not a PKCS#8 key")
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("the private key is not an Ed25519 key")
	}
	return key, nil
}

// loadPublicKey reads an Ed25519 public key from a PKIX PEM file.
func loadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the public key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("the public key file is not PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New("the public key is not a PKIX key")
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("the public key is not an Ed25519 key")
	}
	return key, nil
}

func digestHex(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

// signFile writes path.sha256 and path.sig.
func signFile(key ed25519.PrivateKey, path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	line := digestHex(contents) + "  " + filepath.Base(path) + "\n"
	if err := os.WriteFile(path+".sha256", []byte(line), 0o644); err != nil {
		return err
	}
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(key, contents)) + "\n"
	return os.WriteFile(path+".sig", []byte(signature), 0o644)
}

// verifyFile checks path.sha256 and path.sig against the contents of path.
func verifyFile(pub ed25519.PublicKey, path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recorded, err := os.ReadFile(path + ".sha256")
	if err != nil {
		return fmt.Errorf("missing %s.sha256: %w", filepath.Base(path), err)
	}
	fields := strings.Fields(string(recorded))
	if len(fields) < 1 || fields[0] != digestHex(contents) {
		return fmt.Errorf("%s does not match its .sha256", filepath.Base(path))
	}
	encoded, err := os.ReadFile(path + ".sig")
	if err != nil {
		return fmt.Errorf("missing %s.sig: %w", filepath.Base(path), err)
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("%s.sig is not a base64 Ed25519 signature", filepath.Base(path))
	}
	if !ed25519.Verify(pub, contents, signature) {
		return fmt.Errorf("the signature of %s does not verify", filepath.Base(path))
	}
	return nil
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: hydra-sign sign --key PRIVATE.pem FILE... | verify --pub PUBLIC.pem FILE... | pubkey --key PRIVATE.pem")
		return 2
	}
	command, rest := args[0], args[1:]
	flags := flag.NewFlagSet("hydra-sign "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	keyPath := flags.String("key", "", "private key (PKCS#8 PEM)")
	pubPath := flags.String("pub", "", "public key (PKIX PEM)")
	if err := flags.Parse(rest); err != nil {
		return 2
	}
	files := flags.Args()
	switch command {
	case "sign":
		if *keyPath == "" || len(files) == 0 {
			fmt.Fprintln(stderr, "sign needs --key and at least one file")
			return 2
		}
		key, err := loadPrivateKey(*keyPath)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		for _, file := range files {
			if err := signFile(key, file); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			fmt.Fprintf(stdout, "signed %s\n", filepath.Base(file))
		}
		return 0
	case "verify":
		if *pubPath == "" || len(files) == 0 {
			fmt.Fprintln(stderr, "verify needs --pub and at least one file")
			return 2
		}
		pub, err := loadPublicKey(*pubPath)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		status := 0
		for _, file := range files {
			if err := verifyFile(pub, file); err != nil {
				fmt.Fprintln(stderr, "FAILED:", err)
				status = 1
				continue
			}
			fmt.Fprintf(stdout, "OK %s\n", filepath.Base(file))
		}
		return status
	case "pubkey":
		if *keyPath == "" {
			fmt.Fprintln(stderr, "pubkey needs --key")
			return 2
		}
		key, err := loadPrivateKey(*keyPath)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		der, err := x509.MarshalPKIXPublicKey(key.Public())
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		_ = pem.Encode(stdout, &pem.Block{Type: "PUBLIC KEY", Bytes: der})
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", command)
		return 2
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
