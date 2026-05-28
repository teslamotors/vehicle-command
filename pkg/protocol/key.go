package protocol

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"github.com/teslamotors/vehicle-command/internal/authentication"
)

// Expose some interfaces from the otherwise internal package

// ECDHPrivateKey represents a local private key.
//
// See warnings in [Session] if implementing this interface using an HSM or
// other Trusted Execution Environment (TEE).
type ECDHPrivateKey authentication.ECDHPrivateKey

// Session is a type alias for authentication.Session, exposed here so that
// external packages can implement ECDHPrivateKey. The Exchange method on
// ECDHPrivateKey returns Session, and without this alias external code cannot
// reference the return type of that method, which makes ECDHPrivateKey
// effectively unimplementable from outside this module.
//
// This matters for HSM and enclave integrations: code that holds the private
// key in protected memory (Nitro Enclave, KMS, hardware HSM) needs to provide
// its own ECDHPrivateKey implementation that proxies to the protected boundary.
// The package comment in internal/authentication/ecdh.go specifically calls
// out HSM integration as the reason ECDHPrivateKey is an interface rather
// than a concrete type, so re-exporting Session restores that capability.
//
// If this interface is implemented using an HSM or other trusted execution
// environment (TEE), then protecting long-term key material from a host
// compromise requires:
//
//   - The TEE must NOT expose an API to export the shared secret (i.e., an
//     ECDH interface).
//   - The TEE must NOT expose an AES interface that uses the ECDH-derived
//     key. This would allow a compromised host to compute the derived
//     AES-GCM integrity-protection key.
//   - The TEE must NOT expose an AES-GCM encryption interface that accepts
//     the nonce as a host-provided input. Again, this would allow a
//     compromised host to compute the derived AES-GCM integrity-protection
//     key.
//
// The Session object should not include any of the above derived secrets,
// just handles that can be provided to the TEE.
//
// If the TEE interface cannot meet the above requirements, then there may
// still be value in using one because the host will only be able to
// exfiltrate long-term shared secrets for existing, known vehicle public
// keys.
type Session = authentication.Session

// LoadPrivateKey loads a P256 EC private key from a file.
func LoadPrivateKey(filename string) (ECDHPrivateKey, error) {
	return authentication.LoadExternalECDHKey(filename)
}

func SavePrivateKey(skey ECDHPrivateKey, filename string) error {
	nativeKey, ok := skey.(*authentication.NativeECDHKey)
	if !ok {
		return fmt.Errorf("key is not exportable")
	}
	derKey, err := x509.MarshalECPrivateKey(nativeKey.PrivateKey)
	if err != nil {
		return err
	}
	pemKey := pem.Block{Type: "EC PRIVATE KEY", Bytes: derKey}
	return os.WriteFile(filename, pem.EncodeToMemory(&pemKey), 0600)
}

// LoadPublicKey loads a P256 EC public key from a file.
//
// The function is flexible, supporting the following formats (note that this list includes private
// key files, for convenience):
//   - PKIX PEM ("BEGIN PUBLIC KEY")
//   - Non-password protected PKCS8 PEM ("BEGIN PRIVATE KEY")
//   - SEC1 ("BEGIN EC PRIVATE KEY")
//   - Binary uncompressed SEC1 curve point (0x04, ..., 65 bytes)
//   - Hex-encoded uncompressed SEC1 curve point (04..., 130 bytes)
func LoadPublicKey(filename string) (*ecdh.PublicKey, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	pemBlock, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var pkey *ecdh.PublicKey
	if len(pemBlock) == 65 {
		return ecdh.P256().NewPublicKey(pemBlock)
	}
	// Check for hex-encoded curve point. Allow for trailing "\n".
	if len(pemBlock) == 130 || len(pemBlock) == 131 {
		var decoded [65]byte
		if _, err = hex.Decode(decoded[:], pemBlock[:130]); err == nil {
			return ecdh.P256().NewPublicKey(decoded[:])
		}
		// Continue to decode as PEM. It's not going to work, but it might provide a more
		// descriptive error message.
	}

	block, _ := pem.Decode([]byte(pemBlock))
	if block == nil {
		return nil, ErrInvalidPublicKey
	}

	switch block.Type {
	case "EC PRIVATE KEY":
		skey, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		pkey, err = skey.PublicKey.ECDH()
		if err != nil {
			return nil, err
		}
	case "PRIVATE KEY":
		skey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		if ecdsaPrivateKey, ok := skey.(*ecdsa.PrivateKey); !ok {
			return nil, ErrInvalidPublicKey
		} else {
			pkey, err = ecdsaPrivateKey.PublicKey.ECDH()
			if err != nil {
				return nil, err
			}
		}
	case "PUBLIC KEY":
		publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		if ecdsaPublicKey, ok := publicKey.(*ecdsa.PublicKey); !ok {
			return nil, ErrInvalidPublicKey
		} else {
			pkey, err = ecdsaPublicKey.ECDH()
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unrecognized PEM block type %s", block.Type)
	}
	if pkey.Curve() != ecdh.P256() {
		return nil, ErrInvalidPublicKey
	}
	return pkey, nil
}

// PublicKeyBytesFromHex verifies h encodes a valid public key and returns the binary encoding.
func PublicKeyBytesFromHex(h string) (*ecdh.PublicKey, error) {
	publicKeyBytes, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}
	return ecdh.P256().NewPublicKey(publicKeyBytes)
}

func UnmarshalECDHPrivateKey(keyBytes []byte) ECDHPrivateKey {
	return authentication.UnmarshalECDHPrivateKey(keyBytes)
}
