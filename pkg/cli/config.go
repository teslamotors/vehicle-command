/*
Package cli facilitates building command-line applications for sending commands to vehicles. It
defines a [Config] type that can be used to register common command-line flags (using the Golang
flag package) and environment variable equivalents.

The package uses [keyring]'s platform-agnostic interface for storing sensitive values (private keys
and OAuth tokens) in an OS-dependent credential store.

# Examples

	import flag

	config, err := NewConfig(FlagAll)
	if err != nil {
		panic(err)
	}
	config.RegisterCommandLineFlags() // Adds command-line flags for private keys, OAuth, etc.
	flag.Parse()
	config.ReadFromEnvironment()      // Fills in missing fields using environment variables
	config.LoadCredentials()          // Prompt for Keyring password if needed

	// Initializes car and acct if relevant fields are populated (from a combination of command-line
	// flags and environment variables). The car and acct may be nil even if error is nil. The car
	// connection might be over BLE or over the Internet.
	car, acct, err := config.Connect()
	if err != nil {
		panic(err)
	}
	defer config.UpdateCachedSessions()

You can also specify the connection type (which will then fail if the necessary fields are unpopulated):

	car, err = config.ConnectRemote() // Connect to a car over the Internet.
	car, err = config.ConnectLocal() // Connect to a car over BLE.

Alternatively, you can use a [Flag] mask to control what [Config] fields are populated. Note that in
the examples below, config.Flags must be set before calling [flag.Parse] or
[Config.ReadFromEnvironment]:

	config, err = NewConfig(FlagOAuth | FlagPrivateKey | FlagVIN) // config.Connect() will use the Internet, not BLE.
	config, err = NewConfig(FlagBLE | FlagPrivateKey | FlagVIN) // config.Connect() will use BLE, not the Internet.
	config, err = NewConfig(FlagBLE | FlagVIN) // config.Connect() will create an unauthenticated vehicle connection.

The last option will not attempt to load private keys when calling [Config.Connect], and therefore
will not result in an error if a private key is defined in the environment but cannot be loaded.
However, most [vehicle.Vehicle] commands do not work over unauthenticated connections.
*/
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/cache"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"

	"github.com/99designs/keyring"
)

var DomainsByName = map[string]protocol.Domain{
	"VCSEC":        protocol.DomainVCSEC,
	"INFOTAINMENT": protocol.DomainInfotainment,
}

var DomainNames = map[protocol.Domain]string{
	protocol.DomainVCSEC:        "VCSEC",
	protocol.DomainInfotainment: "INFOTAINMENT",
}

// DomainList is used to translate domains provided at the command line into native protocol.Domain
// values.
type DomainList []protocol.Domain

// Set updates a DomainList from a command-line argument.
func (d *DomainList) Set(value string) error {
	canonicalName := strings.ToUpper(value)
	if domain, ok := DomainsByName[canonicalName]; ok {
		*d = append(*d, domain)
	} else {
		return fmt.Errorf("unknown domain '%s'", value)
	}
	return nil
}

func (d *DomainList) String() string {
	var names []string
	for _, domain := range *d {
		if name, ok := DomainNames[domain]; ok {
			names = append(names, name)
		}
	}
	return strings.Join(names, ",")
}

// Environment variable names used are used by [Config.ReadFromEnvironment] to set common parameters.
const (
	EnvTeslaKeyName      = "TESLA_KEY_NAME"
	EnvTeslaKeyFile      = "TESLA_KEY_FILE"
	EnvTeslaTokenName    = "TESLA_TOKEN_NAME"
	EnvTeslaTokenFile    = "TESLA_TOKEN_FILE"
	EnvTeslaVIN          = "TESLA_VIN"
	EnvTeslaCacheFile    = "TESLA_CACHE_FILE"
	EnvTeslaKeyringType  = "TESLA_KEYRING_TYPE"
	EnvTeslaKeyringPass  = "TESLA_KEYRING_PASSWORD"
	EnvTeslaKeyringPath  = "TESLA_KEYRING_PATH"
	EnvTeslaKeyringDebug = "TESLA_KEYRING_DEBUG"
)

// Flag controls what options should be scanned from the command line and/or environment variables.
type Flag int

func (f Flag) isSet(other Flag) bool {
	return (f & other) == other
}

const (
	FlagVIN        Flag = 1 // Enable VIN option.
	FlagOAuth      Flag = 2 // Enable OAuth options.
	FlagPrivateKey Flag = 4 // Enable Private Key options. Required for sending vehicle commands.
	FlagBLE        Flag = 8 // Enable BLE options. Requires FlagVIN.
	FlagAll        Flag = FlagVIN | FlagOAuth | FlagPrivateKey | FlagBLE
)

var (
	ErrNoKeySpecified        = errors.New("private key location not provided")
	ErrNoAvailableTransports = errors.New("no available transports (configuration must permit BLE and/or OAuth)")
	ErrKeyNotFound           = keyring.ErrKeyNotFound
)

// Config fields determine how a client authenticates to vehicles and/or Tesla's backend.
type Config struct {
	Flags            Flag   // Controls which set of environment variables/CLI flags to use.
	KeyringKeyName   string // Username for private key in system keyring
	KeyringTokenName string // Username for OAuth token in system keyring
	VIN              string
	TokenFilename    string
	KeyFilename      string
	CacheFilename    string
	Backend          keyring.Config
	BackendType      backendType
	Debug            bool // Enable keyring debug messages

	// Domains can limit a vehicle connection to relevant subsystems, which can reduce
	// connection latency and avoid waking up the infotainment system unnecessarily.
	Domains DomainList

	password   *string
	sessions   *cache.SessionCache
	acct       *account.Account
	skey       protocol.ECDHPrivateKey
	oauthToken string
}

func NewConfig(flags Flag) (*Config, error) {
	c := Config{
		Flags: flags,
		Backend: keyring.Config{
			ServiceName:              keyringServiceName,
			KeychainTrustApplication: true,
			KeyCtlScope:              "user",
		},
	}
	c.BackendType = backendType{&c}
	c.Backend.KeychainPasswordFunc = c.getPassword
	c.Backend.FilePasswordFunc = c.getPassword

	return &c, nil
}

func (c *Config) RegisterCommandLineFlags() {
	if c.Flags.isSet(FlagVIN) {
		flag.StringVar(&c.VIN, "vin", "", "Vehicle Identification Number. Defaults to $TESLA_VIN.")
	}
	if c.Flags.isSet(FlagPrivateKey) {
		if !c.Flags.isSet(FlagVIN) {
			log.Debug("FlagPrivateKey is set but FlagVIN is not. A VIN is required to send vehicle commands.")
		}
		flag.StringVar(&c.CacheFilename, "session-cache", "", "Load session info cache from `file`. Defaults to $TESLA_CACHE_FILE.")
		flag.StringVar(&c.KeyringKeyName, "key-name", "", "System keyring `name` for private key. Defaults to $TESLA_KEY_NAME.")
		flag.StringVar(&c.KeyFilename, "key-file", "", "A `file` containing private key. Defaults to $TESLA_KEY_FILE.")
		flag.Var(&c.Domains, "domain", "Domains to connect to (can be repeated; omit for all)")
	}
	if c.Flags.isSet(FlagOAuth) {
		flag.StringVar(&c.KeyringTokenName, "token-name", "", "System keyring `name` for OAuth token. Defaults to $TESLA_TOKEN_NAME.")
		flag.StringVar(&c.TokenFilename, "token-file", "", "`File` containing OAuth token. Defaults to $TESLA_TOKEN_FILE.")
	}
	if c.Flags.isSet(FlagOAuth) || c.Flags.isSet(FlagPrivateKey) {
		var names []string
		for _, name := range keyring.AvailableBackends() {
			names = append(names, string(name))
		}
		sort.Strings(names)
		flag.Var(&c.BackendType, "keyring-type", "Keyring `type` ("+strings.Join(names, "|")+"). Defaults to $TESLA_KEYRING_TYPE.")
		flag.StringVar(&c.Backend.FileDir, "keyring-file-dir", keyringDirectory, "keyring `directory` for file-backed keyring types")
		flag.BoolVar(&c.Debug, "keyring-debug", false, "Enable keyring debug logging")
	}
}

// LoadCredentials attempts to open a keyring, prompting for a password if not needed. Call this
// method before [config.Connect] to prevent interactive prompts from counting against timeouts.
func (c *Config) LoadCredentials() error {
	if c.Flags.isSet(FlagOAuth) {
		if _, err := c.token(); err != nil {
			return err
		}
	}
	if c.Flags.isSet(FlagPrivateKey) {
		if _, err := c.PrivateKey(); err != nil {
			return err
		}
	}
	return nil
}

// ReadFromEnvironment populates c using environment variables. Values that are already populated
// are not overwritten.
//
// Calling ReadFromEnvironment after flag.Parse() (or other initialization method) will prevent the
// environment from overriding explicit command-line parameters and avoid potentially misleading
// debug log messages.
func (c *Config) ReadFromEnvironment() {
	if c.Flags.isSet(FlagVIN) {
		if c.VIN == "" {
			c.VIN = os.Getenv(EnvTeslaVIN)
			log.Debug("Set VIN to '%s'", c.VIN)
		}
	}
	if c.Flags.isSet(FlagPrivateKey) {
		if c.CacheFilename == "" {
			c.CacheFilename = os.Getenv(EnvTeslaCacheFile)
			log.Debug("Set session cache file to '%s'", c.CacheFilename)
		}
		if c.KeyringKeyName == "" && c.KeyFilename == "" {
			c.KeyringKeyName = os.Getenv(EnvTeslaKeyName)
			log.Debug("Set key name to '%s'", c.KeyringKeyName)

			c.KeyFilename = os.Getenv(EnvTeslaKeyFile)
			log.Debug("Set key file to '%s'", c.KeyFilename)
		}
	}
	if c.Flags.isSet(FlagOAuth) {
		if c.KeyringTokenName == "" && c.TokenFilename == "" {
			c.KeyringTokenName = os.Getenv(EnvTeslaTokenName)
			log.Debug("Set OAuth token name to '%s'", c.KeyringTokenName)

			c.TokenFilename = os.Getenv(EnvTeslaTokenFile)
			log.Debug("Set OAuth token file to '%s'", c.TokenFilename)
		}
	}
	if c.Flags.isSet(FlagOAuth) || c.Flags.isSet(FlagPrivateKey) {
		if c.BackendType.String() == string(keyring.InvalidBackend) {
			if err := c.BackendType.Set(os.Getenv(EnvTeslaKeyringType)); err == nil {
				log.Debug("Set keyring type to '%s'", c.BackendType)
			}
		}
		if c.password == nil {
			password := os.Getenv(EnvTeslaKeyringPass)
			c.password = &password
			if len(password) > 0 {
				log.Debug("Set keyring File Password to %s", strings.Repeat("*", len("hunter2")))
			}
		}
		if c.Backend.FileDir == "" {
			c.Backend.FileDir = os.Getenv(EnvTeslaKeyringPath)
			log.Debug("Set keyring File Path to '%s'", c.Backend.FileDir)
		}
		if !c.Debug {
			_, c.Debug = os.LookupEnv(EnvTeslaKeyringDebug)
			log.Debug("Set keyring Debug Logging to '%v'", c.Debug)
		}
	}
}

// UpdateCachedSessions updates c.CacheFilename with updated session state.
//
// If c.CacheFilename is not set or no vehicle handshake has occurred, then this method does
// nothing.
func (c *Config) UpdateCachedSessions(v *vehicle.Vehicle) {
	if c.CacheFilename != "" && c.sessions != nil {
		v.UpdateCachedSessions(c.sessions)
		if err := c.sessions.ExportToFile(c.CacheFilename); err != nil {
			log.Error("Error updating cache: %s", err)
		}
	}
}

// PrivateKey loads a private key from the location specified in c.
//
// If c does not specify a private key location, both skey and err will be nil. The private key is
// cached after it is first loaded, and subsequent calls will always return the same private key.
func (c *Config) PrivateKey() (skey protocol.ECDHPrivateKey, err error) {
	if c.skey != nil {
		return c.skey, nil
	}
	if !c.Flags.isSet(FlagPrivateKey) {
		log.Debug("Skipping private key loading because FlagPrivateKey is not set")
		return nil, ErrNoKeySpecified
	}
	if c.KeyFilename == "" && c.KeyringKeyName == "" {
		return nil, ErrNoKeySpecified
	}
	if c.KeyFilename != "" {
		skey, err = protocol.LoadPrivateKey(c.KeyFilename)
	}
	if skey == nil && c.KeyringKeyName != "" {
		skey, err = c.LoadKeyFromKeyring()
	}
	if err := c.loadCache(); err != nil {
		return nil, err
	}
	c.skey = skey
	return skey, err
}

// Connect to vehicle and/or account.
//
// If c.TokenFilename is set, the returned account will not be nil and the vehicle will use a
// connector.inet connection if a VIN was provided. If no token filename is set, c.VIN is required,
// the account will be nil, and the vehicle will use a connector.ble connection.
func (c *Config) Connect(ctx context.Context) (acct *account.Account, car *vehicle.Vehicle, err error) {
	if c.VIN == "" && c.KeyringTokenName == "" && c.TokenFilename == "" {
		return nil, nil, fmt.Errorf("must provide VIN and/or OAuth token")
	}

	// A private key is required to authorize commands. Load a private key from a file if the caller
	// provided ones.
	var skey protocol.ECDHPrivateKey
	skey, err = c.PrivateKey()
	if err != nil && err != ErrNoKeySpecified {
		return nil, nil, err
	}

	if skey == nil {
		log.Debug("No private key available")
	} else {
		log.Debug("Client public key: %02x", skey.PublicBytes())
	}

	if c.Flags.isSet(FlagOAuth) && (c.KeyringTokenName != "" || c.TokenFilename != "") {
		log.Debug("Required OAuth parameters supplied by CLI and/or environment. Connecting over the Internet...")
		acct, car, err = c.ConnectRemote(ctx, skey)
	} else if c.Flags.isSet(FlagBLE) && c.Flags.isSet(FlagVIN) {
		log.Debug("Connecting over BLE...")
		car, err = c.ConnectLocal(ctx, skey)
	} else {
		err = ErrNoAvailableTransports
	}

	if err != nil {
		return nil, nil, err
	}

	if car == nil {
		// We don't need to connect to car, return early (acct may be non-nil).
		return
	}

	log.Info("Connecting to car...")
	if err := car.Connect(ctx); err != nil {
		return nil, nil, err
	}
	if skey != nil {
		log.Info("Securing connection...")
		if err := car.StartSession(ctx, c.Domains); err != nil {
			return nil, nil, err
		}
	}
	return
}

func (c *Config) loadCache() error {
	if c.CacheFilename == "" {
		return nil
	}
	log.Debug("Loading cache from %s...", c.CacheFilename)
	var err error
	c.sessions, err = cache.ImportFromFile(c.CacheFilename)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to load session cache: %s", err)
		}
		// Create a new cache if one couldn't be loaded from the file
		c.sessions = cache.New(0)
	}
	return nil
}

func (c *Config) token() (string, error) {
	if c.oauthToken != "" {
		return c.oauthToken, nil
	}
	var err error
	if c.TokenFilename != "" {
		token, err := os.ReadFile(c.TokenFilename)
		if err == nil {
			c.oauthToken = string(token)
			return c.oauthToken, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		// If the token file doesn't exist, fall through to trying to load from the system keyring.
	}
	c.oauthToken, err = c.LoadTokenFromKeyring()
	return c.oauthToken, err
}

// Account logs into and returns the configured Tesla account.
func (c *Config) Account() (*account.Account, error) {
	token, err := c.token()
	if err != nil {
		return nil, err
	}
	return account.New(token, "")
}

// SavePrivateKey writes skey to the system keyring or file, depending on what options are
// configured. The method prefers the keyring if both options are available.
func (c *Config) SavePrivateKey(skey protocol.ECDHPrivateKey) error {
	if c.KeyringKeyName != "" {
		return c.saveKeyToKeyring(skey)
	}
	if c.KeyFilename != "" {
		return protocol.SavePrivateKey(skey, c.KeyFilename)
	}
	return ErrNoKeySpecified
}

// ConnectRemote logs in to the configured Tesla account, and, if c includes a VIN, also fetches the
// corresponding vehicle.
func (c *Config) ConnectRemote(ctx context.Context, skey protocol.ECDHPrivateKey) (acct *account.Account, car *vehicle.Vehicle, err error) {
	if c.acct == nil {
		c.acct, err = c.Account()
		if err != nil {
			return
		}
	}

	acct = c.acct

	if c.Flags.isSet(FlagVIN) && c.VIN != "" {
		car, err = acct.GetVehicle(ctx, c.VIN, skey, c.sessions)

		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize vehicle connection: %s", err)
		}
	}
	return
}

// ConnectLocal connects to a vehicle over BLE.
func (c *Config) ConnectLocal(ctx context.Context, skey protocol.ECDHPrivateKey) (car *vehicle.Vehicle, err error) {
	conn, err := ble.NewConnection(ctx, c.VIN)
	if err != nil {
		return nil, err
	}

	car, err = vehicle.NewVehicle(conn, skey, c.sessions)
	if err != nil {
		return nil, err
	}
	return
}
