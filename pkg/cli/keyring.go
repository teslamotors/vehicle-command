package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/pkg/protocol"

	"github.com/99designs/keyring"
	"golang.org/x/term"
)

const (
	keyringServiceName  = "com.tesla.auth"
	keyringKeyService   = "vehicleCommandKey"
	keyringTokenService = "oauthtoken"
	keyringDirectory    = "~/.tesla_keys"
)

type backendType struct {
	config *Config
}

func (b backendType) String() string {
	if b.config == nil || len(b.config.Backend.AllowedBackends) == 0 {
		return string(keyring.InvalidBackend)
	}
	return string(b.config.Backend.AllowedBackends[0])
}

func (b backendType) Set(v string) error {
	value := keyring.BackendType(v)
	if b.config == nil {
		return fmt.Errorf("invalid backendType")
	}
	if v == "" {
		return nil
	}
	for _, name := range keyring.AvailableBackends() {
		if name == value {
			b.config.Backend.AllowedBackends = []keyring.BackendType{name}
			return nil
		}
	}
	return fmt.Errorf("unsupported credential storage")
}

func (c *Config) getPassword(prompt string) (string, error) {
	if c.password != nil && *c.password != "" {
		return *c.password, nil
	}

	var w io.Writer
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		fd = int(os.Stderr.Fd())
		if !term.IsTerminal(fd) {
			return "", fmt.Errorf("no terminal output available for password prompt")
		} else {
			w = os.Stderr
		}
	} else {
		w = os.Stdout
	}

	fmt.Fprintf(w, "%s: ", prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Fprintln(w)
	password := string(b)
	c.password = &password
	return password, nil
}

func (c *Config) openKeyring() (keyring.Keyring, error) {
	return keyring.Open(c.Backend)
}

// LoadTokenFromKeyring loads an OAuth token from the system keyring.
//
// The user must match the value provided to SaveTokenToKeyring.
func (c *Config) LoadTokenFromKeyring() (string, error) {
	kr, err := c.openKeyring()
	if err != nil {
		return "", err
	}

	item, err := kr.Get(keyringTokenService + "." + c.KeyringTokenName)
	if err != nil {
		return "", fmt.Errorf("could not load token: %s", err)
	}
	return string(item.Data), nil
}

// SaveTokenToKeyring writes the account's OAuth token to the system keyring.
//
// The user identifies the OAuth token for future use with account.LoadTokenFromKeyring and does not
// necessarily need to match the system username.
func (c *Config) SaveTokenToKeyring(token string) error {
	kr, err := c.openKeyring()
	if err != nil {
		return err
	}

	if err := kr.Set(keyring.Item{
		Key:  keyringTokenService + "." + c.KeyringTokenName,
		Data: []byte(token),
	}); err != nil {
		return fmt.Errorf("failed to enroll token in keyring: %s", err)
	}
	return nil
}

// LoadKeyFromKeyring reads a private key from the system keyring.
//
// The provided name is an arbitrary string that identifies the key.
func (c *Config) LoadKeyFromKeyring() (protocol.ECDHPrivateKey, error) {
	kr, err := c.openKeyring()
	if err != nil {
		return nil, err
	}
	item, err := kr.Get(keyringKeyService + "." + c.KeyringKeyName)
	if err != nil {
		return nil, fmt.Errorf("could not load key: %s", err)
	}
	keyBytes := item.Data
	key := protocol.UnmarshalECDHPrivateKey(keyBytes)
	if key == nil {
		return nil, fmt.Errorf("invalid private key")
	}
	return key, nil
}

func (c *Config) fullKeyName() string {
	return keyringKeyService + "." + c.KeyringKeyName
}

// SaveKeyToKeyring writes a private key to the system keyring.
func (c *Config) saveKeyToKeyring(key protocol.ECDHPrivateKey) error {
	nativeKey, ok := key.(*authentication.NativeECDHKey)
	if !ok {
		return fmt.Errorf("key is not exportable")
	}

	kr, err := c.openKeyring()
	if err != nil {
		return err
	}

	scalar := make([]byte, 32)
	if (nativeKey.D.BitLen()+7)/8 != len(scalar) {
		return fmt.Errorf("invalid private key")
	}

	if err := kr.Set(keyring.Item{
		Key:  c.fullKeyName(),
		Data: nativeKey.D.FillBytes(scalar),
	}); err != nil {
		return fmt.Errorf("failed to enroll key in keyring: %s", err)
	}
	return nil
}

// DeletePrivateKey removes the private key from the system keyring.
func (c *Config) DeletePrivateKey() error {
	kr, err := c.openKeyring()
	if err != nil {
		return err
	}
	return kr.Remove(c.fullKeyName())
}
